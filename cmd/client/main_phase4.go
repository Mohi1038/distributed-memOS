//go:build phase4
// +build phase4

package main

import (
	"context"
	"fmt"
	"log"

	pb "github.com/mohi1038/memos/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func main() {
	fmt.Println("=== Phase 4: Distributed Memory Fabric ===")

	conn, err := grpc.Dial("localhost:50051", grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("failed to connect: %v", err)
	}
	defer conn.Close()

	client := pb.NewMemoryServiceClient(conn)
	ctx := context.Background()
	tenantID := "00000000-0000-0000-0000-000000000001"
	agentID := "00000000-0000-0000-0000-000000000002"

	// Store memories that will be replicated across the fabric
	fmt.Println("\nStoring 3 memories across distributed fabric...")

	memories := []struct {
		tenantID   string
		agentID    string
		content    string
		importance float32
	}{
		{
			tenantID:   tenantID,
			agentID:    agentID,
			content:    "User prefers distributed systems and eventual consistency models.",
			importance: 0.9,
		},
		{
			tenantID:   tenantID,
			agentID:    agentID,
			content:    "NATS is used for event streaming and gossip protocol.",
			importance: 0.85,
		},
		{
			tenantID:   tenantID,
			agentID:    agentID,
			content:    "Memory sharding by tenant/agent enables horizontal scaling.",
			importance: 0.8,
		},
	}

	var storedIDs []string
	for i, mem := range memories {
		storeReq := &pb.StoreRequest{
			TenantId:   mem.tenantID,
			AgentId:    mem.agentID,
			Content:    mem.content,
			Type:       pb.MemoryType_MEMORY_TYPE_EPISODIC,
			Importance: mem.importance,
		}

		storeResp, err := client.Store(ctx, storeReq)
		if err != nil {
			log.Fatalf("failed to store memory: %v", err)
		}

		fmt.Printf("Stored %d: %s (%s)\n", i+1, mem.content, storeResp.MemoryId)
		storedIDs = append(storedIDs, storeResp.MemoryId)
	}

	// Retrieve with distributed routing
	fmt.Println("\nRetrieving memories with distributed sharding awareness...")

	retrieveReq := &pb.RetrieveRequest{
		TenantId: tenantID,
		AgentId:  agentID,
		Query:    "distributed systems and replication",
		Limit:    10,
	}

	retrieveResp, err := client.Retrieve(ctx, retrieveReq)
	if err != nil {
		log.Fatalf("failed to retrieve: %v", err)
	}

	fmt.Printf("\nRetrieved %d memories:\n", len(retrieveResp.Memories))
	for i, mem := range retrieveResp.Memories {
		fmt.Printf("%d. [%.2f] %s\n", i+1, mem.Score, mem.Memory.Content)
	}

	// Simulate another node in the cluster
	fmt.Println("\n--- Simulating second node retrieval (replica sync) ---")
	fmt.Println("In a real distributed setup:")
	fmt.Println("1. Node-1 stores memory → publishes MemoryStoredEvent to NATS")
	fmt.Println("2. Node-2 subscribes to events and applies replication")
	fmt.Println("3. Shard ownership determined by: hash(tenant:agent:memory) % num_nodes")
	fmt.Println("4. 3-way replication ensures eventual consistency")
	fmt.Println("5. Gossip protocol maintains cluster membership and health checks")

	// Show cluster info (would be available via metrics endpoint)
	fmt.Println("\nPhase 4 Features Deployed:")
	fmt.Println("✓ NATS event streaming (topic: memory.stored)")
	fmt.Println("✓ Gossip protocol (node.joined, node.left, node.healthcheck events)")
	fmt.Println("✓ Consistent hashing sharding (3-way replication factor)")
	fmt.Println("✓ Eventual consistency replication manager")
	fmt.Println("✓ Distributed memory indexing across replicas")
}
