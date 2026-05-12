-- Initial Schema for MemOS

CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

CREATE TABLE IF NOT EXISTS tenants (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name TEXT NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS agents (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    role TEXT NOT NULL DEFAULT 'writer',
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS memories (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    agent_id UUID NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
    type TEXT NOT NULL, -- episodic, semantic, etc.
    content TEXT NOT NULL,
    importance FLOAT DEFAULT 0.5,
    metadata JSONB DEFAULT '{}',
    last_accessed TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    retrieval_count BIGINT DEFAULT 0,
    reinforcement_score FLOAT DEFAULT 0.0,
    decay_factor FLOAT DEFAULT 0.1, -- Lambda in the decay formula
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS audit_logs (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id UUID NOT NULL,
    agent_id UUID NOT NULL,
    memory_id UUID,
    action TEXT NOT NULL,
    resource_type TEXT NOT NULL,
    status TEXT NOT NULL,
    metadata JSONB DEFAULT '{}',
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

ALTER TABLE tenants ENABLE ROW LEVEL SECURITY;
ALTER TABLE agents ENABLE ROW LEVEL SECURITY;
ALTER TABLE memories ENABLE ROW LEVEL SECURITY;
ALTER TABLE audit_logs ENABLE ROW LEVEL SECURITY;

ALTER TABLE tenants FORCE ROW LEVEL SECURITY;
ALTER TABLE agents FORCE ROW LEVEL SECURITY;
ALTER TABLE memories FORCE ROW LEVEL SECURITY;
ALTER TABLE audit_logs FORCE ROW LEVEL SECURITY;

DROP POLICY IF EXISTS tenants_tenant_isolation ON tenants;
DROP POLICY IF EXISTS agents_tenant_isolation ON agents;
DROP POLICY IF EXISTS memories_tenant_isolation ON memories;
DROP POLICY IF EXISTS audit_logs_tenant_isolation ON audit_logs;

-- RLS Policies with safe NULL handling for session variables
-- These policies use NULLIF to safely convert empty strings to NULL before UUID casting
-- When session variable is not set, NULLIF returns NULL, and the policy returns false (denies access)

CREATE POLICY tenants_tenant_isolation ON tenants
    FOR ALL
    USING (
        current_setting('app.rls_bypass', true) = 'on' 
        OR (NULLIF(current_setting('app.current_tenant', true), '') IS NOT NULL 
            AND id = NULLIF(current_setting('app.current_tenant', true), '')::uuid)
    )
    WITH CHECK (
        current_setting('app.rls_bypass', true) = 'on' 
        OR (NULLIF(current_setting('app.current_tenant', true), '') IS NOT NULL 
            AND id = NULLIF(current_setting('app.current_tenant', true), '')::uuid)
    );

CREATE POLICY agents_tenant_isolation ON agents
    FOR ALL
    USING (
        current_setting('app.rls_bypass', true) = 'on' 
        OR (NULLIF(current_setting('app.current_tenant', true), '') IS NOT NULL 
            AND tenant_id = NULLIF(current_setting('app.current_tenant', true), '')::uuid)
    )
    WITH CHECK (
        current_setting('app.rls_bypass', true) = 'on' 
        OR (NULLIF(current_setting('app.current_tenant', true), '') IS NOT NULL 
            AND tenant_id = NULLIF(current_setting('app.current_tenant', true), '')::uuid)
    );

CREATE POLICY memories_tenant_isolation ON memories
    FOR ALL
    USING (
        current_setting('app.rls_bypass', true) = 'on' 
        OR (NULLIF(current_setting('app.current_tenant', true), '') IS NOT NULL 
            AND tenant_id = NULLIF(current_setting('app.current_tenant', true), '')::uuid)
    )
    WITH CHECK (
        current_setting('app.rls_bypass', true) = 'on' 
        OR (NULLIF(current_setting('app.current_tenant', true), '') IS NOT NULL 
            AND tenant_id = NULLIF(current_setting('app.current_tenant', true), '')::uuid)
    );

CREATE POLICY audit_logs_tenant_isolation ON audit_logs
    FOR ALL
    USING (
        current_setting('app.rls_bypass', true) = 'on' 
        OR (NULLIF(current_setting('app.current_tenant', true), '') IS NOT NULL 
            AND tenant_id = NULLIF(current_setting('app.current_tenant', true), '')::uuid)
    )
    WITH CHECK (
        current_setting('app.rls_bypass', true) = 'on' 
        OR (NULLIF(current_setting('app.current_tenant', true), '') IS NOT NULL 
            AND tenant_id = NULLIF(current_setting('app.current_tenant', true), '')::uuid)
    );

CREATE INDEX idx_memories_tenant_agent ON memories(tenant_id, agent_id);
CREATE INDEX idx_memories_type ON memories(type);
CREATE INDEX idx_memories_metadata ON memories USING GIN (metadata);
CREATE INDEX IF NOT EXISTS idx_audit_logs_tenant_created_at ON audit_logs(tenant_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_agents_tenant_role ON agents(tenant_id, role);
