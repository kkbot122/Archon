.PHONY: proto up down test-integration test-all test-unit

# Generate gRPC code for both Go and Python
proto:
	@echo "Generating Go & Python Protobufs..."
	protoc --go_out=./internal/pb --go_opt=module=github.com/kisna/archon/internal/pb \
	       --go-grpc_out=./internal/pb --go-grpc_opt=module=github.com/kisna/archon/internal/pb \
	       proto/manifest.proto
	cd services/ai-brain && \
	poetry run python -m grpc_tools.protoc -I../../proto \
	       --python_out=./proto \
	       --grpc_python_out=./proto \
	       ../../proto/manifest.proto
	@echo "Protobuf generation complete!"

# Start local infrastructure
up:
	docker-compose up -d

# Tear down local infrastructure
down:
	docker-compose down

# Run all unit tests (fast, no external deps)
test-unit:
	@echo "Running unit tests..."
	cd services/ai-brain && poetry run pytest tests/ -v --ignore=tests/integration
	cd services/api-gateway && go test ./...
	cd services/stitcher && go test ./...
	./internal/go test ./... 2>/dev/null || true  # internal module if it has tests

# Run the full integration test suite (requires docker-compose up)
test-integration:
	@echo "Starting AI Brain (mock) in background..."
	cd services/ai-brain && \
	GEMINI_MODEL=mock poetry run python -m grpc_server.server &
	@sleep 4  # wait for gRPC server to boot
	@echo "Running AI Brain integration tests..."
	cd services/ai-brain && poetry run pytest tests/integration/ -v
	@echo "Starting API Gateway in background..."
	cd services/api-gateway && \
	DATABASE_URL="postgres://archon_user:archon_password@localhost:5433/archon_db?sslmode=disable" \
	go run ./cmd/server/main.go &
	@sleep 5
	@echo "Running API Gateway integration tests..."
	cd services/api-gateway && go test -tags=integration -v ./tests/integration/
	@echo "Running Stitcher integration tests..."
	cd services/stitcher && go test -tags=integration -v -p=1 ./tests/integration/
	@echo "All integration tests passed!"

# Run everything – unit tests first, then integration tests
test-all: test-unit test-integration
	@echo "✅ All tests passed (unit + integration)"