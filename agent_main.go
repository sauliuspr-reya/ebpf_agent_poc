package main

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"regexp"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/cilium/ebpf/link"
	"github.com/cilium/ebpf/perf"
	"github.com/cilium/ebpf/rlimit"
	"github.com/nats-io/nats.go"
)

// Configuration - can be overridden by environment variables
var (
	NatsURL         = getEnv("NATS_URL", "nats://localhost:4222")
	AppID           = getEnv("APP_ID", "arbitrum-node-service")
	TargetBinary    = getEnv("TARGET_BINARY", "/usr/local/bin/geth")
	TargetSymbolRet = getEnv("TARGET_SYMBOL", "github.com/ethereum/go-ethereum/rpc.(*Server).serveRequest")
	TargetPID       = getEnvInt("TARGET_PID", 0) // 0 means attach to all processes
	DebugMode       = getEnv("DEBUG", "false") == "true"
)

//go:generate go run github.com/cilium/ebpf/cmd/bpf2go -cc clang -target bpfel rpc rpc_tracer.c -- -D__TARGET_ARCH_x86 -I/usr/include/x86_64-linux-gnu

// Agent holds the core components for the tracing service.
type Agent struct {
	NatsConn     *nats.Conn
	Ctx          context.Context
	Cancel       context.CancelFunc
	EBPFObjs     *rpcObjects
	FeatureCache *sync.Map
	PerfReader   *perf.Reader
}

// RPCEvent represents data sent from the BPF program to the Go User-Space Agent.
// Must match the C struct (network_event_t) defined in rpc_tracer.c exactly.
type RPCEvent struct {
	PID         uint64
	TimestampNs uint64
	DataLen     uint32
	IsSend      uint32 // 1 = send (tcp_sendmsg), 0 = recv (tcp_recvmsg)
	DestIP      uint32 // Destination IPv4 address
	DestPort    uint16 // Destination port
	Comm        [16]byte
	Data        [512]byte // HTTP headers + JSON-RPC payload
}

// MonitoringFeature is the standard structure published to NATS.
type MonitoringFeature struct {
	AppID       string                 `json:"app_id"`
	Protocol    string                 `json:"protocol"`
	FeatureType string                 `json:"feature_type"`
	Timestamp   time.Time              `json:"timestamp"`
	Value       float64                `json:"value"`
	ContextHash string                 `json:"context_hash"`
	Details     map[string]interface{} `json:"details"`
}

// getEnv retrieves an environment variable or returns a default value
func getEnv(key, defaultVal string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultVal
}

// getEnvInt retrieves an integer environment variable or returns a default value
func getEnvInt(key string, defaultVal int) int {
	if value := os.Getenv(key); value != "" {
		var intVal int
		if _, err := fmt.Sscanf(value, "%d", &intVal); err == nil {
			return intVal
		}
	}
	return defaultVal
}

// connectNATS establishes a connection to the NATS server with retry logic.
func connectNATS(url string) (*nats.Conn, error) {
	log.Printf("Connecting to NATS at %s...", url)
	for i := 0; i < 5; i++ {
		nc, err := nats.Connect(url, nats.Timeout(5*time.Second))
		if err == nil {
			log.Println("Successfully connected to NATS.")
			return nc, nil
		}
		log.Printf("NATS connection attempt %d failed: %v. Retrying in %d seconds...", i+1, err, 1<<i)
		time.Sleep(time.Duration(1<<i) * time.Second)
	}
	return nil, fmt.Errorf("failed to connect to NATS after multiple retries")
}

// PublishFeature sends a MonitoringFeature to NATS on the appropriate topic
func (a *Agent) PublishFeature(feature MonitoringFeature) error {
	// Construct the NATS subject with hierarchical structure
	// Format: rpc.{destination}.{protocol}.{method}.{metric}
	subject := feature.ContextHash // This now contains the full subject

	data, err := json.Marshal(feature)
	if err != nil {
		return fmt.Errorf("failed to marshal feature: %w", err)
	}

	if err := a.NatsConn.Publish(subject, data); err != nil {
		return fmt.Errorf("failed to publish to subject %s: %w", subject, err)
	}

	log.Printf("Published to NATS [%s]: %s", subject, string(data))
	return nil
}

// ipToString converts uint32 IP to dotted notation
func ipToString(ip uint32) string {
	return fmt.Sprintf("%d.%d.%d.%d",
		byte(ip), byte(ip>>8), byte(ip>>16), byte(ip>>24))
}

// getHostnameFromIP tries to resolve IP to hostname, returns sanitized version for NATS subject
func getHostnameFromIP(ipStr string) string {
	// Try reverse DNS lookup (with timeout)
	names, err := net.LookupAddr(ipStr)
	if err == nil && len(names) > 0 {
		// Use first hostname, sanitize for NATS subject
		hostname := names[0]
		// Remove trailing dot
		hostname = strings.TrimSuffix(hostname, ".")
		// Replace dots and special chars with hyphens for NATS subject
		hostname = strings.ReplaceAll(hostname, ".", "-")
		return hostname
	}
	
	// If DNS fails, use IP with hyphens
	return strings.ReplaceAll(ipStr, ".", "-")
}

// extractETHMethodFromPayload attempts to extract eth_* method from HTTP/JSON-RPC payload
func extractETHMethodFromPayload(payload string) string {
	// Look for JSON-RPC method in payload
	// Patterns: {"method":"eth_call",...} or {"jsonrpc":"2.0","method":"eth_getBalance",...}
	
	re := regexp.MustCompile(`"method"\s*:\s*"(eth_[a-zA-Z0-9_]+)"`)
	matches := re.FindStringSubmatch(payload)
	if len(matches) > 1 {
		return matches[1]
	}
	
	// Also check for HTTP POST path (some RPC endpoints use path-based routing)
	if strings.Contains(payload, "POST /") {
		// Look for common patterns
		if strings.Contains(payload, "eth_call") {
			return "eth_call"
		}
		if strings.Contains(payload, "eth_sendTransaction") {
			return "eth_sendTransaction"
		}
		if strings.Contains(payload, "eth_getBalance") {
			return "eth_getBalance"
		}
	}
	
	return "unknown"
}

// RunTracer initializes eBPF, attaches the probes, and starts the event loop.
func (a *Agent) RunTracer() error {
	// Allow the BPF programs to be loaded (required for Kubernetes/restricted environments)
	if err := rlimit.RemoveMemlock(); err != nil {
		return fmt.Errorf("failed to remove memlock limit: %w", err)
	}

	// Load pre-compiled eBPF programs
	objs := &rpcObjects{}
	if err := loadRpcObjects(objs, nil); err != nil {
		return fmt.Errorf("failed to load eBPF objects: %w", err)
	}
	a.EBPFObjs = objs

	log.Println("Attaching Kprobe to tcp_sendmsg...")

	// Attach Kprobe to tcp_sendmsg (kernel function for sending TCP data)
	kp, err := link.Kprobe("tcp_sendmsg", a.EBPFObjs.TraceTcpSendmsg, nil)
	if err != nil {
		return fmt.Errorf("failed to attach Kprobe to tcp_sendmsg: %w", err)
	}
	defer kp.Close()
	log.Println("Kprobe attached successfully to tcp_sendmsg")

	// Start reading from the Perf Buffer
	rd, err := perf.NewReader(a.EBPFObjs.Events, os.Getpagesize()*64)
	if err != nil {
		return fmt.Errorf("failed to create perf event reader: %w", err)
	}
	a.PerfReader = rd

	log.Println("Starting Perf Buffer reader...")
	if DebugMode {
		log.Println("DEBUG: Debug mode enabled - verbose logging active")
	}
	go a.readAndProcessEvents()

	// Wait for context cancellation
	<-a.Ctx.Done()
	return nil
}

// readAndProcessEvents continuously reads raw events from the kernel and processes them.
func (a *Agent) readAndProcessEvents() {
	var event RPCEvent
	eventCount := 0

	for {
		record, err := a.PerfReader.Read()
		if err != nil {
			if errors.Is(err, perf.ErrClosed) {
				log.Println("Perf buffer reader closed.")
				return
			}
			log.Printf("Error reading perf buffer: %v", err)
			continue
		}

		if record.LostSamples > 0 {
			log.Printf("Warning: Lost %d samples due to a full buffer", record.LostSamples)
		}

		// Parse binary data into our Go struct
		if err := binary.Read(bytes.NewBuffer(record.RawSample), binary.LittleEndian, &event); err != nil {
			log.Printf("Failed to parse event: %v", err)
			continue
		}

		eventCount++
		if DebugMode {
			log.Printf("DEBUG: Received event #%d: PID=%d, DataLen=%d, IsSend=%d, Comm=%s",
				eventCount, event.PID, event.DataLen, event.IsSend, 
				string(bytes.TrimRight(event.Comm[:], "\x00")))
		}

		// Feature Engineering and Publishing
		a.processAndPublishRPCEvent(event)
	}
}

// processAndPublishRPCEvent performs feature extraction and sends the feature over NATS.
func (a *Agent) processAndPublishRPCEvent(event RPCEvent) {
	processName := string(bytes.TrimRight(event.Comm[:], "\x00"))
	
	// Convert destination IP to string
	destIPStr := ipToString(event.DestIP)
	destHostname := getHostnameFromIP(destIPStr)
	
	// Determine direction and metric type
	direction := "recv"
	metricType := "response"
	if event.IsSend == 1 {
		direction = "send"
		metricType = "request"
	}
	
	// Try to extract ETH JSON-RPC method from payload
	payload := string(bytes.TrimRight(event.Data[:], "\x00"))
	ethMethod := extractETHMethodFromPayload(payload)
	
	// Construct hierarchical NATS subject
	// Format: rpc.{destination}.{protocol}.{method}.{metric}
	// Example: rpc.rpc-reya-cronos-gelato-digital.https.eth_call.request_size
	protocol := "https"
	if event.DestPort == 8545 || event.DestPort == 8547 {
		protocol = "http"
	}
	
	subject := fmt.Sprintf("rpc.%s.%s.%s.%s_size", 
		destHostname, protocol, ethMethod, metricType)
	
	if DebugMode {
		log.Printf("DEBUG: Processing %s to %s:%d (PID %d): method=%s, size=%d",
			direction, destIPStr, event.DestPort, event.PID, ethMethod, event.DataLen)
		if len(payload) > 0 && len(payload) < 200 {
			log.Printf("DEBUG: Payload preview: %s", payload)
		}
	}

	// Create monitoring feature
	feature := MonitoringFeature{
		AppID:       AppID,
		Protocol:    "jsonrpc",
		FeatureType: fmt.Sprintf("%s_size", metricType),
		Timestamp:   time.Now(),
		Value:       float64(event.DataLen),
		ContextHash: subject, // Full subject path
		Details: map[string]interface{}{
			"pid":           event.PID,
			"process":       processName,
			"method":        ethMethod,
			"direction":     direction,
			"size_bytes":    event.DataLen,
			"timestamp_ns":  event.TimestampNs,
			"dest_ip":       destIPStr,
			"dest_port":     event.DestPort,
			"dest_hostname": destHostname,
		},
	}

	// Publish to NATS
	if err := a.PublishFeature(feature); err != nil {
		log.Printf("Failed to publish RPC feature: %v", err)
	} else if DebugMode {
		log.Printf("DEBUG: Published to NATS [%s]: method=%s, size=%d",
			subject, ethMethod, event.DataLen)
	}
}

// min returns the minimum of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// extractJSONRPCMethod attempts to extract the "method" field from a JSON-RPC payload
func extractJSONRPCMethod(payload string) string {
	// Simple string search for "method":"xxx"
	// More robust parsing could use json.Unmarshal
	if len(payload) == 0 {
		return "unknown"
	}
	
	// Look for "method":"
	methodStart := bytes.Index([]byte(payload), []byte(`"method":"`))
	if methodStart == -1 {
		methodStart = bytes.Index([]byte(payload), []byte(`"method": "`))
	}
	if methodStart == -1 {
		return "unknown"
	}
	
	// Find the start of the method value
	valueStart := methodStart + bytes.Index([]byte(payload[methodStart:]), []byte(`"`))
	valueStart = valueStart + bytes.Index([]byte(payload[valueStart+1:]), []byte(`"`)) + 1
	
	// Find the end quote
	valueEnd := valueStart + 1 + bytes.Index([]byte(payload[valueStart+1:]), []byte(`"`))
	
	if valueEnd > valueStart && valueEnd < len(payload) {
		return payload[valueStart+1 : valueEnd]
	}
	
	return "unknown"
}

func main() {
	log.Println("Starting JSON-RPC eBPF Agent for Arbitrum traffic monitoring...")
	log.Printf("Configuration:")
	log.Printf("  NATS URL: %s", NatsURL)
	log.Printf("  App ID: %s", AppID)
	log.Printf("  Target Binary: %s", TargetBinary)
	log.Printf("  Target Symbol: %s", TargetSymbolRet)
	log.Printf("  Target PID: %d (0 = all processes)", TargetPID)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle OS signals for graceful shutdown
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		sig := <-sigs
		log.Printf("Received signal %v. Shutting down agent...", sig)
		cancel()
	}()

	// 1. Connect to NATS
	nc, err := connectNATS(NatsURL)
	if err != nil {
		log.Fatalf("Fatal: %v", err)
	}
	defer nc.Close()

	agent := &Agent{
		NatsConn:     nc,
		Ctx:          ctx,
		Cancel:       cancel,
		FeatureCache: &sync.Map{},
	}

	// 2. Start the eBPF Tracer
	if err := agent.RunTracer(); err != nil {
		log.Fatalf("Fatal: Failed to run eBPF tracer: %v", err)
	}

	// Clean up resources
	if agent.PerfReader != nil {
		agent.PerfReader.Close()
	}
	if agent.EBPFObjs != nil {
		agent.EBPFObjs.Close()
	}

	log.Println("Agent stopped gracefully.")
}
