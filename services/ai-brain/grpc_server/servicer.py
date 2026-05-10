import grpc
from pydantic import ValidationError
from google.protobuf.json_format import MessageToDict

from proto import manifest_pb2
from proto import manifest_pb2_grpc
from brain.graph.graph import build_graph
from validators.manifest_schema import ManifestSchema

class ArchitectServicer(manifest_pb2_grpc.ArchitectBrainServicer):
    def RefineManifest(self, request, context):
        # 1. Strict Boundary Validation
        if request.HasField("current_manifest"):
            manifest_dict = MessageToDict(request.current_manifest, preserving_proto_field_name=True)
            try:
                # Convert proto -> Pydantic for strict schema validation
                ManifestSchema(**manifest_dict)
            except ValidationError as e:
                # Reject invalid manifests before they ever reach Gemini
                context.set_code(grpc.StatusCode.INVALID_ARGUMENT)
                context.set_details(f"Invalid manifest provided: {str(e)}")
                return manifest_pb2.RefineManifestResponse(
                    is_valid=False,
                    ai_reasoning="The provided manifest failed strict schema validation."
                )

        # 2. Map Protobuf -> LangGraph State
        initial_state = {
            "user_prompt": request.user_prompt,
            "proposed_manifest": None,
            "errors": [],
            "is_valid": False
        }

        # 3. Execute LangGraph
        graph = build_graph()
        final_state = graph.invoke(initial_state)

        # 4. Handle theoretical failures
        if not final_state.get("is_valid") or not final_state.get("proposed_manifest"):
            context.set_code(grpc.StatusCode.INTERNAL)
            context.set_details("Failed to generate a valid architecture manifest.")
            return manifest_pb2.RefineManifestResponse(is_valid=False)

        manifest_data = final_state["proposed_manifest"]

        # 5. Map LangGraph State -> Protobuf
        response = manifest_pb2.RefineManifestResponse(is_valid=True)
        response.updated_manifest.metadata.project_name = manifest_data["project_name"]

        for node_data in manifest_data.get("nodes", []):
            proto_node = response.updated_manifest.nodes.add()
            proto_node.id = node_data["id"]
            proto_node.type = node_data["type"]
            proto_node.version = node_data["version"]
            
            if "config" in node_data:
                proto_node.config.update(node_data["config"])

        return response