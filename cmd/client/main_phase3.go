//go:build phase3
// +build phase3

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

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	tenantID := "00000000-0000-0000-0000-000000000001"
	agentID := "00000000-0000-0000-0000-000000000002"

	memories := []string{
		"Alice prefers dark mode for dashboard reviews.",
		"Alice uses Figma to design UI mockups.",
		"Alice usually works best between 9 AM and 12 PM.",
	}

	fmt.Println("=== Phase 3: Graph-Augmented Retrieval ===")
	for i, content := range memories {
		res, err := c.Store(ctx, &pb.StoreRequest{
			TenantId:   tenantID,
			AgentId:    agentID,
			Type:       pb.MemoryType_MEMORY_TYPE_EPISODIC,
			Content:    content,
			Importance: 0.9,
		})
		if err != nil {
			log.Fatalf("store %d failed: %v", i+1, err)
		}
		fmt.Printf("Stored %d: %s (%s)\n", i+1, content, res.MemoryId)
	}

	fmt.Println("\nRetrieving with a graph-friendly query...")
	retrieveRes, err := c.Retrieve(ctx, &pb.RetrieveRequest{
		TenantId:            tenantID,
		AgentId:             agentID,
		Query:               "What does Alice know about UI preferences and work habits?",
		Limit:               5,
		SimilarityThreshold: 0.0,
	})
	if err != nil {
		log.Fatalf("retrieve failed: %v", err)
	}

	fmt.Printf("Retrieved %d memories:\n", len(retrieveRes.Memories))
	for i, item := range retrieveRes.Memories {
		fmt.Printf("%d. [%.2f] %s\n", i+1, item.Score, item.Memory.Content)
	}
}