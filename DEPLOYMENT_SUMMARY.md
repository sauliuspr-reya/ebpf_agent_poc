# ✅ eBPF Agent - Deployment Ready

## Image Built Successfully

**Image**: `gcr.io/testnet-473109/ebpf-agent:latest`  
**Architecture**: `linux/amd64`  
**Status**: ✅ Pushed to GCR

## Quick Deploy to Testnet

### Step 1: Verify Cluster Connection

```bash
# Switch to testnet
gcloud container clusters get-credentials gke-de-test \
    --zone=europe-west3 \
    --project=testnet-473109

# Verify NATS is running
kubectl get svc -n nats
```

### Step 2: Deploy the Agent

```bash
# Deploy everything (ConfigMap, DaemonSet, RBAC)
kubectl apply -f k8s-deployment.yaml

# Watch the rollout
kubectl rollout status daemonset/ebpf-agent -n nats
```

### Step 3: Verify Deployment

```bash
# Check pods
kubectl get pods -n nats -l app=ebpf-agent

# View logs
kubectl logs -f daemonset/ebpf-agent -n nats

# Check one pod in detail
kubectl describe pod -n nats -l app=ebpf-agent | head -50
```

## Configuration

Current configuration in `k8s-deployment.yaml`:

- **NATS URL**: `nats://nats.nats.svc.cluster.local:4222`
- **App ID**: `testnet-rpc-monitor`
- **Target Binary**: `/usr/local/bin/geth`
- **Target Symbol**: `github.com/ethereum/go-ethereum/rpc.(*Server).serveRequest`
- **Namespace**: `nats`

## Monitor NATS Messages

### Using Local NATS Context

```bash
# Switch to testnet NATS context
nats context select testnet

# Subscribe to all messages
nats sub ">"

# Subscribe to specific app messages
nats sub "testnet-rpc-monitor.>"

# Subscribe to specific feature
nats sub "*.jsonrpc.response-size"
```

### From Inside the Cluster

```bash
# Get a NATS pod
NATS_POD=$(kubectl get pod -n nats -l app=nats -o jsonpath='{.items[0].metadata.name}')

# Subscribe to messages
kubectl exec -it $NATS_POD -n nats -- nats sub ">"
```

## Expected Behavior

The agent will:

1. ✅ Start on each node (DaemonSet)
2. ✅ Connect to NATS service
3. ✅ Load eBPF programs
4. ⚠️  Attempt to attach to `/usr/local/bin/geth` (may not exist)
5. ✅ Publish mock events to NATS (for testing)

**Note**: The agent currently uses mock data. To capture real traffic, you need to:
- Identify the actual RPC service binary on your nodes
- Update `TARGET_BINARY` and `TARGET_SYMBOL` in the ConfigMap
- Restart the DaemonSet

## Troubleshooting

### Pods Not Starting

```bash
kubectl describe pod -n nats -l app=ebpf-agent
```

Common issues:
- **ImagePullBackOff**: Check GCR permissions
- **CrashLoopBackOff**: Check logs for errors

### eBPF Load Failures

```bash
# Check kernel version (needs 4.18+)
kubectl get nodes -o wide

# Debug on a specific node
kubectl debug node/<node-name> -it --image=ubuntu
```

### NATS Connection Issues

```bash
# Test NATS connectivity
kubectl run -it --rm debug --image=alpine --restart=Never -n nats -- sh
apk add curl
curl -v nats.nats.svc.cluster.local:4222
```

## Update Configuration

To change target application:

```bash
# Edit ConfigMap
kubectl edit configmap ebpf-agent-config -n nats

# Restart pods to pick up changes
kubectl rollout restart daemonset/ebpf-agent -n nats
```

## Clean Up

```bash
# Delete the deployment
kubectl delete -f k8s-deployment.yaml

# Or just the daemonset
kubectl delete daemonset ebpf-agent -n nats
```

## Next Steps

1. **Verify Deployment**: Check that pods are running
2. **Monitor Logs**: Watch for eBPF loading and NATS connection
3. **Subscribe to NATS**: Verify mock events are being published
4. **Identify Target**: Find the actual RPC service to monitor
5. **Update Config**: Set correct TARGET_BINARY and TARGET_SYMBOL
6. **Implement Real Data Capture**: Phase 2 of the PoC (see POC_PLAN.md)

## Quick Commands

```bash
# Deploy
kubectl apply -f k8s-deployment.yaml

# Status
kubectl get all -n nats -l app=ebpf-agent

# Logs
kubectl logs -f daemonset/ebpf-agent -n nats

# NATS subscription
nats context select testnet && nats sub ">"

# Delete
kubectl delete -f k8s-deployment.yaml
```

---

**Ready to Deploy!** 🚀

Run: `kubectl apply -f k8s-deployment.yaml`
