// Simple NATS subscriber for testing the eBPF agent output
// Run this to see the messages being published by the agent
//
// Usage:
//   go run nats_subscriber_example.go

package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/nats-io/nats.go"
)

type MonitoringFeature struct {
	AppID       string                 `json:"app_id"`
	Protocol    string                 `json:"protocol"`
	FeatureType string                 `json:"feature_type"`
	Timestamp   time.Time              `json:"timestamp"`
	Value       float64                `json:"value"`
	ContextHash string                 `json:"context_hash"`
	Details     map[string]interface{} `json:"details"`
}

func main() {
	natsURL := os.Getenv("NATS_URL")
	if natsURL == "" {
		natsURL = "nats://localhost:4222"
	}

	log.Printf("Connecting to NATS at %s...", natsURL)
	nc, err := nats.Connect(natsURL)
	if err != nil {
		log.Fatalf("Failed to connect to NATS: %v", err)
	}
	defer nc.Close()

	log.Println("Connected to NATS successfully!")

	// Subscribe to all messages (wildcard)
	subject := ">"
	log.Printf("Subscribing to subject: %s", subject)

	_, err = nc.Subscribe(subject, func(msg *nats.Msg) {
		// Parse the JSON payload
		var feature MonitoringFeature
		if err := json.Unmarshal(msg.Data, &feature); err != nil {
			log.Printf("Failed to parse message: %v", err)
			log.Printf("Raw message: %s", string(msg.Data))
			return
		}

		// Pretty print the feature
		fmt.Println("\n" + "="*80)
		fmt.Printf("📊 New Event Received\n")
		fmt.Println("-" * 80)
		fmt.Printf("Subject:      %s\n", msg.Subject)
		fmt.Printf("App ID:       %s\n", feature.AppID)
		fmt.Printf("Protocol:     %s\n", feature.Protocol)
		fmt.Printf("Feature Type: %s\n", feature.FeatureType)
		fmt.Printf("Timestamp:    %s\n", feature.Timestamp.Format(time.RFC3339))
		fmt.Printf("Value:        %.2f\n", feature.Value)
		fmt.Printf("Context Hash: %s\n", feature.ContextHash)
		fmt.Println("Details:")
		for key, value := range feature.Details {
			fmt.Printf("  - %s: %v\n", key, value)
		}
		fmt.Println("=" * 80)
	})

	if err != nil {
		log.Fatalf("Failed to subscribe: %v", err)
	}

	log.Println("✅ Listening for messages... Press Ctrl+C to exit")

	// Wait for interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	log.Println("\nShutting down subscriber...")
}
