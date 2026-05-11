package fabric

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/nats-io/nats.go"
)

// Node represents a cluster node.
type Node struct {
	ID              string
	Host            string
	Port            int
	LastHeartbeat   time.Time
	MemoryCount     int64
	QdrantStatus    string
	shardRangeStart int // hash range owned by this node
	shardRangeEnd   int
}

// GossipProtocol manages node discovery and health checks.
type GossipProtocol struct {
	localNode      *Node
	nodes          map[string]*Node
	nodesMu        sync.RWMutex
	publisher      *EventPublisher
	subscriber     *EventSubscriber
	healthCheckTTL time.Duration
	ticker         *time.Ticker
	done           chan struct{}
}

// NewGossipProtocol creates a new gossip protocol instance.
func NewGossipProtocol(nodeID, host string, port int, nc *nats.Conn) *GossipProtocol {
	pub := NewEventPublisher(nc)
	sub := NewEventSubscriber(nc)

	localNode := &Node{
		ID:            nodeID,
		Host:          host,
		Port:          port,
		LastHeartbeat: time.Now(),
		MemoryCount:   0,
	}

	return &GossipProtocol{
		localNode:      localNode,
		nodes:          make(map[string]*Node),
		publisher:      pub,
		subscriber:     sub,
		healthCheckTTL: 30 * time.Second,
		done:           make(chan struct{}),
	}
}

// Start initializes the gossip protocol and starts heartbeat + discovery loops.
func (gp *GossipProtocol) Start(ctx context.Context) error {
	// Subscribe to health checks and node join/leave events
	_, err := gp.subscriber.SubscribeToHealthCheck(gp.handleHealthCheck)
	if err != nil {
		return fmt.Errorf("subscribe to health check: %w", err)
	}

	_, err = gp.subscriber.SubscribeToNodeJoined(gp.handleNodeJoined)
	if err != nil {
		return fmt.Errorf("subscribe to node joined: %w", err)
	}

	_, err = gp.subscriber.SubscribeToNodeLeft(gp.handleNodeLeft)
	if err != nil {
		return fmt.Errorf("subscribe to node left: %w", err)
	}

	// Announce this node joining
	event := NodeJoinedEvent{
		NodeID: gp.localNode.ID,
		Host:   gp.localNode.Host,
		Port:   gp.localNode.Port,
	}
	if err := gp.publisher.PublishNodeJoined(ctx, event); err != nil {
		return fmt.Errorf("publish node joined: %w", err)
	}

	// Start background heartbeat loop
	gp.ticker = time.NewTicker(10 * time.Second)
	go gp.heartbeatLoop(ctx)

	return nil
}

// heartbeatLoop periodically sends health check events.
func (gp *GossipProtocol) heartbeatLoop(ctx context.Context) {
	for {
		select {
		case <-gp.ticker.C:
			gp.nodesMu.RLock()
			memCount := int64(len(gp.nodes))
			gp.nodesMu.RUnlock()

			event := HealthCheckEvent{
				NodeID:      gp.localNode.ID,
				MemoryCount: memCount,
				QdrantStatus: "healthy",
			}
			_ = gp.publisher.PublishHealthCheck(ctx, event)

		case <-gp.done:
			return
		case <-ctx.Done():
			return
		}
	}
}

// handleHealthCheck processes incoming health check events.
func (gp *GossipProtocol) handleHealthCheck(event HealthCheckEvent) error {
	gp.nodesMu.Lock()
	defer gp.nodesMu.Unlock()

	if node, exists := gp.nodes[event.NodeID]; exists {
		node.LastHeartbeat = event.Timestamp
		node.MemoryCount = event.MemoryCount
		node.QdrantStatus = event.QdrantStatus
	}
	return nil
}

// handleNodeJoined processes node joined events.
func (gp *GossipProtocol) handleNodeJoined(event NodeJoinedEvent) error {
	gp.nodesMu.Lock()
	defer gp.nodesMu.Unlock()

	if event.NodeID == gp.localNode.ID {
		return nil // Ignore our own join event
	}

	gp.nodes[event.NodeID] = &Node{
		ID:            event.NodeID,
		Host:          event.Host,
		Port:          event.Port,
		LastHeartbeat: event.Timestamp,
		MemoryCount:   0,
	}

	fmt.Printf("[Gossip] Node %s joined cluster at %s:%d\n", event.NodeID, event.Host, event.Port)
	return nil
}

// handleNodeLeft processes node left events.
func (gp *GossipProtocol) handleNodeLeft(event NodeLeftEvent) error {
	gp.nodesMu.Lock()
	defer gp.nodesMu.Unlock()

	delete(gp.nodes, event.NodeID)
	fmt.Printf("[Gossip] Node %s left cluster\n", event.NodeID)
	return nil
}

// Stop stops the gossip protocol.
func (gp *GossipProtocol) Stop(ctx context.Context) error {
	close(gp.done)
	if gp.ticker != nil {
		gp.ticker.Stop()
	}

	// Announce node leaving
	event := NodeLeftEvent{
		NodeID: gp.localNode.ID,
	}
	_ = gp.publisher.PublishNodeLeft(ctx, event)
	return nil
}

// GetNodes returns all known nodes in the cluster.
func (gp *GossipProtocol) GetNodes() []*Node {
	gp.nodesMu.RLock()
	defer gp.nodesMu.RUnlock()

	nodes := make([]*Node, 0, len(gp.nodes)+1)
	nodes = append(nodes, gp.localNode)
	for _, node := range gp.nodes {
		nodes = append(nodes, node)
	}
	return nodes
}

// GetLocalNode returns the local node.
func (gp *GossipProtocol) GetLocalNode() *Node {
	return gp.localNode
}

// GetActiveNodes returns nodes that have sent a heartbeat within the TTL.
func (gp *GossipProtocol) GetActiveNodes() []*Node {
	gp.nodesMu.RLock()
	defer gp.nodesMu.RUnlock()

	now := time.Now()
	active := make([]*Node, 0)
	active = append(active, gp.localNode) // Local node is always active

	for _, node := range gp.nodes {
		if now.Sub(node.LastHeartbeat) < gp.healthCheckTTL {
			active = append(active, node)
		}
	}
	return active
}

// UpdateLocalMemoryCount updates the local node's memory count.
func (gp *GossipProtocol) UpdateLocalMemoryCount(count int64) {
	gp.localNode.MemoryCount = count
}
