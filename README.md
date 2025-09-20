# RAG-MCP-intro - Document Ingestion Orchestrator

A containerized microservices architecture for document ingestion into vector databases using ChromaDB and advanced embedding models.

## 🏗️ Architecture

The system consists of independent microservices orchestrated via Docker Compose:

```
┌─────────────────┐    ┌──────────────────┐    ┌─────────────────┐
│ RAG-MCP-intro   │───▶│  DocParser       │───▶│  Embedding      │
│ (Orchestrator)  │    │  Service         │    │  Service        │
│  Port: N/A      │    │  Port: 8080      │    │  Port: 5001     │
└─────────────────┘    └──────────────────┘    └─────────────────┘
         │                                               │
         ▼                                               ▼
┌─────────────────┐                              ┌─────────────────┐
│ Embeddingestion │◀─────────────────────────────│     ChromaDB    │
│ Service         │                              │ Vector Database │
│ Port: 8081      │                              │ Port: 8000      │
└─────────────────┘                              └─────────────────┘
```

### Services Overview:
- **ChromaDB**: Vector database for storing document embeddings
- **DocParser**: Extracts text from documents (PDF, Word, etc.)
- **Embedding Service**: Generates embeddings using Google EmbeddingGemma-300M
- **Embeddingestion**: Handles vector storage and retrieval operations
- **RAG Orchestrator**: Coordinates the document ingestion pipeline

## 🚀 Quick Start

### Prerequisites

1. **Docker & Docker Compose** installed
2. **Hugging Face Account** with access to Google EmbeddingGemma-300M model
3. **Git** for cloning repositories

### Option 1: Docker Compose (Recommended)

```bash
# Clone the repository
git clone https://github.com/Zuful/RAG-MCP-intro.git
cd RAG-MCP-intro

# Set your Hugging Face token (see Authentication section below)
export HF_TOKEN="your_hf_token_here"

# Start all services
make docker-run
# OR
docker-compose up --build

# The orchestrator will automatically process documents in ./data directory
```

### Option 2: Individual Services

```bash
# Clone all repositories
git clone https://github.com/Zuful/docparser.git
git clone https://github.com/Zuful/embedding-service.git  
git clone https://github.com/Zuful/embeddingestion.git
git clone https://github.com/Zuful/RAG-MCP-intro.git

# Build and run each service individually
cd docparser && make docker-run &
cd embedding-service && make docker-run &
cd embeddingestion && make docker-run &
cd RAG-MCP-intro && make run-ingest
```

## 🔐 Hugging Face Authentication

The Embedding Service uses Google's EmbeddingGemma-300M model, which requires Hugging Face authentication.

### Method 1: Environment Variable (Recommended for Deployment)

```bash
# Set your Hugging Face token before starting services
export HF_TOKEN="hf_xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx"
docker-compose up
```

### Method 2: Login via Hugging Face CLI (Local Development)

```bash
# Install Hugging Face CLI
pip install huggingface_hub

# Login (this will store token in ~/.cache/huggingface/)
huggingface-cli login

# Now you can run the services
docker-compose up
```

### Method 3: Manual Token File

```bash
# Create token file manually
mkdir -p ~/.cache/huggingface
echo "hf_xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx" > ~/.cache/huggingface/token

# Run services
docker-compose up
```

### Getting Your Hugging Face Token

1. Visit [Hugging Face Tokens](https://huggingface.co/settings/tokens)
2. Create a new token with "Read" permissions
3. Request access to [Google EmbeddingGemma-300M](https://huggingface.co/google/embeddinggemma-300m)
4. Use the token with one of the methods above

## ⚙️ Configuration

Environment variables are automatically configured by Docker Compose. For manual configuration:

```bash
# Service URLs (automatically set in Docker Compose)
CHROMA_HOST=chromadb
CHROMA_PORT=8000
EMBEDDING_SERVICE_HOST=embedding-service
EMBEDDING_SERVICE_PORT=5001
DOCPARSER_SERVICE_HOST=docparser
DOCPARSER_SERVICE_PORT=8080
EMBEDDINGESTION_SERVICE_HOST=embeddingestion
EMBEDDINGESTION_SERVICE_PORT=8081

# Hugging Face Authentication
HF_TOKEN=your_token_here
```

```

## 📁 Usage

### Document Processing

1. **Add Documents**: Place your documents in the `./data` directory
   - Supported formats: PDF, Word, Plain Text
   - The orchestrator automatically processes all files in this directory

2. **Start Processing**: Documents are automatically ingested when you run:
   ```bash
   make docker-run
   # OR
   docker-compose up
   ```

3. **Monitor Progress**: Check the container logs to see processing status:
   ```bash
   docker-compose logs -f rag-orchestrator
   ```

### Available Commands

```bash
# Development
make run-ingest       # Run ingest orchestrator locally
make build-all        # Build all services
make clean           # Clean build artifacts

# Docker Operations
make docker-build    # Build Docker image
make docker-push     # Push to Docker Hub
make docker-run      # Start all services with Docker Compose
make docker-stop     # Stop all services
make docker-logs     # Show service logs

# Setup
make setup           # Full development setup
make help            # Show all available commands
```

## 🔧 Development

### Local Development

```bash
# Clone and set up the project
git clone https://github.com/Zuful/RAG-MCP-intro.git
cd RAG-MCP-intro
make setup

# Start services in development mode
make docker-run

# Or build and run individual components
make build-ingest
./build/ingest
```

### Service Architecture

Each microservice is independently deployable:

- **[DocParser](https://github.com/Zuful/docparser)**: Go service for document text extraction
- **[Embedding Service](https://github.com/Zuful/embedding-service)**: Python service using Google EmbeddingGemma-300M
- **[Embeddingestion](https://github.com/Zuful/embeddingestion)**: Go service for vector storage in ChromaDB
- **RAG Orchestrator**: Coordinates the document processing pipeline

### Docker Images

All services are available on Docker Hub:

```bash
# Pull individual images
docker pull zuful/docparser:latest
docker pull zuful/embedding-service:latest
docker pull zuful/embeddingestion:latest

# Or let Docker Compose handle it automatically
docker-compose pull
```

## 🌟 Benefits of This Architecture

- ✅ **Containerized**: Full Docker support with compose orchestration
- ✅ **Microservices**: Each service has a single responsibility  
- ✅ **Scalable**: Services can be scaled independently
- ✅ **Reusable**: Services can be used by other applications
- ✅ **Maintainable**: Easier to update and debug individual components
- ✅ **Technology Agnostic**: Each service can use different technologies
- ✅ **Production Ready**: Includes proper health checks and networking
- ✅ **Advanced ML**: Uses state-of-the-art Google EmbeddingGemma model

## 🚨 Troubleshooting

### Hugging Face Authentication Issues

```bash
# Check if token is set
echo $HF_TOKEN

# Verify token works
curl -H "Authorization: Bearer $HF_TOKEN" \
     https://huggingface.co/api/whoami

# Check container logs
docker-compose logs embedding-service
```

### Service Connection Issues

```bash
# Check service health
curl http://localhost:5001/health  # Embedding service
curl http://localhost:8000/api/v2/heartbeat  # ChromaDB

# Check network connectivity
docker network ls
docker network inspect rag-mcp-intro_rag-network
```

### Common Issues

1. **Port conflicts**: Ensure ports 8000, 8080, 5001, 8081 are available
2. **Memory issues**: Embedding model requires ~2GB RAM
3. **Disk space**: ChromaDB needs space for vector storage
4. **Docker permissions**: Ensure Docker daemon is running

## 📄 License

This project is open source and available under the MIT License.
