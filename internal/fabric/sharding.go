package fabric

import (
	"fmt"
	"hash/fnv"
)

// ShardingStrategy determines which node owns a piece of data.
type ShardingStrategy struct {
	replicationFactor int
}

// NewShardingStrategy creates a new sharding strategy.
func NewShardingStrategy(replicationFactor int) *ShardingStrategy {
	return &ShardingStrategy{
		replicationFactor: replicationFactor,
	}
}

// ComputeShardKey generates a shard key for tenant/agent/memory.
func (ss *ShardingStrategy) ComputeShardKey(tenantID, agentID, memoryID string) string {
	return fmt.Sprintf("%s:%s:%s", tenantID, agentID, memoryID)
}

// ComputeShardHash computes the hash of a shard key for distribution.
func (ss *ShardingStrategy) ComputeShardHash(shardKey string) uint32 {
	h := fnv.New32a()
	h.Write([]byte(shardKey))
	return h.Sum32()
}

// GetReplicaNodes determines which nodes should hold a replica of this shard.
func (ss *ShardingStrategy) GetReplicaNodes(shardKey string, allNodes []*Node) []*Node {
	if len(allNodes) == 0 {
		return allNodes
	}

	hash := ss.ComputeShardHash(shardKey)
	replicas := make([]*Node, 0, ss.replicationFactor)

	// Consistent hashing: start at hash % len(nodes) and take next replicationFactor nodes
	startIdx := int(hash) % len(allNodes)
	for i := 0; i < ss.replicationFactor && i < len(allNodes); i++ {
		idx := (startIdx + i) % len(allNodes)
		replicas = append(replicas, allNodes[idx])
	}

	return replicas
}

// IsShardOwner checks if a node owns (or replicates) a shard.
func (ss *ShardingStrategy) IsShardOwner(nodeID, shardKey string, allNodes []*Node) bool {
	replicas := ss.GetReplicaNodes(shardKey, allNodes)
	for _, replica := range replicas {
		if replica.ID == nodeID {
			return true
		}
	}
	return false
}

// IsPrimaryOwner checks if a node is the primary owner (first replica).
func (ss *ShardingStrategy) IsPrimaryOwner(nodeID, shardKey string, allNodes []*Node) bool {
	replicas := ss.GetReplicaNodes(shardKey, allNodes)
	if len(replicas) == 0 {
		return false
	}
	return replicas[0].ID == nodeID
}

// GetPrimaryNode returns the primary owner node for a shard.
func (ss *ShardingStrategy) GetPrimaryNode(shardKey string, allNodes []*Node) *Node {
	replicas := ss.GetReplicaNodes(shardKey, allNodes)
	if len(replicas) == 0 {
		return nil
	}
	return replicas[0]
}
