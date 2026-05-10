from brain.graph.state import GraphState
from brain.graph.nodes import validate_manifest_node

from unittest.mock import patch

# Note: We haven't written generate_manifest_node yet, so this import will fail!
from brain.graph.nodes import generate_manifest_node

def test_validate_manifest_node_success():
    """If the AI generates a perfect manifest, the node should mark it as valid."""
    # Define the initial state (what comes into the node)
    initial_state: GraphState = {
        "user_prompt": "Build me a database",
        "proposed_manifest": {
            "version": "1.0",
            "project_name": "TestDB",
            "nodes": [
                {"id": "db_1", "type": "postgres_db", "version": "15", "config": {"db_user": "admin"}}
            ]
        },
        "errors": [],
        "is_valid": False
    }
    
    # Run the pure function
    updated_state = validate_manifest_node(initial_state)
    
    # Assert the state was updated correctly
    assert updated_state["is_valid"] is True
    assert len(updated_state["errors"]) == 0

def test_validate_manifest_node_failure():
    """If the AI hallucinates, the node should catch it and append errors to the state."""
    initial_state: GraphState = {
        "user_prompt": "Build me a database",
        "proposed_manifest": {
            "version": "1.0",
            "project_name": "TestDB",
            "nodes": [
                {"id": "db_1", "type": "alien_database", "version": "99", "config": {}} # Bad type!
            ]
        },
        "errors": [],
        "is_valid": False
    }
    
    updated_state = validate_manifest_node(initial_state)
    
    assert updated_state["is_valid"] is False
    assert len(updated_state["errors"]) == 1
    assert "alien_database" in updated_state["errors"][0]

@patch("brain.graph.nodes.call_gemini") 
def test_generate_manifest_node_success(mock_call_gemini):
    """The node should call Gemini and append the proposed JSON to the state."""
    
    # 1. Fake the AI's response so we don't hit the real API
    mock_call_gemini.return_value = {
        "version": "1.0",
        "project_name": "MockedProject",
        "nodes": [
            {"id": "api", "type": "go_backend", "version": "1.21", "config": {"port": "8080"}}
        ]
    }
    
    # 2. Setup the initial state
    initial_state = {
        "user_prompt": "Build me a Go backend",
        "proposed_manifest": None,
        "errors": [],
        "is_valid": False
    }
    
    # 3. Run our node
    updated_state = generate_manifest_node(initial_state)
    
    # 4. Verify it captured the AI's output correctly
    assert updated_state["proposed_manifest"]["project_name"] == "MockedProject"
    
    # 5. Verify the node actually passed the errors to the LLM (for self-correction)
    mock_call_gemini.assert_called_once_with(user_prompt="Build me a Go backend", errors=[])

@patch("brain.graph.nodes.call_gemini")
def test_generate_manifest_node_with_errors(mock_call_gemini):
    """If there are previous errors, the node should pass them to Gemini to fix."""
    mock_call_gemini.return_value = {"fixed": "manifest"}
    
    initial_state = {
        "user_prompt": "Build me a Go backend",
        "proposed_manifest": {"bad": "data"},
        "errors": ["Missing project_name"], # We have an error from the validator!
        "is_valid": False
    }
    
    generate_manifest_node(initial_state)
    
    # Verify the LLM was told about the error!
    mock_call_gemini.assert_called_once_with(user_prompt="Build me a Go backend", errors=["Missing project_name"])