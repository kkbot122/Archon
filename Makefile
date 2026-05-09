.PHONY: proto up down

# Generate gRPC code for both Go and Python
proto:
	@echo "Generating Go & Python Protobufs..."
	protoc --go_out=./internal/pb --go_opt=paths=source_relative \
	       --go-grpc_out=./internal/pb --go-grpc_opt=paths=source_relative \
	       proto/manifest.proto
	cd services/ai-brain && poetry run python -m grpc_tools.protoc -I../../ \
	       --python_out=./src \
	       --grpc_python_out=./src \
	       ../../proto/manifest.proto
	@echo "Protobuf generation complete!"

# Start local infrastructure
up:
	docker-compose up -d

# Tear down local infrastructure
down:
	docker-compose down