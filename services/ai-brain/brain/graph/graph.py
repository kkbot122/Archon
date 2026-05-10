from langgraph.graph import StateGraph, START, END
from brain.graph.state import GraphState
from brain.graph.nodes import generate_manifest_node, validate_manifest_node
from brain.graph.edges import route_after_validation

def build_graph():
    """Compiles the LangGraph execution engine."""
    workflow = StateGraph(GraphState)

    # Add nodes
    workflow.add_node("generate_manifest", generate_manifest_node)
    workflow.add_node("validate_manifest", validate_manifest_node)

    # Add edges
    workflow.add_edge(START, "generate_manifest")
    workflow.add_edge("generate_manifest", "validate_manifest")
    
    # Add conditional routing
    workflow.add_conditional_edges(
        "validate_manifest",
        route_after_validation,
        {
            "generate_manifest": "generate_manifest",
            "__end__": END
        }
    )

    return workflow.compile()