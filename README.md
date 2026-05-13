<div align="center">
  <img src="logo.png" alt="MemOS Logo" width="160" />
  <h1>Distributed MemOS</h1>
  <p><em>Distributed adaptive memory infrastructure for AI agents</em></p>

  [![Go Version](https://img.shields.io/badge/Go-1.22+-00ADD8?style=for-the-badge&logo=go)](https://golang.org/)
  [![Python SDK](https://img.shields.io/badge/Python-SDK-3776AB?style=for-the-badge&logo=python)](https://pypi.org/project/memos-sdk/)
  [![TypeScript SDK](https://img.shields.io/badge/TypeScript-SDK-3178C6?style=for-the-badge&logo=typescript)](sdk/typescript/README.md)
  [![gRPC](https://img.shields.io/badge/API-gRPC-244c5a?style=for-the-badge)](proto/memory.proto)
  [![License](https://img.shields.io/badge/License-MIT-green.svg?style=for-the-badge)](LICENSE)
</div>

---

## What is MemOS?

MemOS is a distributed adaptive memory infrastructure for AI agents. It enables agents to store, retrieve, reinforce, and evolve memory over time, with support for distributed consistency, memory decay, explainable retrieval, and contextual ranking.

MemOS is built for long-running agent systems where memory needs to survive beyond a single prompt, process, or machine.

- **Persistent memory across sessions and agents** through a tenant-aware memory service.
- **Adaptive retrieval** using semantic relevance, temporal decay, importance, and reinforcement.
- **Distributed synchronization with automatic repair** using gossip, NATS replication, sharding, Merkle comparison, and anti-entropy repair.

```mermaid
flowchart LR
    classDef agent fill:#e3f2fd,stroke:#1565c0,stroke-width:2px,color:#0d1b2a;
    classDef memos fill:#e8f5e9,stroke:#2e7d32,stroke-width:2px,color:#102a12;
    classDef fabric fill:#fff3e0,stroke:#ef6c00,stroke-width:2px,color:#2b1700;
    classDef result fill:#f3e5f5,stroke:#6a1b9a,stroke-width:2px,color:#1d1026;

    Agent([AI Agent]):::agent --> Store[Store interaction]:::agent
    Store --> MemOS[MemOS Memory Engine]:::memos
    MemOS --> Rank[Adaptive ranking]:::memos
    MemOS --> Fabric[Distributed sync + repair]:::fabric
    Rank --> Context[Ranked context]:::result
    Context --> Decision([Next decision]):::agent
```

<!-- toc -->
- [What Actually Works](#what-actually-works)
- [Why MemOS vs Vector Databases](#why-memos-vs-vector-databases)
- [Why MemOS vs Managed Memory APIs](#why-memos-vs-managed-memory-apis)
- [When To Use MemOS](#when-to-use-memos)
- [Quick Start](#quick-start)
- [Agent Integration](#agent-integration)
- [Memory Lifecycle](#memory-lifecycle)
- [Architecture Layers](#architecture-layers)
- [Distributed Architecture](#distributed-architecture)
  - [Cluster Topology](#cluster-topology)
  - [Replication Pipeline](#replication-pipeline)
  - [Anti-Entropy & Consistency](#anti-entropy--consistency)
- [System Components](#system-components)
- [Cognitive Retrieval Pipeline](#cognitive-retrieval-pipeline)
- [Explainable Retrieval](#explainable-retrieval)
- [Distributed Behavior](#distributed-behavior)
- [Data Model & Isolation](#data-model--isolation)
- [Evaluation](#evaluation)
- [Installation](#installation)
- [Running Locally](#running-locally)
- [Observability & Automation](#observability--automation)
- [Repository Map](#repository-map)
- [License](#license)
<!-- tocstop -->

## What Actually Works

MemOS is not only an architecture sketch. The repository includes a running Go service, SDKs, storage adapters, ranking logic, distributed fabric primitives, benchmark scripts, and generated reports.

| Area | Status | Proof |
| :--- | :--- | :--- |
| gRPC memory API | Store and retrieve service contract | [proto/memory.proto](proto/memory.proto) |
| Go memory node | Service entrypoint and handlers | [cmd/memos/main.go](cmd/memos/main.go), [internal/api/memory_handler.go](internal/api/memory_handler.go) |
| Adaptive ranking | Semantic, temporal, importance, reinforcement scoring | [internal/core/ranking.go](internal/core/ranking.go) |
| Explainable retrieval | Score breakdown in API and SDK types | [proto/memory.proto](proto/memory.proto), [sdk/typescript/src/types.ts](sdk/typescript/src/types.ts) |
| Memory aging | Archive/delete pipeline for cold low-value memory | [internal/core/aging.go](internal/core/aging.go) |
| Consolidation | Episodic-to-semantic consolidation worker | [internal/core/consolidation.go](internal/core/consolidation.go) |
| Conflict handling | Conflict-aware core module | [internal/core/conflict.go](internal/core/conflict.go) |
| Distributed fabric | Gossip, replication, sharding, Merkle, anti-entropy | [internal/fabric](internal/fabric) |
| Python SDK | Installable gRPC client | [sdk/python/README.md](sdk/python/README.md) |
| TypeScript SDK | Node SDK with framework adapters | [sdk/typescript/README.md](sdk/typescript/README.md) |
| Evaluation suite | Gold-standard retrieval metrics and reports | [scripts/benchmarks](scripts/benchmarks) |
| Dashboards | Prometheus/Grafana/dashboard assets | [deployments](deployments) |

## Why MemOS vs Vector Databases

Vector databases are excellent similarity search systems. MemOS uses vector search as one signal, then adds the lifecycle and distributed behavior that agent memory needs.

| Problem | Vector DB | MemOS |
| :--- | :--- | :--- |
| Memory growth | Retrieval quality can degrade as context accumulates | Adaptive decay, archival, and consolidation |
| Relevance | Static similarity | Multi-factor scoring across semantic, time, importance, reinforcement |
| Memory conflicts | Usually not modeled | Conflict-aware storage path |
| Multi-agent sync | Manual app-level coordination | Built-in gossip, NATS replication, sharding, and anti-entropy repair |
| Forgetting | None by default | Controlled decay and cold-memory archival |
| Debuggability | Often returns only similarity | Explainable score breakdown |
| Tenant isolation | Often handled in application code | PostgreSQL Row-Level Security model |

MemOS is closest to a memory infrastructure layer: it can sit behind LangGraph, CrewAI, AutoGen, custom agents, n8n flows, or internal AI systems that need memory to persist and evolve.

## Why MemOS vs Managed Memory APIs

Managed memory systems and memory APIs are useful when you want to add agent memory quickly. MemOS is useful when you want the memory layer itself to be inspectable, self-hosted, distributed, and tunable.

| Need | Managed memory API | MemOS |
| :--- | :--- | :--- |
| Fast integration | Strong default | SDKs plus self-hosted service |
| Infrastructure control | Usually abstracted away | Full Go service, storage, fabric, and benchmark code in-repo |
| Distributed repair | Product-dependent | Explicit gossip, shards, Merkle checks, and anti-entropy repair |
| Retrieval tuning | Product-dependent | Exposed ranking weights and score breakdown |
| Data isolation | Product-dependent | PostgreSQL RLS-oriented tenant model |
| Research visibility | Limited internals | Inspectable lifecycle, ranking, aging, consolidation, telemetry |

That makes MemOS a better fit for infrastructure-heavy agent systems, interview demos, research prototypes, and teams that want to own the memory runtime instead of only calling a black-box memory endpoint.

## When To Use MemOS

Use MemOS if:

- You build long-running agents that need memory across sessions.
- You need persistent memory shared by multiple agents or nodes.
- You want retrieval that accounts for recency, importance, and reinforcement instead of semantic similarity alone.
- You need multi-tenant isolation for agent memory.
- You want distributed memory synchronization and repair.
- You care about explaining why a memory was retrieved.

Do not use MemOS if:

- You only need a stateless chatbot.
- Your context fits comfortably in a single prompt.
- You only need basic semantic search without lifecycle management.
- You want a single embedded local store with no distributed behavior.

## Quick Start

Start infrastructure and the Go memory node:

```bash
docker-compose -f deployments/docker-compose.yml up -d
export POSTGRES_URL='postgres://app_user:app_secure_password@localhost:5432/memos_db?sslmode=disable'
go run cmd/memos/main.go
```

Use the Python SDK:

```bash
pip install memos-sdk
```

```python
from memos_sdk import MemOSClient, MemoryType

client = MemOSClient("localhost:50051")

memory_id = client.store(
    tenant_id="00000000-0000-0000-0000-000000000001",
    agent_id="00000000-0000-0000-0000-000000000002",
    content="User prefers Go for backend systems.",
    importance=0.9,
)

results = client.retrieve(
    tenant_id="00000000-0000-0000-0000-000000000001",
    agent_id="00000000-0000-0000-0000-000000000002",
    query="backend language preference",
)

for item in results:
    print(item.score, item.memory.content)
```

Use the TypeScript SDK:

```bash
npm install @memos/sdk
```

```typescript
import { createClient, MemoryType } from '@memos/sdk';

const client = createClient({
  endpoint: 'localhost',
  port: 50051,
});

const memoryId = await client.store('User prefers Go for backend systems.', {
  tenantId: '00000000-0000-0000-0000-000000000001',
  agentId: '00000000-0000-0000-0000-000000000002',
  type: MemoryType.SEMANTIC,
  importance: 0.9,
});

const memories = await client.retrieve('backend language preference', {
  tenantId: '00000000-0000-0000-0000-000000000001',
  agentId: '00000000-0000-0000-0000-000000000002',
  limit: 5,
});

console.log(memoryId, memories[0]?.score, memories[0]?.breakdown);
```

More SDK details:

- [Python SDK README](sdk/python/README.md)
- [TypeScript SDK README](sdk/typescript/README.md)
- [gRPC service definition](proto/memory.proto)

## Agent Integration

MemOS is designed to plug into agent frameworks and custom LLM pipelines. Agents can store interactions, retrieve ranked context before planning, reinforce useful memory through repeated access, and share memory across a distributed cluster.

```mermaid
sequenceDiagram
    participant Agent
    participant SDK as Python/TypeScript SDK
    participant MemOS as MemOS Node
    participant Stores as PG + Qdrant + Neo4j
    participant Peer as Peer Nodes

    Agent->>SDK: store("User prefers Go")
    SDK->>MemOS: StoreMemory
    MemOS->>Stores: Persist metadata + vector + graph links
    MemOS-->>Peer: Replicate memory event
    Agent->>SDK: retrieve("backend preference")
    SDK->>MemOS: Retrieve
    MemOS->>Stores: Semantic search + metadata hydration
    MemOS-->>Agent: Ranked context + score breakdown
```

The TypeScript SDK includes adapters for:

- [LangGraph](sdk/typescript/src/adapters/langgraph.ts)
- [CrewAI](sdk/typescript/src/adapters/crewai.ts)
- [AutoGen](sdk/typescript/src/adapters/autogen.ts)

## Memory Lifecycle

MemOS treats memory as a lifecycle, not a static row in a database.

```mermaid
flowchart LR
    classDef hot fill:#e3f2fd,stroke:#1565c0,stroke-width:2px,color:#0d1b2a;
    classDef rank fill:#e8f5e9,stroke:#2e7d32,stroke-width:2px,color:#102a12;
    classDef cold fill:#fff8e1,stroke:#f9a825,stroke-width:2px,color:#2b2200;
    classDef archive fill:#fce4ec,stroke:#ad1457,stroke-width:2px,color:#2b0614;

    Ingestion[Ingestion]:::hot --> Ranking[Ranking]:::rank
    Ranking --> Retrieval[Context retrieval]:::rank
    Retrieval --> Reinforcement[Reinforcement]:::rank
    Reinforcement --> Stronger[Frequently used memory strengthens]:::rank
    Ranking --> Decay[Unused memory decays]:::cold
    Decay --> Consolidation[Repeated memory consolidates]:::cold
    Consolidation --> Semantic[Semantic memory]:::rank
    Decay --> Archive[Cold archival]:::archive
    Archive --> Delete[Optional deletion]:::archive
```

| Stage | What happens | Implementation |
| :--- | :--- | :--- |
| Ingestion | Agent stores an event, fact, preference, instruction, or observation | [internal/api/memory_handler.go](internal/api/memory_handler.go) |
| Ranking | Query candidates are scored by multiple signals | [internal/core/ranking.go](internal/core/ranking.go) |
| Reinforcement | Repeatedly useful memories receive a retrieval boost | [proto/memory.proto](proto/memory.proto) exposes `reinforcement_score` |
| Decay | Older or unused memories lose retrieval strength | [internal/core/ranking.go](internal/core/ranking.go) |
| Consolidation | Related episodic memories can become semantic memory | [internal/core/consolidation.go](internal/core/consolidation.go) |
| Archival | Cold, low-importance memories move out of active vector search | [internal/core/aging.go](internal/core/aging.go) |

The practical behavior is simple: frequently used memory is strengthened, unused memory decays, repeated memory can be consolidated, and low-value cold memory can be archived.

## Architecture Layers

Instead of one giant memory component, MemOS is split into layers with clear responsibilities.

```mermaid
flowchart TB
    classDef core fill:#e8f5e9,stroke:#2e7d32,stroke-width:2px,color:#102a12;
    classDef enhance fill:#ede7f6,stroke:#5e35b1,stroke-width:2px,color:#160b2b;
    classDef dist fill:#fff3e0,stroke:#ef6c00,stroke-width:2px,color:#2b1700;
    classDef iface fill:#e1f5fe,stroke:#0277bd,stroke-width:2px,color:#071f2e;
    classDef store fill:#f5f5f5,stroke:#424242,stroke-width:2px,color:#111;

    Interface[Interface Layer<br/>SDKs, gRPC, MCP, n8n]:::iface
    Core[Core Layer<br/>memory, ranking, decay, reinforcement]:::core
    Enhancement[Enhancement Layer<br/>graph augmentation, reflection, consolidation]:::enhance
    Distributed[Distributed Layer<br/>gossip, NATS, shards, anti-entropy]:::dist
    Storage[Storage Layer<br/>Postgres, Qdrant, Neo4j, Redis]:::store

    Interface --> Core
    Core --> Enhancement
    Core --> Storage
    Enhancement --> Storage
    Core --> Distributed
    Distributed --> Storage
```

| Layer | Role |
| :--- | :--- |
| Core Layer | Ranking, decay, reinforcement, memory lifecycle, conflict handling |
| Enhancement Layer | Reflection workers, entity relationships, consolidation, graph context |
| Distributed Layer | Gossip discovery, NATS replication, sharding, Merkle repair |
| Interface Layer | gRPC API, Python SDK, TypeScript SDK, MCP server, automation integrations |
| Storage Layer | PostgreSQL metadata/RLS, Qdrant vectors, Neo4j graph, Redis cache |

## Distributed Architecture

MemOS is designed for high availability and horizontal scalability with a decentralized shared-nothing topology.

### Cluster Topology

Nodes discover each other via gossip. Control-plane signals such as node join and leave happen over gossip, while higher-volume data replication is handled by NATS.

```mermaid
graph TD
    classDef node fill:#e3f2fd,stroke:#1565c0,stroke-width:2px,color:#0d1b2a;
    classDef bus fill:#fff3e0,stroke:#ef6c00,stroke-width:2px,color:#2b1700;
    classDef client fill:#e8f5e9,stroke:#2e7d32,stroke-width:2px,color:#102a12;

    subgraph Cluster [MemOS Distributed Cluster]
        NodeA[Node A]:::node <--> |Gossip| NodeB[Node B]:::node
        NodeB <--> |Gossip| NodeC[Node C]:::node
        NodeC <--> |Gossip| NodeA

        NodeA -.-> |Publish memory events| NATS{NATS Bus}:::bus
        NodeB -.-> |Publish memory events| NATS
        NodeC -.-> |Publish memory events| NATS
        NATS -.-> |Deliver| NodeA
        NATS -.-> |Deliver| NodeB
        NATS -.-> |Deliver| NodeC
    end

    LB[Load Balancer]:::client --> NodeA
    LB --> NodeB
    LB --> NodeC
```

### Replication Pipeline

When a memory is stored on a node, it is committed locally and broadcast to peer nodes through the replication fabric.

```mermaid
sequenceDiagram
    participant Client
    participant Node as Node Primary
    participant Store as Local Storage
    participant Bus as NATS Fabric
    participant Peer as Node Replica

    Client->>Node: StoreMemory(data)
    Node->>Store: 1. Local write (PG/Qdrant/Neo4j)
    Node->>Bus: 2. Publish(memory.stored)
    Node-->>Client: 200 OK + memory_id
    Bus->>Peer: 3. Deliver event
    Peer->>Peer: 4. Idempotent replicate
    Peer->>Store: 5. Persist replica state
```

### Anti-Entropy & Consistency

To provide strong eventual consistency, nodes periodically compute shard summaries, compare Merkle hashes, and repair divergence.

```mermaid
graph LR
    classDef step fill:#e3f2fd,stroke:#1565c0,stroke-width:2px,color:#0d1b2a;
    classDef decision fill:#fff3e0,stroke:#ef6c00,stroke-width:2px,color:#2b1700;
    classDef repair fill:#e8f5e9,stroke:#2e7d32,stroke-width:2px,color:#102a12;

    Start[Cycle start]:::step --> Hash[Compute local shard hashes]:::step
    Hash --> Gossip[Broadcast shard summaries]:::step
    Gossip --> Compare{Divergence detected?}:::decision
    Compare -- Yes --> Sync[Request missing blocks]:::repair
    Compare -- No --> Sleep[Sleep 300s]:::step
    Sync --> Repair[Apply repairs]:::repair
    Repair --> Verify[Verify Merkle root]:::repair
    Verify --> Sleep
```

## System Components

MemOS uses polyglot persistence so each storage engine does the job it is best at.

```mermaid
flowchart TB
    classDef client fill:#f5f5f5,stroke:#333,stroke-width:2px,color:#000;
    classDef gateway fill:#e1f5fe,stroke:#0288d1,stroke-width:2px,color:#000;
    classDef engine fill:#e8f5e9,stroke:#388e3c,stroke-width:2px,color:#000;
    classDef storage fill:#fff3e0,stroke:#f57c00,stroke-width:2px,color:#000;
    classDef fabric fill:#f3e5f5,stroke:#7b1fa2,stroke-width:2px,color:#000;

    Client([AI Agent / SDK]):::client
    n8n([n8n Automation]):::client
    MCP([MCP Server]):::client

    subgraph Gateway ["Gateway Layer"]
        API[gRPC Handler]:::gateway
        Auth[RBAC / RLS Scope]:::gateway
        Breaker[Circuit Breaker]:::gateway
        API --> Auth --> Breaker
    end

    subgraph Core ["Adaptive Memory Core"]
        Rank[Ranking Engine]:::engine
        Reflect[Reflection Worker]:::engine
        Age[Aging Pipeline]:::engine
        Consolidate[Consolidation Worker]:::engine
        Conflict[Conflict Handling]:::engine
    end

    subgraph Fabric ["Distributed Fabric"]
        Gossip[Gossip Discovery]:::fabric
        Replicate[NATS Replication]:::fabric
        Merkle[Merkle Repair]:::fabric
        Shards[Shard Manager]:::fabric
    end

    subgraph Storage ["Storage Layer"]
        PG[(Postgres Metadata + RLS)]:::storage
        QD[(Qdrant Vectors)]:::storage
        Neo[(Neo4j Graph)]:::storage
        Redis[(Redis Cache)]:::storage
        DLQ[(NATS DLQ)]:::storage
    end

    Client ==gRPC==> API
    n8n --Webhooks--> API
    MCP --> API
    Breaker --> Rank
    Breaker --> Conflict
    Rank --> Redis
    Rank -.Cache miss.-> QD
    Rank -.Hydrate.-> PG
    Reflect --> PG
    Reflect --> Neo
    Consolidate --> PG
    Age --> PG
    Age --> QD
    Rank --> Neo
    Replicate --> DLQ
    Gossip --> Shards
    Shards --> Merkle
    Merkle --> PG
```

## Cognitive Retrieval Pipeline

Unlike a plain vector lookup, MemOS computes a retrieval score from multiple memory signals:

- **Semantic relevance (S)**: Does the content match the query meaning?
- **Temporal decay (T)**: Is the memory still fresh enough to matter?
- **Importance (I)**: Was the memory marked as high-value?
- **Reinforcement (R)**: Has this memory repeatedly helped previous retrievals?

The ranking formula is:

```text
Score = alpha*S + beta*T + gamma*I + delta*R
```

Default weights are implemented in [internal/core/ranking.go](internal/core/ranking.go):

| Signal | Default weight | Why it exists |
| :--- | ---: | :--- |
| Semantic relevance | 0.40 | Keeps retrieval grounded in query meaning |
| Temporal decay | 0.30 | Keeps stale memories from dominating forever |
| Importance | 0.20 | Preserves facts, preferences, and critical decisions |
| Reinforcement | 0.10 | Strengthens memories that repeatedly prove useful |

```mermaid
sequenceDiagram
    participant Agent
    participant Gateway
    participant Cache as Redis
    participant Vector as Qdrant
    participant Meta as Postgres
    participant Graph as Neo4j

    Agent->>Gateway: Retrieve(query)
    Gateway->>Cache: 1. Check hot context
    alt Cache hit
        Cache-->>Gateway: Cached candidates
    else Cache miss
        Gateway->>Vector: 2. Semantic search
        Vector-->>Gateway: Semantic scores (S)
    end
    Gateway->>Meta: 3. Fetch metadata
    Meta-->>Gateway: Created_at (T), importance (I), reinforcement (R)
    Gateway->>Graph: 4. Entity augmentation
    Graph-->>Gateway: Related entities
    Note over Gateway: Score = alpha*S + beta*T + gamma*I + delta*R
    Gateway-->>Agent: Ranked results + explanation
```

## Explainable Retrieval

Every retrieved memory can include a score breakdown so developers can debug why a memory was selected.

```json
{
  "memory": "User prefers Go for backend systems",
  "semantic_score": 0.82,
  "recency_score": 0.15,
  "importance_score": 0.7,
  "reinforcement_score": 0.3,
  "final_score": 0.74,
  "layer": "long_term"
}
```

This makes retrieval transparent enough for production agent systems: you can inspect whether a result came from semantic similarity, recency, importance, repeated reinforcement, or a mix of all four.

## Distributed Behavior

These scenarios are supported by the distributed fabric modules in [internal/fabric](internal/fabric).

| Scenario | Expected behavior | Relevant code |
| :--- | :--- | :--- |
| Node join | New node discovers peers and receives replicated memory events | [gossip.go](internal/fabric/gossip.go), [replication.go](internal/fabric/replication.go) |
| Node failure | Other nodes continue serving memory through local storage and replicated state | [events.go](internal/fabric/events.go), [replication.go](internal/fabric/replication.go) |
| Divergence | Shard hashes differ and anti-entropy requests missing blocks | [antientropy.go](internal/fabric/antientropy.go), [merkle.go](internal/fabric/merkle.go) |
| Shard ownership | Memory can be partitioned across nodes for scale | [sharding.go](internal/fabric/sharding.go) |
| Delivery failure | Replication can route failed events to the dead-letter path | [replication.go](internal/fabric/replication.go) |

```mermaid
flowchart TD
    classDef ok fill:#e8f5e9,stroke:#2e7d32,stroke-width:2px,color:#102a12;
    classDef warn fill:#fff3e0,stroke:#ef6c00,stroke-width:2px,color:#2b1700;
    classDef fix fill:#e3f2fd,stroke:#1565c0,stroke-width:2px,color:#0d1b2a;

    A[Node B joins cluster]:::ok --> B[Gossip membership update]:::ok
    B --> C[Shard summary exchange]:::fix
    C --> D{Missing memory blocks?}:::warn
    D -- Yes --> E[Anti-entropy repair]:::fix
    D -- No --> F[Node ready for retrieval]:::ok
    E --> F

    G[Node C unavailable]:::warn --> H[Peers continue serving local replicas]:::ok
    H --> I[Node C returns]:::fix
    I --> C
```

## Data Model & Isolation

Tenant data is isolated using PostgreSQL Row-Level Security (RLS). Every memory belongs to a tenant and agent, and every request is scoped accordingly.

```mermaid
erDiagram
    TENANT ||--o{ AGENT : manages
    TENANT ||--o{ MEMORY : isolates
    AGENT ||--o{ MEMORY : owns

    TENANT {
        uuid id PK
        string name
    }
    AGENT {
        uuid id PK
        uuid tenant_id FK
        string name
    }
    MEMORY {
        uuid id PK
        uuid tenant_id FK
        uuid agent_id FK
        string type
        text content
        float importance
        timestamp created_at
        timestamp updated_at
    }
```

## Evaluation

MemOS includes reproducible retrieval evaluation scripts and generated benchmark reports. The current gold-standard dataset is intentionally small and inspectable: 3 query cases, 4 relevant judgments, and a Recall@5 / Precision@5 cutoff. It compares semantic-only, recency-only, and hybrid adaptive ranking.

| Strategy | Recall@5 | Precision@5 | MRR | Top-1 Hit Rate | Proof |
| :--- | ---: | ---: | ---: | ---: | :--- |
| Semantic Only | 1.000 | 0.333 | 0.833 | 0.667 | [retrieval_report.md](scripts/benchmarks/retrieval_report.md) |
| Recency Only | 1.000 | 0.333 | 0.417 | 0.000 | [retrieval_report.md](scripts/benchmarks/retrieval_report.md) |
| Hybrid Adaptive | 1.000 | 0.333 | 1.000 | 1.000 | [retrieval_report.md](scripts/benchmarks/retrieval_report.md) |

### Benchmark Proofs

- [Gold-standard dataset](scripts/benchmarks/retrieval_gold_standard.json)
- [Retrieval metrics implementation](scripts/benchmarks/retrieval_metrics.py)
- [Retrieval benchmark runner](scripts/benchmarks/run_retrieval_benchmark.py)
- [Generated retrieval report](scripts/benchmarks/retrieval_report.md)
- [Latency benchmark runner](scripts/benchmarks/run_latency.py)
- [Seed data script](scripts/benchmarks/seed_data.py)
- [Legacy phase 6 report](scripts/benchmarks/phase6_report.md)

### Methodology

| Field | Current setup |
| :--- | :--- |
| Dataset | Human-annotated synthetic agent-memory cases in [retrieval_gold_standard.json](scripts/benchmarks/retrieval_gold_standard.json) |
| Query types | UI preference recall, distributed failure recovery, agent setup recall |
| Compared strategies | Semantic-only, recency-only, hybrid adaptive |
| Metrics | Recall@5, Precision@5, MRR, Top-1 hit rate |
| Embedding model | Determined by the running MemOS embedding/storage configuration |
| Latency method | End-to-end SDK retrieve loop in [run_latency.py](scripts/benchmarks/run_latency.py) |

Run the retrieval evaluation:

```bash
python3 scripts/benchmarks/run_retrieval_benchmark.py --report scripts/benchmarks/retrieval_report.md
```

Run the latency benchmark against a local node:

```bash
python3 scripts/benchmarks/run_latency.py
```

## Installation

### Python SDK

```bash
pip install memos-sdk
```

From source:

```bash
git clone https://github.com/Mohi1038/distributed-memOS
cd distributed-memOS/sdk/python
pip install -e .
```

### TypeScript SDK

```bash
npm install @memos/sdk
```

From source:

```bash
git clone https://github.com/Mohi1038/distributed-memOS
cd distributed-memOS/sdk/typescript
npm install
npm run build
```

### Go Service

```bash
git clone https://github.com/Mohi1038/distributed-memOS
cd distributed-memOS
go mod download
go run cmd/memos/main.go
```

## Running Locally

### 1. Start Infrastructure

```bash
docker-compose -f deployments/docker-compose.yml up -d
```

### 2. Start MemOS Node

```bash
export POSTGRES_URL='postgres://app_user:app_secure_password@localhost:5432/memos_db?sslmode=disable'
go run cmd/memos/main.go
```

### 3. Optional Cluster Mode

```bash
docker-compose -f deployments/docker-compose.cluster.yml up -d
```

### 4. Run Demo Assets

```bash
./scripts/demo_visual.sh
```

More demo notes are in [scripts/DEMO_README.md](scripts/DEMO_README.md).

## Observability & Automation

MemOS ships with telemetry hooks, Prometheus configuration, Grafana provisioning, a lightweight dashboard, and n8n automation support.

| Tool | URL / File | Purpose |
| :--- | :--- | :--- |
| Prometheus | `http://localhost:9091` | Metrics collection |
| Grafana | `http://localhost:3000` | Dashboards, default local credentials are `Admin/Admin` |
| Dashboard | [deployments/dashboard/index.html](deployments/dashboard/index.html) | Lightweight operational UI |
| Grafana JSON | [memos-overview.json](deployments/grafana/provisioning/dashboards/memos-overview.json) | Prebuilt dashboard |
| n8n | `http://localhost:5678` | Automation for Slack, Discord, email, RSS, and workflow ingestion |
| Metrics code | [internal/core/telemetry.go](internal/core/telemetry.go) | Store/retrieve/audit/cache/replication latency counters |

```mermaid
flowchart LR
    classDef svc fill:#e3f2fd,stroke:#1565c0,stroke-width:2px,color:#0d1b2a;
    classDef obs fill:#e8f5e9,stroke:#2e7d32,stroke-width:2px,color:#102a12;
    classDef auto fill:#fff3e0,stroke:#ef6c00,stroke-width:2px,color:#2b1700;

    MemOS[MemOS Node]:::svc --> Metrics[Prometheus metrics]:::obs
    Metrics --> Grafana[Grafana dashboards]:::obs
    MemOS --> Dashboard[Local dashboard]:::obs
    n8n[n8n workflows]:::auto --> MemOS
    Slack[Slack / Discord / Email / RSS]:::auto --> n8n
```

## Repository Map

| Path | What it contains |
| :--- | :--- |
| [cmd/memos](cmd/memos) | Main MemOS node |
| [cmd/mcp-server](cmd/mcp-server) | MCP server entrypoint |
| [internal/api](internal/api) | gRPC handlers and dashboard API |
| [internal/core](internal/core) | Ranking, aging, reflection, consolidation, authorization, telemetry |
| [internal/fabric](internal/fabric) | Gossip, replication, sharding, Merkle, anti-entropy |
| [internal/storage](internal/storage) | PostgreSQL, Qdrant, Neo4j models and storage adapters |
| [proto](proto) | gRPC contract and generated Go bindings |
| [sdk/python](sdk/python) | Python SDK |
| [sdk/typescript](sdk/typescript) | TypeScript SDK and agent-framework adapters |
| [scripts/benchmarks](scripts/benchmarks) | Evaluation datasets, benchmark runners, reports |
| [deployments](deployments) | Docker Compose, Prometheus, Grafana, dashboard assets |

## License

Distributed MemOS is licensed under the [MIT License](LICENSE).
