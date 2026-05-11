from brain.tools.manifest_ops import validate_node_config

def test_validate_node_config_success():
    """It should return Valid for correct configurations."""
    result = validate_node_config.invoke({
        "node_type": "postgres_db",
        "node_config": {"storage_size_gb": "50", "max_connections": "100"}
    })
    assert "Valid" in result

def test_validate_node_config_missing_keys():
    """It should flag missing required configuration keys."""
    result = validate_node_config.invoke({
        "node_type": "postgres_db",
        "node_config": {"storage_size_gb": "50"}
    })
    assert "Missing required keys" in result