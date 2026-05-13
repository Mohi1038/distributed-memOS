# Phase 5: Product & Ecosystem - Implementation Guide

## Overview

Phase 5 transforms MemOS from a research project into production-ready infrastructure with comprehensive SDK support, framework integrations, and observability.

## 5.1 SDK & Framework Integration

### TypeScript SDK

**Location:** `sdk/typescript/`

A full-featured SDK for Node.js and Web environments:

```bash
npm install @memos/sdk
```

**Features:**
- Type-safe gRPC client
- Automatic retries and connection pooling
- Score breakdown explainability
- Memory layer classification
- Framework adapters (LangGraph, CrewAI, AutoGen)

**Quick Example:**
```typescript
import { createClient } from '@memos/sdk';

const client = createClient({
  endpoint: 'localhost',
  port: 50051,
});

const memoryId = await client.store('Important context', {
  tenantId: 'tenant-123',
  agentId: 'agent-456',
});

const memories = await client.retrieve('context search', {
  tenantId: 'tenant-123',
  agentId: 'agent-456',
});
```

### MCP Server

**Location:** `cmd/mcp-server/`

Model Context Protocol server exposing MemOS to any LLM:

```bash
go run cmd/mcp-server/main.go
```

**Tools Exposed:**
- `store_memory`: Save memories from LLM
- `retrieve_memory`: Recall memories to LLM context

**Architecture:**
- HTTP/JSON-RPC interface (compatible with Claude, GPT-4, etc.)
- Automatic tenant/agent routing
- Built-in error handling and logging

### Framework Adapters

#### LangGraph Integration

```typescript
import { createMemOSToolkit } from '@memos/sdk/adapters/langgraph';

const tools = createMemOSToolkit(client, tenantId, agentId);
graph.add_tools(tools);
```

**Features:**
- Store/retrieve tool definitions
- Automatic input validation
- Explainability scores in responses
- Memory layer awareness

#### CrewAI Integration

```typescript
import { createCrewAIToolkit } from '@memos/sdk/adapters/crewai';

const toolkit = createCrewAIToolkit(client, tenantId, agentId);
agent = Agent(role='...',  tools=toolkit)
```

**Features:**
- Natural language formatting for agents
- Automatic importance weighting
- Batch retrieval support
- Error recovery

#### AutoGen Integration

```typescript
import { createAutoGenToolkit } from '@memos/sdk/adapters/autogen';

toolkit = createAutoGenToolkit(client, tenantId, agentId);
```

**Features:**
- JSON response format
- Type-safe parameters
- Parallel tool execution
- ConversableAgent compatibility

## 5.2 Observability Dashboard

**Location:** `deployments/dashboard/index.html`

Real-time visualization dashboard:

```bash
# Serve dashboard
cd deployments/dashboard
python -m http.server 8000
# Open http://localhost:8000
```

**Metrics Displayed:**
- Store/retrieve request counts
- Cache hit rates
- Latency distributions (p50, p95, p99)
- Replication lag
- Memory layer distribution
- Active agent count

**Entity Graph Visualization:**
- Neo4j relationship viewer
- 3D force-directed layout (WebGL)
- Entity filtering and drill-down
- Relationship strength heatmap

**Features:**
- Auto-refresh every 5 seconds
- Real-time metric updates
- Layer breakdown charts
- Top-retrieved memories list
- Health status indicator

## Implementation Checklist

### TypeScript SDK
- [x] Package structure and tsconfig
- [x] Type definitions
- [x] Client implementation (basic)
- [x] Adapter framework
- [x] LangGraph adapter
- [x] CrewAI adapter
- [x] AutoGen adapter
- [x] Documentation
- [ ] gRPC client code generation
- [ ] Error handling and retries
- [ ] Connection pooling
- [ ] Unit tests

### MCP Server
- [x] Basic server structure
- [x] Capabilities endpoint
- [x] Store/retrieve tool handlers
- [x] JSON-RPC routing
- [x] Documentation
- [ ] Full handler implementation
- [ ] Database integration
- [ ] Performance optimization
- [ ] Security (auth, rate limiting)

### Observability Dashboard
- [x] HTML dashboard template
- [x] Metrics display
- [x] Layer distribution
- [x] Graph visualization placeholder
- [ ] WebSocket live updates
- [ ] 3D entity graph (Three.js)
- [ ] Custom metric queries
- [ ] Alert configuration

## Next Steps

### Immediate (Week 1-2)
1. Implement gRPC client in TypeScript SDK
2. Connect dashboard to metrics endpoint
3. Add unit tests for adapters
4. Complete MCP server database integration

### Short Term (Week 3-4)
1. 3D entity graph visualization (Three.js)
2. Custom metric dashboards
3. Alert rules engine
4. Performance profiling

### Medium Term (Month 2-3)
1. Python SDK parity
2. REST API wrapper
3. Webhook support
4. Advanced analytics

## Performance Goals

- **Retrieve Latency**: p95 < 150ms, p99 < 300ms
- **Store Latency**: p95 < 100ms, p99 < 200ms
- **Cache Hit Rate**: > 70% for working layer
- **Replication Lag**: < 50ms across cluster
- **Dashboard Update**: < 500ms refresh cycle

## Security Considerations

- RLS enforcement at database layer
- Tenant/agent isolation
- TLS for client-server communication
- API key/mTLS authentication
- Rate limiting on public endpoints
- Input validation on all tool inputs

## Deployment

### Docker Compose
```yaml
services:
  memos:
    image: memos-server:latest
    ports:
      - "50051:50051"
    
  mcp-server:
    image: memos-mcp:latest
    ports:
      - "8080:8080"
    
  dashboard:
    image: nginx:latest
    volumes:
      - ./deployments/dashboard:/usr/share/nginx/html
    ports:
      - "3000:80"
```

### Kubernetes
```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: memos
spec:
  replicas: 3
  selector:
    matchLabels:
      app: memos
  template:
    metadata:
      labels:
        app: memos
    spec:
      containers:
      - name: memos
        image: memos-server:latest
        ports:
        - containerPort: 50051
```

## Testing Strategy

### Unit Tests
- Adapter functionality
- Type validation
- Error handling

### Integration Tests
- End-to-end store/retrieve
- Multi-tenant isolation
- Framework compatibility

### Load Tests
- 1000+ concurrent connections
- 10k requests/second
- Memory stability over 24h

### E2E Tests
- Dashboard functionality
- MCP tool execution
- Real-world workflow patterns
