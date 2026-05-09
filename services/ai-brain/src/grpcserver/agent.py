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
# 1. Schema Definitions (MUST BE ABOVE THE LLM INIT)
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
    is_valid: bool = Field(description="True if valid architecture request, False if impossible.")
    ai_reasoning: str = Field(description="Explanation of what was changed, referencing the atomic library.")
    updated_manifest: ProjectManifest

# =================================================================
# 2. Module-Level Initialization & Caching
# =================================================================

def _load_atomic_library() -> dict:
    try:
        index_path = Path(__file__).resolve().parents[4] / "atomic-library" / "index.json"
        with open(index_path, 'r') as f:
            return json.load(f)
    except Exception as e:
        print(f"⚠️ Atomic library unavailable: {e}")
        return {}

# Load once into memory on boot
ATOMIC_LIBRARY = _load_atomic_library()

# Initialize LLM client once (safely below AIResponse definition)
_llm = ChatGoogleGenerativeAI(model=os.getenv("GEMINI_MODEL", "gemini-1.5-flash-latest"), temperature=0)
_structured_llm = _llm.with_structured_output(AIResponse)

# =================================================================
# 2. LangGraph State Management
# =================================================================

class GraphState(TypedDict):
    project_id: str         # NEW: Used as the Redis Key
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
    """Loads History and passes cached Atomic Library."""
    history_key = f"archon:history:{state['project_id']}"
    
    try:
        raw_history = rdb.lrange(history_key, 0, -1)
        chat_history = [json.loads(msg) for msg in raw_history]
    except Exception as e:
        print(f"⚠️ Redis connection failed: {e}")
        chat_history = []

    return {"atomic_library": ATOMIC_LIBRARY, "chat_history": chat_history}


def architect_solution(state: GraphState):
    """Mutates the architecture with Memory, Constraints, and Error Handling."""
    prompt_text = state["prompt"]
    manifest_str = json.dumps(state["current_manifest"], indent=2)
    library_str = json.dumps(state["atomic_library"], indent=2)
    
    history_str = "\n".join([f"{msg['role'].capitalize()}: {msg['content']}" for msg in state["chat_history"]])
    if not history_str:
        history_str = "No prior conversation."

    system_msg = """... (Keep your exact system prompt from before) ..."""

    prompt_template = ChatPromptTemplate.from_messages([
        ("system", system_msg),
        ("human", "Current Architecture:\n{manifest}\n\nUser Request: {prompt}")
    ])

    chain = prompt_template | _structured_llm
    
    # CRITICAL FIX: Error handling for API timeouts/rate limits
    try:
        result = chain.invoke({
            "library": library_str,
            "history": history_str,
            "manifest": manifest_str, 
            "prompt": prompt_text
        })
    except Exception as e:
        print(f"❌ Gemini API failure: {e}")
        return {"final_response": AIResponse(
            is_valid=False,
            ai_reasoning="The AI service is temporarily unavailable. Your architecture was not changed.",
            updated_manifest=ProjectManifest.model_validate(state["current_manifest"])
        )}

    # MAJOR FIX: Use Pydantic model_validate instead of unpacking kwargs
    if not result or not result.updated_manifest:
        result.updated_manifest = ProjectManifest.model_validate(state["current_manifest"])

    # MAJOR FIX: Atomic Redis Pipeline
    history_key = f"archon:history:{state['project_id']}"
    try:
        pipe = rdb.pipeline()
        pipe.rpush(history_key, json.dumps({"role": "user", "content": prompt_text}))
        pipe.rpush(history_key, json.dumps({"role": "assistant", "content": result.ai_reasoning}))
        pipe.ltrim(history_key, -10, -1)
        pipe.execute()
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