# Multi-stage Dockerfile for eBPF Agent

# Stage 1: Build environment with eBPF toolchain
FROM ubuntu:22.04 AS builder

# Install build dependencies
RUN apt-get update && apt-get install -y \
    clang \
    llvm \
    golang-1.21 \
    make \
    git \
    libbpf-dev \
    linux-headers-generic \
    && rm -rf /var/lib/apt/lists/*

# Set up Go environment
ENV PATH="/usr/lib/go-1.21/bin:${PATH}"
ENV GOPATH=/go
ENV PATH="${GOPATH}/bin:${PATH}"

# Install bpf2go tool
RUN go install github.com/cilium/ebpf/cmd/bpf2go@latest

# Set working directory
WORKDIR /build

# Copy source files
COPY go.mod go.sum ./
RUN go mod download

COPY . .

# Generate eBPF bytecode and build the agent
RUN go generate ./...
RUN go build -o ebpf-agent .

# Stage 2: Runtime environment (minimal)
FROM ubuntu:22.04

# Install runtime dependencies
RUN apt-get update && apt-get install -y \
    ca-certificates \
    libbpf0 \
    && rm -rf /var/lib/apt/lists/*

# Copy the compiled binary from builder
COPY --from=builder /build/ebpf-agent /usr/local/bin/ebpf-agent

# Set working directory
WORKDIR /app

# Default environment variables (can be overridden)
ENV NATS_URL="nats://nats-service:4222"
ENV APP_ID="arbitrum-node-service"
ENV TARGET_BINARY="/usr/local/bin/geth"
ENV TARGET_SYMBOL="github.com/ethereum/go-ethereum/rpc.(*Server).serveRequest"
ENV TARGET_PID="0"

# Run as root (required for eBPF)
USER root

# Start the agent
ENTRYPOINT ["/usr/local/bin/ebpf-agent"]
