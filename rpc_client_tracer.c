//go:build ignore

#include <linux/bpf.h>
#include <bpf/bpf_helpers.h>
#include <linux/ptrace.h>

// Trace outgoing HTTP/HTTPS requests from client applications
// This captures TCP sendmsg to track RPC requests

#define TASK_COMM_LEN 16
#define MAX_DATA_SIZE 256

struct rpc_client_event_t {
    __u64 pid;
    __u64 timestamp_ns;
    __u32 data_len;
    char comm[TASK_COMM_LEN];
    char data[MAX_DATA_SIZE];  // Partial payload for JSON-RPC method detection
};

// Perf buffer to send events to userspace
struct {
    __uint(type, BPF_MAP_TYPE_PERF_EVENT_ARRAY);
    __uint(key_size, sizeof(__u32));
    __uint(value_size, sizeof(__u32));
} client_events SEC(".maps");

// Kprobe on tcp_sendmsg to capture outgoing TCP traffic
SEC("kprobe/tcp_sendmsg")
int trace_tcp_sendmsg(struct pt_regs *ctx) {
    struct rpc_client_event_t event = {};
    
    // Get PID and timestamp
    __u64 id = bpf_get_current_pid_tgid();
    event.pid = id >> 32;
    event.timestamp_ns = bpf_ktime_get_ns();
    
    // Get process name
    bpf_get_current_comm(&event.comm, sizeof(event.comm));
    
    // Get the iov structure (contains the data being sent)
    // struct msghdr *msg = (struct msghdr *)PT_REGS_PARM2(ctx);
    // For now, we'll just track that data was sent
    event.data_len = 0; // Would need to read from msghdr
    
    // Send event to userspace
    bpf_perf_event_output(ctx, &client_events, BPF_F_CURRENT_CPU, &event, sizeof(event));
    
    return 0;
}

// Kprobe on tcp_recvmsg to capture incoming responses
SEC("kprobe/tcp_recvmsg")
int trace_tcp_recvmsg(struct pt_regs *ctx) {
    struct rpc_client_event_t event = {};
    
    __u64 id = bpf_get_current_pid_tgid();
    event.pid = id >> 32;
    event.timestamp_ns = bpf_ktime_get_ns();
    
    bpf_get_current_comm(&event.comm, sizeof(event.comm));
    event.data_len = 0;
    
    bpf_perf_event_output(ctx, &client_events, BPF_F_CURRENT_CPU, &event, sizeof(event));
    
    return 0;
}

char LICENSE[] SEC("license") = "GPL";
