import json
import os
from langchain_core.tools import tool

@tool
def get_allowed_nodes() -> str:
    """
    Retrieves the list of supported infrastructure bricks from the atomic library.
    Use this tool to see available node types, versions, and required config.
    """
    library_path = os.path.join("..", "..", "atomic-library", "index.json")
    
    try:
        with open(library_path, "r") as f:
            data = json.load(f)
            nodes = data.get("available_nodes", [])
            return json.dumps(nodes, indent=2)
    except FileNotFoundError:
        return "Error: Atomic library index.json not found."
    except Exception as e:
        return f"Error reading library: {str(e)}"