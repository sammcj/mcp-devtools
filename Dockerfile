# Build stage
FROM golang:1.24-alpine AS builder

# Set working directory
WORKDIR /app

# Copy go.mod and go.sum files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy the source code
COPY . .

# Build the application with version information
ARG VERSION=0.1.0-dev
ARG COMMIT=unknown
ARG BUILD_DATE=unknown

# Build with version information
RUN CGO_ENABLED=0 GOOS=linux go build \
  -ldflags "-X main.Version=${VERSION} -X main.Commit=${COMMIT} -X main.BuildDate=${BUILD_DATE}" \
  -o mcp-devtools .

# Final stage
FROM python:3.13-alpine

# Set working directory
WORKDIR /app

# Install system dependencies
RUN apk --no-cache add \
    ca-certificates \
    gcc \
    musl-dev \
    libffi-dev \
    openssl-dev \
    python3-dev \
    libxml2-dev \
    libxslt-dev \
    py3-pip \
    g++ \
    cython \
    make

# Install Python dependencies for document processing
# RUN pip install --no-cache-dir docling

# Copy the binary from the builder stage
COPY --from=builder /app/mcp-devtools .

# Copy the Python scripts
COPY internal/tools/docprocessing/docling_processor.py ./internal/tools/docprocessing/

# Create cache directory
RUN mkdir -p /app/.mcp-devtools/docling-cache

# Set environment variables for document processing
ENV DOCLING_PYTHON_PATH=/usr/local/bin/python3
ENV DOCLING_CACHE_DIR=/app/.mcp-devtools/docling-cache
ENV DOCLING_CACHE_ENABLED=true
ENV DOCLING_HARDWARE_ACCELERATION=auto
ENV DOCLING_TIMEOUT=300
ENV DOCLING_MAX_FILE_SIZE=100
# ENV MEMORY_FILE_PATH=
# ENV BRAVE_API_KEY=
# ENV SEARXNG_BASE_URL=
# ENV SEARXNG_USERNAME=
# ENV SEARXNG_PASSWORD=
# ENV DISABLED_FUNCTIONS=

VOLUME ["/app/.mcp-devtools/docling-cache"]

# Expose port
EXPOSE 18080

# Run the application with HTTP transport by default
CMD [ "./mcp-devtools", "--transport", "http", "--port", "18080", "--base-url", "http://0.0.0.0" ]
