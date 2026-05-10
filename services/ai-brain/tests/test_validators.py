import pytest
from pydantic import ValidationError
from validators.manifest_schema import NodeSchema, ManifestSchema

def test_valid_manifest_passes():
    """A perfectly formatted manifest should parse without errors."""
    valid_data = {
        "version": "1.0",
        "project_name": "TestProject",
        "nodes": [
            {
                "id": "db_1",
                "type": "postgres_db",
                "version": "15",
                "config": {"db_user": "admin"}
            }
        ]
    }
    manifest = ManifestSchema(**valid_data)
    assert manifest.project_name == "TestProject"
    assert len(manifest.nodes) == 1
    assert manifest.nodes[0].id == "db_1"

def test_missing_required_fields_fails():
    """Manifests missing a project_name or nodes list should raise a ValidationError."""
    invalid_data = {
        "version": "1.0",
        # missing project_name and nodes
    }
    with pytest.raises(ValidationError) as exc_info:
        ManifestSchema(**invalid_data)
    
    assert "project_name" in str(exc_info.value)
    assert "nodes" in str(exc_info.value)

def test_invalid_node_type_fails():
    """Nodes must match the supported types in the Atomic Library."""
    invalid_data = {
        "version": "1.0",
        "project_name": "BadNodeProject",
        "nodes": [
            {
                "id": "db_1",
                "type": "quantum_database", # Unsupported!
                "version": "1.0",
                "config": {}
            }
        ]
    }
    with pytest.raises(ValidationError) as exc_info:
        ManifestSchema(**invalid_data)
    
    assert "Input should be" in str(exc_info.value) # Pydantic Enum/Literal error