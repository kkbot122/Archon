from langchain_google_genai import ChatGoogleGenerativeAI
from brain.tools.atomic_lookup import get_allowed_nodes

def get_architect_agent():
    """
    Initializes Gemini and binds it to the atomic library tool.
    This ensures the AI always looks up the tech stack before generating.
    """
    llm = ChatGoogleGenerativeAI(model="gemini-2.5-flash", temperature=0.2)
    
    # We bind the tool so Gemini can 'decide' to call it
    return llm.bind_tools([get_allowed_nodes])