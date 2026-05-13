package api

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/mohi1038/memos/internal/core"
	"github.com/mohi1038/memos/internal/storage"
	"golang.org/x/net/websocket"
)

// DashboardService powers the live dashboard summary and websocket stream.
type DashboardService struct {
	store     *storage.PostgresStore
	telemetry *core.Telemetry
}

// NewDashboardService creates a new dashboard service.
func NewDashboardService(store *storage.PostgresStore, telemetry *core.Telemetry) *DashboardService {
	return &DashboardService{store: store, telemetry: telemetry}
}

type dashboardSnapshot struct {
	GeneratedAt    time.Time         `json:"generatedAt"`
	Metrics        dashboardMetrics  `json:"metrics"`
	MemoryTypes    []dashboardCount  `json:"memoryTypes"`
	MemoryLayers   []dashboardCount  `json:"memoryLayers"`
	RecentMemories []dashboardMemory `json:"recentMemories"`
	Agents         []dashboardAgent  `json:"agents"`
	ActivityByHour []int             `json:"activityByHour"`
	Insights       []string          `json:"insights"`
	Health         dashboardHealth   `json:"health"`
	Feed           []dashboardEvent  `json:"feed"`
}

type dashboardMetrics struct {
	TotalMemories        int64   `json:"totalMemories"`
	ActiveAgents         int64   `json:"activeAgents"`
	StoreOps             int64   `json:"storeOps"`
	RetrieveOps          int64   `json:"retrieveOps"`
	AuditWrites          int64   `json:"auditWrites"`
	AuditReads           int64   `json:"auditReads"`
	CacheHitRate         float64 `json:"cacheHitRate"`
	StoreLatencyAvgMs    float64 `json:"storeLatencyAvgMs"`
	RetrieveLatencyAvgMs float64 `json:"retrieveLatencyAvgMs"`
	ReplicationLagAvgMs  float64 `json:"replicationLagAvgMs"`
	ReplicationLagMaxMs  float64 `json:"replicationLagMaxMs"`
	LiveRetrievals       int64   `json:"liveRetrievals"`
}

type dashboardCount struct {
	Label string `json:"label"`
	Value int64  `json:"value"`
}

type dashboardMemory struct {
	ID           string  `json:"id"`
	Preview      string  `json:"preview"`
	Layer        string  `json:"layer"`
	Type         string  `json:"type"`
	Score        float64 `json:"score"`
	AccessCount  int64   `json:"accessCount"`
	LastAccessed string  `json:"lastAccessed"`
	AgentID      string  `json:"agentId"`
}

type dashboardAgent struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Memories     int64  `json:"memories"`
	LastActivity string `json:"lastActivity"`
}

type dashboardEvent struct {
	Label     string `json:"label"`
	Meta      string `json:"meta"`
	Timestamp string `json:"timestamp"`
}

type dashboardHealth struct {
	Status   string `json:"status"`
	Database string `json:"database"`
	Vector   string `json:"vector"`
	Graph    string `json:"graph"`
	Metrics  string `json:"metrics"`
}

// SummaryHandler returns a JSON dashboard snapshot.
func (s *DashboardService) SummaryHandler(w http.ResponseWriter, r *http.Request) {
	allowDashboardCORS(w, r)
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	snapshot, err := s.snapshot(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(snapshot)
}

// StreamHandler returns a websocket handler for live dashboard updates.
func (s *DashboardService) StreamHandler() http.Handler {
	return websocket.Handler(func(ws *websocket.Conn) {
		defer ws.Close()

		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()

		for {
			snapshot, err := s.snapshot(context.Background())
			if err != nil {
				log.Printf("dashboard websocket snapshot error: %v", err)
				return
			}
			if err := websocket.JSON.Send(ws, snapshot); err != nil {
				return
			}
			<-ticker.C
		}
	})
}

func (s *DashboardService) snapshot(ctx context.Context) (*dashboardSnapshot, error) {
	if s.store == nil {
		return nil, fmt.Errorf("dashboard store is not configured")
	}

	telemetrySnapshot := core.TelemetrySnapshot{}
	if s.telemetry != nil {
		telemetrySnapshot = s.telemetry.Snapshot()
	}

	totalMemories, err := s.store.CountMemories(ctx)
	if err != nil {
		return nil, err
	}

	activeAgents, err := s.store.CountDistinctAgents(ctx)
	if err != nil {
		return nil, err
	}

	recentMemories, err := s.store.GetRecentMemoriesForDashboard(ctx, 250)
	if err != nil {
		return nil, err
	}

	dashboardMemories := renderRecentMemories(recentMemories)
	memoryTypes := aggregateMemoryTypes(recentMemories)
	memoryLayers := aggregateMemoryLayers(recentMemories)
	agents := aggregateAgents(recentMemories)
	activityByHour := aggregateActivityByHour(recentMemories)
	feed := buildFeed(recentMemories)
	insights := buildInsights(telemetrySnapshot, memoryLayers, memoryTypes, activeAgents)
	health := dashboardHealth{
		Status:   "healthy",
		Database: "online",
		Vector:   "online",
		Graph:    "online",
		Metrics:  "online",
	}

	return &dashboardSnapshot{
		GeneratedAt: time.Now().UTC(),
		Metrics: dashboardMetrics{
			TotalMemories:        totalMemories,
			ActiveAgents:         activeAgents,
			StoreOps:             telemetrySnapshot.StoreCount,
			RetrieveOps:          telemetrySnapshot.RetrieveCount,
			AuditWrites:          telemetrySnapshot.AuditWriteCount,
			AuditReads:           telemetrySnapshot.AuditReadCount,
			CacheHitRate:         telemetrySnapshot.CacheHitRate,
			StoreLatencyAvgMs:    telemetrySnapshot.StoreLatencyAvgMs,
			RetrieveLatencyAvgMs: telemetrySnapshot.RetrieveLatencyAvgMs,
			ReplicationLagAvgMs:  telemetrySnapshot.ReplicationLagAvgMs,
			ReplicationLagMaxMs:  telemetrySnapshot.ReplicationLagMaxMs,
			LiveRetrievals:       telemetrySnapshot.RetrieveCount,
		},
		MemoryTypes:    memoryTypes,
		MemoryLayers:   memoryLayers,
		RecentMemories: dashboardMemories,
		Agents:         agents,
		ActivityByHour: activityByHour,
		Insights:       insights,
		Health:         health,
		Feed:           feed,
	}, nil
}

func aggregateMemoryTypes(memories []*storage.MemoryModel) []dashboardCount {
	counts := map[string]int64{}
	for _, mem := range memories {
		label := normalizeType(mem.Type)
		counts[label]++
	}
	return sortedCounts(counts)
}

func renderRecentMemories(memories []*storage.MemoryModel) []dashboardMemory {
	rendered := make([]dashboardMemory, 0, len(memories))
	for _, mem := range memories {
		rendered = append(rendered, dashboardMemory{
			ID:           mem.ID.String(),
			Preview:      mem.Content,
			Layer:        classifyLayer(mem, time.Now()),
			Type:         normalizeType(mem.Type),
			Score:        computeMemoryScore(mem, time.Now()),
			AccessCount:  mem.RetrievalCount,
			LastAccessed: mem.UpdatedAt.Format("15:04:05"),
			AgentID:      mem.AgentID.String(),
		})
	}
	return rendered
}

func aggregateMemoryLayers(memories []*storage.MemoryModel) []dashboardCount {
	counts := map[string]int64{}
	now := time.Now()
	for _, mem := range memories {
		label := classifyLayer(mem, now)
		counts[label]++
	}
	return sortedCounts(counts)
}

func aggregateAgents(memories []*storage.MemoryModel) []dashboardAgent {
	byAgent := map[string]int64{}
	latest := map[string]time.Time{}
	for _, mem := range memories {
		agentID := mem.AgentID.String()
		if agentID == "" || agentID == "00000000-0000-0000-0000-000000000000" {
			continue
		}
		byAgent[agentID]++
		if ts, ok := latest[agentID]; !ok || mem.UpdatedAt.After(ts) {
			latest[agentID] = mem.UpdatedAt
		}
	}

	agents := make([]dashboardAgent, 0, len(byAgent))
	for agentID, count := range byAgent {
		agents = append(agents, dashboardAgent{
			ID:           agentID,
			Name:         fmt.Sprintf("Agent %s", shortID(agentID)),
			Memories:     count,
			LastActivity: latest[agentID].Format("15:04:05"),
		})
	}
	sort.Slice(agents, func(i, j int) bool { return agents[i].Memories > agents[j].Memories })
	if len(agents) > 6 {
		agents = agents[:6]
	}
	return agents
}

func aggregateActivityByHour(memories []*storage.MemoryModel) []int {
	activity := make([]int, 24)
	for _, mem := range memories {
		hour := mem.UpdatedAt.Hour()
		if hour < 0 || hour > 23 {
			continue
		}
		activity[hour]++
	}
	return activity
}

func buildFeed(memories []*storage.MemoryModel) []dashboardEvent {
	feed := make([]dashboardEvent, 0, 8)
	for _, mem := range memories {
		feed = append(feed, dashboardEvent{
			Label:     fmt.Sprintf("%s · %s", normalizeType(mem.Type), classifyLayer(mem, time.Now())),
			Meta:      fmt.Sprintf("Score %.2f · %d accesses", computeMemoryScore(mem, time.Now()), mem.RetrievalCount),
			Timestamp: mem.UpdatedAt.Format("15:04:05"),
		})
		if len(feed) == 8 {
			break
		}
	}
	return feed
}

func buildInsights(telemetrySnapshot core.TelemetrySnapshot, layers []dashboardCount, types []dashboardCount, activeAgents int64) []string {
	insights := []string{}
	if telemetrySnapshot.CacheHitRate >= 75 {
		insights = append(insights, fmt.Sprintf("Cache hit rate is strong at %.1f%%.", telemetrySnapshot.CacheHitRate))
	} else {
		insights = append(insights, fmt.Sprintf("Cache hit rate is %.1f%% and can still be improved.", telemetrySnapshot.CacheHitRate))
	}
	if telemetrySnapshot.RetrieveLatencyAvgMs > 0 {
		insights = append(insights, fmt.Sprintf("Average retrieval latency is %.1fms.", telemetrySnapshot.RetrieveLatencyAvgMs))
	}
	if len(layers) > 0 {
		insights = append(insights, fmt.Sprintf("Most active layer: %s.", layers[0].Label))
	}
	if activeAgents > 0 {
		insights = append(insights, fmt.Sprintf("Active agents observed: %d.", activeAgents))
	}
	if len(types) > 0 {
		insights = append(insights, fmt.Sprintf("Dominant memory type: %s.", types[0].Label))
	}
	return insights
}

func sortedCounts(counts map[string]int64) []dashboardCount {
	items := make([]dashboardCount, 0, len(counts))
	for label, value := range counts {
		items = append(items, dashboardCount{Label: label, Value: value})
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Value > items[j].Value })
	return items
}

func normalizeType(memoryType string) string {
	switch strings.TrimSpace(memoryType) {
	case "MEMORY_TYPE_EPISODIC":
		return "episodic"
	case "MEMORY_TYPE_SEMANTIC":
		return "semantic"
	case "MEMORY_TYPE_PROCEDURAL":
		return "procedural"
	case "MEMORY_TYPE_REFLECTIVE":
		return "reflective"
	case "MEMORY_TYPE_TRANSIENT":
		return "transient"
	default:
		return strings.ToLower(strings.TrimPrefix(memoryType, "MEMORY_TYPE_"))
	}
}

func classifyLayer(mem *storage.MemoryModel, now time.Time) string {
	age := now.Sub(mem.UpdatedAt)
	if age < 10*time.Minute && mem.RetrievalCount > 0 {
		return "working"
	}
	if mem.Importance >= 0.75 || mem.ReinforcementScore >= 10 {
		return "long_term"
	}
	if age > 30*24*time.Hour || mem.RetrievalCount > 12 {
		return "archive"
	}
	return "episodic"
}

func computeMemoryScore(mem *storage.MemoryModel, now time.Time) float64 {
	ageHours := now.Sub(mem.UpdatedAt).Hours()
	ageScore := math.Max(0, 1-(ageHours/240))
	retrievalScore := math.Min(1, float64(mem.RetrievalCount)/20)
	reinforcementScore := math.Min(1, mem.ReinforcementScore/20)
	importanceScore := math.Min(1, mem.Importance)
	return (ageScore * 0.2) + (retrievalScore * 0.3) + (reinforcementScore * 0.25) + (importanceScore * 0.25)
}

func shortID(id string) string {
	if len(id) <= 8 {
		return id
	}
	return id[:8]
}

func allowDashboardCORS(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
	}
}
