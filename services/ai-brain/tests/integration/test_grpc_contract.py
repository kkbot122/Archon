"""
tests/integration/test_grpc_contract.py

Prerequisites:
- AI Brain gRPC server running on localhost:50051 with GEMINI_MODEL=mock
- Redis accessible
"""

import grpc
import pytest

from proto import manifest_pb2
from proto import manifest_pb2_grpc


@pytest.fixture(scope="module")
def grpc_channel():
    channel = grpc.insecure_channel("localhost:50051")
    yield channel
    channel.close()

@pytest.fixture(scope="module")
def stub(grpc_channel):
    return manifest_pb2_grpc.ArchitectBrainStub(grpc_channel)

@pytest.fixture
def valid_request():
    meta = manifest_pb2.Metadata(project_name="TestProject", target_cloud="aws", schema_version="1.0")
    manifest = manifest_pb2.ProjectManifest(metadata=meta)
    return manifest_pb2.RefineManifestRequest(
        trace_id="trace-001",
        user_id="user-001",
        project_id="proj-001",
        user_prompt="Add a postgres database",
        current_manifest=manifest,
    )

# ---- Tests ----

def test_refine_manifest_happy_path(stub, valid_request):
    response = stub.RefineManifest(valid_request)
    assert response.is_valid is True
    assert len(response.ai_reasoning) > 0
    assert len(response.updated_manifest.nodes) >= 1
    node_types = [node.type for node in response.updated_manifest.nodes]
    assert "postgres_db" in node_types

def test_refine_manifest_rejects_empty_prompt(stub):
    request = manifest_pb2.RefineManifestRequest(
        trace_id="t1",
        user_id="u1",
        project_id="p1",
        user_prompt="",
        current_manifest=manifest_pb2.ProjectManifest(
            metadata=manifest_pb2.Metadata(project_name="Test")
        ),
    )
    with pytest.raises(grpc.RpcError) as exc_info:
        stub.RefineManifest(request)
    assert exc_info.value.code() == grpc.StatusCode.INVALID_ARGUMENT
    assert "cannot be empty" in exc_info.value.details().lower()

def test_refine_manifest_rejects_long_prompt(stub):
    long_prompt = "x" * 2001
    request = manifest_pb2.RefineManifestRequest(
        trace_id="t2",
        user_id="u2",
        project_id="p2",
        user_prompt=long_prompt,
        current_manifest=manifest_pb2.ProjectManifest(
            metadata=manifest_pb2.Metadata(project_name="Test")
        ),
    )
    with pytest.raises(grpc.RpcError) as exc_info:
        stub.RefineManifest(request)
    assert exc_info.value.code() == grpc.StatusCode.INVALID_ARGUMENT
    assert "2000" in exc_info.value.details()

def test_refine_manifest_trace_id_propagation(stub, valid_request):
    response = stub.RefineManifest(valid_request)
    assert response.trace_id == "trace-001"

def test_refine_manifest_project_id_history(stub):
    req = manifest_pb2.RefineManifestRequest(
        trace_id="h1",
        user_id="u",
        project_id="history-test",
        user_prompt="Add a postgres database",
        current_manifest=manifest_pb2.ProjectManifest(
            metadata=manifest_pb2.Metadata(project_name="HistProject")
        ),
    )
    resp1 = stub.RefineManifest(req)
    assert resp1.is_valid

    req2 = manifest_pb2.RefineManifestRequest(
        trace_id="h2",
        user_id="u",
        project_id="history-test",
        user_prompt="Now add redis",
        current_manifest=resp1.updated_manifest,
    )
    resp2 = stub.RefineManifest(req2)
    assert resp2.is_valid