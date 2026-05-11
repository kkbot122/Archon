from langchain_core.tools import tool

@tool
def validate_node_config(node_type: str, node_config: dict) -> str:
    """
    Validates if a given configuration dictionary contains all required 
    keys for a specific node type.
    """
    required_keys_map = {
        "postgres_db": ["storage_size_gb", "max_connections"],
        "redis_cache": ["memory_mb"],
        "go_backend": ["port"],
        "react_frontend": ["port"]
    }

    required_keys = required_keys_map.get(node_type, [])
    # Check against node_config instead of config
    missing_keys = [key for key in required_keys if key not in node_config]

    if missing_keys:
        return f"Invalid: Missing required keys for {node_type}: {', '.join(missing_keys)}"

    return f"Valid configuration for {node_type}."