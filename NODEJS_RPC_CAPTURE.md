# Capturing JSON-RPC Traffic from Node.js/TypeScript Apps

## Your Use Case

**Oracle Updater** (TypeScript + ethers.js) → **HTTPS** → `rpc.reya-cronos.gelato.digital`

## Why eBPF is Perfect Here

✅ **No code changes** - Zero instrumentation needed  
✅ **Language agnostic** - Works with TypeScript, Python, Go, any language  
✅ **Captures encrypted traffic** - Intercepts BEFORE SSL encryption  
✅ **Low overhead** - Kernel-level tracing  

## How It Works

```
┌─────────────────────────────────────────────┐
│ TypeScript App (cronos-oracle-offchain)    │
│                                             │
│  ethers.js.JsonRpcProvider                 │
│      ↓                                      │
│  fetch() / axios / http client             │
│      ↓                                      │
│  Node.js built-in https module             │
│      ↓                                      │
│  OpenSSL (libssl.so)                       │ ← eBPF UPROBE ATTACHES HERE
│      ↓ SSL_write(buf, len)                │ ← Captures BEFORE encryption
│      ↓                                      │
└─────┼───────────────────────────────────────┘
      │ Encrypted HTTPS
      ▼
  rpc.reya-cronos.gelato.digital
```

## What We Capture

### Request (SSL_write)
```json
{
  "jsonrpc": "2.0",
  "method": "eth_call",
  "params": [...],
  "id": 1
}
```

### Response (SSL_read)
```json
{
  "jsonrpc": "2.0",
  "result": "0x...",
  "id": 1
}
```

## Current Configuration

The ConfigMap is now set to:

```yaml
TARGET_BINARY: "/usr/lib/x86_64-linux-gnu/libssl.so.3"
TARGET_SYMBOL: "SSL_write"
TARGET_PID: "0"  # Captures from ALL processes
```

## Deployment Steps

### Step 1: Apply Updated Config

```bash
kubectl apply -f k8s-deployment.yaml
kubectl rollout restart daemonset/ebpf-agent -n nats
```

### Step 2: Verify It's Running

```bash
# Check logs
kubectl logs -f daemonset/ebpf-agent -n nats

# You should see:
# ✓ Successfully connected to NATS
# ✓ Attaching Uprobe to symbol 'SSL_write' in binary '/usr/lib/x86_64-linux-gnu/libssl.so.3'
# ✓ Uprobe attached successfully
```

### Step 3: Test - Trigger Some Oracle Updates

Your oracle should naturally make RPC calls. When it does:

```bash
# Subscribe to NATS
nats context select testnet
nats sub "testnet-rpc-monitor.>"

# You should see events like:
{
  "app_id": "testnet-rpc-monitor",
  "protocol": "jsonrpc",
  "feature_type": "response-size",
  "value": 1024,
  "details": {
    "pid": 1234,
    "process": "node",
    "method": "eth_call"  # Extracted from JSON-RPC payload
  }
}
```

## Important Notes

### Process Filtering

Currently captures from **all processes** using OpenSSL. To filter only your oracle:

**Option A**: Filter in the agent code (check `comm` field)
**Option B**: Use `TARGET_PID` with oracle's specific PID

### SSL Library Path

The path `/usr/lib/x86_64-linux-gnu/libssl.so.3` works for:
- Ubuntu 22.04+
- Debian 11+
- Most modern containers

If your oracle container uses a different base image, find the correct path:

```bash
kubectl exec -it cronos-oracle-offchain-xxx -n <namespace> -- \
  find /usr -name "libssl.so*"
```

Common alternatives:
- `/usr/lib/libssl.so.3`
- `/lib/x86_64-linux-gnu/libssl.so.3`
- `/usr/local/lib/libssl.so.3`

### Payload Parsing

The current eBPF program captures up to 512 bytes of the SSL payload. This is enough to extract:
- JSON-RPC method name
- Request/response size
- Timestamp

For full payload capture, increase `MAX_PAYLOAD_SIZE` in the C code.

## Troubleshooting

### "open libssl.so.3: no such file or directory"

The SSL library path is wrong. Find the correct one:

```bash
# Check what SSL library Node.js uses
kubectl exec -it <oracle-pod> -- ldd $(which node) | grep ssl

# Or find all SSL libraries
kubectl exec -it <oracle-pod> -- find / -name "libssl.so*" 2>/dev/null
```

### No Events Being Captured

1. **Verify oracle is making requests:**
   ```bash
   kubectl logs <oracle-pod> -n <namespace>
   ```

2. **Check eBPF agent logs:**
   ```bash
   kubectl logs daemonset/ebpf-agent -n nats
   ```

3. **Verify uprobe attached:**
   ```bash
   kubectl exec -it <agent-pod> -n nats -- cat /sys/kernel/debug/tracing/uprobe_events
   ```

### Capturing Specific Processes Only

Update the ConfigMap with the oracle's PID:

```bash
# Find the PID
kubectl exec -it <oracle-pod> -- ps aux | grep node

# Update config
kubectl edit configmap ebpf-agent-config -n nats
# Set: TARGET_PID: "1234"  # Replace with actual PID

kubectl rollout restart daemonset/ebpf-agent -n nats
```

## Next Steps: Real Data Extraction

The current implementation uses **mock data**. To capture real JSON-RPC methods:

### Phase 2: Parse JSON Payload

1. Extract `method` field from JSON in SSL_write buffer
2. Parse ethers.js request structure
3. Track request/response pairs (correlate by ID)

See `POC_PLAN.md` Phase 2 for details.

### What You'll Capture

- **Method counts**: How many `eth_call`, `eth_sendTransaction`, etc.
- **Latency**: Time between SSL_write (request) and SSL_read (response)
- **Error rates**: Failed requests, timeouts
- **Data volume**: Bytes sent/received per method
- **Traffic patterns**: Peak times, unusual spikes

## Example NATS Messages

### Request Event
```json
{
  "app_id": "testnet-rpc-monitor",
  "protocol": "jsonrpc",
  "feature_type": "request",
  "timestamp": "2024-10-23T12:00:00Z",
  "value": 512,
  "context_hash": "method:eth_call",
  "details": {
    "pid": 1234,
    "process": "node",
    "direction": "outgoing",
    "method": "eth_call",
    "size_bytes": 512
  }
}
```

### Response Event
```json
{
  "app_id": "testnet-rpc-monitor",
  "protocol": "jsonrpc",
  "feature_type": "response-size",
  "timestamp": "2024-10-23T12:00:01Z",
  "value": 2048,
  "context_hash": "method:eth_call",
  "details": {
    "pid": 1234,
    "process": "node",
    "direction": "incoming",
    "latency_ms": 150,
    "size_bytes": 2048
  }
}
```

## Architecture Diagram

```
┌──────────────────────────────────────────────────────┐
│ GKE Testnet Cluster                                  │
│                                                      │
│ ┌────────────────────────────────────────────────┐ │
│ │ cronos-oracle-offchain Pod                     │ │
│ │                                                │ │
│ │  ┌──────────────────────────────────────┐    │ │
│ │  │ TypeScript App                       │    │ │
│ │  │ ethers.js → HTTPS → libssl.so        │    │ │
│ │  └──────────────┬───────────────────────┘    │ │
│ │                 │                              │ │
│ │                 │ SSL_write/SSL_read           │ │
│ │                 ▼                              │ │
│ │  ┌──────────────────────────────────────┐    │ │
│ │  │ eBPF Uprobe (on host)                │    │ │
│ │  │ Captures payload BEFORE encryption   │    │ │
│ │  └──────────────┬───────────────────────┘    │ │
│ └─────────────────┼────────────────────────────┘ │
│                   │                                │
│                   ▼ Events                         │
│ ┌──────────────────────────────────────────────┐ │
│ │ eBPF Agent (DaemonSet)                       │ │
│ │ - Parses JSON-RPC                            │ │
│ │ - Extracts method, size, latency             │ │
│ │ - Publishes to NATS                          │ │
│ └──────────────┬───────────────────────────────┘ │
│                │                                  │
│                ▼                                  │
│ ┌──────────────────────────────────────────────┐ │
│ │ NATS Service                                 │ │
│ └──────────────────────────────────────────────┘ │
│                                                  │
└──────────────────────────────────────────────────┘
                 │
                 ▼ Subscribers
         Monitoring/Analytics
```

---

**Ready to capture your RPC traffic without touching your code!** 🚀
