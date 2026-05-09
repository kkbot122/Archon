import json
import os
import redis
from pathlib import Path
from typing import List, Dict, TypedDict
from pydantic import BaseModel, Field

from langchain_google_genai import ChatGoogleGenerativeAI
from langchain_core.prompts import ChatPromptTemplate
from langgraph.graph import StateGraph, END

# =================================================================
# 1. Schema Definitions
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

class AIResponse(BaseModel):
    is_valid: bool = Field(description="True if valid architecture request, False if impossible or non-software.")
    ai_reasoning: str = Field(description="Explanation of what was changed, referencing the atomic library.")
    updated_manifest: ProjectManifest

# =================================================================
# 2. LangGraph State Management
# =================================================================

class GraphState(TypedDict):
    project_name: str         # NEW: Used as the Redis Key
    prompt: str
    current_manifest: dict
    atomic_library: dict      
    chat_history: list        # NEW: The fetched conversation history
    final_response: AIResponse

# =================================================================
# 3. Redis Setup
# =================================================================

# Connect to the Redis container running on your local machine
redis_addr = os.getenv("REDIS_ADDR", "localhost:6379")
redis_host, redis_port = redis_addr.split(":")
rdb = redis.Redis(host=redis_host, port=int(redis_port), decode_responses=True)

# =================================================================
# 4. Graph Nodes
# =================================================================

def fetch_context(state: GraphState):
    """NODE 1: Loads the Atomic Library AND the Conversation History from Redis."""
    # 1. Fetch Library
    root_dir = Path(__file__).resolve().parents[4]
    index_path = root_dir / "atomic-library" / "index.json"
    
    try:
        with open(index_path, 'r') as f:
            library_data = json.load(f)
    except Exception as e:
        print(f"⚠️ Failed to load atomic library: {e}")
        library_data = {"error": "Atomic library unavailable."}

    # 2. Fetch History from Redis
    history_key = f"archon:history:{state['project_name']}"
    
    try:
        # Fetch the last 10 messages (5 conversation turns)
        raw_history = rdb.lrange(history_key, 0, -1)
        chat_history = [json.loads(msg) for msg in raw_history]
    except Exception as e:
        print(f"⚠️ Redis connection failed: {e}")
        chat_history = []

    return {"atomic_library": library_data, "chat_history": chat_history}


def architect_solution(state: GraphState):
    """NODE 2: Mutates the architecture with Memory & Constraints."""
    llm = ChatGoogleGenerativeAI(model="gemini-2.5-flash", temperature=0)
    structured_llm = llm.with_structured_output(AIResponse)

    prompt_text = state["prompt"]
    manifest_str = json.dumps(state["current_manifest"], indent=2)
    library_str = json.dumps(state["atomic_library"], indent=2)
    
    # Format the history for the LLM prompt
    history_str = "\n".join([f"{msg['role'].capitalize()}: {msg['content']}" for msg in state["chat_history"]])
    if not history_str:
        history_str = "No prior conversation."

    system_msg = """You are Archon, an expert cloud software architect.
    Your job is to read a JSON project manifest, listen to the user's prompt, and return an updated JSON manifest.
    
    CRITICAL CONSTRAINT: You MUST cross-reference your decisions with the ATOMIC LIBRARY.
    - You can ONLY add nodes with a 'type' that exists in the Atomic Library.
    - You MUST include the 'required_config' keys for that node type.
    
    RECENT CONVERSATION HISTORY (Use this for context if the user asks for revisions):
    {history}
    
    ATOMIC LIBRARY:
    {library}
    
    RULES:
    1. If the request is impossible or asks for a stack NOT in the Atomic Library, set is_valid to false, explain why, and return the original manifest untouched.
    2. Make sure any new nodes have unique, snake_case string IDs.
    3. Set is_valid to true and explain exactly what you added/removed in the ai_reasoning field."""

    prompt_template = ChatPromptTemplate.from_messages([
        ("system", system_msg),
        ("human", "Current Architecture:\n{manifest}\n\nUser Request: {prompt}")
    ])

    chain = prompt_template | structured_llm
    
    result = chain.invoke({
        "library": library_str,
        "history": history_str,
        "manifest": manifest_str, 
        "prompt": prompt_text
    })

    if not result.updated_manifest:
        result.updated_manifest = ProjectManifest(**state["current_manifest"])

    # --- SAVE MEMORY TO REDIS ---
    history_key = f"archon:history:{state['project_name']}"
    try:
        # Push User Prompt
        rdb.rpush(history_key, json.dumps({"role": "user", "content": prompt_text}))
        # Push AI Reasoning
        rdb.rpush(history_key, json.dumps({"role": "assistant", "content": result.ai_reasoning}))
        # Trim history to keep only the latest 10 messages so the LLM context window doesn't blow up
        rdb.ltrim(history_key, -10, -1)
    except Exception as e:
        print(f"⚠️ Failed to save history to Redis: {e}")

    return {"final_response": result}

# =================================================================
# 5. Compile the Pipeline
# =================================================================

workflow = StateGraph(GraphState)
workflow.add_node("fetch_context", fetch_context)
workflow.add_node("architect", architect_solution)
workflow.set_entry_point("fetch_context")
workflow.add_edge("fetch_context", "architect")
workflow.add_edge("architect", END)

app = workflow.compile()