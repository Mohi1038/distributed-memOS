# MemOS TypeScript SDK

A comprehensive SDK for integrating Distributed MemOS into Node.js and Web applications.

## Features

- **Adaptive Memory Retrieval**: Semantically search memories with explainable scoring
- **Multi-Memory Layers**: Access working, episodic, long-term, and archive memories
- **Framework Integrations**: Built-in adapters for LangGraph, CrewAI, and AutoGen
- **Explainable Retrieval**: Get detailed breakdowns of why memories were selected
- **RLS Support**: Tenant and agent isolation with Row-Level Security
- **Production Transport**: Real gRPC client for store and retrieve, plus health and metrics helpers

## Installation

```bash
npm install @memos/sdk
```

## Quick Start

### Basic Usage

```typescript
import { createClient, MemoryType } from '@memos/sdk';

const client = createClient({
  endpoint: 'localhost',
  port: 50051,
});

// Store a memory
const memoryId = await client.store('User prefers decaf coffee', {
  tenantId: 'tenant-123',
  agentId: 'agent-456',
  type: MemoryType.EPISODIC,
  importance: 0.8,
});

// Retrieve related memories
const memories = await client.retrieve('coffee preferences', {
  tenantId: 'tenant-123',
  agentId: 'agent-456',
  limit: 5,
});

memories.forEach((m) => {
  console.log(`Score: ${m.score.toFixed(3)} (${m.breakdown?.layer})`);
  console.log(`Content: ${m.memory.content}`);
});
```

### With LangGraph

```typescript
import { createClient } from '@memos/sdk';
import { createMemOSToolkit } from '@memos/sdk/adapters/langgraph';

const client = createClient({
  endpoint: 'localhost',
  port: 50051,
});

const tools = createMemOSToolkit(client, 'tenant-123', 'agent-456');

// Use with LangGraph...
graph.add_tools(tools);
```

### With CrewAI

```typescript
import { createClient } from '@memos/sdk';
import { createCrewAIToolkit } from '@memos/sdk/adapters/crewai';

const client = createClient({
  endpoint: 'localhost',
  port: 50051,
});

const toolkit = createCrewAIToolkit(client, 'tenant-123', 'agent-456');

// Use with CrewAI agent...
```

### With AutoGen

```typescript
import { createClient } from '@memos/sdk';
import { createAutoGenToolkit } from '@memos/sdk/adapters/autogen';

const client = createClient({
  endpoint: 'localhost',
  port: 50051,
});

const toolkit = createAutoGenToolkit(client, 'tenant-123', 'agent-456');

// Use with AutoGen agents...
```

## API Reference

### Client Methods

#### `store(content: string, options: StoreOptions): Promise<string>`

Store a new memory for an agent.

**Parameters:**
- `content`: The memory content
- `options.tenantId`: Tenant identifier
- `options.agentId`: Agent identifier
- `options.type`: Memory type (default: EPISODIC)
- `options.importance`: Importance score 0-1 (default: 0.5)
- `options.metadata`: Optional metadata object

**Returns:** Memory ID

#### `retrieve(query: string, options: RetrieveOptions): Promise<ScoredMemory[]>`

Search for memories matching a query.

**Parameters:**
- `query`: Search query
- `options.tenantId`: Tenant identifier
- `options.agentId`: Agent identifier
- `options.limit`: Max results (default: 10)
- `options.similarityThreshold`: Min score 0-1 (default: 0.5)
- `options.alphaSemantic`: Weight for semantic similarity (default: 0.4)
- `options.betaTemporal`: Weight for temporal decay (default: 0.3)
- `options.gammaImportance`: Weight for importance (default: 0.2)

**Returns:** Array of scored memories with breakdown

#### `batchRetrieve(queries: string[], options: RetrieveOptions): Promise<ScoredMemory[][]>`

Retrieve for multiple queries in parallel.

#### `getMetrics(): Promise<TelemetrySnapshot>`

Fetch Prometheus metrics from the configured metrics endpoint, or the default `http://<endpoint>:9090/metrics`.

#### `health(): Promise<boolean>`

Check gRPC readiness using the configured timeout.

#### `getMemory(memoryId: string, tenantId: string): Promise<Memory | null>`

This method currently throws `UnsupportedOperationError` because the gRPC service contract only exposes `Store` and `Retrieve`.

#### `updateImportance(memoryId: string, importance: number, tenantId: string): Promise<void>`

This method currently throws `UnsupportedOperationError` because the gRPC service contract only exposes `Store` and `Retrieve`.

#### `deleteMemory(memoryId: string, tenantId: string): Promise<void>`

This method currently throws `UnsupportedOperationError` because the gRPC service contract only exposes `Store` and `Retrieve`.

## Score Breakdown

Retrieved memories include a `ScoreBreakdown` showing how the final score was calculated:

```typescript
{
  semanticScore: 0.85,        // Vector similarity match
  temporalScore: 0.6,         // Recency/decay adjustment
  importanceScore: 0.8,       // User-marked importance
  reinforcementScore: 0.1,    // Retrieval history boost
  layer: 'episodic'           // Memory classification
}
```

The final score is: `0.4 * semantic + 0.3 * temporal + 0.2 * importance + 0.1 * reinforcement`

## Memory Layers

- **Working**: In-memory cache (< 5 min old, actively retrieved)
- **Episodic**: Recent memories (< 30 days, in vector DB)
- **Long-Term**: Reinforced/important memories (>30 days, high importance)
- **Archive**: Cold storage (> 90 days, low importance)

## Architecture

The SDK communicates with MemOS via gRPC:

```
┌─────────────────┐
│  Your App       │
└────────┬────────┘
         │
    ┌────▼─────────────────────┐
    │  MemOS TypeScript SDK    │
    │ ┌──────────────────────┐ │
    │ │ Client               │ │
    │ ├──────────────────────┤ │
    │ │ Adapters             │ │
    │ │ - LangGraph          │ │
    │ │ - CrewAI             │ │
    │ │ - AutoGen            │ │
    │ └──────────────────────┘ │
    └────┬─────────────────────┘
         │ gRPC
    ┌────▼──────────────────────┐
    │  MemOS Service            │
    │  - Postgres (RLS)         │
    │  - Qdrant (vectors)       │
    │  - Neo4j (entity graph)   │
    └──────────────────────────┘
```

## Configuration

### Environment Variables

```bash
# MemOS Server
MEMOS_ENDPOINT=localhost
MEMOS_PORT=50051
MEMOS_TLS_ENABLED=false

# SDK
MEMOS_TENANT_ID=your-tenant
MEMOS_AGENT_ID=your-agent
```

## Examples

### Example 1: Interview Bot with Memory

```typescript
import { createClient, MemoryType } from '@memos/sdk';

const client = createClient({
  endpoint: 'localhost',
  port: 50051,
});

async function interviewBot() {
  const tenantId = 'interview-bot';
  const agentId = 'candidate-123';

  // Store candidate info
  await client.store('Candidate has 10 years experience in Go and Rust', {
    tenantId,
    agentId,
    type: MemoryType.SEMANTIC,
    importance: 0.9,
  });

  // Later: Recall context
  const memories = await client.retrieve('What is the candidates programming experience?', {
    tenantId,
    agentId,
  });

  console.log('Relevant memory found:', memories[0]?.memory.content);
}
```

### Example 2: Multi-Agent Collaboration

```typescript
async function multiAgentTask() {
  const client = createClient({ endpoint: 'localhost', port: 50051 });

  // Agent 1 stores findings
  await client.store('Market analysis shows 30% growth in Q3', {
    tenantId: 'company-x',
    agentId: 'research-agent',
    type: MemoryType.SEMANTIC,
    importance: 0.95,
  });

  // Agent 2 recalls for decision making
  const findings = await client.retrieve('market growth trends', {
    tenantId: 'company-x',
    agentId: 'strategy-agent',
    limit: 3,
  });

  console.log('Using findings for strategy:', findings);
}
```

## Performance

- **Retrieve**: ~50-100ms (p95) for vector search + ranking
- **Store**: ~30-50ms (p95) for indexing
- **Cache Hit**: <5ms (Redis layer for active sessions)

## Security

- **Tenant Isolation**: All queries enforced by RLS in PostgreSQL
- **Agent Isolation**: Agents only access their own memories
- **TLS Support**: Optional encryption for client-server communication
- **Credentials**: Support for API keys and mTLS

## Troubleshooting

### Connection refused

```
Error: Failed to connect to MemOS service
```

Make sure MemOS is running:

```bash
cd /path/to/distributed-memOS
docker-compose -f deployments/docker-compose.yml up
```

### No memories returned

Try lowering the similarity threshold:

```typescript
const memories = await client.retrieve('query', {
  tenantId,
  agentId,
  similarityThreshold: 0.3, // Lower threshold
});
```

### Permission denied

Check that tenant/agent IDs match what's configured in your MemOS instance.

## Contributing

Contributions welcome! See [CONTRIBUTING.md](../../CONTRIBUTING.md)

## License

Apache License 2.0
