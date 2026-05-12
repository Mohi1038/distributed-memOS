// Distributed MemOS - Storage: Polyglot Persistence and RLS
package storage

import (
	"time"

	"github.com/jackc/pgx/v5/pgtype"
)

type MemoryModel struct {
	ID         pgtype.UUID `db:"id"`
	TenantID   pgtype.UUID `db:"tenant_id"`
	AgentID            pgtype.UUID `db:"agent_id"`
	Type               string      `db:"type"`
	Content            string      `db:"content"`
	Version            int64       `db:"version"` // Vector clock or logical version
	Importance         float64     `db:"importance"`
	Metadata           []byte      `db:"metadata"`
	LastAccessed       time.Time   `db:"last_accessed"`
	RetrievalCount     int64       `db:"retrieval_count"`
	ReinforcementScore float64     `db:"reinforcement_score"`
	DecayFactor        float64     `db:"decay_factor"`
	CreatedAt          time.Time   `db:"created_at"`
	UpdatedAt          time.Time   `db:"updated_at"`
}

type AuditLogModel struct {
	ID           pgtype.UUID `db:"id"`
	TenantID     pgtype.UUID `db:"tenant_id"`
	AgentID      pgtype.UUID `db:"agent_id"`
	MemoryID     pgtype.UUID `db:"memory_id"`
	Action       string      `db:"action"`
	ResourceType string      `db:"resource_type"`
	Status       string      `db:"status"`
	Metadata     []byte      `db:"metadata"`
	CreatedAt    time.Time   `db:"created_at"`
}
