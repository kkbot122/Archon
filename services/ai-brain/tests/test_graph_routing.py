from brain.graph.state import GraphState
from brain.graph.edges import route_after_validation

def test_router_loops_back_on_failure():
    """If the manifest is invalid, the router should point back to the generator node."""
    state: GraphState = {
        "user_prompt": "Build me a DB",
        "proposed_manifest": {"bad": "data"},
        "errors": ["Missing project_name"],
        "is_valid": False
    }
    
    next_node = route_after_validation(state)
    assert next_node == "generate_manifest"

def test_router_ends_on_success():
    """If the manifest is valid, the router should end the graph execution."""
    state: GraphState = {
        "user_prompt": "Build me a DB",
        "proposed_manifest": {"good": "data"},
        "errors": [],
        "is_valid": True
    }
    
    next_node = route_after_validation(state)
    assert next_node == "__end__"