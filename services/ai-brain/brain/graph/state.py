from typing import TypedDict, List, Dict, Any, Optional

class GraphState(TypedDict):
    """
    The single, immutable state shape that flows through every node in the LangGraph.
    """
    user_prompt: str
    proposed_manifest: Optional[Dict[str, Any]]
    errors: List[str]
    is_valid: bool