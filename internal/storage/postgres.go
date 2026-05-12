// Distributed MemOS - Storage: Polyglot Persistence and RLS
package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresStore struct {
	Pool *pgxpool.Pool
}

func NewPostgresStore(ctx context.Context, connStr string) (*PostgresStore, error) {
	pool, err := pgxpool.New(ctx, connStr)
	if err != nil {
		return nil, fmt.Errorf("unable to connect to database: %v", err)
	}

	if err := pool.Ping(ctx); err != nil {
		return nil, fmt.Errorf("unable to ping database: %v", err)
	}

	return &PostgresStore{Pool: pool}, nil
}

func (s *PostgresStore) Close() {
	s.Pool.Close()
}

func (s *PostgresStore) withSession(ctx context.Context, tenantID *uuid.UUID, bypass bool, fn func(pgx.Tx) error) error {
	conn, err := s.Pool.Acquire(ctx)
	if err != nil {
		return err
	}
	defer conn.Release()

	tx, err := conn.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	if tenantID != nil {
		if _, err := tx.Exec(ctx, `SELECT set_config('app.current_tenant', $1, true)`, tenantID.String()); err != nil {
			return err
		}
	}
	if bypass {
		if _, err := tx.Exec(ctx, `SELECT set_config('app.rls_bypass', 'on', true)`); err != nil {
			return err
		}
	}

	if err := fn(tx); err != nil {
		return err
	}

	return tx.Commit(ctx)
}

func (s *PostgresStore) withTenantSession(ctx context.Context, tenantID uuid.UUID, fn func(pgx.Tx) error) error {
	return s.withSession(ctx, &tenantID, false, fn)
}

func (s *PostgresStore) withBypassSession(ctx context.Context, fn func(pgx.Tx) error) error {
	return s.withSession(ctx, nil, true, fn)
}

// EnsureAuditSchema creates the enterprise audit table if it does not exist.
func (s *PostgresStore) EnsureAuditSchema(ctx context.Context) error {
	query := `
		CREATE TABLE IF NOT EXISTS audit_logs (
			id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
			tenant_id UUID NOT NULL,
			agent_id UUID NOT NULL,
			memory_id UUID,
			action TEXT NOT NULL,
			resource_type TEXT NOT NULL,
			status TEXT NOT NULL,
			metadata JSONB DEFAULT '{}',
			created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
		)
	`
	if _, err := s.Pool.Exec(ctx, query); err != nil {
		return err
	}

	_, err := s.Pool.Exec(ctx, `CREATE INDEX IF NOT EXISTS idx_audit_logs_tenant_created_at ON audit_logs(tenant_id, created_at DESC)`)
	return err
}

// EnsureEnterpriseSchema updates the schema for RBAC and tenant-aware access.
func (s *PostgresStore) EnsureEnterpriseSchema(ctx context.Context) error {
	if err := s.EnsureAuditSchema(ctx); err != nil {
		return err
	}

	statements := []string{
		`ALTER TABLE tenants ENABLE ROW LEVEL SECURITY`,
		`ALTER TABLE agents ENABLE ROW LEVEL SECURITY`,
		`ALTER TABLE memories ENABLE ROW LEVEL SECURITY`,
		`ALTER TABLE audit_logs ENABLE ROW LEVEL SECURITY`,
		`ALTER TABLE tenants FORCE ROW LEVEL SECURITY`,
		`ALTER TABLE agents FORCE ROW LEVEL SECURITY`,
		`ALTER TABLE memories FORCE ROW LEVEL SECURITY`,
		`ALTER TABLE audit_logs FORCE ROW LEVEL SECURITY`,
		`CREATE INDEX IF NOT EXISTS idx_agents_tenant_role ON agents(tenant_id, role)`,
	}
	for _, statement := range statements {
		if _, err := s.Pool.Exec(ctx, statement); err != nil {
			return err
		}
	}

	policies := []string{
		`DROP POLICY IF EXISTS tenants_tenant_isolation ON tenants`,
		`DROP POLICY IF EXISTS agents_tenant_isolation ON agents`,
		`DROP POLICY IF EXISTS memories_tenant_isolation ON memories`,
		`DROP POLICY IF EXISTS audit_logs_tenant_isolation ON audit_logs`,
		`CREATE POLICY tenants_tenant_isolation ON tenants FOR ALL USING (current_setting('app.rls_bypass', true) = 'on' OR (NULLIF(current_setting('app.current_tenant', true), '') IS NOT NULL AND id = NULLIF(current_setting('app.current_tenant', true), '')::uuid)) WITH CHECK (current_setting('app.rls_bypass', true) = 'on' OR (NULLIF(current_setting('app.current_tenant', true), '') IS NOT NULL AND id = NULLIF(current_setting('app.current_tenant', true), '')::uuid))`,
		`CREATE POLICY agents_tenant_isolation ON agents FOR ALL USING (current_setting('app.rls_bypass', true) = 'on' OR (NULLIF(current_setting('app.current_tenant', true), '') IS NOT NULL AND tenant_id = NULLIF(current_setting('app.current_tenant', true), '')::uuid)) WITH CHECK (current_setting('app.rls_bypass', true) = 'on' OR (NULLIF(current_setting('app.current_tenant', true), '') IS NOT NULL AND tenant_id = NULLIF(current_setting('app.current_tenant', true), '')::uuid))`,
		`CREATE POLICY memories_tenant_isolation ON memories FOR ALL USING (current_setting('app.rls_bypass', true) = 'on' OR (NULLIF(current_setting('app.current_tenant', true), '') IS NOT NULL AND tenant_id = NULLIF(current_setting('app.current_tenant', true), '')::uuid)) WITH CHECK (current_setting('app.rls_bypass', true) = 'on' OR (NULLIF(current_setting('app.current_tenant', true), '') IS NOT NULL AND tenant_id = NULLIF(current_setting('app.current_tenant', true), '')::uuid))`,
		`CREATE POLICY audit_logs_tenant_isolation ON audit_logs FOR ALL USING (current_setting('app.rls_bypass', true) = 'on' OR (NULLIF(current_setting('app.current_tenant', true), '') IS NOT NULL AND tenant_id = NULLIF(current_setting('app.current_tenant', true), '')::uuid)) WITH CHECK (current_setting('app.rls_bypass', true) = 'on' OR (NULLIF(current_setting('app.current_tenant', true), '') IS NOT NULL AND tenant_id = NULLIF(current_setting('app.current_tenant', true), '')::uuid))`,
	}
	for _, statement := range policies {
		if _, err := s.Pool.Exec(ctx, statement); err != nil {
			return err
		}
	}

	return nil
}

// EnsureCognitiveSchema adds cognitive aging and reinforcement fields to the memories table.
func (s *PostgresStore) EnsureCognitiveSchema(ctx context.Context) error {
	statements := []string{
		`ALTER TABLE memories ADD COLUMN IF NOT EXISTS last_accessed TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP`,
		`ALTER TABLE memories ADD COLUMN IF NOT EXISTS retrieval_count BIGINT DEFAULT 0`,
		`ALTER TABLE memories ADD COLUMN IF NOT EXISTS reinforcement_score FLOAT DEFAULT 0.0`,
		`ALTER TABLE memories ADD COLUMN IF NOT EXISTS decay_factor FLOAT DEFAULT 0.1`,
	}
	for _, statement := range statements {
		if _, err := s.Pool.Exec(ctx, statement); err != nil {
			return err
		}
	}
	return nil
}

// EnsureDevPrincipal seeds a development tenant and agent when the database is empty.
func (s *PostgresStore) EnsureDevPrincipal(ctx context.Context, tenantID, agentID uuid.UUID) error {
	return s.withBypassSession(ctx, func(tx pgx.Tx) error {
		var tenantCount int
		if err := tx.QueryRow(ctx, `SELECT COUNT(*) FROM tenants`).Scan(&tenantCount); err != nil {
			return err
		}
		if tenantCount > 0 {
			return nil
		}

		tenantName := "memos-dev"
		agentName := "memos-dev-agent"

		if _, err := tx.Exec(ctx, `INSERT INTO tenants (id, name) VALUES ($1, $2) ON CONFLICT (id) DO NOTHING`, tenantID, tenantName); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `INSERT INTO agents (id, tenant_id, name, role) VALUES ($1, $2, $3, $4) ON CONFLICT (id) DO NOTHING`, agentID, tenantID, agentName, "owner"); err != nil {
			return err
		}
		return nil
	})
}

// WriteAuditLog records an immutable audit trail entry.
func (s *PostgresStore) WriteAuditLog(ctx context.Context, event *AuditLogModel) error {
	return s.withTenantSession(ctx, uuid.UUID(event.TenantID.Bytes), func(tx pgx.Tx) error {
		query := `
			INSERT INTO audit_logs (tenant_id, agent_id, memory_id, action, resource_type, status, metadata)
			VALUES ($1, $2, $3, $4, $5, $6, $7)
			RETURNING id, created_at
		`
		var createdAt time.Time
		if err := tx.QueryRow(ctx, query, event.TenantID, event.AgentID, event.MemoryID, event.Action, event.ResourceType, event.Status, event.Metadata).
			Scan(&event.ID, &createdAt); err != nil {
			return err
		}
		event.CreatedAt = createdAt
		return nil
	})
}

// GetAgentRole returns the role for a tenant-scoped agent.
func (s *PostgresStore) GetAgentRole(ctx context.Context, tenantID, agentID pgtype.UUID) (string, error) {
	var role string
	err := s.withTenantSession(ctx, uuid.UUID(tenantID.Bytes), func(tx pgx.Tx) error {
		query := `
			SELECT role
			FROM agents
			WHERE tenant_id = $1 AND id = $2
		`
		return tx.QueryRow(ctx, query, tenantID, agentID).Scan(&role)
	})
	if err != nil {
		return "", err
	}
	if role == "" {
		return "writer", nil
	}
	return role, nil
}

// SetAgentRole updates a tenant-scoped agent role.
func (s *PostgresStore) SetAgentRole(ctx context.Context, tenantID, agentID pgtype.UUID, role string) error {
	return s.withTenantSession(ctx, uuid.UUID(tenantID.Bytes), func(tx pgx.Tx) error {
		query := `
			UPDATE agents
			SET role = $3
			WHERE tenant_id = $1 AND id = $2
		`
		_, err := tx.Exec(ctx, query, tenantID, agentID, role)
		return err
	})
}

// ListAuditLogs returns recent audit events for a tenant.
func (s *PostgresStore) ListAuditLogs(ctx context.Context, tenantID pgtype.UUID, limit int) ([]*AuditLogModel, error) {
	if limit <= 0 {
		limit = 100
	}
	var logs []*AuditLogModel
	err := s.withTenantSession(ctx, uuid.UUID(tenantID.Bytes), func(tx pgx.Tx) error {
		query := `
			SELECT id, tenant_id, agent_id, memory_id, action, resource_type, status, metadata, created_at
			FROM audit_logs
			WHERE tenant_id = $1
			ORDER BY created_at DESC
			LIMIT $2
		`
		rows, err := tx.Query(ctx, query, tenantID, limit)
		if err != nil {
			return err
		}
		defer rows.Close()

		logs = make([]*AuditLogModel, 0, limit)
		for rows.Next() {
			var item AuditLogModel
			if err := rows.Scan(&item.ID, &item.TenantID, &item.AgentID, &item.MemoryID, &item.Action, &item.ResourceType, &item.Status, &item.Metadata, &item.CreatedAt); err != nil {
				return err
			}
			logs = append(logs, &item)
		}
		return rows.Err()
	})
	if err != nil {
		return nil, err
	}
	return logs, nil
}

func (s *PostgresStore) SaveMemory(ctx context.Context, m *MemoryModel) error {
	return s.withTenantSession(ctx, uuid.UUID(m.TenantID.Bytes), func(tx pgx.Tx) error {
		query := `
			INSERT INTO memories (tenant_id, agent_id, type, content, importance, metadata, decay_factor)
			VALUES ($1, $2, $3, $4, $5, $6, $7)
			RETURNING id, last_accessed, retrieval_count, reinforcement_score, decay_factor, created_at, updated_at
		`
		return tx.QueryRow(ctx, query, m.TenantID, m.AgentID, m.Type, m.Content, m.Importance, m.Metadata, m.DecayFactor).
			Scan(&m.ID, &m.LastAccessed, &m.RetrievalCount, &m.ReinforcementScore, &m.DecayFactor, &m.CreatedAt, &m.UpdatedAt)
	})
}

func (s *PostgresStore) GetMemoriesByAgent(ctx context.Context, tenantID, agentID pgtype.UUID) ([]*MemoryModel, error) {
	var memories []*MemoryModel
	err := s.withTenantSession(ctx, uuid.UUID(tenantID.Bytes), func(tx pgx.Tx) error {
		query := `
			SELECT id, tenant_id, agent_id, type, content, importance, metadata, 
			       last_accessed, retrieval_count, reinforcement_score, decay_factor,
			       created_at, updated_at
			FROM memories
			WHERE tenant_id = $1 AND agent_id = $2
		`
		rows, err := tx.Query(ctx, query, tenantID, agentID)
		if err != nil {
			return err
		}
		defer rows.Close()

		memories = make([]*MemoryModel, 0)
		for rows.Next() {
			var m MemoryModel
			if err := rows.Scan(&m.ID, &m.TenantID, &m.AgentID, &m.Type, &m.Content, &m.Importance, &m.Metadata, &m.CreatedAt, &m.UpdatedAt); err != nil {
				return err
			}
			memories = append(memories, &m)
		}
		return rows.Err()
	})
	if err != nil {
		return nil, err
	}
	return memories, nil
}

func (s *PostgresStore) GetMemoryByID(ctx context.Context, id string) (*MemoryModel, error) {
	query := `
		SELECT id, tenant_id, agent_id, type, content, importance, metadata, created_at, updated_at
		FROM memories
		WHERE id = $1
	`
	var m MemoryModel
	err := s.Pool.QueryRow(ctx, query, id).Scan(&m.ID, &m.TenantID, &m.AgentID, &m.Type, &m.Content, &m.Importance, &m.Metadata, &m.CreatedAt, &m.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &m, nil
}

// GetMemoryByIDForTenant returns a memory only if it belongs to the tenant.
func (s *PostgresStore) GetMemoryByIDForTenant(ctx context.Context, tenantID pgtype.UUID, id string) (*MemoryModel, error) {
	var m MemoryModel
	err := s.withTenantSession(ctx, uuid.UUID(tenantID.Bytes), func(tx pgx.Tx) error {
		query := `
			SELECT id, tenant_id, agent_id, type, content, importance, metadata, 
			       last_accessed, retrieval_count, reinforcement_score, decay_factor,
			       created_at, updated_at
			FROM memories
			WHERE id = $1 AND tenant_id = $2
		`
		return tx.QueryRow(ctx, query, id, tenantID).Scan(&m.ID, &m.TenantID, &m.AgentID, &m.Type, &m.Content, &m.Importance, &m.Metadata, &m.CreatedAt, &m.UpdatedAt)
	})
	if err != nil {
		return nil, err
	}
	return &m, nil
}

// GetEpisodicCandidates retrieves episodic memories older than minAge with importance > threshold.
func (s *PostgresStore) GetEpisodicCandidates(ctx context.Context, minAge time.Duration, importanceThreshold float64) ([]*MemoryModel, error) {
	cutoffTime := time.Now().Add(-minAge)
	var memories []*MemoryModel
	err := s.withBypassSession(ctx, func(tx pgx.Tx) error {
		query := `
			SELECT id, tenant_id, agent_id, type, content, importance, metadata, 
			       last_accessed, retrieval_count, reinforcement_score, decay_factor,
			       created_at, updated_at
			FROM memories
			WHERE type = 'MEMORY_TYPE_EPISODIC'
			AND created_at < $1
			AND importance > $2
			AND (metadata->>'reflected' IS NULL OR metadata->>'reflected' != 'true')
			ORDER BY created_at DESC
			LIMIT 100
		`
		rows, err := tx.Query(ctx, query, cutoffTime, importanceThreshold)
		if err != nil {
			return err
		}
		defer rows.Close()

		memories = make([]*MemoryModel, 0)
		for rows.Next() {
			var m MemoryModel
			if err := rows.Scan(&m.ID, &m.TenantID, &m.AgentID, &m.Type, &m.Content, &m.Importance, &m.Metadata, &m.CreatedAt, &m.UpdatedAt); err != nil {
				return err
			}
			memories = append(memories, &m)
		}
		return rows.Err()
	})
	if err != nil {
		return nil, err
	}
	return memories, nil
}

// MarkAsReflected marks a memory as having been converted to semantic.
func (s *PostgresStore) MarkAsReflected(ctx context.Context, id string) error {
	return s.withBypassSession(ctx, func(tx pgx.Tx) error {
		query := `
			UPDATE memories
			SET metadata = jsonb_set(metadata, '{reflected}', 'true'::jsonb),
			    updated_at = NOW()
			WHERE id = $1
		`
		_, err := tx.Exec(ctx, query, id)
		return err
	})
}

// encodeAuditMetadata safely serializes audit metadata.
func encodeAuditMetadata(v interface{}) []byte {
	data, err := json.Marshal(v)
	if err != nil {
		return []byte(`{}`)
	}
	return data
}

// GetMemoryByIDRaw fetches a memory by ID bypassing RLS (used internally by conflict resolver).
func (s *PostgresStore) GetMemoryByIDRaw(ctx context.Context, id string) (*MemoryModel, error) {
	var m MemoryModel
	err := s.withBypassSession(ctx, func(tx pgx.Tx) error {
		query := `
			SELECT id, tenant_id, agent_id, type, content, importance, metadata, 
			       last_accessed, retrieval_count, reinforcement_score, decay_factor,
			       created_at, updated_at
			FROM memories
			WHERE id = $1
		`
		return tx.QueryRow(ctx, query, id).Scan(
			&m.ID, &m.TenantID, &m.AgentID, &m.Type, &m.Content,
			&m.Importance, &m.Metadata, &m.CreatedAt, &m.UpdatedAt,
		)
	})
	if err != nil {
		return nil, err
	}
	return &m, nil
}

// ReduceImportance lowers a memory's importance by delta (soft conflict deprecation).
// Importance is clamped to a minimum of 0.0.
func (s *PostgresStore) ReduceImportance(ctx context.Context, id string, delta float64) error {
	return s.withBypassSession(ctx, func(tx pgx.Tx) error {
		query := `
			UPDATE memories
			SET importance  = GREATEST(0.0, importance - $2),
			    metadata    = jsonb_set(COALESCE(metadata, '{}'), '{conflict_loser}', 'true'::jsonb),
			    updated_at  = NOW()
			WHERE id = $1
		`
		_, err := tx.Exec(ctx, query, id, delta)
		return err
	})
}

// ListAllMemoriesForAntiEntropy returns lightweight memory records (no content blob)
// for all tenants. Used by the anti-entropy manager to compute shard checksums.
func (s *PostgresStore) ListAllMemoriesForAntiEntropy(ctx context.Context) ([]*MemoryModel, error) {
	var memories []*MemoryModel
	err := s.withBypassSession(ctx, func(tx pgx.Tx) error {
		query := `
			SELECT id, tenant_id, agent_id, type, importance, created_at, updated_at
			FROM memories
			ORDER BY created_at DESC
		`
		rows, err := tx.Query(ctx, query)
		if err != nil {
			return err
		}
		defer rows.Close()

		for rows.Next() {
			var m MemoryModel
			if err := rows.Scan(&m.ID, &m.TenantID, &m.AgentID, &m.Type, &m.Importance, &m.CreatedAt, &m.UpdatedAt); err != nil {
				return err
			}
			memories = append(memories, &m)
		}
		return rows.Err()
	})
	return memories, err
}

// GetColdMemories returns episodic memories older than a certain duration with importance below a threshold.
func (s *PostgresStore) GetColdMemories(ctx context.Context, olderThan time.Duration, maxImportance float64) ([]*MemoryModel, error) {
	var memories []*MemoryModel
	err := s.withBypassSession(ctx, func(tx pgx.Tx) error {
		query := `
			SELECT id, tenant_id, agent_id, type, content, importance, metadata, created_at, updated_at
			FROM memories
			WHERE type = 'MEMORY_TYPE_EPISODIC'
			  AND importance < $1
			  AND last_accessed < NOW() - $2::interval
			  AND reinforcement_score < $1 * 2 -- Reinforced memories survive longer
			  AND (metadata->>'archived') IS NULL
			ORDER BY created_at ASC
			LIMIT 50
		`
		rows, err := tx.Query(ctx, query, maxImportance, fmt.Sprintf("%d seconds", int(olderThan.Seconds())))
		if err != nil {
			return err
		}
		defer rows.Close()

		for rows.Next() {
			var m MemoryModel
			if err := rows.Scan(&m.ID, &m.TenantID, &m.AgentID, &m.Type, &m.Content, &m.Importance, &m.Metadata, &m.CreatedAt, &m.UpdatedAt); err != nil {
				return err
			}
			memories = append(memories, &m)
		}
		return rows.Err()
	})
	return memories, err
}

// ArchiveMemory sets the archived flag in metadata.
func (s *PostgresStore) ArchiveMemory(ctx context.Context, id string) error {
	return s.withBypassSession(ctx, func(tx pgx.Tx) error {
		query := `
			UPDATE memories
			SET metadata = jsonb_set(COALESCE(metadata, '{}'), '{archived}', 'true'::jsonb),
			    updated_at = NOW()
			WHERE id = $1
		`
		_, err := tx.Exec(ctx, query, id)
		return err
	})
}

// DeleteMemory fully removes a memory from PostgreSQL.
func (s *PostgresStore) DeleteMemory(ctx context.Context, id string) error {
	return s.withBypassSession(ctx, func(tx pgx.Tx) error {
		query := `DELETE FROM memories WHERE id = $1`
		_, err := tx.Exec(ctx, query, id)
		return err
	})
}
// IncrementRetrievalStats updates the last_accessed time and increments the count/reinforcement.
func (s *PostgresStore) IncrementRetrievalStats(ctx context.Context, id string, reinforcementDelta float64) error {
	return s.withBypassSession(ctx, func(tx pgx.Tx) error {
		query := `
			UPDATE memories
			SET last_accessed = NOW(),
			    retrieval_count = retrieval_count + 1,
			    reinforcement_score = reinforcement_score + $2,
			    updated_at = NOW()
			WHERE id = $1
		`
		_, err := tx.Exec(ctx, query, id, reinforcementDelta)
		return err
	})
}

// UpdateDecayFactor adjusts the decay rate for a memory (used by reinforcement logic).
func (s *PostgresStore) UpdateDecayFactor(ctx context.Context, id string, factor float64) error {
	return s.withBypassSession(ctx, func(tx pgx.Tx) error {
		query := `UPDATE memories SET decay_factor = $2, updated_at = NOW() WHERE id = $1`
		_, err := tx.Exec(ctx, query, id, factor)
		return err
	})
}
