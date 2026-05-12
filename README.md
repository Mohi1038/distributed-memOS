<div align="center">
  <img src="https://img.icons8.com/?size=512&id=vmlQ0k1U6D5D&format=png" alt="Brain Logo" width="120" />
  <h1>Distributed MemOS</h1>
  <p><em>A Production-Ready Cognitive Memory Infrastructure for Autonomous AI Systems</em></p>
  
  [![Go Version](https://img.shields.io/badge/Go-1.22+-00ADD8?style=for-the-badge&logo=go)](https://golang.org/)
  [![Python SDK](https://img.shields.io/badge/Python-SDK-3776AB?style=for-the-badge&logo=python)](sdk/python)
  [![License](https://img.shields.io/badge/License-MIT-green.svg?style=for-the-badge)](#)
  [![Status](https://img.shields.io/badge/Status-Stable-success?style=for-the-badge)](#)
</div>

---

## What is MemOS?

LLMs and AI agents suffer from amnesia. **MemOS** solves this by providing a highly scalable, distributed, and cognitive "operating system" for agent memory. It doesn't just store text—it **understands** it, **ranks** it based on human-like cognitive decay, **resolves contradictions**, and **syncs** it across a distributed cluster of nodes.

---

## Core System Architecture

MemOS employs a **Polyglot Persistence** strategy, routing different types of memory data to specialized storage engines, coordinated by a high-speed gRPC API.

```mermaid
flowchart TB
    %% Styling definitions
    classDef client fill:#f5f5f5,stroke:#333,stroke-width:2px,color:#000;
    classDef gateway fill:#e1f5fe,stroke:#0288d1,stroke-width:2px,color:#000;
    classDef engine fill:#e8f5e9,stroke:#388e3c,stroke-width:2px,color:#000;
    classDef storage fill:#fff3e0,stroke:#f57c00,stroke-width:2px,color:#000;

    Client([AI Agent / API Client]):::client
    n8n([n8n Workflow Engine]):::client
    
    subgraph Gateway ["MemOS Gateway Layer"]
        API[gRPC Handler]:::gateway
        Auth[RBAC & Tenant Isolation]:::gateway
        Breaker[Circuit Breaker]:::gateway
        API --> Auth
        Auth --> Breaker
    end

    subgraph Core ["Cognitive Core"]
        Rank[Cognitive Ranking Engine]:::engine
        Reflect[Reflection Worker]:::engine
        Conflict[Conflict Resolver]:::engine
        Age[Aging Pipeline]:::engine
    end

    subgraph Storage ["Polyglot Storage Layer"]
        PG[(PostgreSQL<br/>Metadata & RLS)]:::storage
        QD[(Qdrant<br/>Vector Search)]:::storage
        Neo[(Neo4j<br/>Graph/Entities)]:::storage
        Redis[(Redis<br/>LRU Cache)]:::storage
        DLQ[(NATS DLQ)]:::storage
    end

    Client ==gRPC==> API
    n8n --Webhooks--> API
    Breaker --> Rank
    Breaker --> Reflect
    
    Rank --> Redis
    Rank -.Cache Miss.-> QD
    Rank -.Hydrate.-> PG
    
    Conflict --> Neo
    Reflect --> PG
    Age --> PG
    Replicate[Replication] --> DLQ
```

---

## Cognitive Retrieval Pipeline

Unlike standard vector databases that rely solely on cosine similarity, MemOS mimics human recall by combining semantic relevance, temporal decay (forgetting curve), and emotional/user-defined importance.

```mermaid
sequenceDiagram
    participant Agent as AI Agent
    participant Handler as gRPC Handler
    participant Qdrant as Qdrant (Vectors)
    participant PG as PostgreSQL (Metadata)
    participant Neo4j as Neo4j (Graph)
    
    Agent->>Handler: Retrieve(query="dark mode")
    activate Handler
    
    Handler->>Qdrant: 1. Vector Search (Semantic)
    Qdrant-->>Handler: Return Top 20 Candidates
    
    Handler->>PG: 2. Hydrate Metadata & Importance
    PG-->>Handler: Return created_at, importance
    
    Handler->>Neo4j: 3. Graph Augmentation
    Note over Neo4j: Find related memories via<br/>shared entity mentions
    Neo4j-->>Handler: Return Connected Memory IDs
    
    Note over Handler: 4. Compute Cognitive Score:<br/>R = α*S + β*T + γ*I + δ*C
    Note over Handler: 5. Deduplicate & Re-rank
    
    Handler-->>Agent: Return Top 5 Ranked Memories
    deactivate Handler
```

---

## Distributed Fabric & Anti-Entropy

MemOS clusters use a Gossip protocol for decentralized node discovery and NATS for asynchronous replication. To guarantee eventual consistency, a background Anti-Entropy manager calculates and compares shard checksums.

```mermaid
stateDiagram-v2
    [*] --> NodeStart
    
    state NodeStart {
        JoinCluster: Broadcast "node.joined"
        Gossip: Start 10s Heartbeats
        JoinCluster --> Gossip
    }
    
    state ActiveState {
        WaitEvent: Await gRPC / NATS
        Store: memory.stored
        Replicate: Replicate to Replicas
        WaitEvent --> Store
        Store --> Replicate
    }
    
    state AntiEntropy {
        ComputeHash: Hash Local Shards
        BroadcastHash: Publish shard.sync
        Compare: Compare Peer Hashes
        Repair: Re-publish Divergent Data
        
        ComputeHash --> BroadcastHash
        BroadcastHash --> Compare
        Compare --> Repair: If Divergence
    }
    
    NodeStart --> ActiveState
    ActiveState --> AntiEntropy: Every 5 minutes
    AntiEntropy --> ActiveState
```

### Reliability & Resilience
MemOS is built for production stability:
- **Circuit Breakers**: External calls to OpenAI/Embeddings are protected by circuit breakers to prevent cascading failures.
- **Dead Letter Queues (DLQ)**: Failed replication events are automatically routed to NATS DLQ subjects for manual recovery and audit.
- **Health Checks**: Deep health monitoring for all backing stores (Postgres, Qdrant, Neo4j).

---

## Data Model & Isolation

Tenant data is strictly isolated using PostgreSQL **Row-Level Security (RLS)**. Even the application database user (`app_user`) cannot read data without setting a valid `app.current_tenant` session variable.

```mermaid
erDiagram
    TENANT ||--o{ AGENT : manages
    TENANT ||--o{ MEMORY : isolates
    AGENT ||--o{ MEMORY : owns
    MEMORY ||--o{ AUDIT_LOG : tracks
    
    TENANT {
        uuid id PK
        string name
        timestamp created_at
    }
    AGENT {
        uuid id PK
        uuid tenant_id FK
        string name
        string role
    }
    MEMORY {
        uuid id PK
        uuid tenant_id FK
        uuid agent_id FK
        string type
        text content
        float importance
        jsonb metadata
    }
    AUDIT_LOG {
        uuid id PK
        uuid tenant_id FK
        string action
        jsonb metadata
    }
```

---

## Python SDK Quickstart

Agent developers can interact with MemOS instantly using the official Python SDK. It wraps the raw gRPC calls into an elegant, developer-friendly interface.

### 1. Installation

The official SDK is available on PyPI:
```bash
pip install memos-sdk
```

Or install it locally from the source:
```bash
cd sdk/python
pip install -e .
```

### 2. Using MemOS in your Agent

```python
from memos_sdk import MemOSClient, MemoryType

# Connect to the MemOS Cluster
client = MemOSClient("localhost:50051")

tenant_id = "00000000-0000-0000-0000-000000000001"
agent_id = "00000000-0000-0000-0000-000000000002"

# 1. Store a Memory
memory_id = client.store(
    tenant_id=tenant_id,
    agent_id=agent_id,
    content="The user prefers a high-contrast dark mode with large fonts.",
    memory_type=MemoryType.MEMORY_TYPE_EPISODIC,
    importance=0.85 # Highly important
)
print(f"Stored Memory: {memory_id}")

# 2. Retrieve with Cognitive Search
results = client.retrieve(
    tenant_id=tenant_id,
    agent_id=agent_id,
    query="UI preferences dark mode",
    limit=3,
    alpha_semantic=0.5,   # Emphasize meaning
    beta_temporal=0.2,    # Emphasize recency
    gamma_importance=0.3  # Emphasize flagged importance
)

for res in results:
    print(f"[{res.score:.2f}] {res.memory.content}")
```

---

## Running the Cluster Locally

### Prerequisites
- Docker & Docker Compose
- Go 1.22+

### 1. Spin up the Storage Infrastructure
This will start PostgreSQL, Qdrant, NATS, Redis, Neo4j, and Prometheus/Grafana.
```bash
docker-compose -f deployments/docker-compose.yml up -d
```
*Note: The database is automatically seeded with schemas, RLS policies, and an `app_user` role.*

### 2. Start the MemOS Node
```bash
export POSTGRES_URL='postgres://app_user:app_secure_password@localhost:5432/memos_db?sslmode=disable'

# To use OpenAI embeddings instead of mock ones:
# export USE_REAL_EMBEDDING=true
# export OPENAI_API_KEY="your-key"

go run cmd/memos/main.go
```

### 3. Monitoring (Telemetry)
MemOS exposes real-time Prometheus metrics.
- **Metrics Endpoint**: `http://localhost:9090/metrics`
- Tracks: Memory latency, cache hit rates, replication lag, and RBAC auth denials.

### 4. Workflow Automation (n8n)
MemOS integrates seamlessly with **n8n** for visual workflow orchestration.
- **Dashboard**: `http://localhost:5678`
- **Use Case**: Automatically ingest Slack messages, emails, or RSS feeds into agent memory.
