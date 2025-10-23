//go:build ignore

#include <linux/bpf.h>
#include <linux/ptrace.h>
#include <bpf/bpf_helpers.h>
#include <bpf/bpf_tracing.h>
#include <linux/types.h>

// Kernel-level network traffic tracer
// Captures TCP traffic from ALL processes (including containers)

#define TASK_COMM_LEN 16
#define MAX_DATA_SIZE 256

struct network_event_t {
    __u64 pid;
    __u64 timestamp_ns;
    __u32 data_len;
    __u32 is_send;  // 1 = send (tcp_sendmsg), 0 = recv (tcp_recvmsg)
    char comm[TASK_COMM_LEN];
    char data[MAX_DATA_SIZE];  // First bytes of payload
};

// Perf buffer for events
struct {
    __uint(type, BPF_MAP_TYPE_PERF_EVENT_ARRAY);
    __uint(key_size, sizeof(__u32));
    __uint(value_size, sizeof(__u32));
} events SEC(".maps");

// Kprobe on tcp_sendmsg (outgoing TCP traffic)
SEC("kprobe/tcp_sendmsg")
int trace_tcp_sendmsg(struct pt_regs *ctx) {
    struct network_event_t event = {};
    
    // Get PID and timestamp
    __u64 id = bpf_get_current_pid_tgid();
    event.pid = id >> 32;
    event.timestamp_ns = bpf_ktime_get_ns();
    event.is_send = 1;
    
    // Get process name
    bpf_get_current_comm(&event.comm, sizeof(event.comm));
    
    // Filter: only capture from "node" processes (our oracle)
    char node_name[] = "node";
    char comm[TASK_COMM_LEN];
    bpf_get_current_comm(&comm, sizeof(comm));
    
    // Simple string comparison for "node"
    int is_node = 1;
    for (int i = 0; i < 4; i++) {
        if (comm[i] != node_name[i]) {
            is_node = 0;
            break;
        }
    }
    
    if (!is_node) {
        return 0;  // Skip non-node processes
    }
    
    // tcp_sendmsg(struct sock *sk, struct msghdr *msg, size_t size)
    // size is in PT_REGS_PARM3
    size_t size = (size_t)PT_REGS_PARM3(ctx);
    
    if (size <= 0 || size > 65536) {
        return 0;
    }
    
    event.data_len = size;
    
    // Try to read data from msghdr->msg_iter
    // This is complex, for now just capture metadata
    // TODO: Read actual payload from iovec structures
    
    // Send event
    bpf_perf_event_output(ctx, &events, BPF_F_CURRENT_CPU, &event, sizeof(event));
    
    return 0;
}

// Kretprobe on tcp_sendmsg to get actual bytes sent
SEC("kretprobe/tcp_sendmsg")
int trace_tcp_sendmsg_return(struct pt_regs *ctx) {
    // Return value is number of bytes sent
    int ret = (int)PT_REGS_RC(ctx);
    
    if (ret <= 0) {
        return 0;  // Error or no data
    }
    
    // We could track request/response pairs here
    // For now, the kprobe entry is enough
    
    return 0;
}

char LICENSE[] SEC("license") = "GPL";
