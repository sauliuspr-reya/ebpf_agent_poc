//go:build ignore

#include <linux/bpf.h>
#include <linux/ptrace.h>
#include <bpf/bpf_helpers.h>
#include <bpf/bpf_tracing.h>
#include <linux/types.h>
#include <linux/socket.h>
#include <linux/in.h>
#include <linux/tcp.h>

// Enhanced network tracer with destination IP and payload capture

#define TASK_COMM_LEN 16
#define MAX_DATA_SIZE 512  // Increased to capture full JSON-RPC requests

struct network_event_t {
    __u64 pid;
    __u64 timestamp_ns;
    __u32 data_len;
    __u32 is_send;
    __u32 dest_ip;    // Destination IPv4 address
    __u16 dest_port;  // Destination port
    char comm[TASK_COMM_LEN];
    char data[MAX_DATA_SIZE];  // HTTP headers + JSON-RPC payload
};

// Perf buffer for events
struct {
    __uint(type, BPF_MAP_TYPE_PERF_EVENT_ARRAY);
    __uint(key_size, sizeof(__u32));
    __uint(value_size, sizeof(__u32));
} events SEC(".maps");

// Helper to extract destination address from socket
static __always_inline int get_sock_info(struct sock *sk, __u32 *dest_ip, __u16 *dest_port) {
    __u16 family;
    
    // Read socket family
    bpf_probe_read_kernel(&family, sizeof(family), &sk->__sk_common.skc_family);
    
    if (family == AF_INET) {
        // IPv4
        bpf_probe_read_kernel(dest_ip, sizeof(*dest_ip), &sk->__sk_common.skc_daddr);
        bpf_probe_read_kernel(dest_port, sizeof(*dest_port), &sk->__sk_common.skc_dport);
        // Convert from network byte order
        *dest_port = __builtin_bswap16(*dest_port);
        return 0;
    }
    
    return -1;
}

// Kprobe on tcp_sendmsg
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
    
    // Filter: only "node" processes
    char node_name[] = "node";
    char comm[TASK_COMM_LEN];
    bpf_get_current_comm(&comm, sizeof(comm));
    
    int is_node = 1;
    for (int i = 0; i < 4; i++) {
        if (comm[i] != node_name[i]) {
            is_node = 0;
            break;
        }
    }
    
    if (!is_node) {
        return 0;
    }
    
    // tcp_sendmsg(struct sock *sk, struct msghdr *msg, size_t size)
    struct sock *sk = (struct sock *)PT_REGS_PARM1(ctx);
    struct msghdr *msg = (struct msghdr *)PT_REGS_PARM2(ctx);
    __u32 size = (__u32)PT_REGS_PARM3(ctx);
    
    if (size == 0 || size > 65536) {
        return 0;
    }
    
    event.data_len = size;
    
    // Extract destination IP and port
    if (get_sock_info(sk, &event.dest_ip, &event.dest_port) != 0) {
        // Not IPv4, skip for now
        return 0;
    }
    
    // Filter by destination port (443 for HTTPS RPC calls)
    // You can adjust this to capture specific ports
    if (event.dest_port != 443 && event.dest_port != 8545 && event.dest_port != 8547) {
        return 0;  // Only capture HTTPS (443) or common RPC ports
    }
    
    // Try to read data from msghdr
    // This is complex - msghdr contains iov_iter with multiple iovecs
    // For now, we'll try to read the first iovec
    struct iovec iov;
    struct iov_iter *iter;
    
    // Read iter from msghdr
    bpf_probe_read_kernel(&iter, sizeof(iter), &msg->msg_iter);
    
    // Try to read first iovec (simplified - production code needs more robust handling)
    void *iov_base;
    unsigned long iov_len;
    
    // This is a simplified approach - real implementation would need to handle
    // different iter types (ITER_IOVEC, ITER_KVEC, etc.)
    // For HTTP/HTTPS traffic, we usually get ITER_IOVEC
    
    // Read up to MAX_DATA_SIZE bytes
    __u32 read_size = size < MAX_DATA_SIZE ? size : MAX_DATA_SIZE;
    if (read_size > 0 && read_size <= MAX_DATA_SIZE) {
        // BPF verifier needs bounds check
        read_size = read_size & (MAX_DATA_SIZE - 1);
        if (read_size > 0) {
            // Note: This is a placeholder - actual payload reading from msghdr
            // requires more complex logic to traverse iov_iter
            // For now, we just send metadata
        }
    }
    
    // Send event
    bpf_perf_event_output(ctx, &events, BPF_F_CURRENT_CPU, &event, sizeof(event));
    
    return 0;
}

char LICENSE[] SEC("license") = "GPL";
