# NATS Subject Structure for RPC Monitoring

## Hierarchical Subject Format

```
rpc.{destination}.{protocol}.{method}.{metric}
```

### Components

1. **destination**: DNS hostname or IP (with hyphens replacing dots)
   - Example: `rpc-reya-cronos-gelato-digital`
   - Example: `34-18-237-112` (if DNS fails)

2. **protocol**: Transport protocol
   - `https` - Port 443
   - `http` - Port 8545, 8547, or other HTTP ports

3. **method**: ETH JSON-RPC method name
   - `eth_call`
   - `eth_sendTransaction`
   - `eth_getBalance`
   - `eth_blockNumber`
   - etc. (or `unknown` if not parsed)

4. **metric**: Measurement type
   - `request_size` - Bytes sent (outgoing)
   - `response_size` - Bytes received (incoming)
   - `latency_ms` - Request/response time (future)

## Example Subjects

```
rpc.rpc-reya-cronos-gelato-digital.https.eth_call.request_size
rpc.rpc-reya-cronos-gelato-digital.https.eth_call.response_size
rpc.rpc-reya-cronos-gelato-digital.https.eth_sendTransaction.request_size
rpc.rpc-reya-cronos-gelato-digital.https.eth_getBalance.request_size
rpc.rpc-reya-cronos-gelato-digital.https.unknown.request_size
```

## NATS Subscription Patterns

### Subscribe to All RPC Traffic
```bash
nats sub "rpc.>"
```

### Subscribe to Specific Destination
```bash
# All traffic to rpc.reya-cronos.gelato.digital
nats sub "rpc.rpc-reya-cronos-gelato-digital.>"
```

### Subscribe to Specific Method
```bash
# All eth_call requests across all destinations
nats sub "rpc.*.*.eth_call.>"

# eth_call to specific destination
nats sub "rpc.rpc-reya-cronos-gelato-digital.*.eth_call.>"
```

### Subscribe to Specific Metric
```bash
# All request sizes
nats sub "rpc.*.*.*.request_size"

# All response sizes from specific destination
nats sub "rpc.rpc-reya-cronos-gelato-digital.*.*.response_size"
```

### Subscribe to HTTPS vs HTTP
```bash
# Only HTTPS traffic
nats sub "rpc.*.https.>.>"

# Only HTTP traffic
nats sub "rpc.*.http.>.>"
```

## Message Payload Structure

```json
{
  "app_id": "testnet-rpc-monitor",
  "protocol": "jsonrpc",
  "feature_type": "request_size",
  "timestamp": "2025-10-23T12:40:00Z",
  "value": 342,
  "context_hash": "rpc.rpc-reya-cronos-gelato-digital.https.eth_call.request_size",
  "details": {
    "pid": 279776,
    "process": "node",
    "method": "eth_call",
    "direction": "send",
    "size_bytes": 342,
    "timestamp_ns": 604493271156577,
    "dest_ip": "34.185.237.112",
    "dest_port": 443,
    "dest_hostname": "rpc-reya-cronos-gelato-digital"
  }
}
```

## Analytics Use Cases

### 1. Traffic Volume by Destination
```bash
# Stream all RPC traffic and group by destination
nats sub "rpc.>" --raw | jq -r '.details.dest_hostname' | sort | uniq -c
```

### 2. Most Common ETH Methods
```bash
# Extract methods from all traffic
nats sub "rpc.>.>.>" --raw | jq -r '.details.method' | sort | uniq -c | sort -rn
```

### 3. Request Size Distribution
```bash
# Get all request sizes for eth_call
nats sub "rpc.*.*.eth_call.request_size" --raw | jq '.value'
```

### 4. Monitor Specific RPC Endpoint
```bash
# Watch all traffic to production RPC
nats sub "rpc.rpc-reya-cronos-gelato-digital.>"
```

### 5. Detect Large Responses
```bash
# Alert on responses > 50KB
nats sub "rpc.*.*.*.response_size" --raw | \
  jq 'select(.value > 51200) | {method: .details.method, size: .value}'
```

## NATS Stream Configuration

### Create a Stream for Historical Data
```bash
nats stream add RPC_TRAFFIC \
  --subjects "rpc.>" \
  --storage file \
  --retention limits \
  --max-msgs=-1 \
  --max-bytes=10G \
  --max-age=7d \
  --replicas=1
```

### Create Filtered Streams

**High-Value Methods Stream:**
```bash
nats stream add RPC_WRITE_OPS \
  --subjects "rpc.*.*.eth_sendTransaction.>" \
  --subjects "rpc.*.*.eth_sendRawTransaction.>" \
  --storage file \
  --retention limits \
  --max-age=30d
```

**Read-Only Queries Stream:**
```bash
nats stream add RPC_READ_OPS \
  --subjects "rpc.*.*.eth_call.>" \
  --subjects "rpc.*.*.eth_getBalance.>" \
  --subjects "rpc.*.*.eth_getBlockByNumber.>" \
  --storage file \
  --retention limits \
  --max-age=1d
```

## Consumer Patterns

### Real-Time Monitoring Consumer
```bash
nats consumer add RPC_TRAFFIC realtime_monitor \
  --filter "rpc.>" \
  --deliver all \
  --ack none \
  --replay instant
```

### Historical Analysis Consumer
```bash
nats consumer add RPC_TRAFFIC analytics \
  --filter "rpc.>" \
  --deliver all \
  --ack explicit \
  --replay original \
  --max-deliver 3
```

## Integration Examples

### Prometheus Metrics
Subscribe to NATS and export as Prometheus metrics:
```go
// Subscribe to all eth_call requests
nc.Subscribe("rpc.*.*.eth_call.request_size", func(msg *nats.Msg) {
    var event MonitoringFeature
    json.Unmarshal(msg.Data, &event)
    
    // Export to Prometheus
    rpcRequestSize.WithLabelValues(
        event.Details["dest_hostname"],
        event.Details["method"],
    ).Observe(event.Value)
})
```

### Grafana Dashboard Queries
- Request rate: `rate(rpc.*.*.*.request_size[5m])`
- P95 response size: `histogram_quantile(0.95, rpc.*.*.*.response_size)`
- Error rate by method: `rate(rpc.*.*.*.error[5m]) by (method)`

## Benefits of This Structure

1. **Hierarchical**: Easy to filter at any level
2. **Scalable**: Can add new metrics without changing structure
3. **Discoverable**: Subject structure is self-documenting
4. **Flexible**: Wildcards allow powerful filtering
5. **Efficient**: NATS handles routing based on subjects
6. **Standards-Based**: Follows NATS best practices

## Future Enhancements

Potential additions to the structure:
- Add environment: `rpc.{env}.{destination}.{protocol}.{method}.{metric}`
- Add service name: `rpc.{service}.{destination}.{protocol}.{method}.{metric}`
- Add region: `rpc.{region}.{destination}.{protocol}.{method}.{metric}`

Example:
```
rpc.prod.oracle-offchain.rpc-reya-cronos.https.eth_call.request_size
```
