import pytest
from unittest.mock import patch, MagicMock

from grpc_server.servicer import ArchitectServicer
from proto import manifest_pb2

@patch("grpc_server.servicer.build_graph")
def test_generate_architecture_success(mock_build_graph):
    mock_graph_instance = MagicMock()
    mock_graph_instance.invoke.return_value = {
        "user_prompt": "Build a database",
        "proposed_manifest": {
            "version": "1.0",
            "project_name": "TestDB",
            "nodes": [
                {"id": "db1", "type": "postgres_db", "version": "15", "config": {"port": "5432"}}
            ]
        },
        "errors": [],
        "is_valid": True
    }
    mock_build_graph.return_value = mock_graph_instance

    servicer = ArchitectServicer()
    # Fixed field name: user_prompt
    request = manifest_pb2.RefineManifestRequest(user_prompt="Build a database") 
    mock_context = MagicMock()

    response = servicer.RefineManifest(request, mock_context) 

    assert isinstance(response, manifest_pb2.RefineManifestResponse) 
    assert response.is_valid is True
    # Fixed assertion paths based on ProjectManifest nesting
    assert response.updated_manifest.metadata.project_name == "TestDB"
    assert len(response.updated_manifest.nodes) == 1
    assert response.updated_manifest.nodes[0].id == "db1"
    assert response.updated_manifest.nodes[0].type == "postgres_db"
    assert response.updated_manifest.nodes[0].config["port"] == "5432"