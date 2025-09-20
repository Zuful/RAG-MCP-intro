# Build stage
FROM golang:1.23.4-alpine AS builder

WORKDIR /app

# Install build dependencies including git and build tools for CGO
RUN apk add --no-cache git gcc musl-dev

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the ingest service (main orchestrator)
RUN CGO_ENABLED=1 GOOS=linux go build -a -ldflags '-extldflags "-static"' -o ingest ./cmd/ingest

# Runtime stage
FROM alpine:latest

# Install ca-certificates for HTTPS requests
RUN apk --no-cache add ca-certificates

WORKDIR /root/

# Copy the binary from builder stage
COPY --from=builder /app/ingest .

# The data directory will be mounted from host via docker-compose
# so we don't need to copy it here

# Run the ingest orchestrator
CMD ["./ingest"]
