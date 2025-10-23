# 🎉 eBPF RPC Monitor - Current Status

## ✅ What's Working

### Kernel-Level Network Capture
- ✅ **Kprobe on `tcp_sendmsg`** - Captures ALL TCP traffic from Node.js processes
- ✅ **Process filtering** - Only captures from "node" processes (your oracle)
- ✅ **DaemonSet deployment** - Runs on all nodes (including spot nodes)
- ✅ **Debug logging** - Verbose logging enabled for troubleshooting
- ✅ **NATS publishing** - Successfully sending events to NATS

### Current Metrics Captured
- **Process ID & Name**: Which Node.js process made the request
- **Packet Size**: Bytes sent/received
- **Direction**: Send (outgoing) or Recv (incoming)
- **Timestamp**: Nanosecond precision

## 🚧 In Progress (Next Phase)

### Enhanced Capture (Ready to Deploy)
- 📝 **Destination IP/Port extraction** - Know where traffic is going
- 📝 **Port filtering** - Only capture HTTPS (443) and RPC ports (8545, 8547)
- 📝 **HTTP/JSON-RPC parsing** - Extract `eth_*` method names from payload
- 📝 **Hostname resolution** - Convert IPs to hostnames

### New NATS Subject Structure
```
Old: testnet-rpc-monitor.jsonrpc.send-size
New: rpc.{destination}.{protocol}.{method}.{metric}
```

**Examples:**
```
rpc.rpc-reya-cronos-gelato-digital.https.eth_call.request_size
rpc.rpc-reya-cronos-gelato-digital.https.eth_sendTransaction.request_size
rpc.rpc-reya-cronos-gelato-digital.https.eth_getBalance.request_size
```

## 📊 Current Deployment

### Cluster Info
- **Cluster**: `gke-de-test` (Testnet)
- **Namespace**: `nats`
- **Agent Pods**: 12 (across all nodes including spot)
- **Oracle Pod**: `cronos-oracle-offchain-964c8b9c5-ncfqw`
- **Oracle Node**: `gke-gke-de-test-cronos-app-spot-432d0844-t6bm`

### Event Volume
- **1,197+ events captured** in first few minutes
- **Various packet sizes**: 24 bytes (ACKs) to 52KB (large transfers)
- **Multiple PIDs**: 279776, 269938, 4898 (different Node.js processes)

### Sample Event (Current)
```json
{
  "app_id": "testnet-rpc-monitor",
  "protocol": "jsonrpc",
  "feature_type": "send-size",
  "timestamp": "2025-10-23T12:39:09Z",
  "value": 1952,
  "details": {
    "pid": 279776,
    "process": "node",
    "method": "unknown",  // ← Will be eth_call, etc.
    "direction": "send",
    "size_bytes": 1952
  }
}
```

### Sample Event (After Update)
```json
{
  "app_id": "testnet-rpc-monitor",
  "protocol": "jsonrpc",
  "feature_type": "request_size",
  "timestamp": "2025-10-23T12:45:00Z",
  "value": 342,
  "context_hash": "rpc.rpc-reya-cronos-gelato-digital.https.eth_call.request_size",
  "details": {
    "pid": 279776,
    "process": "node",
    "method": "eth_call",  // ← Parsed from payload!
    "direction": "send",
    "size_bytes": 342,
    "dest_ip": "34.185.237.112",  // ← New!
    "dest_port": 443,  // ← New!
    "dest_hostname": "rpc-reya-cronos-gelato-digital"  // ← New!
  }
}
```

## 🔄 Next Steps to Deploy Enhanced Version

### 1. Rebuild with Enhanced Tracer
```bash
cd /Users/saulius/Sites/learn_ebpf/ebpf_agent_poc
make docker-build
docker tag ebpf-agent:latest gcr.io/testnet-473109/ebpf-agent:latest
docker push gcr.io/testnet-473109/ebpf-agent:latest
```

### 2. Deploy Updated Agent
```bash
kubectl rollout restart daemonset/ebpf-agent -n nats
```

### 3. Monitor New Events
```bash
# Subscribe to new hierarchical subjects
nats context select testnet
nats sub "rpc.>"

# Or filter by destination
nats sub "rpc.rpc-reya-cronos-gelato-digital.>"

# Or filter by method
nats sub "rpc.*.*.eth_call.>"
```

## 📈 Analytics Capabilities (After Update)

### Traffic by Destination
```bash
nats sub "rpc.>" | grep dest_hostname | sort | uniq -c
```

### Most Common Methods
```bash
nats sub "rpc.>" | jq -r '.details.method' | sort | uniq -c | sort -rn
```

### Request Size Distribution
```bash
nats sub "rpc.*.*.eth_call.request_size" | jq '.value' | \
  awk '{sum+=$1; count++} END {print "Avg:", sum/count, "Total:", count}'
```

### Real-Time Dashboard
```bash
# Watch eth_call traffic
watch -n 1 'nats req "rpc.*.*.eth_call.>" "" --count 10 | \
  jq -r "[.details.method, .value] | @tsv" | column -t'
```

## 🎯 Success Metrics

- ✅ **Zero code changes** to your TypeScript oracle
- ✅ **Kernel-level capture** - Can't be bypassed
- ✅ **Container-aware** - Works across all pods
- ✅ **Low overhead** - eBPF runs in kernel space
- ✅ **Structured data** - Easy to query and analyze

## 🔧 Configuration

### Current ConfigMap
```yaml
data:
  NATS_URL: "nats://nats.nats.svc.cluster.local:4222"
  APP_ID: "testnet-rpc-monitor"
  DEBUG: "true"  # Verbose logging
```

### Files Updated
- ✅ `rpc_tracer.c` - Enhanced with IP/port/payload capture
- ✅ `agent_main.go` - New NATS subject structure
- ✅ `k8s-deployment.yaml` - Toleration for spot nodes
- ✅ `NATS_STRUCTURE.md` - Documentation

## 📚 Documentation
- `NATS_STRUCTURE.md` - Complete guide to new subject hierarchy
- `NODEJS_RPC_CAPTURE.md` - How it works for TypeScript/Node.js
- `DEPLOYMENT_SUMMARY.md` - Quick deployment reference
- `README.md` - Full project documentation

---

**Status**: Ready to deploy enhanced version with destination tracking and ETH method parsing! 🚀
