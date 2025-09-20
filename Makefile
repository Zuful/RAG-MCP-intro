# Application configuration
PROJECT_NAME=RAG-MCP-intro
BUILD_DIR=build

# Docker configuration
DOCKER_IMAGE=zuful/rag-mcp-intro
DOCKER_TAG=latest

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOMOD=$(GOCMD) mod

.PHONY: all build-ingest build-novabot build-ticket-tool build-all run-ingest run-novabot run-ticket-tool clean deps test docker-build docker-push docker-run help

all: build-all

# Build individual services
build-ingest:
	@echo "🏗️  Building ingest service..."
	@mkdir -p $(BUILD_DIR)
	$(GOBUILD) -o $(BUILD_DIR)/ingest ./cmd/ingest

build-novabot:
	@echo "🤖 Building novabot service..."
	@mkdir -p $(BUILD_DIR)
	$(GOBUILD) -o $(BUILD_DIR)/novabot ./cmd/novabot

build-ticket-tool:
	@echo "🎫 Building ticket-tool service..."
	@mkdir -p $(BUILD_DIR)
	$(GOBUILD) -o $(BUILD_DIR)/ticket-tool ./cmd/ticket-tool

# Build all services
build-all: build-ingest build-novabot build-ticket-tool
	@echo "✅ All services built successfully!"

# Run services
run-ingest: build-ingest
	@echo "🚀 Running ingest orchestrator..."
	./$(BUILD_DIR)/ingest

run-novabot: build-novabot
	@echo "🚀 Running novabot..."
	./$(BUILD_DIR)/novabot

run-ticket-tool: build-ticket-tool
	@echo "🚀 Running ticket-tool..."
	./$(BUILD_DIR)/ticket-tool

# Development dependencies setup
deps:
	@echo "📦 Downloading dependencies..."
	$(GOMOD) download
	$(GOMOD) tidy

# Run tests
test:
	@echo "🧪 Running tests..."
	$(GOTEST) -v ./...

# Clean build artifacts
clean:
	@echo "🧹 Cleaning build artifacts..."
	$(GOCLEAN)
	rm -rf $(BUILD_DIR)

# Format code
fmt:
	@echo "🎨 Formatting code..."
	$(GOCMD) fmt ./...

# Lint code (requires golangci-lint)
lint:
	@if command -v golangci-lint >/dev/null 2>&1; then \
		echo "🔍 Linting code..."; \
		golangci-lint run; \
	else \
		echo "golangci-lint not found. Install with: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest"; \
	fi

# Build Docker image (defaults to ingest service)
docker-build:
	@echo "🐳 Building Docker image..."
	docker build -t $(DOCKER_IMAGE):$(DOCKER_TAG) .

# Push Docker image to registry
docker-push:
	@echo "📤 Pushing Docker image to registry..."
	docker push $(DOCKER_IMAGE):$(DOCKER_TAG)

# Run with Docker Compose (start all microservices)
docker-run:
	@echo "🚀 Starting all services with Docker Compose..."
	docker-compose up --build

# Stop Docker Compose services
docker-stop:
	@echo "🛑 Stopping Docker Compose services..."
	docker-compose down

# Show Docker Compose logs
docker-logs:
	@echo "📋 Showing Docker Compose logs..."
	docker-compose logs -f

# Initialize environment file
init-env:
	@if [ ! -f .env ]; then \
		echo "📝 Creating .env file..."; \
		echo "CHROMA_HOST=localhost" > .env; \
		echo "CHROMA_PORT=8000" >> .env; \
		echo "EMBEDDING_SERVICE_HOST=localhost" >> .env; \
		echo "EMBEDDING_SERVICE_PORT=5001" >> .env; \
		echo "DOCPARSER_SERVICE_HOST=localhost" >> .env; \
		echo "DOCPARSER_SERVICE_PORT=8080" >> .env; \
		echo ".env file created with default values."; \
	else \
		echo ".env file already exists."; \
	fi

# Full development setup
setup: deps init-env
	@echo "✅ Development setup complete!"
	@echo "1. Review and update the .env file if needed"
	@echo "2. Start microservices: make docker-run"
	@echo "3. Run ingestion: make run-ingest"

# Help
help:
	@echo "Available commands for $(PROJECT_NAME):"
	@echo "  build-ingest     - Build ingest orchestrator"
	@echo "  build-novabot    - Build novabot service"
	@echo "  build-ticket-tool- Build ticket-tool service"
	@echo "  build-all        - Build all services"
	@echo "  run-ingest       - Run ingest orchestrator"
	@echo "  run-novabot      - Run novabot service"
	@echo "  run-ticket-tool  - Run ticket-tool service"
	@echo "  deps             - Download and tidy dependencies"
	@echo "  test             - Run tests"
	@echo "  clean            - Clean build artifacts"
	@echo "  fmt              - Format code"
	@echo "  lint             - Lint code"
	@echo "  docker-build     - Build Docker image"
	@echo "  docker-push      - Push Docker image to registry"
	@echo "  docker-run       - Start all services with Docker Compose"
	@echo "  docker-stop      - Stop Docker Compose services"
	@echo "  docker-logs      - Show Docker Compose logs"
	@echo "  init-env         - Create .env file with defaults"
	@echo "  setup            - Full development setup"
	@echo "  help             - Show this help message"
