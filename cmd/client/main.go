//go:build !phase2 && !phase3
// +build !phase2,!phase3

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

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()

	tenantID := "00000000-0000-0000-0000-000000000001"
	agentID := "00000000-0000-0000-0000-000000000002"

	// 1. Store a memory
	fmt.Println("Storing memory...")
	storeRes, err := c.Store(ctx, &pb.StoreRequest{
		TenantId:   tenantID,
		AgentId:    agentID,
		Type:       pb.MemoryType_MEMORY_TYPE_EPISODIC,
		Content:    "The user prefers dark mode for all dashboards.",
		Importance: 0.9,
	})
	if err != nil {
		log.Fatalf("could not store: %v", err)
	}
	fmt.Printf("Memory stored with ID: %s\n", storeRes.MemoryId)

	// 2. Retrieve memory
	fmt.Println("\nRetrieving memory...")
	retrieveRes, err := c.Retrieve(ctx, &pb.RetrieveRequest{
		TenantId:            tenantID,
		AgentId:             agentID,
		Query:               "What are the user's UI preferences?",
		Limit:               5,
		SimilarityThreshold: 0.1,
	})
	if err != nil {
		log.Fatalf("could not retrieve: %v", err)
	}

	fmt.Printf("Found %d memories:\n", len(retrieveRes.Memories))
	for _, m := range retrieveRes.Memories {
		fmt.Printf("- [%.2f] %s\n", m.Score, m.Memory.Content)
	}
}
