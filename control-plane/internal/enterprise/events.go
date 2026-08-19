// Package enterprise: Event-driven Kafka topics, ACL, and consumer groups for AI workflows.
package enterprise

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

// Event names for asynchronous platform integration.
const (
	EventOrderProcessed   = "ai.events.order_processed"
	EventApprovalNeeded   = "ai.events.approval_needed"
	EventDocumentIngested = "ai.events.document_ingested"
	EventAnomalyDetected  = "ai.events.anomaly_detected"
)

// EnterpriseEvent represents a message published to a Kafka topic.
type EnterpriseEvent struct {
	ID        string         `json:"id"`
	Topic     string         `json:"topic"`
	CompanyID string         `json:"companyId"`
	Timestamp time.Time      `json:"timestamp"`
	Payload   map[string]any `json:"payload"`
}

// EventBus provides the contract for publishing and subscribing to enterprise events.
type EventBus interface {
	Publish(ctx context.Context, event EnterpriseEvent) error
	Subscribe(topic string, handler func(event EnterpriseEvent))
}

// MemoryKafkaEventBus simulates a governed Kafka event broker in development.
type MemoryKafkaEventBus struct {
	mu          sync.RWMutex
	subscribers map[string][]func(EnterpriseEvent)
	events      []EnterpriseEvent
}

// NewMemoryKafkaEventBus creates a new in-memory Kafka event bus.
func NewMemoryKafkaEventBus() *MemoryKafkaEventBus {
	return &MemoryKafkaEventBus{
		subscribers: make(map[string][]func(EnterpriseEvent)),
		events:      make([]EnterpriseEvent, 0),
	}
}

func (b *MemoryKafkaEventBus) Publish(_ context.Context, event EnterpriseEvent) error {
	if event.Topic == "" {
		return fmt.Errorf("event topic is required")
	}
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now().UTC()
	}

	b.mu.Lock()
	b.events = append(b.events, event)
	handlers := append([]func(EnterpriseEvent){}, b.subscribers[event.Topic]...)
	b.mu.Unlock()

	for _, handler := range handlers {
		go handler(event)
	}
	return nil
}

func (b *MemoryKafkaEventBus) Subscribe(topic string, handler func(event EnterpriseEvent)) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.subscribers[topic] = append(b.subscribers[topic], handler)
}

// Events returns recorded events for testing and outbox audit.
func (b *MemoryKafkaEventBus) Events() []EnterpriseEvent {
	b.mu.RLock()
	defer b.mu.RUnlock()
	copied := make([]EnterpriseEvent, len(b.events))
	copy(copied, b.events)
	return copied
}

// JSONBytes converts event to bytes for Kafka transport.
func (e EnterpriseEvent) JSONBytes() ([]byte, error) {
	return json.Marshal(e)
}
