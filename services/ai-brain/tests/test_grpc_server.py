"""
tests/test_grpc_server.py

Tests for the active gRPC servicer (ArchitectBrainServicer) without
depending on a running LLM or Redis.
"""
import pytest
from unittest.mock import MagicMock, patch

import grpc
from google.protobuf import json_format

from grpc_server.server import ArchitectBrainServicer
from grpc_server.agent import AIResponse, ProjectManifest, Metadata, ArchitectureNode
from proto import manifest_pb2


# ------------------------------------------------------------------
# Helper: build a valid protobuf ProjectManifest
# ------------------------------------------------------------------
def make_proto_manifest(name: str = "TestDB") -> manifest_pb2.ProjectManifest:
    meta = manifest_pb2.Metadata(project_name=name, target_cloud="aws", schema_version="1.0")
    return manifest_pb2.ProjectManifest(metadata=meta)


# ------------------------------------------------------------------
# Helper: mock AIResponse (Pydantic) simulating a successful agent reply
# ------------------------------------------------------------------
def mock_ai_response() -> AIResponse:
    return AIResponse(
        is_valid=True,
        ai_reasoning="Added postgres_db node.",
        updated_manifest=ProjectManifest(
            metadata=Metadata(project_name="TestDB", target_cloud="aws", schema_version="1.0"),
            nodes=[
                ArchitectureNode(
                    id="db1",
                    type="postgres_db",
                    version="15",
                    config={"port": "5432"},
                )
            ],
            connections=[],
            feature_flags=[],
        ),
    )


# ------------------------------------------------------------------
# Tests
# ------------------------------------------------------------------
class TestArchitectBrainServicer:
    """Tests for the servicer defined in grpc_server/server.py"""

    @patch("grpc_server.server.ai_agent")
    def test_refine_manifest_success(self, mock_agent):
        """A normal request should map proto → state → proto correctly."""
        mock_agent.invoke.return_value = {"final_response": mock_ai_response()}

        servicer = ArchitectBrainServicer()

        request = manifest_pb2.RefineManifestRequest(
            trace_id="trace-001",
            user_id="user-abc",
            project_id="proj-xyz",                # now available after regeneration
            user_prompt="Add a database",
            current_manifest=make_proto_manifest("TestDB"),
        )
        context = MagicMock()

        response = servicer.RefineManifest(request, context)

        # Basic proto fields
        assert response.is_valid is True
        assert response.ai_reasoning == "Added postgres_db node."
        assert response.updated_manifest.metadata.project_name == "TestDB"
        assert len(response.updated_manifest.nodes) == 1
        assert response.updated_manifest.nodes[0].id == "db1"
        assert response.updated_manifest.nodes[0].type == "postgres_db"
        assert response.updated_manifest.nodes[0].config["port"] == "5432"

        # Verify the agent was called with the correct state keys
        mock_agent.invoke.assert_called_once()
        call_args = mock_agent.invoke.call_args[0][0]
        assert call_args["project_id"] == "proj-xyz"
        assert call_args["prompt"] == "Add a database"
        assert call_args["current_manifest"]["metadata"]["project_name"] == "TestDB"

    @patch("grpc_server.server.ai_agent")
    def test_refine_manifest_invalid_prompt_empty(self, mock_agent):
        """An empty prompt should abort with INVALID_ARGUMENT and not call the agent."""
        servicer = ArchitectBrainServicer()
        request = manifest_pb2.RefineManifestRequest(
            user_prompt="",
            current_manifest=make_proto_manifest(),
        )
        context = MagicMock()

        servicer.RefineManifest(request, context)

        # The agent must NOT be called
        mock_agent.invoke.assert_not_called()

        # The abort must have been called exactly once
        context.abort.assert_called_once()
        args, _ = context.abort.call_args
        # args[0] is grpc.StatusCode.INVALID_ARGUMENT, args[1] is the detail string
        assert args[0] == grpc.StatusCode.INVALID_ARGUMENT
        assert "user_prompt cannot be empty" in args[1]

    @patch("grpc_server.server.ai_agent")
    def test_refine_manifest_agent_crash(self, mock_agent):
        """When the agent raises an exception, the servicer catches it and returns INTERNAL."""
        mock_agent.invoke.side_effect = RuntimeError("Gemini quota exceeded")
        servicer = ArchitectBrainServicer()
        request = manifest_pb2.RefineManifestRequest(
            user_prompt="Valid prompt",
            current_manifest=make_proto_manifest(),
        )
        context = MagicMock()

        servicer.RefineManifest(request, context)

        context.abort.assert_called_once()
        args, _ = context.abort.call_args
        assert args[0] == grpc.StatusCode.INTERNAL
        assert "AI Brain Internal Error" in args[1]