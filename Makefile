.PHONY: all generate build clean docker-build docker-run test

# Variables
BINARY_NAME=ebpf-agent
DOCKER_IMAGE=ebpf-agent:latest
GO=go
CLANG=clang

all: generate build

# Install required tools
tools:
	@echo "Installing required tools..."
	$(GO) install github.com/cilium/ebpf/cmd/bpf2go@latest

# Generate eBPF bytecode from C source using bpf2go
generate:
	@echo "Generating eBPF bytecode..."
	@if [ "$$(uname)" = "Darwin" ]; then \
		echo "⚠️  macOS detected - eBPF compilation requires Linux kernel headers"; \
		echo "Use 'make docker-build' to build in a Linux container instead"; \
		exit 1; \
	fi
	$(GO) generate ./...

# Build the Go agent binary
build: generate
	@echo "Building Go agent..."
	$(GO) build -o $(BINARY_NAME) .

# Run the agent locally (requires root privileges)
run: build
	@echo "Running eBPF agent (requires sudo)..."
	sudo ./$(BINARY_NAME)

# Clean generated files and binaries
clean:
	@echo "Cleaning up..."
	rm -f $(BINARY_NAME)
	rm -f rpc_bpfel.go rpc_bpfel.o
	rm -f rpc_bpfeb.go rpc_bpfeb.o

# Build Docker image (amd64 for GKE compatibility)
docker-build:
	@echo "Building Docker image for linux/amd64..."
	docker build --platform linux/amd64 -t $(DOCKER_IMAGE) .

# Run Docker container (requires privileged mode for eBPF)
docker-run: docker-build
	@echo "Running Docker container..."
	docker run --rm --privileged \
		-v /sys/kernel/debug:/sys/kernel/debug:ro \
		-v /proc:/host/proc:ro \
		--pid=host \
		-e NATS_URL=nats://host.docker.internal:4222 \
		$(DOCKER_IMAGE)

# Download dependencies
deps:
	@echo "Downloading Go dependencies..."
	$(GO) mod download

# Run tests
test:
	@echo "Running tests..."
	$(GO) test -v ./...

# Install the agent to /usr/local/bin (requires sudo)
install: build
	@echo "Installing agent to /usr/local/bin..."
	sudo cp $(BINARY_NAME) /usr/local/bin/

# Test NATS subscriber (works on macOS)
test-subscriber:
	@echo "Starting NATS subscriber..."
	$(GO) run nats_subscriber_example.go

# macOS-specific: Check environment
check-env:
	@echo "Checking build environment..."
	@if [ "$$(uname)" = "Darwin" ]; then \
		echo "🍎 macOS detected"; \
		echo ""; \
		echo "ℹ️  eBPF programs require Linux to build and run."; \
		echo ""; \
		echo "Options for development:"; \
		echo "  1. Use 'make docker-build' to build in Linux container"; \
		echo "  2. Test NATS integration with 'make test-subscriber'"; \
		echo "  3. Deploy to a Linux environment for full testing"; \
	else \
		echo "🐧 Linux detected - ready for eBPF development"; \
	fi

help:
	@echo "Available targets:"
	@echo "  tools        - Install required build tools (bpf2go)"
	@echo "  generate     - Generate eBPF bytecode from C source (Linux only)"
	@echo "  build        - Build the Go agent binary (Linux only)"
	@echo "  run          - Run the agent locally (requires sudo, Linux only)"
	@echo "  clean        - Remove generated files and binaries"
	@echo "  docker-build - Build Docker container image"
	@echo "  docker-run   - Run the agent in Docker"
	@echo "  deps         - Download Go dependencies"
	@echo "  test         - Run tests"
	@echo "  test-subscriber - Run NATS subscriber (works on macOS)"
	@echo "  check-env    - Check if environment is ready for eBPF"
	@echo "  install      - Install agent to /usr/local/bin"
