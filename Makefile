# Makefile for mcp-devtools

# Variables
BINARY_NAME=mcp-devtools
BINARY_PATH=bin/$(BINARY_NAME)
GO=go
GOFLAGS=-v
GOFMT=$(GO) fmt
GOTEST=$(GO) test
DOCKER=docker
DOCKER_IMAGE=$(BINARY_NAME)

# Default target
.PHONY: all
all: build

# Build the server
.PHONY: build
build:
	mkdir -p bin
	$(GO) build $(GOFLAGS) -o $(BINARY_PATH) \
		-ldflags "-X main.Version=$(shell git describe --tags --always --dirty 2>/dev/null || echo '0.1.0-dev') \
		-X main.Commit=$(shell git rev-parse --short HEAD 2>/dev/null || echo 'unknown') \
		-X main.BuildDate=$(shell date -u +"%Y-%m-%dT%H:%M:%SZ")" \
		.

# Run the server with stdio transport (default)
.PHONY: run
run: build
	./$(BINARY_PATH)

# Run the server with HTTP transport
.PHONY: run-http
run-http: build
	./$(BINARY_PATH) --transport http --port 18080 --base-url http://localhost

# Run tests (all tests including external dependencies)
.PHONY: test
test:
	$(GOTEST) $(GOFLAGS) ./...

# Run fast tests (no external dependencies)
.PHONY: test-fast
test-fast:
	$(GOTEST) -short -v ./tests/...

# Clean build artifacts
.PHONY: clean
clean:
	rm -rf bin/

# Format code
.PHONY: fmt
fmt:
	$(GOFMT) ./...

# Lint code
.PHONY: lint
lint:
	gofmt -w -s .
	golangci-lint run

# Install dependencies
.PHONY: deps
deps:
	$(GO) mod download

# Update dependencies
.PHONY: update-deps
update-deps:
	$(GO) get -u ./...
	$(GO) mod tidy

# Install Python dependencies for document processing
.PHONY: install-docling
install-docling:
	@echo "Installing Python dependencies for document processing..."
	@if command -v python3 >/dev/null 2>&1; then \
		echo "Found python3, installing docling..."; \
		python3 -m pip install --user docling; \
	elif command -v python >/dev/null 2>&1; then \
		echo "Found python, installing docling..."; \
		python -m pip install --user docling; \
	else \
		echo "Error: Python 3.10+ is required for document processing"; \
		echo "Please install Python 3.10+ and try again"; \
		exit 1; \
	fi
	@echo "Docling installation complete!"

# Check if docling is available
.PHONY: check-docling
check-docling:
	@echo "Checking docling availability..."
	@if command -v python3 >/dev/null 2>&1; then \
		python3 -c "import docling; print('✓ Docling is available')" 2>/dev/null || \
		(echo "✗ Docling not found. Run 'make install-docling' to install it."; exit 1); \
	elif command -v python >/dev/null 2>&1; then \
		python -c "import docling; print('✓ Docling is available')" 2>/dev/null || \
		(echo "✗ Docling not found. Run 'make install-docling' to install it."; exit 1); \
	else \
		echo "✗ Python 3.10+ is required for document processing"; \
		exit 1; \
	fi

# Install all dependencies (Go + Python)
.PHONY: install-all
install-all: deps install-docling
	@echo "All dependencies installed successfully!"

# Build Docker image
.PHONY: docker-build
docker-build:
	$(DOCKER) build -t $(DOCKER_IMAGE) .

# Run Docker container
.PHONY: docker-run
docker-run: docker-build
	$(DOCKER) run -p 18080:18080 $(DOCKER_IMAGE)

# Create a new release
.PHONY: release
release:
	@if [ -z "$(VERSION)" ]; then echo "VERSION is required. Use: make release VERSION=x.y.z"; exit 1; fi
	git tag -a v$(VERSION) -m "Release v$(VERSION)"
	git push origin v$(VERSION)

# Help target
.PHONY: help
help:
	@echo "Available targets:"
	@echo "  all          : Build the server (default)"
	@echo "  build        : Build the server"
	@echo "  run          : Run the server with stdio transport (default)"
	@echo "  run-http     : Run the server with Streamable HTTP transport"
	@echo "  test         : Run all tests (including external dependencies)"
	@echo "  test-fast    : Run fast tests (no external dependencies)"
	@echo "  clean        : Clean build artifacts"
	@echo "  fmt          : Format code"
	@echo "  lint         : Lint code"
	@echo "  deps         : Install Go dependencies"
	@echo "  update-deps  : Update Go dependencies"
	@echo "  install-docling : Install Python dependencies for document processing"
	@echo "  check-docling   : Check if docling is available"
	@echo "  install-all     : Install all dependencies (Go + Python)"
	@echo "  docker-build : Build Docker image"
	@echo "  docker-run   : Run Docker container with HTTP transport"
	@echo "  release      : Create a new release (requires VERSION=x.y.z)"
	@echo "  help         : Show this help message"
