//go:build ignore

#include <linux/bpf.h>
#include <bpf/bpf_helpers.h>

// This file defines the BPF program for tracing JSON-RPC calls (Arbitrum/Ethereum nodes).
#define TASK_COMM_LEN 16
#define METHOD_NAME_MAX 128

// 1. Raw data structure sent from kernel space to user space (Go Agent)
// This MUST match the RPCEvent struct in agent_main.go exactly.
struct rpc_event_t {
    __u64 pid;
    __u64 timestamp_ns;
    __u64 response_size;
    char method_name[METHOD_NAME_MAX];
    char comm[TASK_COMM_LEN];
};

// 2. Define the Perf Buffer map to send data to userspace
struct {
    __uint(type, BPF_MAP_TYPE_PERF_EVENT_ARRAY);
    __uint(key_size, sizeof(__u32));
    __uint(value_size, sizeof(__u32));
} events SEC(".maps");

// 3. Uprobe attached to the return of a JSON-RPC handler function
// This can be attached to various symbols depending on the target:
// - Arbitrum/geth: github.com/ethereum/go-ethereum/rpc.(*Server).serveRequest
// - Generic HTTP handlers: net/http.(*conn).serve
SEC("uprobe/rpc_handle_ret")
int uprobe_rpc_handle_ret(struct pt_regs *ctx) {
    struct rpc_event_t event = {};
    
    // Get PID and current time
    __u64 id = bpf_get_current_pid_tgid();
    event.pid = id >> 32; // Extract PID
    event.timestamp_ns = bpf_ktime_get_ns();
    
    // Get process name
    bpf_get_current_comm(&event.comm, sizeof(event.comm));

    // --- MOCK DATA FOR INITIAL PoC ---
    // In production, this would use bpf_probe_read_user() to extract actual data
    // from the Go RPC handler's stack/registers based on the function signature
    
    // Mock method name (common Arbitrum/Ethereum JSON-RPC methods)
    __builtin_memcpy(event.method_name, "eth_getBlockByNumber", 20);
    event.response_size = 1536; // Mock response size in bytes
    
    // Submit the event to the user-space agent via the Perf Buffer
    bpf_perf_event_output(ctx, &events, BPF_F_CURRENT_CPU, &event, sizeof(event));
    
    return 0;
}

char LICENSE[] SEC("license") = "GPL";
