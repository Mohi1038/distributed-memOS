// Distributed MemOS - Fabric: Gossip, Replication, and Anti-Entropy
package fabric

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/mohi1038/memos/internal/storage"
)

// ReplicationManager handles asynchronous replication of memories across replicas.
type ReplicationManager struct {
	gossip              *GossipProtocol
	sharding            *ShardingStrategy
	publisher           *EventPublisher
	subscriber          *EventSubscriber
	localQdrant         *storage.QdrantStore
	localPostgres       *storage.PostgresStore
	telemetry           interface{ RecordReplicationLag(time.Duration) }
	replicationQueue    chan MemoryStoredEvent
	maxReplicationLag   time.Duration
	syncIntervalSeconds int
	mu                  sync.RWMutex
	done                chan struct{}
}

// NewReplicationManager creates a new replication manager.
func NewReplicationManager(
	gossip *GossipProtocol,
	sharding *ShardingStrategy,
	pub *EventPublisher,
	sub *EventSubscriber,
	qdrantStore *storage.QdrantStore,
	postgresStore *storage.PostgresStore,
	telemetry interface{ RecordReplicationLag(time.Duration) },
) *ReplicationManager {
	return &ReplicationManager{
		gossip:              gossip,
		sharding:            sharding,
		publisher:           pub,
		subscriber:          sub,
		localQdrant:         qdrantStore,
		localPostgres:       postgresStore,
		telemetry:           telemetry,
		replicationQueue:    make(chan MemoryStoredEvent, 1000),
		maxReplicationLag:   5 * time.Second,
		syncIntervalSeconds: 30,
		done:                make(chan struct{}),
	}
}

// Start initializes the replication manager.
func (rm *ReplicationManager) Start(ctx context.Context) error {
	// Subscribe to memory stored events for replication
	_, err := rm.subscriber.SubscribeToMemoryStored(func(event MemoryStoredEvent) error {
		select {
		case rm.replicationQueue <- event:
		default:
			fmt.Printf("replication queue full, dropping event for memory %s\n", event.MemoryID)
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("subscribe to memory stored: %w", err)
	}

	// Start background replication loop
	go rm.replicationLoop(ctx)

	return nil
}

// replicationLoop processes the replication queue and applies changes locally.
func (rm *ReplicationManager) replicationLoop(ctx context.Context) {
	for {
		select {
		case event := <-rm.replicationQueue:
			if err := rm.applyReplication(ctx, event); err != nil {
				fmt.Printf("failed to apply replication for memory %s: %v\n", event.MemoryID, err)
			}

		case <-rm.done:
			return
		case <-ctx.Done():
			return
		}
	}
}

// applyReplication applies a memory event to local stores if this node is a replica.
func (rm *ReplicationManager) applyReplication(ctx context.Context, event MemoryStoredEvent) error {
	shardKey := rm.sharding.ComputeShardKey(event.TenantID, event.AgentID, event.MemoryID)
	localNodeID := rm.gossip.GetLocalNode().ID
	allNodes := rm.gossip.GetActiveNodes()

	// Check if this node should replicate this shard
	if !rm.sharding.IsShardOwner(localNodeID, shardKey, allNodes) {
		return nil // This node is not a replica owner
	}

	// Skip if this is the primary node writing the event (it already stored)
	primaryNode := rm.sharding.GetPrimaryNode(shardKey, allNodes)
	if primaryNode != nil && primaryNode.ID == localNodeID {
		return nil
	}

	// Apply to local Postgres (with conflict resolution)
	memModel := &storage.MemoryModel{
		ID:         pgtype.UUID{Bytes: uuid.MustParse(event.MemoryID), Valid: true},
		TenantID:   pgtype.UUID{Bytes: uuid.MustParse(event.TenantID), Valid: true},
		AgentID:    pgtype.UUID{Bytes: uuid.MustParse(event.AgentID), Valid: true},
		Type:       event.Type,
		Content:    event.Content,
		Importance: float64(event.Importance),
		Version:    event.Version,
	}

	updated, err := rm.localPostgres.UpsertMemoryWithConflictResolution(ctx, memModel)
	if err != nil {
		return fmt.Errorf("upsert to postgres: %w", err)
	}

	if updated {
		// Apply to local Qdrant (vector index)
		payload := map[string]interface{}{
			"tenant_id": event.TenantID,
			"agent_id":  event.AgentID,
			"type":      event.Type,
		}
		if err := rm.localQdrant.UpsertMemory(ctx, "memories", event.MemoryID, event.Embedding, payload); err != nil {
			return fmt.Errorf("upsert to qdrant: %w", err)
		}
		fmt.Printf("[Replication] Replicated memory %s (v%d) to local node\n", event.MemoryID, event.Version)
	} else {
		fmt.Printf("[Replication] Ignored stale memory %s (v%d)\n", event.MemoryID, event.Version)
	}

	if rm.telemetry != nil {
		rm.telemetry.RecordReplicationLag(time.Since(event.Timestamp))
	}
	return nil
}

// EnqueueMemoryEvent enqueues a memory for replication.
func (rm *ReplicationManager) EnqueueMemoryEvent(event MemoryStoredEvent) {
	select {
	case rm.replicationQueue <- event:
	default:
		fmt.Printf("replication queue full, dropping event for memory %s\n", event.MemoryID)
	}
}

// Stop stops the replication manager.
func (rm *ReplicationManager) Stop() {
	close(rm.done)
}

// GetReplicationStats returns replication statistics.
func (rm *ReplicationManager) GetReplicationStats() map[string]interface{} {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	return map[string]interface{}{
		"queue_length": len(rm.replicationQueue),
		"queue_cap":    cap(rm.replicationQueue),
		"max_lag_ms":   rm.maxReplicationLag.Milliseconds(),
	}
}
