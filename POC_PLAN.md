# 🚀 PoC Implementation Plan: JSON-RPC eBPF Agent for Arbitrum Traffic

## Objective
Create a working PoC that captures JSON-RPC traffic from Arbitrum/Ethereum nodes using eBPF and streams telemetry to NATS.io.

---

## Phase 1: Foundation (✅ COMPLETED)

### 1.1 Project Setup
- [x] Initialize Go module
- [x] Create project structure
- [x] Set up eBPF C program file
- [x] Configure build toolchain (Makefile)
- [x] Add dependencies (cilium/ebpf, nats.go)

### 1.2 Core eBPF Program
- [x] Define `rpc_event_t` struct matching Go
- [x] Create perf buffer map
- [x] Implement basic uprobe handler
- [x] Add mock data capture (for initial testing)
- [x] Test BPF program compilation

### 1.3 Go Agent
- [x] Implement NATS connection with retry logic
- [x] Load eBPF objects using cilium/ebpf
- [x] Attach uprobe to target binary
- [x] Set up perf buffer reader
- [x] Parse events from kernel space
- [x] Publish to NATS with proper subject structure

### 1.4 Build & Deployment
- [x] Create Makefile with all targets
- [x] Docker support (multi-stage build)
- [x] Kubernetes manifests (DaemonSet)
- [x] Configuration via environment variables

### 1.5 Documentation
- [x] Comprehensive README
- [x] Architecture overview
- [x] Quick start guide
- [x] Troubleshooting section

---

## Phase 2: Real Data Capture (🔄 NEXT)

### 2.1 Function Symbol Discovery
**Goal**: Identify the exact function to trace in geth/Arbitrum

**Tasks**:
- [ ] Analyze geth binary symbols
  ```bash
  nm -D /usr/local/bin/geth | grep -i rpc
  objdump -T /usr/local/bin/geth | grep serveRequest
  ```
- [ ] Test common symbols:
  - `github.com/ethereum/go-ethereum/rpc.(*Server).serveRequest`
  - `github.com/ethereum/go-ethereum/rpc.(*handler).handleCall`
  - `net/http.(*conn).serve` (fallback to HTTP layer)
- [ ] Document symbol discovery process

**Success Criteria**: Uprobe successfully attaches to running geth process

### 2.2 Extract Method Names
**Goal**: Replace mock data with actual JSON-RPC method names

**Tasks**:
- [ ] Study Go function ABI (Application Binary Interface)
- [ ] Implement `bpf_probe_read_user()` to read string data
- [ ] Extract method name from function parameters
- [ ] Handle Go string structure (ptr + len)
- [ ] Add validation and bounds checking

**Code Example**:
```c
// Read Go string from user space
char method_name[128] = {};
bpf_probe_read_user(&method_name, sizeof(method_name), 
                    (void *)PT_REGS_PARM1(ctx));
```

**Success Criteria**: Agent prints actual method names like "eth_getBlockByNumber"

### 2.3 Measure Response Size
**Goal**: Capture actual response payload sizes

**Tasks**:
- [ ] Add uretprobe to capture return values
- [ ] Read response buffer length
- [ ] Correlate request/response pairs
- [ ] Handle edge cases (errors, timeouts)

**Success Criteria**: Accurate byte counts for each RPC response

### 2.4 Extract Request Payloads (Optional)
**Goal**: Capture request parameters for detailed analysis

**Tasks**:
- [ ] Parse JSON-RPC request structure
- [ ] Extract parameters (block numbers, addresses, etc.)
- [ ] Implement sampling to avoid overhead
- [ ] Add filtering by method type

**Success Criteria**: Agent captures and publishes request details

---

## Phase 3: Enhanced Features (📅 PLANNED)

### 3.1 Latency Measurement
**Goal**: Track request processing time

**Implementation**:
- [ ] Add entry uprobe (function start)
- [ ] Add exit uretprobe (function return)
- [ ] Store start timestamp in BPF map
- [ ] Calculate delta on exit
- [ ] Publish latency feature to NATS

**NATS Subject**: `{app_id}.jsonrpc.latency`

### 3.2 Error Tracking
**Goal**: Detect and report RPC errors

**Implementation**:
- [ ] Check return values for error codes
- [ ] Parse error messages
- [ ] Categorize error types (400, 500, timeout, etc.)
- [ ] Publish error features

**NATS Subject**: `{app_id}.jsonrpc.errors`

### 3.3 Traffic Pattern Analysis
**Goal**: Real-time traffic analytics

**Features**:
- [ ] Request rate per method
- [ ] Response size distribution
- [ ] Peak hour detection
- [ ] Anomaly detection (sudden spikes)

### 3.4 Multi-Protocol Support
**Goal**: Extend beyond JSON-RPC

**Protocols**:
- [ ] gRPC tracing
- [ ] GraphQL tracing
- [ ] WebSocket connections
- [ ] REST API calls

---

## Phase 4: Production Hardening (🔮 FUTURE)

### 4.1 Performance Optimization
- [ ] Implement event batching before NATS publish
- [ ] Add local aggregation (reduce NATS traffic)
- [ ] Adaptive sampling based on traffic volume
- [ ] Memory pool for event buffers
- [ ] Benchmark and profile agent

**Target**: < 1% CPU overhead, < 50MB memory

### 4.2 Reliability & Resilience
- [ ] NATS reconnection logic
- [ ] Event buffer for temporary disconnections
- [ ] Graceful degradation on high load
- [ ] Health check endpoints
- [ ] Prometheus metrics export

### 4.3 Security
- [ ] NATS TLS/authentication
- [ ] Sensitive data filtering (API keys, passwords)
- [ ] RBAC for Kubernetes deployment
- [ ] Security audit (OWASP, CIS benchmarks)
- [ ] Non-root container support (where possible)

### 4.4 Observability
- [ ] Structured logging (JSON)
- [ ] OpenTelemetry integration
- [ ] Distributed tracing
- [ ] Debug mode with verbose output
- [ ] Performance dashboard (Grafana)

### 4.5 Testing
- [ ] Unit tests for Go agent
- [ ] BPF program testing (bpftool)
- [ ] Integration tests with mock geth
- [ ] Load testing (1M+ events/sec)
- [ ] Chaos testing (network failures, OOM, etc.)

---

## Phase 5: Deployment & Operations (🚀 FUTURE)

### 5.1 CI/CD Pipeline
- [ ] GitHub Actions for build/test
- [ ] Container image scanning (Trivy)
- [ ] Automated versioning (semantic-release)
- [ ] Helm chart creation
- [ ] ArgoCD/Flux GitOps integration

### 5.2 Monitoring & Alerting
- [ ] Set up Prometheus exporters
- [ ] Create Grafana dashboards
- [ ] Define SLOs/SLIs
- [ ] Alert rules (PagerDuty, Slack)
- [ ] Runbook documentation

### 5.3 Scaling Strategy
- [ ] Horizontal scaling with DaemonSet
- [ ] Vertical scaling recommendations
- [ ] Resource limits tuning
- [ ] Network bandwidth considerations
- [ ] NATS clustering for high availability

---

## Testing Checklist

### Local Development
- [ ] Agent builds successfully
- [ ] eBPF program loads without errors
- [ ] Uprobe attaches to test binary
- [ ] NATS messages are published
- [ ] Graceful shutdown works

### Docker Environment
- [ ] Image builds successfully
- [ ] Container runs with --privileged
- [ ] Host PID namespace access works
- [ ] NATS connectivity from container
- [ ] Resource limits respected

### Kubernetes Deployment
- [ ] DaemonSet deploys to all nodes
- [ ] Pods reach Running state
- [ ] No permission errors in logs
- [ ] Messages appear in NATS
- [ ] Scaling works correctly

### Integration Testing
- [ ] Deploy sample geth node
- [ ] Send test JSON-RPC requests
- [ ] Verify events captured
- [ ] Check NATS message accuracy
- [ ] Measure overhead/performance

---

## Success Metrics

### PoC Phase 1 (Current)
- ✅ Agent compiles and runs
- ✅ eBPF program loads successfully
- ✅ Mock events published to NATS
- ✅ Docker/K8s deployment works

### PoC Phase 2 (Next Milestone)
- 🎯 Capture real JSON-RPC method names
- 🎯 Measure actual response sizes
- 🎯 99.9% uptime in 24h test
- 🎯 < 5% CPU overhead

### Production Ready (Final Goal)
- 🔮 Multi-protocol support
- 🔮 < 1% CPU overhead at scale
- 🔮 99.99% uptime SLA
- 🔮 Security audit passed
- 🔮 Full observability stack

---

## Timeline Estimate

| Phase | Duration | Status |
|-------|----------|--------|
| Phase 1: Foundation | 1 week | ✅ Complete |
| Phase 2: Real Data Capture | 1-2 weeks | 🔄 Next |
| Phase 3: Enhanced Features | 2-3 weeks | 📅 Planned |
| Phase 4: Production Hardening | 2-4 weeks | 🔮 Future |
| Phase 5: Deployment & Ops | 1-2 weeks | 🔮 Future |

**Total Estimated Time**: 7-12 weeks for production-ready system

---

## Risk Assessment

### Technical Risks
1. **Symbol Resolution**: Target function may not be present or may change
   - *Mitigation*: Maintain list of fallback symbols, version-specific configs

2. **eBPF Verifier Limits**: Complex programs may be rejected
   - *Mitigation*: Keep BPF programs simple, use maps for state

3. **Performance Overhead**: Tracing may impact node performance
   - *Mitigation*: Implement sampling, benchmarking, kill switch

4. **Kernel Compatibility**: Older kernels lack required BPF features
   - *Mitigation*: Document minimum kernel version, feature detection

### Operational Risks
1. **NATS Downtime**: Message loss during outages
   - *Mitigation*: Local buffering, persistent queues, retry logic

2. **High Cardinality**: Too many unique method names
   - *Mitigation*: Aggregation, sampling, rate limiting

3. **Security Vulnerabilities**: Privileged container exploits
   - *Mitigation*: Regular audits, minimal privileges, network policies

---

## Next Actions

### Immediate (This Week)
1. ✅ Complete Phase 1 foundation
2. 🔄 Test agent with mock data
3. 🔄 Deploy to local Docker environment
4. 🔄 Set up NATS subscriber for verification

### Short Term (Next 2 Weeks)
1. Identify correct geth symbol for tracing
2. Implement real method name extraction
3. Test with live Arbitrum node
4. Add latency measurement

### Medium Term (Next Month)
1. Add error tracking
2. Implement sampling/filtering
3. Performance benchmarking
4. Create Grafana dashboard

---

## Resources & References

- [Cilium eBPF Guide](https://ebpf-go.dev/)
- [BPF Performance Tools Book](http://www.brendangregg.com/bpf-performance-tools-book.html)
- [Go Function Call ABI](https://go.dev/doc/asm)
- [NATS Architecture](https://docs.nats.io/nats-concepts/overview)
- [Kubernetes Security Context](https://kubernetes.io/docs/tasks/configure-pod-container/security-context/)

---

**Last Updated**: 2024-10-23  
**Status**: Phase 1 Complete, Phase 2 In Progress
