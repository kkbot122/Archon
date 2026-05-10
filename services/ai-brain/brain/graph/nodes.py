import json
from typing import Dict, Any, List
from brain.agent import get_architect_agent
from brain.graph.state import GraphState
from validators.manifest_schema import ManifestSchema

def call_gemini(user_prompt: str, errors: List[str]) -> Dict[str, Any]:
    agent = get_architect_agent()
    
    system_instructions = """
    You are an expert software architect.
    1. First, call 'get_allowed_nodes' to see the current Atomic Library.
    2. Generate a JSON manifest using ONLY those allowed types/versions.
    3. Return ONLY valid JSON.
    """
    
    error_context = f"\nFIX THESE ERRORS: {errors}" if errors else ""
    full_prompt = f"{system_instructions}\n\nUSER: {user_prompt}{error_context}"
    
    # Simple invocation for now; in a full agentic loop, 
    # LangGraph would handle the tool-calling loop.
    response = agent.invoke(full_prompt)
    
    # Clean and parse JSON
    clean_json = response.content.strip().removeprefix("```json").removesuffix("```").strip()
    return json.loads(clean_json)

def validate_manifest_node(state: GraphState) -> GraphState:
    """
    A pure function that attempts to parse the proposed manifest.
    It returns a strictly updated state without mutating the original.
    """
    # Create a fresh copy of the state to guarantee pure function behavior
    new_state: GraphState = {
        "user_prompt": state["user_prompt"],
        "proposed_manifest": state.get("proposed_manifest"),
        "errors": state.get("errors", []).copy(),
        "is_valid": False
    }

    manifest_data = new_state["proposed_manifest"]
    
    if not manifest_data:
        new_state["errors"].append("No manifest proposed by the AI.")
        return new_state

    try:
        # Run it through our strict Pydantic model
        ManifestSchema(**manifest_data)
        new_state["is_valid"] = True
        new_state["errors"] = []  # Clear previous errors if successful
        
    except ValidationError as e:
        new_state["is_valid"] = False
        # Append the specific Pydantic error so the AI can read it and fix its mistake later
        new_state["errors"].append(str(e))

    return new_state

def generate_manifest_node(state: GraphState) -> GraphState:
    """
    Reads the user prompt and any previous validation errors, 
    asks Gemini to generate a JSON manifest, and updates the state.
    """
    # 1. Create a fresh copy of the state (Pure Function rule)
    new_state: GraphState = {
        "user_prompt": state["user_prompt"],
        "proposed_manifest": state.get("proposed_manifest"),
        "errors": state.get("errors", []).copy(),
        "is_valid": state.get("is_valid", False)
    }
    
    # 2. Call the LLM (which is mocked in our tests)
    generated_json = call_gemini(
        user_prompt=new_state["user_prompt"], 
        errors=new_state["errors"]
    )
    
    # 3. Update the state with the AI's proposal
    new_state["proposed_manifest"] = generated_json
    
    return new_state