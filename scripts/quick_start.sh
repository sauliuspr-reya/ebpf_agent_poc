#!/bin/bash
# Quick start script for eBPF Agent PoC
# This script helps you get started quickly

set -e

COLOR_GREEN='\033[0;32m'
COLOR_BLUE='\033[0;34m'
COLOR_RED='\033[0;31m'
COLOR_YELLOW='\033[1;33m'
COLOR_RESET='\033[0m'

info() {
    echo -e "${COLOR_BLUE}ℹ ${1}${COLOR_RESET}"
}

success() {
    echo -e "${COLOR_GREEN}✓ ${1}${COLOR_RESET}"
}

error() {
    echo -e "${COLOR_RED}✗ ${1}${COLOR_RESET}"
}

warning() {
    echo -e "${COLOR_YELLOW}⚠ ${1}${COLOR_RESET}"
}

# Check if running on Linux
if [[ "$OSTYPE" != "linux-gnu"* ]]; then
    warning "This script is designed for Linux systems."
    warning "eBPF programs can only run on Linux kernels."
    echo ""
    info "For development on macOS:"
    echo "  1. Use Docker Desktop with WSL2 backend"
    echo "  2. Or develop in a Linux VM"
    echo "  3. Or use a remote Linux machine"
    exit 1
fi

info "Starting eBPF Agent PoC setup..."
echo ""

# Check for required tools
info "Checking prerequisites..."

check_command() {
    if command -v $1 &> /dev/null; then
        success "$1 is installed"
        return 0
    else
        error "$1 is not installed"
        return 1
    fi
}

MISSING_DEPS=0

check_command "go" || MISSING_DEPS=1
check_command "clang" || MISSING_DEPS=1
check_command "make" || MISSING_DEPS=1

if [ $MISSING_DEPS -eq 1 ]; then
    echo ""
    error "Missing dependencies. Please install them first."
    echo ""
    info "On Ubuntu/Debian:"
    echo "  sudo apt-get install -y golang clang llvm make libbpf-dev linux-headers-\$(uname -r)"
    echo ""
    exit 1
fi

echo ""
success "All prerequisites are installed!"
echo ""

# Install bpf2go if not present
if ! command -v bpf2go &> /dev/null; then
    info "Installing bpf2go tool..."
    go install github.com/cilium/ebpf/cmd/bpf2go@latest
    success "bpf2go installed"
else
    success "bpf2go is already installed"
fi

echo ""
info "Downloading Go dependencies..."
make deps
success "Dependencies downloaded"

echo ""
info "Building the project..."
make build
success "Build complete!"

echo ""
info "Starting NATS server (in background)..."

# Check if NATS is already running
if lsof -Pi :4222 -sTCP:LISTEN -t >/dev/null ; then
    success "NATS is already running on port 4222"
else
    # Try to start NATS with Docker
    if command -v docker &> /dev/null; then
        docker run -d --name nats-poc -p 4222:4222 -p 8222:8222 nats:latest
        success "NATS started in Docker container"
        info "NATS monitoring available at http://localhost:8222"
    else
        warning "Docker not found. Please start NATS manually:"
        echo "  docker run -d -p 4222:4222 nats:latest"
        exit 1
    fi
fi

echo ""
info "Testing NATS connection..."
sleep 2
if nc -z localhost 4222 2>/dev/null; then
    success "NATS is reachable"
else
    error "Cannot connect to NATS on port 4222"
    exit 1
fi

echo ""
success "Setup complete!"
echo ""
info "Next steps:"
echo ""
echo "1. Run the NATS subscriber (in a new terminal):"
echo "   go run nats_subscriber_example.go"
echo ""
echo "2. Run the eBPF agent (requires sudo):"
echo "   sudo ./ebpf-agent"
echo ""
echo "3. Or use environment variables:"
echo "   sudo NATS_URL=nats://localhost:4222 \\"
echo "        APP_ID=my-app \\"
echo "        TARGET_BINARY=/path/to/binary \\"
echo "        ./ebpf-agent"
echo ""
info "For more information, see README.md"
echo ""
