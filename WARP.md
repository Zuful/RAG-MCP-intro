# WARP.md

This file provides guidance to WARP (warp.dev) when working with code in this repository.

## Project Overview

RAG-MCP-intro is a document ingestion orchestrator that coordinates multiple microservices for RAG (Retrieval-Augmented Generation) workflows. It acts as the central hub that manages document parsing, embedding generation, and vector storage across a distributed microservices architecture.

## Architecture

This project implements a microservices architecture with clear separation of concerns:

```
[RAG-MCP-intro Orchestrator]
           |
           ├── DocParser Service (port 8080)
           |   └── Extracts text from documents (PDF, Word, etc.)
           |
           ├── Embedding Service (port 5001) 
           |   └── Python Flask server using SentenceTransformers
           |
           ├── Embeddingestion Service (port 8081)
           |   └── Stores vectors in ChromaDB with metadata
           |
           ├── MCP Ticket Tool (port 8083)
           |   └── Creates support tickets via HTTP API
           |
           └── ChromaDB (port 8000)
               └── Vector database for embeddings
```

### Key Components

- **Orchestrator** (`cmd/ingest/ingest.go`): Coordinates the full document ingestion pipeline
- **NovaBot** (`cmd/novabot/main.go`): RAG-enabled chatbot with local and OpenAI LLM options
- **Custom Embeddings** (`internal/embeddings/local.go`): ChromaDB integration with local embedding service
- **MCP Tool** (`internal/mcp/ticket_tool.go`): Model Context Protocol tool for ticket creation

## Common Development Commands

### Building and Running

```bash
# Run the document ingestion orchestrator
go run ./cmd/ingest

# Run the NovaBot RAG chatbot
go run ./cmd/novabot

# Run the MCP ticket tool server
go run ./cmd/ticket-tool

# Build all binaries
go build -o bin/ingest ./cmd/ingest
go build -o bin/novabot ./cmd/novabot  
go build -o bin/ticket-tool ./cmd/ticket-tool
```

### Dependencies and Setup

```bash
# Download Go dependencies
go mod tidy

# Start the Python embedding server (Flask)
python3 embed_server.py

# Start external services with Docker Compose
docker-compose up -d chromadb

# Install Python dependencies for embedding server
pip install flask sentence-transformers
```

### Testing and Development

```bash
# Test a specific package
go test ./internal/embeddings/
go test ./internal/mcp/

# Run with verbose output
go test -v ./...

# Format code
go fmt ./...
```

## Development Workflow

### Full System Startup Sequence

1. **Start ChromaDB**: `docker-compose up -d chromadb` (or ensure it's running on port 8000)
2. **Start Python embedding server**: `python3 embed_server.py` (port 5001)  
3. **Start DocParser service**: Ensure it's running on port 8080
4. **Start Embeddingestion service**: Ensure it's running on port 8081
5. **Run document ingestion**: `go run ./cmd/ingest` (processes `./data` directory)
6. **Start MCP tool**: `go run ./cmd/ticket-tool` (port 8083)
7. **Run NovaBot**: `go run ./cmd/novabot` (interactive RAG chatbot)

### Configuration

Environment variables are loaded from `.env` file:

- `DOC_PARSER_URL`: DocParser service URL (default: http://localhost:8080/parse)
- `EMBEDDING_URL`: Embedding service URL (default: http://localhost:5001/embed)
- `EMBEDDINGESTION_URL`: Vector storage service URL (default: http://localhost:8081)
- `COLLECTION_NAME`: ChromaDB collection name (default: novabot-rh)
- `CHROMA_DB_URL`: ChromaDB URL (default: http://localhost:8000)
- `OPENAI_API_KEY`: Optional for OpenAI integration

### Document Processing

- Place documents in `./data/` directory for ingestion
- Supported formats depend on DocParser service capabilities
- Sample documents: `guide-conges.md`, `politique-teletravail.md`, `procedure-note-de-frais.md`

## Code Architecture Patterns

### Microservice Communication

The project uses HTTP REST APIs for all inter-service communication:
- JSON payloads for structured data exchange
- Multipart form data for file uploads to DocParser
- Standardized error handling with HTTP status codes

### Embedding Integration

Custom embedding function (`GemmaEmbeddingFunction`) implements ChromaDB's `EmbeddingFunction` interface:
- Supports both document embedding and query embedding
- Integrates with local Python Flask embedding service
- Uses `google/embeddinggemma-300m` model via SentenceTransformers

### RAG Pipeline

1. **Document Ingestion**: DocParser → Embedding Service → Embeddingestion Service
2. **Query Processing**: User query → Embedding → ChromaDB similarity search → Context retrieval
3. **Response Generation**: Context + Query → LLM (Ollama local or OpenAI) → Response
4. **Tool Integration**: LLM can trigger MCP ticket creation tool

### LLM Integration Modes

- **Local Mode** (`useOllamaLocal = true`): Uses Ollama API with Gemma model
- **Hybrid Mode** (`useOllamaLocal = false`): Uses OpenAI GPT-4 with function calling

## Package Structure

- `cmd/`: Application entry points (ingest, novabot, ticket-tool)
- `internal/embeddings/`: ChromaDB embedding function implementation  
- `internal/mcp/`: Model Context Protocol tool implementation
- `data/`: Sample documents for ingestion
- `embed_server.py`: Python Flask embedding service

## Dependencies

### Go Modules
- `github.com/amikos-tech/chroma-go`: ChromaDB Go client
- `github.com/sashabaranov/go-openai`: OpenAI API client
- `github.com/joho/godotenv`: Environment variable management

### Python Dependencies
- `flask`: Web framework for embedding service
- `sentence-transformers`: Embedding model library

## Integration Notes

This orchestrator is designed to work with separate microservices that may be located in sibling directories:
- DocParser service (separate Go microservice)
- Embeddingestion service (separate Go microservice)
- External services via Docker Compose or direct installation
