/**
 * MemOS TypeScript SDK - Core Types
 * Defines the interface for interacting with Distributed MemOS
 */

export enum MemoryType {
  UNSPECIFIED = 'MEMORY_TYPE_UNSPECIFIED',
  EPISODIC = 'MEMORY_TYPE_EPISODIC',
  SEMANTIC = 'MEMORY_TYPE_SEMANTIC',
  PROCEDURAL = 'MEMORY_TYPE_PROCEDURAL',
  REFLECTIVE = 'MEMORY_TYPE_REFLECTIVE',
  TRANSIENT = 'MEMORY_TYPE_TRANSIENT',
}

/**
 * Score breakdown for retrieval explainability (Phase 4)
 */
export interface ScoreBreakdown {
  semanticScore: number;      // Semantic similarity [0-1]
  temporalScore: number;      // Temporal decay / recency [0-1]
  importanceScore: number;    // User importance [0-1]
  reinforcementScore: number; // Retrieval bonus [0-1]
  layer: 'working' | 'episodic' | 'long_term' | 'archive';
}

/**
 * Memory metadata structure
 */
export interface Memory {
  id: string;
  tenantId: string;
  agentId: string;
  type: MemoryType;
  content: string;
  embedding?: number[];
  importance: number;
  createdAt: Date;
  updatedAt: Date;
  metadata?: Record<string, unknown>;
}

/**
 * Scored memory result with explainability
 */
export interface ScoredMemory {
  memory: Memory;
  score: number;
  breakdown?: ScoreBreakdown;
}

/**
 * Store request options
 */
export interface StoreOptions {
  tenantId: string;
  agentId: string;
  type?: MemoryType;
  importance?: number;
  metadata?: Record<string, unknown>;
}

/**
 * Retrieve request options
 */
export interface RetrieveOptions {
  tenantId: string;
  agentId: string;
  limit?: number;
  similarityThreshold?: number;
  alphaSemantic?: number;
  betaTemporal?: number;
  gammaImportance?: number;
}

/**
 * MemOS client configuration
 */
export interface MemOSConfig {
  endpoint: string;
  port: number;
  tlsEnabled?: boolean;
  credentials?: string;
}

/**
 * Telemetry snapshot for observability
 */
export interface TelemetrySnapshot {
  storeCount: number;
  retrieveCount: number;
  cacheHits: number;
  cacheMisses: number;
  replicationLagMs: number;
  avgStoreLatencyMs: number;
  avgRetrieveLatencyMs: number;
}
