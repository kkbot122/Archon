from typing import Dict, List, Literal
from pydantic import BaseModel, Field

# We use Literal to strictly enforce the allowed tech stack. 
# If the AI hallucinates "quantum_database", Pydantic will instantly reject it.
AllowedNodeTypes = Literal["postgres_db", "redis_cache", "go_backend", "react_frontend"]

class NodeSchema(BaseModel):
    id: str = Field(..., description="Unique identifier for the node")
    type: AllowedNodeTypes = Field(..., description="The exact tech stack brick type")
    version: str = Field(..., description="Version of the brick")
    config: Dict[str, str] = Field(default_factory=dict, description="Key-value config parameters")

class ManifestSchema(BaseModel):
    version: str = Field(..., description="Manifest schema version (e.g., '1.0')")
    project_name: str = Field(..., min_length=1, description="Name of the generated project")
    nodes: List[NodeSchema] = Field(..., min_length=1, description="List of infrastructure bricks")