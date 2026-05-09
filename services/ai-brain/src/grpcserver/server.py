import sys
import os
import time
import logging
from concurrent import futures
from pathlib import Path
import grpc

# 1. Path Hacks (Because Python protoc generation is stubborn)
# This ensures Python can find the generated pb2 files in the proto/ directory
current_dir = Path(__file__).resolve().parent
proto_dir = current_dir.parent / "proto"
sys.path.insert(0, str(proto_dir))

import manifest_pb2
import manifest_pb2_grpc

from google.protobuf import json_format
from dotenv import load_dotenv
from agent import app as ai_agent

load_dotenv(dotenv_path=current_dir.parent.parent.parent / '.env')

# Configure basic logging
logging.basicConfig(level=logging.INFO, format='%(asctime)s - %(levelname)s - %(message)s')
logger = logging.getLogger("AI-Brain")

class ArchitectBrainServicer(manifest_pb2_grpc.ArchitectBrainServicer):
    def RefineManifest(self, request, context):
        logger.info("🧠 --- New Architecture Request ---")
        logger.info(f"Trace ID: {request.trace_id}")
        logger.info(f"User Prompt: '{request.user_prompt}'")
        logger.info(f"Current Nodes: {len(request.current_manifest.nodes)}")
        
        # Convert the Protobuf request into a standard Python dictionary
        manifest_dict = json_format.MessageToDict(
            request.current_manifest, 
            preserving_proto_field_name=True # Keeps snake_case (e.g. project_name)
        )

        logger.info("🤖 Sending to Gemini...")
        
        # Run our LangGraph AI Agent!
        result = ai_agent.invoke({
            "prompt": request.user_prompt,
            "current_manifest": manifest_dict
        })
        
        final_response = result["final_response"]
        
        # Convert the Pydantic dictionary back into a Protobuf object
        updated_manifest_pb = manifest_pb2.ProjectManifest()
        json_format.ParseDict(final_response.updated_manifest.model_dump(), updated_manifest_pb)

        logger.info(f"✅ AI Reasoning: {final_response.ai_reasoning}")
        
        return manifest_pb2.RefineManifestResponse(
            trace_id=request.trace_id,
            is_valid=final_response.is_valid,
            ai_reasoning=final_response.ai_reasoning,
            updated_manifest=updated_manifest_pb
        )

def serve():
    # NEW: Configure Python to accept frequent keepalive pings from the Go client
    server_options = [
        ('grpc.keepalive_time_ms', 10000), # Send pings every 10 seconds
        ('grpc.keepalive_timeout_ms', 3000), # Wait 3 seconds for ping ack
        ('grpc.keepalive_permit_without_calls', True), # Allow pings even when idle
        ('grpc.http2.max_pings_without_data', 0), # 0 means unlimited pings
        ('grpc.http2.min_recv_ping_interval_without_data_ms', 5000), # Allow client to ping every 5s
    ]

    # Create a gRPC server with the new options
    server = grpc.server(futures.ThreadPoolExecutor(max_workers=10), options=server_options)
    
    # Attach our service to the server
    manifest_pb2_grpc.add_ArchitectBrainServicer_to_server(ArchitectBrainServicer(), server)
    
    # Listen on port 50051
    server.add_insecure_port('[::]:50051')
    server.start()
    
    logger.info("🐍 AI Brain (gRPC) listening on port 50051...")
    
    # Keep the main thread alive
    try:
        while True:
            time.sleep(86400)
    except KeyboardInterrupt:
        logger.info("🛑 Shutting down AI Brain...")
        server.stop(0)

if __name__ == '__main__':
    serve()