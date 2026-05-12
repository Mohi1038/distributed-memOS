// Distributed MemOS - API: Tenant-Aware Configuration and Handlers
package api

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sort"
	"time"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/mohi1038/memos/internal/core"
	"github.com/mohi1038/memos/internal/fabric"
	"github.com/mohi1038/memos/internal/storage"
	pb "github.com/mohi1038/memos/proto"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type MemoryHandler struct {
	pb.UnimplementedMemoryServiceServer
	db               *storage.PostgresStore
	qdrant           *storage.QdrantStore
	embedder         core.EmbeddingGenerator
	graph            *storage.Neo4jStore
	cache            *core.MemoryCache
	conflictResolver *core.ConflictResolver
	// Phase 4: Distributed Memory Fabric
	publisher *fabric.EventPublisher
	sharding  *fabric.ShardingStrategy
	gossip    *fabric.GossipProtocol
	telemetry *core.Telemetry
}

func NewMemoryHandler(db *storage.PostgresStore, qdrant *storage.QdrantStore, graph *storage.Neo4jStore, embedder core.EmbeddingGenerator) *MemoryHandler {
	return &MemoryHandler{
		db:               db,
		qdrant:           qdrant,
		graph:            graph,
		embedder:         embedder,
		cache:            core.NewMemoryCache(512, 30*time.Second),
		conflictResolver: core.NewConflictResolver(db, graph),
	}
}

// SetDistributedFabric wires up the distributed memory fabric components.
func (h *MemoryHandler) SetDistributedFabric(pub *fabric.EventPublisher, sharding *fabric.ShardingStrategy, gossip *fabric.GossipProtocol) {
	h.publisher = pub
	h.sharding = sharding
	h.gossip = gossip
}

func (h *MemoryHandler) SetTelemetry(telemetry *core.Telemetry) {
	h.telemetry = telemetry
}

func (h *MemoryHandler) cacheKey(tenantID, memoryID string) string {
	return tenantID + ":" + memoryID
}

func (h *MemoryHandler) authorize(ctx context.Context, tenantID, agentID string, action core.Action) (string, error) {
	tenantUUID, err := uuid.Parse(tenantID)
	if err != nil {
		return "", status.Errorf(codes.InvalidArgument, "invalid tenant_id")
	}
	agentUUID, err := uuid.Parse(agentID)
	if err != nil {
		return "", status.Errorf(codes.InvalidArgument, "invalid agent_id")
	}

	role, err := h.db.GetAgentRole(ctx, pgtype.UUID{Bytes: tenantUUID, Valid: true}, pgtype.UUID{Bytes: agentUUID, Valid: true})
	if err != nil {
		if h.telemetry != nil {
			h.telemetry.RecordAuthDenied()
		}
		return "", status.Errorf(codes.PermissionDenied, "unknown tenant-agent principal")
	}

	if !core.Can(role, action) {
		if h.telemetry != nil {
			h.telemetry.RecordAuthDenied()
		}
		return role, status.Errorf(codes.PermissionDenied, "role %s cannot perform %s", role, action)
	}

	return role, nil
}

func (h *MemoryHandler) Store(ctx context.Context, req *pb.StoreRequest) (*pb.StoreResponse, error) {
	start := time.Now()
	defer func() {
		if h.telemetry != nil {
			h.telemetry.RecordStore(time.Since(start))
		}
	}()

	log.Printf("Storing memory for tenant %s, agent %s", req.TenantId, req.AgentId)
	if _, err := h.authorize(ctx, req.TenantId, req.AgentId, core.ActionStore); err != nil {
		return nil, err
	}

	// 1. Generate Embedding
	embedding, err := h.embedder.Generate(ctx, req.Content)
	if err != nil {
		return nil, fmt.Errorf("failed to generate embedding: %v", err)
	}

	// 2. Prepare metadata
	metaBytes, _ := json.Marshal(req.Metadata)

	// 3. Save to Postgres
	tenantUUID, _ := uuid.Parse(req.TenantId)
	agentUUID, _ := uuid.Parse(req.AgentId)

	model := &storage.MemoryModel{
		TenantID:   pgtype.UUID{Bytes: tenantUUID, Valid: true},
		AgentID:    pgtype.UUID{Bytes: agentUUID, Valid: true},
		Type:       req.Type.String(),
		Content:    req.Content,
		Importance: float64(req.Importance),
		Metadata:   metaBytes,
		Version:    1, // Initial version
	}

	if err := h.db.SaveMemory(ctx, model); err != nil {
		return nil, fmt.Errorf("failed to save to postgres: %v", err)
	}

	if h.telemetry != nil {
		h.telemetry.RecordAuditWrite()
	}
	auditPayload := map[string]interface{}{
		"content_length": len(req.Content),
		"importance":     req.Importance,
		"type":           req.Type.String(),
	}
	if err := h.db.WriteAuditLog(ctx, &storage.AuditLogModel{
		TenantID:     pgtype.UUID{Bytes: tenantUUID, Valid: true},
		AgentID:      pgtype.UUID{Bytes: agentUUID, Valid: true},
		MemoryID:     model.ID,
		Action:       "store",
		ResourceType: "memory",
		Status:       "success",
		Metadata:     encodeAuditJSON(auditPayload),
	}); err != nil {
		log.Printf("Failed to write audit log: %v", err)
	}

	// 4. Save to Qdrant
	idStr := uuid.UUID(model.ID.Bytes).String()
	payload := map[string]interface{}{
		"tenant_id": req.TenantId,
		"agent_id":  req.AgentId,
		"type":      req.Type.String(),
	}

	if err := h.qdrant.UpsertMemory(ctx, "memories", idStr, embedding, payload); err != nil {
		// Log error but don't fail if vector store is down (can be synced later via fabric)
		log.Printf("Failed to save to Qdrant: %v", err)
	}

	// 5. Index in graph (extract simple entities)
	if h.graph != nil {
		entities := core.ExtractEntities(req.Content)
		if err := h.graph.IndexMemory(ctx, idStr, req.TenantId, req.AgentId, req.Content, entities); err != nil {
			log.Printf("Failed to index memory in graph: %v", err)
		}
	}

	// 6. Publish memory stored event for distributed replication (Phase 4)
	if h.publisher != nil {
		event := fabric.MemoryStoredEvent{
			MemoryID:   idStr,
			TenantID:   req.TenantId,
			AgentID:    req.AgentId,
			Content:    req.Content,
			Embedding:  embedding,
			Type:       req.Type.String(),
			Importance: req.Importance,
			Version:    model.Version,
		}
		if err := h.publisher.PublishMemoryStored(ctx, event); err != nil {
			log.Printf("Failed to publish memory stored event: %v", err)
		}
	}

	// 7. Async conflict detection (non-blocking)
	go func() {
		detectCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if h.conflictResolver != nil {
			h.conflictResolver.DetectAndResolve(detectCtx, model)
		}
	}()

	return &pb.StoreResponse{
		MemoryId: idStr,
		Success:  true,
	}, nil
}

func (h *MemoryHandler) Retrieve(ctx context.Context, req *pb.RetrieveRequest) (*pb.RetrieveResponse, error) {
	start := time.Now()
	defer func() {
		if h.telemetry != nil {
			h.telemetry.RecordRetrieve(time.Since(start))
		}
	}()

	log.Printf("Retrieving memory for tenant %s, query: %s", req.TenantId, req.Query)
	if _, err := h.authorize(ctx, req.TenantId, req.AgentId, core.ActionRetrieve); err != nil {
		return nil, err
	}

	// 1. Generate Query Embedding
	queryEmbedding, err := h.embedder.Generate(ctx, req.Query)
	if err != nil {
		return nil, fmt.Errorf("failed to generate query embedding: %v", err)
	}

	// 2. Search Qdrant (fetch more results to re-rank)
	limit := req.Limit
	if limit <= 0 {
		limit = 10
	}
	
	// Fetch 2x results for better ranking
	qdrantResults, err := h.qdrant.SearchMemories(ctx, "memories", queryEmbedding, uint64(limit)*2, req.TenantId, req.AgentId)
	if err != nil {
		return nil, fmt.Errorf("failed to search vector store: %v", err)
	}

	var scoredMemories []*pb.ScoredMemory
	seenByID := make(map[string]int)

	// 3. Hydrate & Apply Cognitive Ranking
	// R = α*S + β*T + γ*I + δ*C
	weights := core.DefaultRankingWeights
	
	// Override with request weights if provided
	if req.AlphaSemantic > 0 {
		weights.Alpha = req.AlphaSemantic
	}
	if req.BetaTemporal > 0 {
		weights.Beta = req.BetaTemporal
	}
	if req.GammaImportance > 0 {
		weights.Gamma = req.GammaImportance
	}

	for _, res := range qdrantResults {
		idStr := res.Id.GetUuid()
		
		// Fetch full details from Postgres
		var model *storage.MemoryModel
		cacheKey := h.cacheKey(req.TenantId, idStr)
		if h.cache != nil {
			if cached, ok := h.cache.Get(cacheKey); ok {
				if h.telemetry != nil {
					h.telemetry.RecordCacheHit()
				}
				model = cached.(*storage.MemoryModel)
			} else if h.telemetry != nil {
				h.telemetry.RecordCacheMiss()
			}
		}
		if model == nil {
			var err error
			model, err = h.db.GetMemoryByIDForTenant(ctx, pgtype.UUID{Bytes: tenantUUID(req.TenantId), Valid: true}, idStr)
			if err != nil {
				log.Printf("Failed to fetch memory %s from DB: %v", idStr, err)
				continue
			}
			if h.cache != nil {
				h.cache.Set(cacheKey, model)
			}
		}

		// Compute cognitive score with adaptive decay and reinforcement
		memoryScore := core.ComputeRank(
			res.Score,                         // Semantic similarity
			model.Importance,                  // User-marked importance
			model.ReinforcementScore,          // Adaptive reinforcement
			model.DecayFactor,                 // Adaptive decay rate
			model.CreatedAt,                   // Memory age
			weights,
		)

		if memoryScore.FinalScore >= req.SimilarityThreshold {
			// Convert pgtype.UUID to string
			tenantStr := uuid.UUID(model.TenantID.Bytes).String()
			agentStr := uuid.UUID(model.AgentID.Bytes).String()

			scoredMemories = append(scoredMemories, &pb.ScoredMemory{
				Score: memoryScore.FinalScore,
				Memory: &pb.Memory{
					Id:         idStr,
					TenantId:   tenantStr,
					AgentId:    agentStr,
					Type:       pb.MemoryType(pb.MemoryType_value[model.Type]),
					Content:    model.Content,
					Importance: float32(model.Importance),
				},
			})
			seenByID[idStr] = len(scoredMemories) - 1
		}
	}

	// Graph-augmented retrieval: expand top results by related memories
	if h.graph != nil {
		// collect top N ids
		maxAug := 5
		var topIDs []string
		for i, m := range scoredMemories {
			if i >= maxAug {
				break
			}
			topIDs = append(topIDs, m.Memory.Id)
		}

		neighborBoosts := make(map[string]float32)
		for _, tid := range topIDs {
			related, err := h.graph.GetRelatedMemoryIDsForTenant(ctx, req.TenantId, tid, 1, 5)
			if err != nil {
				log.Printf("Graph augmentation failed for %s: %v", tid, err)
				continue
			}
			for _, rid := range related {
				if rid == tid {
					continue
				}
				neighborBoosts[rid] += 0.05
			}
		}

		for rid, boost := range neighborBoosts {
			if idx, ok := seenByID[rid]; ok {
				scoredMemories[idx].Score += boost
				continue
			}

			mModel, err := h.db.GetMemoryByIDForTenant(ctx, pgtype.UUID{Bytes: tenantUUID(req.TenantId), Valid: true}, rid)
			if err != nil {
				continue
			}
			if h.cache != nil {
				h.cache.Set(h.cacheKey(req.TenantId, rid), mModel)
			}

			tenantStr := uuid.UUID(mModel.TenantID.Bytes).String()
			agentStr := uuid.UUID(mModel.AgentID.Bytes).String()
			scoredMemories = append(scoredMemories, &pb.ScoredMemory{
				Score: 0.6 + boost,
				Memory: &pb.Memory{
					Id:         rid,
					TenantId:   tenantStr,
					AgentId:    agentStr,
					Type:       pb.MemoryType(pb.MemoryType_value[mModel.Type]),
					Content:    mModel.Content,
					Importance: float32(mModel.Importance),
				},
			})
			seenByID[rid] = len(scoredMemories) - 1
		}
	}

	if len(scoredMemories) > 1 {
		sort.SliceStable(scoredMemories, func(i, j int) bool {
			if scoredMemories[i].Score == scoredMemories[j].Score {
				return scoredMemories[i].Memory.Id < scoredMemories[j].Memory.Id
			}
			return scoredMemories[i].Score > scoredMemories[j].Score
		})
	}

	// Deduplicate semantically identical content and enforce final top-N.
	contentSeen := make(map[string]struct{})
	finalMemories := make([]*pb.ScoredMemory, 0, len(scoredMemories))
	for _, item := range scoredMemories {
		key := normalizeContentKey(item.Memory.Content)
		if _, ok := contentSeen[key]; ok {
			continue
		}
		contentSeen[key] = struct{}{}
		finalMemories = append(finalMemories, item)

		// 4. Update Retrieval Stats (Reinforcement)
		// Each retrieval increases reinforcement by a default delta (e.g., 0.1)
		go func(id string) {
			h.db.IncrementRetrievalStats(context.Background(), id, 0.1)
		}(item.Memory.Id)

		if len(finalMemories) >= int(limit) {
			break
		}
	}

	if h.telemetry != nil {
		h.telemetry.RecordAuditRead()
	}
	if len(finalMemories) > 0 {
		_ = h.db.WriteAuditLog(ctx, &storage.AuditLogModel{
			TenantID:     pgtype.UUID{Bytes: tenantUUID(req.TenantId), Valid: true},
			AgentID:      pgtype.UUID{Bytes: tenantUUID(req.AgentId), Valid: true},
			Action:       "retrieve",
			ResourceType: "memory",
			Status:       "success",
			Metadata:     encodeAuditJSON(map[string]interface{}{"query_length": len(req.Query), "limit": limit}),
		})
	}

	return &pb.RetrieveResponse{
		Memories: finalMemories,
	}, nil
}

func tenantUUID(value string) [16]byte {
	parsed, _ := uuid.Parse(value)
	return parsed
}

func encodeAuditJSON(v interface{}) []byte {
	data, err := json.Marshal(v)
	if err != nil {
		return []byte("{}")
	}
	return data
}

func normalizeContentKey(content string) string {
	normalized := strings.ToLower(strings.Join(strings.Fields(content), " "))
	if normalized == "" {
		return "<empty>"
	}
	return normalized
}
