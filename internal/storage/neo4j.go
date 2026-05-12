// Distributed MemOS - Storage: Polyglot Persistence and RLS
package storage

import (
	"context"
	"fmt"

	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
)

type Neo4jStore struct {
	Driver neo4j.DriverWithContext
}

func NewNeo4jStore(ctx context.Context, uri, user, pass string) (*Neo4jStore, error) {
	auth := neo4j.BasicAuth(user, pass, "")
	driver, err := neo4j.NewDriverWithContext(uri, auth)
	if err != nil {
		return nil, fmt.Errorf("failed to create neo4j driver: %v", err)
	}
	if err := driver.VerifyConnectivity(ctx); err != nil {
		return nil, fmt.Errorf("neo4j connectivity failed: %v", err)
	}
	return &Neo4jStore{Driver: driver}, nil
}

func (s *Neo4jStore) Close(ctx context.Context) error {
	return s.Driver.Close(ctx)
}

// IndexMemory creates a Memory node and MENTIONS relationships to Entity nodes.
func (s *Neo4jStore) IndexMemory(ctx context.Context, memoryID, tenantID, agentID, content string, entities []string) error {
	session := s.Driver.NewSession(ctx, neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite})
	defer session.Close(ctx)

	_, err := session.ExecuteWrite(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
		_, err := tx.Run(ctx,
			`MERGE (m:Memory {id:$id})
			 SET m.content=$content, m.tenant_id=$tenant, m.agent_id=$agent`,
			map[string]any{"id": memoryID, "content": content, "tenant": tenantID, "agent": agentID})
		if err != nil {
			return nil, err
		}
		for _, e := range entities {
			_, err := tx.Run(ctx,
				`MERGE (t:Entity {name:$name})
				 MERGE (m:Memory {id:$id})-[:MENTIONS]->(t)`,
				map[string]any{"name": e, "id": memoryID})
			if err != nil {
				return nil, err
			}
		}
		return nil, nil
	})
	return err
}

// GetRelatedMemoryIDsForTenant returns tenant-scoped memory IDs connected via shared entities.
func (s *Neo4jStore) GetRelatedMemoryIDsForTenant(ctx context.Context, tenantID, memoryID string, depth int, limit int) ([]string, error) {
	session := s.Driver.NewSession(ctx, neo4j.SessionConfig{AccessMode: neo4j.AccessModeRead})
	defer session.Close(ctx)

	res, err := session.ExecuteRead(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
		query := `MATCH (m:Memory {id:$id, tenant_id:$tenant})-[:MENTIONS]->(e:Entity)<-[:MENTIONS]-(related:Memory {tenant_id:$tenant})
		          RETURN DISTINCT related.id AS id LIMIT $limit`
		result, err := tx.Run(ctx, query, map[string]any{"id": memoryID, "tenant": tenantID, "limit": limit})
		if err != nil {
			return nil, err
		}
		var ids []string
		for result.Next(ctx) {
			record := result.Record()
			v, _ := record.Get("id")
			if s, ok := v.(string); ok {
				ids = append(ids, s)
			}
		}
		if err := result.Err(); err != nil {
			return nil, err
		}
		return ids, nil
	})
	if err != nil {
		return nil, err
	}
	return res.([]string), nil
}

// MarkConflict creates a CONTRADICTS relationship between the winner and loser memory nodes.
func (s *Neo4jStore) MarkConflict(ctx context.Context, winnerID, loserID, conflictType string) error {
	session := s.Driver.NewSession(ctx, neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite})
	defer session.Close(ctx)

	_, err := session.ExecuteWrite(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
		_, err := tx.Run(ctx,
			`MATCH (w:Memory {id:$winner}), (l:Memory {id:$loser})
			 MERGE (w)-[r:CONTRADICTS {type:$type}]->(l)
			 SET r.resolved_at = datetime()`,
			map[string]any{"winner": winnerID, "loser": loserID, "type": conflictType})
		return nil, err
	})
	return err
}

// GetConflicts returns all winner→loser conflict pairs for a tenant.
func (s *Neo4jStore) GetConflicts(ctx context.Context, tenantID string) ([][2]string, error) {
	session := s.Driver.NewSession(ctx, neo4j.SessionConfig{AccessMode: neo4j.AccessModeRead})
	defer session.Close(ctx)

	res, err := session.ExecuteRead(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
		query := `MATCH (w:Memory {tenant_id:$tenant})-[:CONTRADICTS]->(l:Memory {tenant_id:$tenant})
		          RETURN w.id AS winner, l.id AS loser`
		result, err := tx.Run(ctx, query, map[string]any{"tenant": tenantID})
		if err != nil {
			return nil, err
		}
		var pairs [][2]string
		for result.Next(ctx) {
			record := result.Record()
			w, _ := record.Get("winner")
			l, _ := record.Get("loser")
			pairs = append(pairs, [2]string{w.(string), l.(string)})
		}
		return pairs, result.Err()
	})
	if err != nil {
		return nil, err
	}
	return res.([][2]string), nil
}
