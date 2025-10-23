package main

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/cilium/ebpf"
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
)

//go:generate go run github.com/cilium/ebpf/cmd/bpf2go -cc clang rpc rpc_tracer.c -- -I./headers

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
// Must match the C struct (rpc_event_t) defined in rpc_tracer.c exactly.
type RPCEvent struct {
	PID          uint64
	TimestampNs  uint64
	ResponseSize uint64
	MethodName   [128]byte
	Comm         [16]byte
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

// PublishFeature serializes a feature and sends it over NATS.
func (a *Agent) PublishFeature(f MonitoringFeature) error {
	// 1. Construct the Subject: [AppID].[Protocol].[FeatureType]
	subject := fmt.Sprintf("%s.%s.%s", f.AppID, f.Protocol, f.FeatureType)

	// 2. Serialize Payload to JSON
	payload, err := json.Marshal(f)
	if err != nil {
		return fmt.Errorf("failed to marshal feature: %w", err)
	}

	// 3. Publish to NATS
	if err := a.NatsConn.Publish(subject, payload); err != nil {
		return fmt.Errorf("failed to publish to NATS subject %s: %w", subject, err)
	}

	log.Printf("Published to NATS [%s]: %s", subject, string(payload))
	return nil
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

	log.Printf("Attaching Uprobe to symbol '%s' in binary '%s' (PID: %d)...",
		TargetSymbolRet, TargetBinary, TargetPID)

	// Attach Uprobe to the target function's return point
	uprobeOpts := &link.UprobeOptions{}
	if TargetPID > 0 {
		uprobeOpts.PID = TargetPID
	}

	up, err := link.Uprobe(TargetBinary, TargetSymbolRet, a.EBPFObjs.UprobeRpcHandleRet, uprobeOpts)
	if err != nil {
		return fmt.Errorf("failed to attach Uprobe: %w", err)
	}
	defer up.Close()
	log.Println("Uprobe attached successfully.")

	// Start reading from the Perf Buffer
	rd, err := perf.NewReader(a.EBPFObjs.Events, os.Getpagesize()*64)
	if err != nil {
		return fmt.Errorf("failed to create perf event reader: %w", err)
	}
	a.PerfReader = rd

	log.Println("Starting Perf Buffer reader...")
	go a.readAndProcessEvents()

	// Wait for context cancellation
	<-a.Ctx.Done()
	return nil
}

// readAndProcessEvents continuously reads raw events from the kernel and processes them.
func (a *Agent) readAndProcessEvents() {
	var event RPCEvent

	for {
		select {
		case <-a.Ctx.Done():
			log.Println("Event reader shut down.")
			return
		default:
			// Read one record from the perf buffer
			record, err := a.PerfReader.Read()
			if err != nil {
				if errors.Is(err, perf.ErrClosed) {
					log.Println("Perf reader closed.")
					return
				}
				log.Printf("Error reading perf event: %v", err)
				continue
			}

			// Parse the raw data into the Go struct
			if err := binary.Read(bytes.NewReader(record.RawSample), binary.LittleEndian, &event); err != nil {
				log.Printf("Failed to parse perf event: %v", err)
				continue
			}

			// Feature Engineering and Publishing
			a.processAndPublishRPCEvent(event)
		}
	}
}

// processAndPublishRPCEvent performs feature extraction and sends the feature over NATS.
func (a *Agent) processAndPublishRPCEvent(event RPCEvent) {
	// Extract method name (null-terminated string)
	methodName := string(bytes.TrimRight(event.MethodName[:], "\x00"))
	processName := string(bytes.TrimRight(event.Comm[:], "\x00"))

	// Create a context hash based on method name
	contextHash := fmt.Sprintf("method:%s", methodName)

	// Create monitoring feature for response size
	feature := MonitoringFeature{
		AppID:       AppID,
		Protocol:    "jsonrpc",
		FeatureType: "response-size",
		Timestamp:   time.Now(),
		Value:       float64(event.ResponseSize),
		ContextHash: contextHash,
		Details: map[string]interface{}{
			"pid":         event.PID,
			"process":     processName,
			"method":      methodName,
			"timestamp_ns": event.TimestampNs,
		},
	}

	// Publish to NATS
	if err := a.PublishFeature(feature); err != nil {
		log.Printf("Failed to publish JSON-RPC feature: %v", err)
	}
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
