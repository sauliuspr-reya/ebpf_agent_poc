# 🚀 Deploy to Reya Testnet

Quick guide to deploy the eBPF Agent to your testnet GKE cluster.

## Prerequisites

- Docker installed and running
- `kubectl` configured for testnet cluster
- `gcloud` CLI authenticated
- Access to `gcr.io/testnet-473109` registry

## Testnet Configuration

- **Cluster**: `gke_testnet-473109_europe-west3_gke-de-test`
- **NATS Service**: `nats.nats.svc.cluster.local:4222`
- **Namespace**: `nats`
- **Registry**: `gcr.io/testnet-473109`

## Quick Deploy (All-in-One)

```bash
# Build, push, and deploy in one command
./scripts/deploy_to_testnet.sh
```

## Step-by-Step Deployment

### Step 1: Build and Push Image

```bash
# Build for amd64 and push to GCR
./scripts/build_and_push.sh

# Or manually:
make docker-build
docker tag ebpf-agent:latest gcr.io/testnet-473109/ebpf-agent:latest
docker push gcr.io/testnet-473109/ebpf-agent:latest
```

### Step 2: Switch to Testnet Context

```bash
# Switch kubectl context
gcloud container clusters get-credentials gke-de-test \
    --zone=europe-west3 \
    --project=testnet-473109

# Verify NATS service is running
kubectl get svc -n nats
```

### Step 3: Deploy to Kubernetes

```bash
# Apply the deployment
kubectl apply -f k8s-deployment.yaml

# Watch the rollout
kubectl rollout status daemonset/ebpf-agent -n nats

# Check pods
kubectl get pods -n nats -l app=ebpf-agent
```

## Verify Deployment

### Check Agent Status

```bash
# View daemonset
kubectl get daemonset ebpf-agent -n nats

# View pods on all nodes
kubectl get pods -n nats -l app=ebpf-agent -o wide

# View logs from agent
kubectl logs -f daemonset/ebpf-agent -n nats
```

### Test NATS Connection

```bash
# Subscribe to all messages (from a pod with nats CLI)
kubectl exec -it <nats-pod> -n nats -- nats sub ">"

# Or use your local NATS context
nats context select testnet
nats sub "testnet-rpc-monitor.>"
```

### Expected Output

The agent should:
1. ✅ Load eBPF programs successfully
2. ✅ Connect to NATS at `nats.nats.svc.cluster.local:4222`
3. ✅ Start publishing mock events to NATS

Sample log output:
```
Starting JSON-RPC eBPF Agent for Arbitrum traffic monitoring...
Configuration:
  NATS URL: nats://nats.nats.svc.cluster.local:4222
  App ID: testnet-rpc-monitor
Connecting to NATS...
Successfully connected to NATS.
Attaching Uprobe to symbol...
Uprobe attached successfully.
Starting Perf Buffer reader...
Published RPC Event: 0xDEADBEEF, Size: 1536 bytes
```

## Update Deployment

### Update Image

```bash
# Build new version
VERSION=v1.0.1 ./scripts/build_and_push.sh

# Update deployment
kubectl set image daemonset/ebpf-agent \
    ebpf-agent=gcr.io/testnet-473109/ebpf-agent:v1.0.1 \
    -n nats

# Or re-apply the manifest
kubectl apply -f k8s-deployment.yaml
```

### Update Configuration

```bash
# Edit ConfigMap
kubectl edit configmap ebpf-agent-config -n nats

# Or update and re-apply
kubectl apply -f k8s-deployment.yaml

# Restart pods to pick up config changes
kubectl rollout restart daemonset/ebpf-agent -n nats
```

## Troubleshooting

### Pod Not Starting

```bash
# Check pod status
kubectl describe pod <pod-name> -n nats

# Common issues:
# 1. Image pull errors - check GCR permissions
# 2. Security context - ensure privileged mode is allowed
# 3. RBAC - check ServiceAccount has correct permissions
```

### eBPF Load Failures

```bash
# Check kernel version (needs 4.18+)
kubectl debug node/<node-name> -it --image=ubuntu
uname -r

# Check BPF filesystem is mounted
ls /sys/fs/bpf

# Check kernel config
zgrep CONFIG_BPF /proc/config.gz
```

### NATS Connection Issues

```bash
# Test NATS service from a pod
kubectl run -it --rm debug --image=alpine --restart=Never -- sh
apk add curl
curl -v nats.nats.svc.cluster.local:4222

# Check NATS service
kubectl get svc nats -n nats
kubectl get endpoints nats -n nats
```

## Clean Up

```bash
# Delete the daemonset
kubectl delete daemonset ebpf-agent -n nats

# Delete RBAC resources
kubectl delete clusterrolebinding ebpf-agent
kubectl delete clusterrole ebpf-agent
kubectl delete serviceaccount ebpf-agent -n nats

# Or delete everything
kubectl delete -f k8s-deployment.yaml
```

## Environment Variables

Override defaults via ConfigMap:

| Variable | Default | Description |
|----------|---------|-------------|
| `NATS_URL` | `nats://nats.nats.svc.cluster.local:4222` | NATS server URL |
| `APP_ID` | `testnet-rpc-monitor` | Application identifier |
| `TARGET_BINARY` | `/usr/local/bin/geth` | Binary to trace |
| `TARGET_SYMBOL` | `github.com/.../rpc.(*Server).serveRequest` | Function to probe |
| `TARGET_PID` | `0` | Process ID (0 = all) |

## Next Steps

1. **Identify Target Process**: Find the actual RPC service to monitor
2. **Discover Symbols**: Use `scripts/find_symbols.sh` on target binary
3. **Update Config**: Set correct `TARGET_BINARY` and `TARGET_SYMBOL`
4. **Monitor NATS**: Set up subscribers for the published events
5. **Real Data Capture**: Implement actual data extraction (Phase 2)

## Quick Commands Reference

```bash
# Build and deploy
./scripts/deploy_to_testnet.sh

# View logs
kubectl logs -f daemonset/ebpf-agent -n nats

# Get status
kubectl get all -n nats -l app=ebpf-agent

# Subscribe to NATS
nats context select testnet
nats sub "testnet-rpc-monitor.>"

# Delete
kubectl delete -f k8s-deployment.yaml
```

## Architecture in Testnet

```
┌─────────────────────────────────────────┐
│         GKE Testnet Cluster             │
│                                         │
│  ┌─────────────────────────────────┐   │
│  │    NATS (nats namespace)        │   │
│  │  nats.nats.svc.cluster.local    │   │
│  │         Port: 4222               │   │
│  └─────────────▲───────────────────┘   │
│                │                         │
│                │ Publishes events        │
│                │                         │
│  ┌─────────────┴───────────────────┐   │
│  │   eBPF Agent (DaemonSet)        │   │
│  │   - Runs on each node           │   │
│  │   - Attaches to target process  │   │
│  │   - Captures RPC traffic        │   │
│  └─────────────────────────────────┘   │
│                                         │
└─────────────────────────────────────────┘
```
