# MemOS: Distributed Adaptive Memory Upgrade Plan

This document outlines the engineering roadmap to transition Distributed MemOS from a high-level architecture into a world-class, adaptive memory infrastructure for autonomous AI agents.

---

## 🟢 PHASE 1: Cognitive Core Legitimacy
**Goal:** Transition from a "weighted vector search" to a dynamic, biological-inspired memory system.

### 1.1 Adaptive Memory Aging (Decay)
- [x] **Infrastructure**: Add `last_accessed`, `reinforcement_score`, and `decay_factor` to the `memory` schema.
- [x] **Ranking**: Implement the retention formula: $Retention(t) = Importance \cdot e^{-\lambda t}$.
- [x] **Worker**: Create a background decay worker that periodically updates `decay_factor` for inactive memories.

### 1.2 Memory Reinforcement
- [x] **Feedback Loop**: Update the `retrieve` API to increment `retrieval_count` and `reinforcement_score` on successful context usage.
- [x] **Dynamic Priority**: Adjust the aging worker to slow down decay for reinforced memories (spaced repetition).

### 1.3 Memory Consolidation
- [x] **Clustering**: Implement a background job using HDBSCAN or similar to cluster semantically redundant episodic memories.
- [x] **Summarization**: Use an LLM worker to consolidate clusters into high-level "Semantic Knowledge" nodes.
- [x] **Cleanup**: Archive or delete the fragmented episodic base after successful consolidation.

---

## 🔵 PHASE 2: Graph-Augmented Retrieval (Neo4j)
**Goal:** Use Neo4j for measurable context expansion, not just architectural visibility.

### 2.1 Entity-Centric Memory Graphs
- [x] **Extraction**: Integrate Entity Extraction (NER) into the storage pipeline.
- [x] **Mapping**: Link memories via shared entities (People, Projects, Technologies) in Neo4j.
- [x] **Graph Search**: Implement a 2-hop neighbor expansion during retrieval to find "related context" that lacks semantic similarity.

### 2.2 Relationship Strengthening
- [x] **Weighted Edges**: Add weights to Neo4j relationships.
- [x] **Hebbian Learning**: Increase edge weights when two entities co-occur in reinforced memories.

---

## 🟡 PHASE 3: Distributed Systems Engineering
**Goal:** Prove the distributed claims through demonstrable failure-handling and repair.

### 3.1 Multi-Node Replication Demo
- [ ] **Orchestration**: Create a `docker-compose.cluster.yml` with 3 independent MemOS nodes.
- [ ] **Validation**: Implement a suite that stores to Node A and verifies immediate availability on Node C.

### 3.2 Real Merkle-Tree Anti-Entropy
- [ ] **Hashing**: Implement per-shard Merkle tree generation for memory content.
- [ ] **Divergence Detection**: Nodes exchange Merkle roots via Gossip to identify specific "dirty" shards.
- [ ] **Selective Repair**: Request only the specific divergent memory blocks from peers.

### 3.3 Conflict Resolution
- [ ] **Versioning**: Implement vector clocks or hybrid logical clocks (HLC) for memory updates.
- [ ] **Resolution Policies**: Add LWW (Last Writer Wins) and Semantic Merge (LLM-based reconciliation) policies.

---

## 🟠 PHASE 4: Intelligent Retrieval Hierarchy
**Goal:** Optimize for scale, latency, and biological accuracy.

### 4.1 Hierarchical Memory Layers
- [ ] **Working**: In-memory Redis cache for active sessions.
- [ ] **Episodic**: Qdrant/Postgres for recent interactions.
- [ ] **Long-Term**: Consolidated/Reinforced memories in Neo4j and PG.
- [ ] **Archive**: Cold storage (S3/Disk) for expired/low-importance memories.

### 4.2 Retrieval Explainability
- [ ] **Scoring Metadata**: Update gRPC response to return a breakdown of the final score (Semantic %, Recency %, Importance %, Reinforcement %).
- [ ] **Tracing**: Add OpenTelemetry spans to trace why a specific memory was selected.

---

## 🔴 PHASE 5: Product & Ecosystem
**Goal:** Move beyond a "project" into an infrastructure substrate.

### 5.1 SDK & Framework Integration
- [ ] **TypeScript SDK**: Build a parity-level SDK for Node.js/Web environments.
- [ ] **Agent Integrations**: Build official adapters for LangGraph, CrewAI, and AutoGen.
- [ ] **MCP Server**: Implement the Model Context Protocol (MCP) to allow any LLM to use MemOS as a tool.

### 5.2 Observability Dashboard
- [ ] **Real-time Viz**: Build a dashboard showing replication lag, shard distribution, and cognitive decay heatmaps.
- [ ] **Graph Visualizer**: A 3D view of the evolving Neo4j entity graph.

---

## ⚪ PHASE 6: Academic & Professional Credibility
**Goal:** Ground the project in scientific rigor.

### 6.1 Scientific Evaluation Metrics
- [ ] **IR Metrics**: Implement Recall@K, Precision@K, and MRR (Mean Reciprocal Rank) evaluation scripts.
- [ ] **Human-Alignment**: Benchmark retrieval against a "Gold Standard" dataset of human-annotated relevance.

### 6.2 Benchmarking Suite
- [ ] **Comparison Study**: Generate a technical report comparing:
    - Semantic-only (Standard Vector DB).
    - Recency-only.
    - **MemOS Hybrid Adaptive Retrieval**.

### 6.3 Professional Tone & Identity
- [ ] **Refactoring Terminology**: Replace marketing jargon with engineering terms (e.g., "Human-like" -> "Adaptive Retrieval").
- [ ] **Final Identity**: Re-brand as: **"A Distributed Adaptive Memory Infrastructure for Autonomous AI Agents."**
