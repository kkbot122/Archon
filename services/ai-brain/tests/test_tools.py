import json
from unittest.mock import patch, mock_open
from brain.tools.atomic_lookup import get_allowed_nodes

def test_atomic_lookup_tool_reads_library():
    """The tool should read the index.json and return available nodes."""
    mock_data = {
        "available_nodes": [
            {"type": "quantum_db", "description": "A fake DB"}
        ]
    }
    
    mock_json_str = json.dumps(mock_data)
    
    with patch("builtins.open", mock_open(read_data=mock_json_str)):
        # Calling with an empty dict for a zero-arg tool
        result = get_allowed_nodes.invoke({})
        
    assert "quantum_db" in result
    assert "A fake DB" in result