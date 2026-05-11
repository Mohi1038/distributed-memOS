package fabric

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/nats-io/nats.go"
)

// Event types for distributed replication
const (
	EventMemoryStored    = "memory.stored"
	EventMemoryRetreived = "memory.retrieved"
	EventNodeJoined      = "node.joined"
	EventNodeLeft        = "node.left"
	EventHealthCheck     = "node.healthcheck"
	EventShardSync       = "shard.sync"
)

// ShardSyncEvent carries shard digest data for anti-entropy reconciliation.
type ShardSyncEvent struct {
	NodeID    string        `json:"node_id"`
	Digests   []ShardDigest `json:"digests"`
	Timestamp time.Time     `json:"timestamp"`
}

// MemoryStoredEvent is published when a memory is stored to NATS.
type MemoryStoredEvent struct {
	MemoryID   string    `json:"memory_id"`
	TenantID   string    `json:"tenant_id"`
	AgentID    string    `json:"agent_id"`
	Content    string    `json:"content"`
	Embedding  []float32 `json:"embedding"`
	Type       string    `json:"type"`
	Importance float32   `json:"importance"`
	Timestamp  time.Time `json:"timestamp"`
}

// NodeJoinedEvent is published when a node joins the cluster.
type NodeJoinedEvent struct {
	NodeID    string    `json:"node_id"`
	Host      string    `json:"host"`
	Port      int       `json:"port"`
	Timestamp time.Time `json:"timestamp"`
}

// NodeLeftEvent is published when a node leaves the cluster.
type NodeLeftEvent struct {
	NodeID    string    `json:"node_id"`
	Timestamp time.Time `json:"timestamp"`
}

// HealthCheckEvent is a periodic heartbeat from a node.
type HealthCheckEvent struct {
	NodeID       string    `json:"node_id"`
	MemoryCount  int64     `json:"memory_count"`
	QdrantStatus string    `json:"qdrant_status"`
	Timestamp    time.Time `json:"timestamp"`
}

// EventPublisher publishes events to NATS.
type EventPublisher struct {
	nc *nats.Conn
}

// NewEventPublisher creates a new event publisher.
func NewEventPublisher(nc *nats.Conn) *EventPublisher {
	return &EventPublisher{nc: nc}
}

// PublishMemoryStored publishes a memory stored event.
func (ep *EventPublisher) PublishMemoryStored(ctx context.Context, event MemoryStoredEvent) error {
	event.Timestamp = time.Now()
	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal event: %w", err)
	}
	return ep.nc.Publish(EventMemoryStored, data)
}

// PublishNodeJoined publishes a node joined event.
func (ep *EventPublisher) PublishNodeJoined(ctx context.Context, event NodeJoinedEvent) error {
	event.Timestamp = time.Now()
	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal event: %w", err)
	}
	return ep.nc.Publish(EventNodeJoined, data)
}

// PublishNodeLeft publishes a node left event.
func (ep *EventPublisher) PublishNodeLeft(ctx context.Context, event NodeLeftEvent) error {
	event.Timestamp = time.Now()
	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal event: %w", err)
	}
	return ep.nc.Publish(EventNodeLeft, data)
}

// PublishHealthCheck publishes a health check event.
func (ep *EventPublisher) PublishHealthCheck(ctx context.Context, event HealthCheckEvent) error {
	event.Timestamp = time.Now()
	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal event: %w", err)
	}
	return ep.nc.Publish(EventHealthCheck, data)
}

// PublishShardSync broadcasts shard digest data for anti-entropy.
func (ep *EventPublisher) PublishShardSync(ctx context.Context, event ShardSyncEvent) error {
	event.Timestamp = time.Now()
	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal shard sync event: %w", err)
	}
	return ep.nc.Publish(EventShardSync, data)
}

// EventSubscriber subscribes to events from NATS.
type EventSubscriber struct {
	nc *nats.Conn
}

// NewEventSubscriber creates a new event subscriber.
func NewEventSubscriber(nc *nats.Conn) *EventSubscriber {
	return &EventSubscriber{nc: nc}
}

// SubscribeToMemoryStored subscribes to memory stored events.
func (es *EventSubscriber) SubscribeToMemoryStored(handler func(event MemoryStoredEvent) error) (*nats.Subscription, error) {
	return es.nc.Subscribe(EventMemoryStored, func(msg *nats.Msg) {
		var event MemoryStoredEvent
		if err := json.Unmarshal(msg.Data, &event); err != nil {
			fmt.Printf("failed to unmarshal memory stored event: %v\n", err)
			return
		}
		if err := handler(event); err != nil {
			fmt.Printf("failed to handle memory stored event: %v\n", err)
		}
	})
}

// SubscribeToHealthCheck subscribes to health check events.
func (es *EventSubscriber) SubscribeToHealthCheck(handler func(event HealthCheckEvent) error) (*nats.Subscription, error) {
	return es.nc.Subscribe(EventHealthCheck, func(msg *nats.Msg) {
		var event HealthCheckEvent
		if err := json.Unmarshal(msg.Data, &event); err != nil {
			fmt.Printf("failed to unmarshal health check event: %v\n", err)
			return
		}
		if err := handler(event); err != nil {
			fmt.Printf("failed to handle health check event: %v\n", err)
		}
	})
}

// SubscribeToNodeJoined subscribes to node joined events.
func (es *EventSubscriber) SubscribeToNodeJoined(handler func(event NodeJoinedEvent) error) (*nats.Subscription, error) {
	return es.nc.Subscribe(EventNodeJoined, func(msg *nats.Msg) {
		var event NodeJoinedEvent
		if err := json.Unmarshal(msg.Data, &event); err != nil {
			fmt.Printf("failed to unmarshal node joined event: %v\n", err)
			return
		}
		if err := handler(event); err != nil {
			fmt.Printf("failed to handle node joined event: %v\n", err)
		}
	})
}

// SubscribeToNodeLeft subscribes to node left events.
func (es *EventSubscriber) SubscribeToNodeLeft(handler func(event NodeLeftEvent) error) (*nats.Subscription, error) {
	return es.nc.Subscribe(EventNodeLeft, func(msg *nats.Msg) {
		var event NodeLeftEvent
		if err := json.Unmarshal(msg.Data, &event); err != nil {
			fmt.Printf("failed to unmarshal node left event: %v\n", err)
			return
		}
		if err := handler(event); err != nil {
			fmt.Printf("failed to handle node left event: %v\n", err)
		}
	})
}

// SubscribeToShardSync subscribes to shard digest events for anti-entropy.
func (es *EventSubscriber) SubscribeToShardSync(handler func(event ShardSyncEvent) error) (*nats.Subscription, error) {
	return es.nc.Subscribe(EventShardSync, func(msg *nats.Msg) {
		var event ShardSyncEvent
		if err := json.Unmarshal(msg.Data, &event); err != nil {
			fmt.Printf("failed to unmarshal shard sync event: %v\n", err)
			return
		}
		if err := handler(event); err != nil {
			fmt.Printf("failed to handle shard sync event: %v\n", err)
		}
	})
}
