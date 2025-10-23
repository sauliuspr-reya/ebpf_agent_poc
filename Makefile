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

# Build Docker image
docker-build:
	@echo "Building Docker image..."
	docker build -t $(DOCKER_IMAGE) .

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

help:
	@echo "Available targets:"
	@echo "  tools        - Install required build tools (bpf2go)"
	@echo "  generate     - Generate eBPF bytecode from C source"
	@echo "  build        - Build the Go agent binary"
	@echo "  run          - Run the agent locally (requires sudo)"
	@echo "  clean        - Remove generated files and binaries"
	@echo "  docker-build - Build Docker container image"
	@echo "  docker-run   - Run the agent in Docker"
	@echo "  deps         - Download Go dependencies"
	@echo "  test         - Run tests"
	@echo "  install      - Install agent to /usr/local/bin"
