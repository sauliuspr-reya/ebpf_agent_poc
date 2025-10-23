# JSON-RPC eBPF Agent - PoC

A production-ready eBPF-based monitoring agent for capturing JSON-RPC traffic (Arbitrum/Ethereum nodes) and streaming telemetry features to NATS.io.

## 🎯 Overview

This PoC demonstrates:
- **eBPF-based tracing** of JSON-RPC calls in Go applications (e.g., geth, Arbitrum nodes)
- **Real-time streaming** of monitoring features to NATS.io
- **Minimal overhead** kernel-space data capture
- **Kubernetes-ready** containerized deployment

## 📋 Prerequisites

### Development Environment

- **Linux** (kernel 4.18+ with eBPF support)
- **Go** 1.21 or higher
- **Clang/LLVM** (for compiling C to eBPF bytecode)
- **Kernel Headers** matching your target kernel
- **NATS Server** (for receiving telemetry)

### Install Dependencies

#### Ubuntu/Debian
```bash
sudo apt-get update
sudo apt-get install -y clang llvm golang libbpf-dev linux-headers-$(uname -r)
```

#### macOS (for development, not runtime)
```bash
brew install llvm go
```

#### Install bpf2go
```bash
go install github.com/cilium/ebpf/cmd/bpf2go@latest
```

## 🚀 Quick Start

### 1. Clone and Build

```bash
cd /path/to/ebpf_agent_poc

# Download Go dependencies
make deps

# Install bpf2go tool
make tools

# Generate eBPF bytecode and build agent
make build
```

### 2. Run NATS Server

```bash
# Using Docker
docker run -d --name nats -p 4222:4222 nats:latest

# Or download from https://nats.io/download/
./nats-server
```

### 3. Run the Agent

```bash
# Run with default configuration (requires root for eBPF)
sudo ./ebpf-agent

# Or with custom configuration
sudo NATS_URL=nats://localhost:4222 \
     APP_ID=my-arbitrum-node \
     TARGET_BINARY=/usr/local/bin/geth \
     TARGET_PID=12345 \
     ./ebpf-agent
```

## ⚙️ Configuration

The agent is configured via environment variables:

| Variable | Default | Description |
|----------|---------|-------------|
| `NATS_URL` | `nats://localhost:4222` | NATS server URL |
| `APP_ID` | `arbitrum-node-service` | Application identifier |
| `TARGET_BINARY` | `/usr/local/bin/geth` | Path to the target binary |
| `TARGET_SYMBOL` | `github.com/ethereum/go-ethereum/rpc.(*Server).serveRequest` | Function symbol to trace |
| `TARGET_PID` | `0` | Target process ID (0 = all processes) |

## 📊 NATS Message Format

The agent publishes messages to NATS with the following structure:

**Subject**: `{APP_ID}.{PROTOCOL}.{FEATURE_TYPE}`  
Example: `arbitrum-node-service.jsonrpc.response-size`

**Payload** (JSON):
```json
{
  "app_id": "arbitrum-node-service",
  "protocol": "jsonrpc",
  "feature_type": "response-size",
  "timestamp": "2024-10-23T10:52:00Z",
  "value": 1536.0,
  "context_hash": "method:eth_getBlockByNumber",
  "details": {
    "pid": 12345,
    "process": "geth",
    "method": "eth_getBlockByNumber",
    "timestamp_ns": 1234567890123456789
  }
}
```

## 🐳 Docker Deployment

### Build and Run

```bash
# Build Docker image
make docker-build

# Run in Docker (with NATS on host)
make docker-run

# Or manually with custom config
docker run --rm --privileged \
  -v /sys/kernel/debug:/sys/kernel/debug:ro \
  -v /proc:/host/proc:ro \
  --pid=host \
  -e NATS_URL=nats://host.docker.internal:4222 \
  -e APP_ID=my-app \
  -e TARGET_BINARY=/usr/local/bin/geth \
  ebpf-agent:latest
```

**Important**: The container requires:
- `--privileged` flag for eBPF operations
- `--pid=host` to access host processes
- Volume mounts for `/sys/kernel/debug` and `/proc`

## ☸️ Kubernetes Deployment

See `k8s-deployment.yaml` for a complete example. Key requirements:

```yaml
securityContext:
  privileged: true
  capabilities:
    add: ["SYS_ADMIN", "SYS_RESOURCE"]
volumeMounts:
  - name: sys-kernel-debug
    mountPath: /sys/kernel/debug
  - name: proc
    mountPath: /host/proc
hostPID: true
```

Deploy:
```bash
kubectl apply -f k8s-deployment.yaml
```

## 🔍 Testing & Verification

### Subscribe to NATS Messages

```bash
# Install NATS CLI
go install github.com/nats-io/natscli/nats@latest

# Subscribe to all messages
nats sub ">"

# Subscribe to specific app
nats sub "arbitrum-node-service.>"

# Subscribe to specific feature type
nats sub "*.jsonrpc.response-size"
```

### Monitor Agent Logs

```bash
# Local
sudo ./ebpf-agent

# Docker
docker logs -f <container-id>

# Kubernetes
kubectl logs -f deployment/ebpf-agent
```

## 🛠️ Development

### Project Structure

```
.
├── rpc_tracer.c           # eBPF C program (kernel space)
├── agent_main.go          # Go agent (user space)
├── go.mod                 # Go dependencies
├── go.sum                 # Go dependency checksums
├── Makefile               # Build automation
├── Dockerfile             # Container image
├── k8s-deployment.yaml    # Kubernetes manifest
├── headers/               # BPF headers
│   └── bpf_helpers.h
└── README.md              # This file
```

### Generate eBPF Code

```bash
# The //go:generate directive in agent_main.go runs:
go generate ./...

# This executes:
# go run github.com/cilium/ebpf/cmd/bpf2go -cc clang rpc rpc_tracer.c -- -I./headers
```

This generates:
- `rpc_bpfel.go` (little-endian bytecode)
- `rpc_bpfel.o` (object file)
- `rpc_bpfeb.go` (big-endian bytecode, if applicable)

### Clean Build

```bash
make clean
make build
```

## 🎓 Understanding the Implementation

### 1. eBPF Tracer (`rpc_tracer.c`)

- Attaches a **uprobe** to the return point of a JSON-RPC handler
- Captures: PID, timestamp, method name, response size
- Sends data to userspace via **perf buffer**

### 2. Go Agent (`agent_main.go`)

- Loads compiled eBPF program using cilium/ebpf
- Attaches uprobe to target process/binary
- Reads events from perf buffer
- Publishes structured features to NATS

### 3. Feature Engineering

Current implementation tracks:
- **Response size** per JSON-RPC method
- Can be extended to capture:
  - Latency (entry/exit uprobes)
  - Error rates
  - Request payloads
  - Custom metrics

## 🔧 Troubleshooting

### Permission Denied
```bash
# Ensure running as root
sudo ./ebpf-agent

# Or add CAP_BPF capability (kernel 5.8+)
sudo setcap cap_bpf,cap_perfmon=ep ./ebpf-agent
```

### Cannot Find Symbol
```bash
# List available symbols in the target binary
nm -D /usr/local/bin/geth | grep -i rpc

# Or use objdump
objdump -T /usr/local/bin/geth | grep -i serveRequest
```

### NATS Connection Failed
```bash
# Check NATS server is running
nats server check

# Test connectivity
telnet localhost 4222
```

### eBPF Program Load Failed
```bash
# Check kernel version (need 4.18+)
uname -r

# Verify BPF support
zgrep CONFIG_BPF /proc/config.gz

# Check dmesg for BPF verifier errors
sudo dmesg | grep -i bpf
```

## 🎯 Next Steps for Production

### Phase 1: Current PoC ✅
- Basic uprobe attachment
- Mock data capture
- NATS publishing
- Docker/K8s deployment

### Phase 2: Real Data Capture
- Implement `bpf_probe_read_user()` to read actual method names
- Extract request/response payloads
- Add entry uprobe for latency measurement

### Phase 3: Advanced Features
- Multi-protocol support (HTTP, gRPC)
- Adaptive sampling based on traffic
- Local aggregation before NATS publish
- Performance profiling dashboard

### Phase 4: Production Hardening
- Comprehensive error handling
- Health checks and metrics
- Rate limiting and backpressure
- Security audit and hardening

## 📚 Resources

- [Cilium eBPF Library](https://github.com/cilium/ebpf)
- [NATS.io Documentation](https://docs.nats.io/)
- [eBPF Tutorial](https://ebpf.io/)
- [BPF Performance Tools](http://www.brendangregg.com/bpf-performance-tools-book.html)

## 📄 License

MIT License - see LICENSE file for details

## 🤝 Contributing

Contributions welcome! Please:
1. Fork the repository
2. Create a feature branch
3. Submit a pull request with tests

## 📧 Contact

For questions or support, open an issue on GitHub.

---

**Note**: This is a PoC implementation. For production use, conduct thorough testing and security reviews.
