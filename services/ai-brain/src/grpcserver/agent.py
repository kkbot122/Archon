import json
from typing import List, Dict, Optional, TypedDict
from pydantic import BaseModel, Field
from dotenv import load_dotenv
from pathlib import Path

from langchain_google_genai import ChatGoogleGenerativeAI
from langchain_core.prompts import ChatPromptTemplate
from langgraph.graph import StateGraph, END

env_path = Path(__file__).resolve().parents[4] / ".env"

load_dotenv(env_path)

# =================================================================
# 1. Schema Definitions (Strict Guardrails for the LLM)
# =================================================================

class Metadata(BaseModel):
    project_name: str
    target_cloud: str
    schema_version: str

class ArchitectureNode(BaseModel):
    id: str
    type: str
    version: str
    config: Dict[str, str]

class Connection(BaseModel):
    source_id: str
    target_id: str
    protocol: str

class ProjectManifest(BaseModel):
    metadata: Metadata
    nodes: List[ArchitectureNode]
    connections: List[Connection]
    feature_flags: List[str]

# This is the exact structure our Go Gateway expects back!
class AIResponse(BaseModel):
    is_valid: bool = Field(description="True if the request is a valid software architecture modification, False if impossible (like building a time machine).")
    ai_reasoning: str = Field(description="Explanation of exactly what was changed, or why it was rejected.")
    updated_manifest: ProjectManifest = Field(description="The updated architecture manifest.")

# =================================================================
# 2. LangGraph State Management
# =================================================================

class GraphState(TypedDict):
    prompt: str
    current_manifest: dict
    final_response: AIResponse

# =================================================================
# 3. The Core LLM Logic
# =================================================================

# We use gemini-1.5-flash for maximum speed. You can swap to 'pro' for complex logic later.
llm = ChatGoogleGenerativeAI(model="gemini-2.5-flash", temperature=0)
structured_llm = llm.with_structured_output(AIResponse)

def process_architecture(state: GraphState):
    prompt_text = state["prompt"]
    manifest_str = json.dumps(state["current_manifest"], indent=2)

    system_msg = """You are Archon, an expert cloud software architect.
    Your job is to read a JSON project manifest, listen to the user's prompt, and return an updated JSON manifest.
    
    RULES:
    1. If the user asks for something impossible or non-software related (e.g., 'build a time machine', 'write me a poem'), set is_valid to false, explain why politely in ai_reasoning, and return the original manifest untouched.
    2. If the request is valid (e.g., 'add a redis cache', 'swap to postgres'), update the nodes and connections arrays. 
    3. Make sure any new nodes have unique, snake_case string IDs.
    4. Set is_valid to true and explain exactly what you added/removed in the ai_reasoning field so the user understands the change."""

    prompt_template = ChatPromptTemplate.from_messages([
        ("system", system_msg),
        ("human", "Current Architecture:\n{manifest}\n\nUser Request: {prompt}")
    ])

    chain = prompt_template | structured_llm
    
    # Run the LLM
    result = chain.invoke({"manifest": manifest_str, "prompt": prompt_text})

    # Failsafe: If the LLM somehow fails to return a manifest, bounce back the current one
    if not result.updated_manifest:
        result.updated_manifest = ProjectManifest(**state["current_manifest"])

    return {"final_response": result}

# =================================================================
# 4. Compile the Graph
# =================================================================

workflow = StateGraph(GraphState)
workflow.add_node("process_architecture", process_architecture)
workflow.set_entry_point("process_architecture")
workflow.add_edge("process_architecture", END)

# This compiled 'app' is what our gRPC server will import and call
app = workflow.compile()