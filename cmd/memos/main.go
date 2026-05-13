package main

import (
	"context"
	"encoding/json"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/mohi1038/memos/internal/api"
	"github.com/mohi1038/memos/internal/config"
	"github.com/mohi1038/memos/internal/core"
	"github.com/mohi1038/memos/internal/fabric"
	"github.com/mohi1038/memos/internal/storage"
	pb "github.com/mohi1038/memos/proto"
	"github.com/nats-io/nats.go"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

type roleUpdateRequest struct {
	TenantID         string `json:"tenant_id"`
	RequesterAgentID string `json:"requester_agent_id"`
	TargetAgentID    string `json:"target_agent_id"`
	Role             string `json:"role"`
}

func main() {
	cfg := config.Load()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Initialize NATS
	natsURL := os.Getenv("NATS_URL")
	if natsURL == "" {
		natsURL = "nats://localhost:4222"
	}

	var nc *nats.Conn
	var err error
	maxRetries := 5
	for i := 0; i < maxRetries; i++ {
		nc, err = nats.Connect(natsURL)
		if err == nil {
			break
		}
		log.Printf("Failed to connect to NATS (attempt %d/%d): %v", i+1, maxRetries, err)
		time.Sleep(2 * time.Second)
	}
	if err != nil {
		log.Fatalf("Failed to connect to NATS after %d attempts: %v", maxRetries, err)
	}
	defer nc.Close()
	log.Printf("Connected to NATS at %s", natsURL)

	// Initialize Postgres
	db, err := storage.NewPostgresStore(ctx, cfg.PostgresURL)
	if err != nil {
		log.Fatalf("Failed to initialize Postgres: %v", err)
	}
	defer db.Close()
	if err := db.EnsureEnterpriseSchema(ctx); err != nil {
		log.Fatalf("Failed to ensure audit schema: %v", err)
	}
	if err := db.EnsureCognitiveSchema(ctx); err != nil {
		log.Fatalf("Failed to ensure cognitive schema: %v", err)
	}
	devTenantID := uuid.MustParse("00000000-0000-0000-0000-000000000001")
	devAgentID := uuid.MustParse("00000000-0000-0000-0000-000000000002")
	if err := db.EnsureDevPrincipal(ctx, devTenantID, devAgentID); err != nil {
		log.Fatalf("Failed to ensure dev principal: %v", err)
	}

	// Initialize Qdrant
	qdrantStore, err := storage.NewQdrantStore(ctx, cfg.QdrantURL)
	if err != nil {
		log.Fatalf("Failed to initialize Qdrant: %v", err)
	}

	// Initialize Neo4j
	neo4jStore, err := storage.NewNeo4jStore(ctx, "neo4j://127.0.0.1:7687", "neo4j", "neo4j_password")
	if err != nil {
		log.Printf("Warning: failed to connect to Neo4j: %v", err)
		neo4jStore = nil
	}

	// Initialize Embedder (Real or Mock)
	var embedder core.EmbeddingGenerator
	var dim uint64 = 384 // Mock dimensionality
	if cfg.UseRealEmbedding && cfg.OpenAIAPIKey != "" {
		log.Println("Using OpenAI embeddings (1536-dim)")
		embedder = core.NewReliableEmbedder(core.NewOpenAIEmbeddingGenerator(cfg.OpenAIAPIKey, cfg.EmbeddingModel))
		dim = 1536
	} else {
		log.Println("Using mock embeddings (set OPENAI_API_KEY and USE_REAL_EMBEDDING=true for production)")
		embedder = &core.MockEmbeddingGenerator{Size: 384}
	}

	// Ensure collection exists
	if err := qdrantStore.CreateCollection(ctx, "memories", dim); err != nil {
		log.Printf("Collection 'memories' already exists or failed to create: %v", err)
	}

	// Initialize Reflection Worker
	summarizer := &core.SimpleSummarizer{}
	reflectionWorker := core.NewReflectionWorker(db, qdrantStore, embedder, summarizer)
	go reflectionWorker.Start(ctx)

	telemetry := core.NewTelemetry()
	dashboardService := api.NewDashboardService(db, telemetry)
	go func() {
		metricsPort := cfg.MetricsPort
		if metricsPort == "" {
			metricsPort = "9090"
		}
		mux := http.NewServeMux()
		mux.HandleFunc("/metrics", telemetry.Handler)
		mux.HandleFunc("/api/dashboard/summary", dashboardService.SummaryHandler)
		mux.Handle("/api/dashboard/stream", dashboardService.StreamHandler())
		mux.HandleFunc("/audit", func(w http.ResponseWriter, r *http.Request) {
			tenantID := r.URL.Query().Get("tenant_id")
			agentID := r.URL.Query().Get("agent_id")
			if tenantID == "" || agentID == "" {
				http.Error(w, "tenant_id and agent_id are required", http.StatusBadRequest)
				return
			}

			tenantUUID, err := uuid.Parse(tenantID)
			if err != nil {
				http.Error(w, "invalid tenant_id", http.StatusBadRequest)
				return
			}
			agentUUID, err := uuid.Parse(agentID)
			if err != nil {
				http.Error(w, "invalid agent_id", http.StatusBadRequest)
				return
			}

			role, err := db.GetAgentRole(r.Context(), pgtype.UUID{Bytes: tenantUUID, Valid: true}, pgtype.UUID{Bytes: agentUUID, Valid: true})
			if err != nil || !core.Can(role, core.ActionAuditRead) {
				if telemetry != nil {
					telemetry.RecordAuthDenied()
				}
				http.Error(w, "permission denied", http.StatusForbidden)
				return
			}

			limit := 100
			if rawLimit := r.URL.Query().Get("limit"); rawLimit != "" {
				if parsedLimit, err := strconv.Atoi(rawLimit); err == nil && parsedLimit > 0 {
					limit = parsedLimit
				}
			}

			logs, err := db.ListAuditLogs(r.Context(), pgtype.UUID{Bytes: tenantUUID, Valid: true}, limit)
			if err != nil {
				http.Error(w, "failed to load audit logs", http.StatusInternalServerError)
				return
			}

			if telemetry != nil {
				telemetry.RecordAuditRead()
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(logs)
		})
		mux.HandleFunc("/admin/role", func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPost {
				http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
				return
			}

			var req roleUpdateRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				http.Error(w, "invalid json body", http.StatusBadRequest)
				return
			}

			tenantUUID, err := uuid.Parse(req.TenantID)
			if err != nil {
				http.Error(w, "invalid tenant_id", http.StatusBadRequest)
				return
			}
			requesterUUID, err := uuid.Parse(req.RequesterAgentID)
			if err != nil {
				http.Error(w, "invalid requester_agent_id", http.StatusBadRequest)
				return
			}
			targetUUID, err := uuid.Parse(req.TargetAgentID)
			if err != nil {
				http.Error(w, "invalid target_agent_id", http.StatusBadRequest)
				return
			}

			requesterRole, err := db.GetAgentRole(r.Context(), pgtype.UUID{Bytes: tenantUUID, Valid: true}, pgtype.UUID{Bytes: requesterUUID, Valid: true})
			if err != nil || !core.Can(requesterRole, core.ActionManageRoles) {
				if telemetry != nil {
					telemetry.RecordAuthDenied()
				}
				http.Error(w, "permission denied", http.StatusForbidden)
				return
			}

			role := core.NormalizeRole(req.Role)
			if err := db.SetAgentRole(r.Context(), pgtype.UUID{Bytes: tenantUUID, Valid: true}, pgtype.UUID{Bytes: targetUUID, Valid: true}, string(role)); err != nil {
				http.Error(w, "failed to update role", http.StatusInternalServerError)
				return
			}

			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{"success": true, "role": role})
		})
		server := &http.Server{Addr: ":" + metricsPort, Handler: mux}
		log.Printf("Metrics endpoint listening on %s", metricsPort)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("metrics server stopped: %v", err)
		}
	}()

	// Initialize Distributed Memory Fabric (Phase 4)
	nodeID := os.Getenv("NODE_ID")
	if nodeID == "" {
		nodeID = "node-1"
	}
	nodeHost := os.Getenv("NODE_HOST")
	if nodeHost == "" {
		nodeHost = "localhost"
	}

	publisher := fabric.NewEventPublisher(nc)
	subscriber := fabric.NewEventSubscriber(nc)
	sharding := fabric.NewShardingStrategy(3) // 3-way replication

	gossip := fabric.NewGossipProtocol(nodeID, nodeHost, 50051, nc)
	if err := gossip.Start(ctx); err != nil {
		log.Fatalf("Failed to start gossip protocol: %v", err)
	}
	log.Printf("Gossip protocol started for node %s", nodeID)

	replicationMgr := fabric.NewReplicationManager(gossip, sharding, publisher, subscriber, qdrantStore, db, telemetry)
	if err := replicationMgr.Start(ctx); err != nil {
		log.Fatalf("Failed to start replication manager: %v", err)
	}
	log.Println("Replication manager started")

	// Anti-Entropy Reconciliation (Phase 4 — eventual consistency)
	antiEntropy := fabric.NewAntiEntropyManager(nodeID, gossip, sharding, publisher, subscriber, db, qdrantStore)
	if err := antiEntropy.Start(ctx); err != nil {
		log.Printf("Warning: Failed to start anti-entropy manager: %v", err)
	}
	log.Println("Anti-entropy manager started")

	// Memory Aging Pipeline (Phase 5)
	agingWorker := core.NewAgingWorker(db, qdrantStore)
	agingWorker.Start(ctx)

	// Memory Consolidation Pipeline (Phase 1.3)
	consolidator := core.NewMemoryConsolidator(db, qdrantStore, summarizer)
	go consolidator.Run(ctx, 4*time.Hour) // Consolidate every 4 hours

	// Initialize gRPC Server
	lis, err := net.Listen("tcp", ":"+cfg.GRPCPort)
	if err != nil {
		log.Fatalf("Failed to listen: %v", err)
	}

	s := grpc.NewServer()
	handler := api.NewMemoryHandler(db, qdrantStore, neo4jStore, embedder)
	handler.SetDistributedFabric(publisher, sharding, gossip)
	handler.SetTelemetry(telemetry)
	pb.RegisterMemoryServiceServer(s, handler)

	// Register reflection service on gRPC server for debugging
	reflection.Register(s)

	log.Printf("MemOS Server listening on %s", cfg.GRPCPort)

	go func() {
		if err := s.Serve(lis); err != nil {
			log.Fatalf("Failed to serve: %v", err)
		}
	}()

	<-ctx.Done()
	log.Println("Shutting down MemOS Server...")
	reflectionWorker.Stop()
	agingWorker.Stop()
	antiEntropy.Stop()
	replicationMgr.Stop()
	gossip.Stop(ctx)
	s.GracefulStop()
}
