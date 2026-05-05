.PHONY: proto up down

# Generate gRPC code for both Go and Python
proto:
	@echo "Generating Go & Python Protobufs..."
	protoc --go_out=./internal --go_opt=paths=source_relative --go-grpc_out=./internal --go-grpc_opt=paths=source_relative proto/manifest.proto
	python -m grpc_tools.protoc -I. --python_out=./services/ai-brain/src/grpcserver --grpc_python_out=./services/ai-brain/src/grpcserver proto/manifest.proto
	@echo "Protobuf generation complete!"

# Start local infrastructure
up:
	docker-compose up -d

# Tear down local infrastructure
down:
	docker-compose down