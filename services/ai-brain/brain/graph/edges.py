from typing import Literal
from brain.graph.state import GraphState

def route_after_validation(state: GraphState) -> Literal["generate_manifest", "__end__"]:
    """
    Reads the state and determines the next node to execute.
    """
    if state.get("is_valid"):
        return "__end__"
    return "generate_manifest"