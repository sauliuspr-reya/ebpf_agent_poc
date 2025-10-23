# ⚡ Quick Start on macOS

## TL;DR

You're on macOS, eBPF needs Linux. Here's what you can do:

### ✅ What You Can Do Right Now

```bash
# 1. Check your setup
make check-env

# 2. Start NATS server
docker run -d --name nats-test -p 4222:4222 nats:latest

# 3. Test NATS subscriber (in one terminal)
make test-subscriber

# 4. Publish a test message (in another terminal)
docker exec nats-test nats pub "test.jsonrpc.response-size" \
  '{"app_id":"test","protocol":"jsonrpc","feature_type":"response-size","value":1234}'
```

### 🐳 Build for Linux (via Docker)

```bash
# Build the complete agent in a Linux container
make docker-build

# This creates: ebpf-agent:latest
# The Docker build handles all the Linux-specific compilation
```

### 🚀 Deploy Options

**Option 1: Kubernetes (Recommended)**
```bash
# Tag and push to your registry
docker tag ebpf-agent:latest your-registry/ebpf-agent:v1
docker push your-registry/ebpf-agent:v1

# Edit k8s-deployment.yaml to use your image
# Deploy:
kubectl apply -f k8s-deployment.yaml
```

**Option 2: Linux VM with Multipass**
```bash
# Install and create VM
brew install multipass
multipass launch --name ebpf --cpus 2 --memory 4G

# Mount and enter
multipass mount . ebpf:/home/ubuntu/ebpf_agent_poc
multipass shell ebpf

# Inside VM:
cd ~/ebpf_agent_poc
sudo apt update && sudo apt install -y clang llvm golang make libbpf-dev linux-headers-$(uname -r)
make build
sudo ./ebpf-agent
```

**Option 3: Remote Linux Server**
```bash
# Copy to server
scp -r . user@linux-server:~/ebpf_agent_poc

# SSH and build
ssh user@linux-server
cd ~/ebpf_agent_poc
make build
sudo ./ebpf-agent
```

## 🎯 Typical Development Workflow

1. **Edit code** on macOS (any editor/IDE)
2. **Test NATS logic** with subscriber
3. **Build in Docker** when ready
4. **Deploy to Linux** for real testing
5. **Iterate** based on results

## 📖 Full Documentation

- **README.md** - Complete project documentation
- **MACOS_DEVELOPMENT.md** - Detailed macOS guide
- **POC_PLAN.md** - Implementation roadmap

## 🆘 Common Issues

**"llvm-strip not found"** → Fixed by adding LLVM to PATH (already handled in Makefile)

**"linux/bpf.h not found"** → Expected on macOS, use Docker instead

**IDE errors about undefined types** → Expected, generated files need Linux

## 📞 Need Help?

```bash
make help    # Show all available commands
```
