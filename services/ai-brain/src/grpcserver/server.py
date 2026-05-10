import os
import time
import logging
from concurrent import futures
import grpc
from google.protobuf import json_format
from dotenv import load_dotenv, find_dotenv
from opentelemetry import trace
from opentelemetry.sdk.trace import TracerProvider
from opentelemetry.sdk.trace.export import BatchSpanProcessor
from opentelemetry.exporter.otlp.proto.grpc.trace_exporter import OTLPSpanExporter
from opentelemetry.sdk.resources import Resource

# 1. Clean Environment Loading (Finds .env at the monorepo root automatically)
load_dotenv(find_dotenv())

# 2. Clean Imports (Enabled by Poetry installing 'src' as a package)
from proto import manifest_pb2
from proto import manifest_pb2_grpc
from grpcserver.agent import app as ai_agent

logging.basicConfig(level=logging.INFO, format='%(asctime)s - %(levelname)s - %(message)s')
logger = logging.getLogger("AI-Brain")

# =================================================================
# OpenTelemetry Setup (Sends traces to Jaeger)
# =================================================================
resource = Resource(attributes={"service.name": "ai-brain-service"})
trace.set_tracer_provider(TracerProvider(resource=resource))
# Jaeger OTLP receiver runs on port 4317
otel_endpoint = os.getenv("OTEL_EXPORTER_OTLP_ENDPOINT", "http://localhost:4317")
otlp_exporter = OTLPSpanExporter(endpoint=otel_endpoint, insecure=True)
trace.get_tracer_provider().add_span_processor(BatchSpanProcessor(otlp_exporter))
tracer = trace.get_tracer("ai-brain.grpc")

class ArchitectBrainServicer(manifest_pb2_grpc.ArchitectBrainServicer):
    def RefineManifest(self, request, context):
        logger.info(f"🧠 --- New Request | Trace: {request.trace_id} ---")

        prompt_length = len(request.user_prompt)
        if prompt_length == 0:
            context.abort(grpc.StatusCode.INVALID_ARGUMENT, "user_prompt cannot be empty")
            return manifest_pb2.RefineManifestResponse()
            
        if prompt_length > 2000:
            logger.warning(f"⚠️ Prompt too long ({prompt_length} chars). Rejecting.")
            context.abort(grpc.StatusCode.INVALID_ARGUMENT, "user_prompt exceeds 2000 character limit")
            return manifest_pb2.RefineManifestResponse()
        
        # Input Validation
        if not request.user_prompt.strip():
            context.abort(grpc.StatusCode.INVALID_ARGUMENT, "user_prompt cannot be empty")
            return manifest_pb2.RefineManifestResponse()

        try:
            manifest_dict = json_format.MessageToDict(
                request.current_manifest, 
                preserving_proto_field_name=True
            )

            logger.info("🤖 Sending to LangGraph Agent...")
            
            with tracer.start_as_current_span(
                "langgraph_architect_execution",
                attributes={"trace_id": request.trace_id, "prompt": request.user_prompt}
            ) as span:
                # Execute the graph
                result = ai_agent.invoke({
                    "project_id": request.user_id,
                    "prompt": request.user_prompt,
                    "current_manifest": manifest_dict
                })
                
                final_response = result["final_response"]
                # Record the AI's decision in the trace
                span.set_attribute("is_valid", final_response.is_valid)
            
            updated_manifest_pb = manifest_pb2.ProjectManifest()
            json_format.ParseDict(final_response.updated_manifest.model_dump(), updated_manifest_pb)

            logger.info(f"✅ AI Reasoning: {final_response.ai_reasoning}")
            
            return manifest_pb2.RefineManifestResponse(
                trace_id=request.trace_id,
                is_valid=final_response.is_valid,
                ai_reasoning=final_response.ai_reasoning,
                updated_manifest=updated_manifest_pb
            )
            
        except Exception as e:
            # Catch unhandled LLM/Graph errors so Go doesn't crash
            logger.error(f"❌ LLM Execution Failed: {str(e)}")
            context.abort(grpc.StatusCode.INTERNAL, f"AI Brain Internal Error: {str(e)}")
            return manifest_pb2.RefineManifestResponse()

def serve():
    # Configurable Max Workers
    max_workers = int(os.getenv("GRPC_MAX_WORKERS", "10"))
    
    server_options = [
        ('grpc.keepalive_time_ms', 10000), 
        ('grpc.keepalive_timeout_ms', 3000), 
        ('grpc.keepalive_permit_without_calls', True), 
        ('grpc.http2.max_pings_without_data', 0), 
        ('grpc.http2.min_recv_ping_interval_without_data_ms', 5000), 
    ]

    server = grpc.server(futures.ThreadPoolExecutor(max_workers=max_workers), options=server_options)
    manifest_pb2_grpc.add_ArchitectBrainServicer_to_server(ArchitectBrainServicer(), server)
    
    port = os.getenv("AI_BRAIN_PORT", "50051")
    server.add_insecure_port(f'[::]:{port}')
    server.start()
    
    logger.info(f"🐍 AI Brain (gRPC) listening on port {port}...")
    
    try:
        while True:
            time.sleep(86400)
    except KeyboardInterrupt:
        logger.info("🛑 Shutting down AI Brain gracefully...")
        # Graceful Shutdown (allows in-flight LLM calls to finish)
        server.stop(grace=30)

if __name__ == '__main__':
    serve()