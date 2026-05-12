<div align="center">
  <img src="https://img.icons8.com/?size=512&id=vmlQ0k1U6D5D&format=png" alt="Brain Logo" width="120" />
  <h1>Distributed MemOS</h1>
  <p><em>A Production-Ready Cognitive Memory Infrastructure for Autonomous AI Systems</em></p>
  
  [![Go Version](https://img.shields.io/badge/Go-1.22+-00ADD8?style=for-the-badge&logo=go)](https://golang.org/)
  [![Python SDK](https://img.shields.io/badge/Python-SDK-3776AB?style=for-the-badge&logo=python)](https://pypi.org/project/memos-sdk/)
  [![License](https://img.shields.io/badge/License-MIT-green.svg?style=for-the-badge)](#)
  [![Status](https://img.shields.io/badge/Status-Stable-success?style=for-the-badge)](#)
</div>

---

## What is MemOS?

LLMs and AI agents suffer from amnesia. **MemOS** solves this by providing a highly scalable, distributed, and cognitive "operating system" for agent memory. It doesn't just store text—it **understands** it, **ranks** it based on human-like cognitive decay, **resolves contradictions**, and **syncs** it across a distributed cluster of nodes.

--------------------------------------------------------------------------------

MemOS provides two high-level features for next-generation AI:
- **Cognitive Memory Ranking**: A multi-factor retrieval engine (α*S + β*T + γ*I) that mimics human recall.
- **Anti-Entropy Storage Fabric**: A decentralized, strong-consistency storage layer built on Gossip and NATS.

<!-- toc -->
- [More About MemOS](#more-about-memos)
- [Distributed Architecture](#distributed-architecture)
  - [Cluster Topology](#cluster-topology)
  - [Replication Pipeline](#replication-pipeline)
  - [Anti-Entropy & Consistency](#anti-entropy--consistency)
- [System Components](#system-components)
- [Cognitive Retrieval Pipeline](#cognitive-retrieval-pipeline)
- [Data Model & Isolation](#data-model--isolation)
- [Performance Benchmarks](#performance-benchmarks)
- [Comparison Table](#comparison-table)
- [Installation](#installation)
- [Running Locally](#running-locally)
- [Telemetry & Automation](#telemetry--automation)
- [License](#license)
<!-- tocstop -->

## More About MemOS

At a granular level, MemOS is a library and service that consists of the following components:

| Component | Description |
| :--- | :--- |
| **memos.core** | Cognitive logic: Ranking math, Reflection workers, and the Aging pipeline. |
| **memos.fabric** | Distributed logic: Gossip discovery, NATS replication, and Merkle-shard repair. |
| **memos.storage** | Polyglot layer: PostgreSQL (Metadata), Qdrant (Vectors), Neo4j (Graph), Redis (Cache). |
| **memos.gateway** | Security & API: gRPC handlers, RBAC, and Circuit Breakers for resilience. |

---

## Distributed Architecture

MemOS is designed for high availability and horizontal scalability, following a decentralized "Shared-Nothing" architecture.

### Cluster Topology
Nodes discover each other via a Gossip protocol. Control-plane signals (Node Join/Leave) happen over Gossip, while high-volume data replication is offloaded to a NATS message bus.

```mermaid
graph TD
    subgraph Cluster [MemOS Distributed Cluster]
        NodeA[Node A] <--> |Gossip| NodeB[Node B]
        NodeB <--> |Gossip| NodeC[Node C]
        NodeC <--> |Gossip| NodeA
        
        NodeA -.-> |Replicate| NATS{NATS Bus}
        NodeB -.-> |Replicate| NATS
        NodeC -.-> |Replicate| NATS
    end
    
    LB[Load Balancer] --> NodeA
    LB --> NodeB
    LB --> NodeC
```

### Replication Pipeline
When a memory is stored on a node, it is committed to local storage and asynchronously broadcast to the rest of the cluster via the Replication Fabric.

```mermaid
sequenceDiagram
    participant Client
    participant Leader as Node (Primary)
    participant Bus as NATS Fabric
    participant Peer as Node (Replica)
    
    Client->>Leader: StoreMemory(data)
    Leader->>Leader: 1. Local Write (PG/QD/Neo)
    Leader->>Bus: 2. Publish(memory.stored, data)
    Leader-->>Client: 200 OK
    
    Bus->>Peer: 3. Deliver(memory.stored)
    Peer->>Peer: 4. Replicate to Local Storage
```

### Anti-Entropy & Consistency
To guarantee strong eventual consistency, nodes run a background Anti-Entropy process. Nodes calculate Merkle-tree hashes for local shards and compare them with peers to detect and repair data divergence.

```mermaid
graph LR
    Start[Cycle Start] --> Hash[Compute Local Shard Hashes]
    Hash --> Gossip[Broadcast Shard Summaries]
    Gossip --> Compare{Divergence Detected?}
    Compare -- Yes --> Sync[Request Missing Blocks]
    Compare -- No --> Sleep[Sleep 300s]
    Sync --> Repair[Apply Repairs]
    Repair --> Sleep
```

---

## System Components

MemOS employs a **Polyglot Persistence** strategy, routing memory data to specialized storage engines.

```mermaid
flowchart TB
    classDef client fill:#f5f5f5,stroke:#333,stroke-width:2px,color:#000;
    classDef gateway fill:#e1f5fe,stroke:#0288d1,stroke-width:2px,color:#000;
    classDef engine fill:#e8f5e9,stroke:#388e3c,stroke-width:2px,color:#000;
    classDef storage fill:#fff3e0,stroke:#f57c00,stroke-width:2px,color:#000;

    Client([AI Agent / SDK]):::client
    n8n([n8n Automation]):::client
    
    subgraph Gateway ["Gateway Layer"]
        API[gRPC Handler]:::gateway
        Auth[RBAC / Isolation]:::gateway
        Breaker[Circuit Breaker]:::gateway
        API --> Auth --> Breaker
    end

    subgraph Core ["Cognitive Core"]
        Rank[Ranking Engine]:::engine
        Reflect[Reflection Worker]:::engine
        Age[Aging Pipeline]:::engine
    end

    subgraph Storage ["Storage Layer"]
        PG[(Postgres Metadata)]:::storage
        QD[(Qdrant Vectors)]:::storage
        Neo[(Neo4j Graph)]:::storage
        Redis[(Redis Cache)]:::storage
        DLQ[(NATS DLQ)]:::storage
    end

    Client ==gRPC==> API
    n8n --Webhooks--> API
    Breaker --> Rank
    
    Rank --> Redis
    Rank -.Cache Miss.-> QD
    Rank -.Hydrate.-> PG
    
    Reflect --> PG
    Age --> PG
```

---

## Cognitive Retrieval Pipeline

Unlike standard vector databases, MemOS computes a **Cognitive Score** based on semantic relevance, temporal decay, and importance.

```mermaid
sequenceDiagram
    participant Agent
    participant Gateway
    participant Vector as Qdrant
    participant Meta as Postgres
    participant Graph as Neo4j
    
    Agent->>Gateway: Retrieve(query)
    Gateway->>Vector: 1. Semantic Search
    Vector-->>Gateway: Semantic Scores (S)
    Gateway->>Meta: 2. Fetch Metadata
    Meta-->>Gateway: Created_at (T), Importance (I)
    Gateway->>Graph: 3. Entity Augmentation
    Graph-->>Gateway: Related Entities
    Note over Gateway: Score = αS + βT + γI + δC
    Gateway-->>Agent: Ranked Results
```

---

## Data Model & Isolation

Tenant data is strictly isolated using PostgreSQL **Row-Level Security (RLS)**.

```mermaid
erDiagram
    TENANT ||--o{ AGENT : manages
    TENANT ||--o{ MEMORY : isolates
    AGENT ||--o{ MEMORY : owns
    
    TENANT {
        uuid id PK
        string name
    }
    MEMORY {
        uuid id PK
        uuid tenant_id FK
        text content
        float importance
    }
```

---

## Performance Benchmarks

MemOS is optimized for production. Use the scripts in `scripts/benchmarks/` to reproduce these metrics.

| Metric | Standard Vector Search | MemOS Cognitive Retrieval |
| :--- | :--- | :--- |
| **Average Latency** | 150ms | 45ms |
| **P99 Latency** | 450ms | 110ms |
| **Contextual Accuracy** | 68% | 94% |

- **Methodology**: Benchmarked on Apple M4 Air, 16GB RAM, with 10,000 synthetic episodic memories using Recall@5.

---

## Comparison Table

| Feature | Standard Vector DB | Distributed MemOS |
| :--- | :--- | :--- |
| **Recall Method** | Semantic Similarity only | Semantic + Temporal + Importance |
| **Consistency** | Eventual | Strong (Anti-Entropy Repair) |
| **Multi-Tenancy** | Schema-level | Hard RLS Isolation |
| **Reliability** | Basic | Circuit Breakers & DLQs |
| **Workflows** | Manual | Native n8n Integration |

---

## Installation

### Python SDK (PyPI)
```bash
pip install memos-sdk
```

### From Source
```bash
git clone https://github.com/Mohi1038/distributed-memOS
cd distributed-memOS
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

## Telemetry & Automation

### Monitoring
- **Prometheus**: `http://localhost:9091`
- **Grafana**: `http://localhost:3000` (Admin/Admin)

### n8n Automation
- **Dashboard**: `http://localhost:5678`
- **Integrations**: Slack, Discord, Email, and RSS ingestors.

---

## License
MemOS is licensed under the MIT License.
