// Distributed MemOS - Fabric: Gossip, Replication, and Anti-Entropy
package fabric

import (
	"context"
	"crypto/md5"
	"fmt"
	"log"
	"sort"
	"sync"
	"time"

	"github.com/mohi1038/memos/internal/storage"
)

// VersionVector is a logical clock mapping nodeID → version counter.
// Each node increments its own entry on every write.
type VersionVector map[string]uint64

// Dominates returns true if v is strictly newer than other for all nodes.
func (v VersionVector) Dominates(other VersionVector) bool {
	for node, ver := range other {
		if v[node] < ver {
			return false
		}
	}
	return true
}

// Merge creates a new vector that takes the max for each node.
func (v VersionVector) Merge(other VersionVector) VersionVector {
	merged := make(VersionVector)
	for k, val := range v {
		merged[k] = val
	}
	for k, val := range other {
		if val > merged[k] {
			merged[k] = val
		}
	}
	return merged
}

// ShardDigest is a checksum of all memory IDs in a shard, used to detect divergence.
type ShardDigest struct {
	ShardKey string
	Checksum string // MD5 of sorted memory IDs
	Count    int
}

// AntiEntropyManager periodically reconciles memory state across nodes.
// Algorithm:
//  1. Compute local shard digests (MD5 of sorted memory IDs per shard).
//  2. Publish digests to NATS.
//  3. Compare received digests from peers.
//  4. For divergent shards, re-publish MEMORY_STORED events to trigger replication.
type AntiEntropyManager struct {
	localNodeID   string
	gossip        *GossipProtocol
	sharding      *ShardingStrategy
	publisher     *EventPublisher
	subscriber    *EventSubscriber
	db            *storage.PostgresStore
	qdrant        *storage.QdrantStore
	interval      time.Duration
	peerDigests   map[string][]ShardDigest // nodeID → digests
	peerDigestsMu sync.RWMutex
	done          chan struct{}
}

// NewAntiEntropyManager creates a new anti-entropy manager.
func NewAntiEntropyManager(
	nodeID string,
	gossip *GossipProtocol,
	sharding *ShardingStrategy,
	publisher *EventPublisher,
	subscriber *EventSubscriber,
	db *storage.PostgresStore,
	qdrant *storage.QdrantStore,
) *AntiEntropyManager {
	return &AntiEntropyManager{
		localNodeID: nodeID,
		gossip:      gossip,
		sharding:    sharding,
		publisher:   publisher,
		subscriber:  subscriber,
		db:          db,
		qdrant:      qdrant,
		interval:    5 * time.Minute,
		peerDigests: make(map[string][]ShardDigest),
		done:        make(chan struct{}),
	}
}

// Start launches the background anti-entropy loop.
func (ae *AntiEntropyManager) Start(ctx context.Context) error {
	// Subscribe to shard digest events from peers
	_, err := ae.subscriber.SubscribeToShardSync(func(event ShardSyncEvent) error {
		if event.NodeID == ae.localNodeID {
			return nil // ignore own broadcast
		}
		ae.peerDigestsMu.Lock()
		ae.peerDigests[event.NodeID] = event.Digests
		ae.peerDigestsMu.Unlock()
		log.Printf("[AntiEntropy] Received %d shard digests from node %s", len(event.Digests), event.NodeID)
		return nil
	})
	if err != nil {
		return fmt.Errorf("subscribe to shard sync: %w", err)
	}

	ticker := time.NewTicker(ae.interval)
	go func() {
		defer ticker.Stop()
		// Run one cycle immediately
		ae.runCycle(ctx)
		for {
			select {
			case <-ticker.C:
				ae.runCycle(ctx)
			case <-ae.done:
				return
			case <-ctx.Done():
				return
			}
		}
	}()

	log.Printf("[AntiEntropy] Started on node %s (interval: %s)", ae.localNodeID, ae.interval)
	return nil
}

// Stop gracefully stops anti-entropy.
func (ae *AntiEntropyManager) Stop() {
	close(ae.done)
}

// runCycle performs one reconciliation pass.
func (ae *AntiEntropyManager) runCycle(ctx context.Context) {
	log.Printf("[AntiEntropy] Starting reconciliation cycle on node %s", ae.localNodeID)

	// 1. Fetch all local memories
	memories, err := ae.db.ListAllMemoriesForAntiEntropy(ctx)
	if err != nil {
		log.Printf("[AntiEntropy] Failed to list memories: %v", err)
		return
	}

	// 2. Group memories by shard key and compute checksums
	shardGroups := make(map[string][]string) // shardKey → []memoryID
	for _, m := range memories {
		tenantID := m.TenantID.String()
		agentID := m.AgentID.String()
		memID := m.ID.String()
		shardKey := ae.sharding.ComputeShardKey(tenantID, agentID, memID)
		shardGroups[shardKey] = append(shardGroups[shardKey], memID)
	}

	localDigests := make([]ShardDigest, 0, len(shardGroups))
	for shardKey, ids := range shardGroups {
		sort.Strings(ids)
		checksum := computeChecksum(ids)
		localDigests = append(localDigests, ShardDigest{
			ShardKey: shardKey,
			Checksum: checksum,
			Count:    len(ids),
		})
	}

	// 3. Publish our digests to peers
	if err := ae.publisher.PublishShardSync(ctx, ShardSyncEvent{
		NodeID:  ae.localNodeID,
		Digests: localDigests,
	}); err != nil {
		log.Printf("[AntiEntropy] Failed to publish shard digests: %v", err)
	}

	// 4. Compare with peer digests to find divergence
	ae.peerDigestsMu.RLock()
	defer ae.peerDigestsMu.RUnlock()

	// Build local digest index for fast lookup
	localIndex := make(map[string]ShardDigest)
	for _, d := range localDigests {
		localIndex[d.ShardKey] = d
	}

	repaired := 0
	for peerNodeID, peerDigests := range ae.peerDigests {
		for _, peerDigest := range peerDigests {
			localDigest, exists := localIndex[peerDigest.ShardKey]
			if !exists {
				// Peer has a shard we don't — check if we should own it
				log.Printf("[AntiEntropy] Missing shard %s from peer %s (%d items)", peerDigest.ShardKey, peerNodeID, peerDigest.Count)
				continue
			}
			if localDigest.Checksum != peerDigest.Checksum {
				// Divergence detected — re-publish affected memories to trigger replication
				log.Printf("[AntiEntropy] Divergence on shard %s: local=%s peer=%s", peerDigest.ShardKey, localDigest.Checksum, peerDigest.Checksum)
				repaired += ae.repairShard(ctx, peerDigest.ShardKey, shardGroups[peerDigest.ShardKey], memories)
			}
		}
	}

	if repaired > 0 {
		log.Printf("[AntiEntropy] Repaired %d memories in this cycle", repaired)
	} else {
		log.Printf("[AntiEntropy] No divergence detected — cluster is consistent")
	}
}

// repairShard re-publishes memory stored events for divergent memories.
func (ae *AntiEntropyManager) repairShard(ctx context.Context, shardKey string, memIDs []string, allMemories []*storage.MemoryModel) int {
	memIndex := make(map[string]*storage.MemoryModel)
	for _, m := range allMemories {
		memIndex[m.ID.String()] = m
	}

	repaired := 0
	for _, memID := range memIDs {
		m, ok := memIndex[memID]
		if !ok {
			continue
		}
		// Re-publish MEMORY_STORED event so replication loop picks it up
		event := MemoryStoredEvent{
			MemoryID:   memID,
			TenantID:   m.TenantID.String(),
			AgentID:    m.AgentID.String(),
			Content:    m.Content,
			Embedding:  nil, // embeddings will be re-fetched by receiving node
			Type:       m.Type,
			Importance: float32(m.Importance),
			Timestamp:  m.CreatedAt,
		}
		if err := ae.publisher.PublishMemoryStored(ctx, event); err != nil {
			log.Printf("[AntiEntropy] Failed to republish memory %s: %v", memID, err)
			continue
		}
		repaired++
	}
	return repaired
}

// computeChecksum creates an MD5 hash of sorted memory IDs.
func computeChecksum(sortedIDs []string) string {
	h := md5.New()
	for _, id := range sortedIDs {
		fmt.Fprint(h, id)
	}
	return fmt.Sprintf("%x", h.Sum(nil))
}
