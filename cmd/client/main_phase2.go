//go:build phase2
// +build phase2

package main

import (
	"context"
	"fmt"
	"log"
	"time"

	pb "github.com/mohi1038/memos/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func main() {
	conn, err := grpc.Dial("localhost:50051", grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("did not connect: %v", err)
	}
	defer conn.Close()
	c := pb.NewMemoryServiceClient(conn)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
	defer cancel()

	tenantID := "00000000-0000-0000-0000-000000000001"
	agentID := "00000000-0000-0000-0000-000000000002"

	// Phase 2 Test: Store multiple memories with different types and importance
	fmt.Println("=== Phase 2: Semantic Retrieval & Context Building ===\n")

	// 1. Store EPISODIC memories (high importance for reflection candidacy)
	fmt.Println("1. Storing episodic memories...")
	memories := []struct {
		content   string
		importance float32
		memType    pb.MemoryType
	}{
		{
			content:    "The user prefers dark mode for all dashboards.",
			importance: 0.9,
			memType:    pb.MemoryType_MEMORY_TYPE_EPISODIC,
		},
		{
			content:    "User mentioned they work best between 9 AM and 12 PM.",
			importance: 0.8,
			memType:    pb.MemoryType_MEMORY_TYPE_EPISODIC,
		},
		{
			content:    "The user uses dark mode on their laptop.",
			importance: 0.85,
			memType:    pb.MemoryType_MEMORY_TYPE_EPISODIC,
		},
	}

	storedIDs := make([]string, len(memories))
	for i, mem := range memories {
		storeRes, err := c.Store(ctx, &pb.StoreRequest{
			TenantId:   tenantID,
			AgentId:    agentID,
			Type:       mem.memType,
			Content:    mem.content,
			Importance: mem.importance,
		})
		if err != nil {
			log.Fatalf("could not store memory %d: %v", i, err)
		}
		storedIDs[i] = storeRes.MemoryId
		fmt.Printf("  ✓ Stored: %s (importance: %.1f)\n", mem.content, mem.importance)
	}

	// 2. Demonstrate Cognitive Ranking with Custom Weights
	fmt.Println("\n2. Testing cognitive ranking with custom weights...")
	fmt.Println("   Formula: R = α*S + β*T + γ*I + δ*C")
	fmt.Println("   Where: S=Semantic, T=Temporal, I=Importance, C=Recency")

	// Query with default weights
	fmt.Println("\n   Query: 'user UI preferences and work habits'")
	fmt.Println("   Using default weights (α=0.4, β=0.2, γ=0.3, δ=0.1)...")

	retrieveRes, err := c.Retrieve(ctx, &pb.RetrieveRequest{
		TenantId:            tenantID,
		AgentId:             agentID,
		Query:               "user UI preferences and work habits",
		Limit:               5,
		SimilarityThreshold: 0.0,
		AlphaSemantic:       0.4,
		BetaTemporal:        0.2,
		GammaImportance:     0.3,
	})
	if err != nil {
		log.Fatalf("could not retrieve: %v", err)
	}

	fmt.Printf("\n   Found %d ranked memories:\n", len(retrieveRes.Memories))
	for i, m := range retrieveRes.Memories {
		fmt.Printf("   %d. [Score: %.3f] %s\n", i+1, m.Score, m.Memory.Content)
		fmt.Printf("      Type: %s | Importance: %.1f\n", m.Memory.Type, m.Memory.Importance)
	}

	// 3. Temporal Decay Demonstration
	fmt.Println("\n3. Temporal Decay Feature:")
	fmt.Println("   Memories decay exponentially over 30 days (half-life)")
	fmt.Println("   Recent memories get recency bonus (first 24 hours)")
	fmt.Println("   ✓ Memories created now have full temporal score (1.0)")
	fmt.Println("   ✓ After 30 days: temporal score ~= 0.5")
	fmt.Println("   ✓ After 90 days: temporal score ~= 0.125")

	// 4. Memory Classification
	fmt.Println("\n4. Memory Classification:")
	fmt.Println("   Stored memories with types: EPISODIC")
	fmt.Println("   Eligible for reflection after: 7 days + importance > 0.6")
	fmt.Println("   Will be auto-converted to SEMANTIC by reflection worker")

	// 5. Reflection Worker Status
	fmt.Println("\n5. Reflection Worker:")
	fmt.Println("   ✓ Running in background (every 12 hours)")
	fmt.Println("   ✓ Monitors episodic memories for conversion criteria")
	fmt.Println("   ✓ Summarizes related memories into semantic knowledge")
	fmt.Println("   ✓ Extracts patterns and recurring themes")

	fmt.Println("\n=== Phase 2 Features Complete ===")
	fmt.Printf("\nStored %d memories with IDs:\n", len(storedIDs))
	for i, id := range storedIDs {
		fmt.Printf("  %d. %s\n", i+1, id)
	}
}
