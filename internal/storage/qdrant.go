// Distributed MemOS - Storage: Polyglot Persistence and RLS
package storage

import (
	"context"
	"fmt"

	"github.com/qdrant/go-client/qdrant"
)

type QdrantStore struct {
	Client *qdrant.Client
}

func NewQdrantStore(ctx context.Context, url string) (*QdrantStore, error) {
	client, err := qdrant.NewClient(&qdrant.Config{
		Host: url,
		Port: 6334,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create qdrant client: %v", err)
	}

	return &QdrantStore{Client: client}, nil
}

func (s *QdrantStore) CreateCollection(ctx context.Context, name string, size uint64) error {
	err := s.Client.CreateCollection(ctx, &qdrant.CreateCollection{
		CollectionName: name,
		VectorsConfig: qdrant.NewVectorsConfig(&qdrant.VectorParams{
			Size:     size,
			Distance: qdrant.Distance_Cosine,
		}),
	})
	return err
}

func (s *QdrantStore) UpsertMemory(ctx context.Context, collection string, id string, vector []float32, payload map[string]interface{}) error {
	// Convert payload to qdrant values
	qPayload := make(map[string]*qdrant.Value)
	for k, v := range payload {
		val, err := qdrant.NewValue(v)
		if err != nil {
			continue
		}
		qPayload[k] = val
	}

	_, err := s.Client.Upsert(ctx, &qdrant.UpsertPoints{
		CollectionName: collection,
		Points: []*qdrant.PointStruct{
			{
				Id:      qdrant.NewID(id),
				Vectors: qdrant.NewVectors(vector...),
				Payload: qPayload,
			},
		},
	})
	return err
}

func (s *QdrantStore) SearchMemories(ctx context.Context, collection string, vector []float32, limit uint64, tenantId, agentId string) ([]*qdrant.ScoredPoint, error) {
	filter := &qdrant.Filter{
		Must: []*qdrant.Condition{
			qdrant.NewMatchKeyword("tenant_id", tenantId),
			qdrant.NewMatchKeyword("agent_id", agentId),
		},
	}

	results, err := s.Client.Query(ctx, &qdrant.QueryPoints{
		CollectionName: collection,
		Query:          qdrant.NewQuery(vector...),
		Filter:         filter,
		Limit:          &limit,
		WithPayload:    qdrant.NewWithPayload(true),
	})
	if err != nil {
		return nil, err
	}

	return results, nil
}

// DeleteMemory removes a memory vector from the collection.
func (s *QdrantStore) DeleteMemory(ctx context.Context, collection, memoryID string) error {
	_, err := s.Client.Delete(ctx, &qdrant.DeletePoints{
		CollectionName: collection,
		Points:         qdrant.NewPointsSelector(qdrant.NewIDUUID(memoryID)),
	})
	return err
}
