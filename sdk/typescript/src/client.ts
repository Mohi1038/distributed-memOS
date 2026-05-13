/**
 * MemOS TypeScript SDK - Main Client
 * Provides a high-level interface to Distributed MemOS
 */

import fs from 'fs';
import path from 'path';
import * as grpc from '@grpc/grpc-js';
import * as protoLoader from '@grpc/proto-loader';
import {
  Memory,
  MemoryType,
  MemOSConfig,
  SDKLogger,
  UnsupportedOperationError,
  RetrieveOptions,
  ScoredMemory,
  StoreOptions,
  TelemetrySnapshot,
} from './types';

const noopLogger: SDKLogger = {
  debug: () => undefined,
  info: () => undefined,
  warn: () => undefined,
  error: () => undefined,
};

type GrpcCallResponse = Record<string, unknown> & {
  memory_id?: string;
  success?: boolean;
  memories?: Array<Record<string, unknown>>;
};

type MemoryServiceClient = grpc.Client & {
  Store(
    request: Record<string, unknown>,
    callback: (error: grpc.ServiceError | null, response: GrpcCallResponse) => void
  ): grpc.ClientUnaryCall;
  Retrieve(
    request: Record<string, unknown>,
    callback: (error: grpc.ServiceError | null, response: GrpcCallResponse) => void
  ): grpc.ClientUnaryCall;
};

type LoadedGrpcModule = {
  memos?: {
    v1?: {
      MemoryService?: new (
        address: string,
        credentials: grpc.ChannelCredentials,
        options?: grpc.ClientOptions
      ) => MemoryServiceClient;
    };
  };
};

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null && !Array.isArray(value);
}

/**
 * MemOS Client - Primary interface for memory operations
 *
 * @example
 * ```typescript
 * const client = new MemOSClient({
 *   endpoint: 'localhost',
 *   port: 50051,
 * });
 *
 * // Store a memory
 * const memoryId = await client.store('Agent context about user preferences', {
 *   tenantId: 'tenant-123',
 *   agentId: 'agent-456',
 *   type: MemoryType.EPISODIC,
 *   importance: 0.8,
 * });
 *
 * // Retrieve related memories
 * const memories = await client.retrieve('user likes coffee', {
 *   tenantId: 'tenant-123',
 *   agentId: 'agent-456',
 *   limit: 5,
 * });
 *
 * memories.forEach(m => {
 *   console.log(`Score: ${m.score.toFixed(3)} (${m.breakdown?.layer})`);
 *   console.log(`Semantic: ${(m.breakdown?.semanticScore || 0) * 100}%`);
 *   console.log(`Memory: ${m.memory.content}`);
 * });
 * ```
 */
export class MemOSClient {
  private config: MemOSConfig;
  private endpoint: string;
  private port: number;
  private logger: SDKLogger;
  private grpcClient: MemoryServiceClient | null = null;

  /**
   * Initialize MemOS client
   * @param config Configuration for MemOS server connection
   */
  constructor(config: MemOSConfig) {
    this.config = {
      tlsEnabled: false,
      rpcTimeoutMs: 10000,
      ...config,
    };
    this.endpoint = this.config.endpoint;
    this.port = this.config.port;
    this.logger = {
      ...noopLogger,
      ...this.config.logger,
    };
  }

  /**
   * Store a new memory for an agent
   * @param content Memory content to store
   * @param options Store options (tenant, agent, type, importance)
   * @returns Memory ID of stored memory
   */
  async store(content: string, options: StoreOptions): Promise<string> {
    const {
      tenantId,
      agentId,
      type = MemoryType.EPISODIC,
      importance = 0.5,
      metadata = {},
    } = options;

    this.logger.debug('[MemOS SDK] Store request', {
      tenantId,
      agentId,
      contentLength: content.length,
      type,
      importance,
    });

    const response = await this.callStore({
      tenant_id: tenantId,
      agent_id: agentId,
      type: this.toGrpcMemoryType(type),
      content,
      importance,
      metadata: this.toStruct(metadata),
    });

    const memoryId = this.readString(response, 'memory_id');
    if (!memoryId) {
      throw new Error('MemOS Store response did not include memory_id');
    }
    return memoryId;
  }

  /**
   * Retrieve memories matching a query
   * @param query Search query
   * @param options Retrieve options (tenant, agent, limit, weights)
   * @returns Array of scored memories with explainability breakdowns
   */
  async retrieve(query: string, options: RetrieveOptions): Promise<ScoredMemory[]> {
    const {
      tenantId,
      agentId,
      limit = 10,
      similarityThreshold = 0.5,
      alphaSemantic = 0.4,
      betaTemporal = 0.3,
      gammaImportance = 0.2,
    } = options;

    this.logger.debug('[MemOS SDK] Retrieve request', {
      tenantId,
      agentId,
      query: query.substring(0, 100),
      limit,
      similarityThreshold,
    });

    const response = await this.callRetrieve({
      tenant_id: tenantId,
      agent_id: agentId,
      query,
      limit,
      similarity_threshold: similarityThreshold,
      alpha_semantic: alphaSemantic,
      beta_temporal: betaTemporal,
      gamma_importance: gammaImportance,
    });

    const memories = Array.isArray(response.memories) ? response.memories : [];
    return memories.map((item) => this.toScoredMemory(item));
  }

  /**
   * Batch retrieve operation for multiple queries
   * @param queries Array of search queries
   * @param options Retrieve options
   * @returns Array of query results
   */
  async batchRetrieve(
    queries: string[],
    options: RetrieveOptions
  ): Promise<ScoredMemory[][]> {
    return Promise.all(queries.map((q) => this.retrieve(q, options)));
  }

  /**
   * Get memory by ID
   * @param memoryId Memory ID to retrieve
   * @param tenantId Tenant ID (for RLS enforcement)
   * @returns Memory object
   */
  async getMemory(_memoryId: string, _tenantId: string): Promise<Memory | null> {
    throw new UnsupportedOperationError('getMemory');
  }

  /**
   * Update memory importance (affects decay and ranking)
   * @param memoryId Memory ID to update
   * @param importance New importance score [0-1]
   * @param tenantId Tenant ID for RLS
   */
  async updateImportance(
    _memoryId: string,
    _importance: number,
    _tenantId: string
  ): Promise<void> {
    throw new UnsupportedOperationError('updateImportance');
  }

  /**
   * Delete a memory
   * @param memoryId Memory ID to delete
   * @param tenantId Tenant ID for RLS
   */
  async deleteMemory(_memoryId: string, _tenantId: string): Promise<void> {
    throw new UnsupportedOperationError('deleteMemory');
  }

  /**
   * Get telemetry metrics for observability
   * @returns Telemetry snapshot
   */
  async getMetrics(): Promise<TelemetrySnapshot> {
    const url = this.config.metricsUrl || `http://${this.endpoint}:9090/metrics`;
    this.logger.debug('[MemOS SDK] Fetching metrics', { url });
    const response = await fetch(url, {
      headers: {
        Accept: 'text/plain',
      },
    });
    if (!response.ok) {
      throw new Error(`Failed to fetch metrics from ${url}: ${response.status}`);
    }

    return this.parsePrometheusTelemetry(await response.text());
  }

  /**
   * Health check
   * @returns true if service is healthy
   */
  async health(): Promise<boolean> {
    try {
      this.logger.debug('[MemOS SDK] Health check', {
        address: this.getGrpcAddress(),
      });
      await this.waitForGrpcReady(this.config.rpcTimeoutMs ?? 10000);
      return true;
    } catch {
      return false;
    }
  }

  /**
   * Close client connection
   */
  close(): void {
    this.logger.debug('[MemOS SDK] Closing client');
    this.grpcClient?.close();
    this.grpcClient = null;
  }

  private getGrpcAddress(): string {
    return `${this.endpoint}:${this.port}`;
  }

  private resolveProtoPath(): string {
    const candidates = [
      path.resolve(__dirname, '../proto/memory.proto'),
      path.resolve(__dirname, '../../proto/memory.proto'),
      path.resolve(process.cwd(), 'proto/memory.proto'),
    ];

    for (const candidate of candidates) {
      if (fs.existsSync(candidate)) {
        return candidate;
      }
    }

    throw new Error('Unable to locate memory.proto for the MemOS TypeScript SDK');
  }

  private resolveGoogleProtoRoot(): string | null {
    try {
      return path.dirname(require.resolve('protobufjs/package.json'));
    } catch {
      return null;
    }
  }

  private getGrpcClient(): MemoryServiceClient {
    if (this.grpcClient) {
      return this.grpcClient;
    }

    const protoPath = this.resolveProtoPath();
    const includeDirs = [path.dirname(protoPath)];
    const googleProtoRoot = this.resolveGoogleProtoRoot();
    if (googleProtoRoot) {
      includeDirs.push(googleProtoRoot);
    }

    const packageDefinition = protoLoader.loadSync(protoPath, {
      keepCase: true,
      longs: String,
      enums: String,
      defaults: true,
      arrays: true,
      objects: true,
      oneofs: true,
      includeDirs,
    });

    const loaded = grpc.loadPackageDefinition(packageDefinition) as LoadedGrpcModule;
    const serviceCtor = loaded.memos?.v1?.MemoryService;
    if (!serviceCtor) {
      throw new Error('MemOS MemoryService is missing from the loaded proto definition');
    }

    const credentials = this.createCredentials();
    this.grpcClient = new serviceCtor(this.getGrpcAddress(), credentials);
    return this.grpcClient;
  }

  private createCredentials(): grpc.ChannelCredentials {
    if (!this.config.tlsEnabled) {
      return grpc.credentials.createInsecure();
    }

    if (!this.config.credentials) {
      return grpc.credentials.createSsl();
    }

    const credentialPath = path.resolve(this.config.credentials);
    const credentialBytes = fs.existsSync(credentialPath)
      ? fs.readFileSync(credentialPath)
      : Buffer.from(this.config.credentials, 'utf8');

    return grpc.credentials.createSsl(credentialBytes);
  }

  private async waitForGrpcReady(timeoutMs: number): Promise<void> {
    const client = this.getGrpcClient();
    await new Promise<void>((resolve, reject) => {
      const deadline = new Date(Date.now() + timeoutMs);
      client.waitForReady(deadline, (error) => {
        if (error) {
          reject(error);
          return;
        }
        resolve();
      });
    });
  }

  private async callStore(request: Record<string, unknown>): Promise<GrpcCallResponse> {
    const client = this.getGrpcClient();
    return await new Promise<GrpcCallResponse>((resolve, reject) => {
      client.Store(request, this.toCallCallback(resolve, reject));
    });
  }

  private async callRetrieve(request: Record<string, unknown>): Promise<GrpcCallResponse> {
    const client = this.getGrpcClient();
    return await new Promise<GrpcCallResponse>((resolve, reject) => {
      client.Retrieve(request, this.toCallCallback(resolve, reject));
    });
  }

  private toCallCallback(
    resolve: (value: GrpcCallResponse) => void,
    reject: (reason: unknown) => void
  ): (error: grpc.ServiceError | null, response: GrpcCallResponse) => void {
    return (error, response) => {
      if (error) {
        reject(error);
        return;
      }
      resolve(response);
    };
  }

  private toGrpcMemoryType(type: MemoryType): string {
    return type;
  }

  private toStruct(value: Record<string, unknown>): Record<string, unknown> {
    return JSON.parse(JSON.stringify(value ?? {})) as Record<string, unknown>;
  }

  private readString(value: GrpcCallResponse, key: 'memory_id' | 'memoryId'): string {
    const result = value[key];
    return typeof result === 'string' ? result : '';
  }

  private toMemoryType(value: unknown): MemoryType {
    if (typeof value === 'string') {
      if ((Object.values(MemoryType) as string[]).includes(value)) {
        return value as MemoryType;
      }
      const prefixed = `MEMORY_TYPE_${value.replace(/^MEMORY_TYPE_/, '').toUpperCase()}`;
      if ((Object.values(MemoryType) as string[]).includes(prefixed)) {
        return prefixed as MemoryType;
      }
    }

    switch (Number(value)) {
      case 1:
        return MemoryType.EPISODIC;
      case 2:
        return MemoryType.SEMANTIC;
      case 3:
        return MemoryType.PROCEDURAL;
      case 4:
        return MemoryType.REFLECTIVE;
      case 5:
        return MemoryType.TRANSIENT;
      default:
        return MemoryType.UNSPECIFIED;
    }
  }

  private toDate(value: unknown): Date {
    if (value instanceof Date) {
      return value;
    }
    if (typeof value === 'string' || typeof value === 'number') {
      const parsed = new Date(value);
      return Number.isNaN(parsed.getTime()) ? new Date(0) : parsed;
    }
    if (isRecord(value)) {
      const seconds = Number(value.seconds ?? value._seconds ?? 0);
      const nanos = Number(value.nanos ?? value._nanos ?? 0);
      if (Number.isFinite(seconds)) {
        return new Date(seconds * 1000 + nanos / 1_000_000);
      }
    }
    return new Date(0);
  }

  private toMemory(value: unknown): Memory {
    const record = isRecord(value) ? value : {};
    const metadata = record.metadata;
    return {
      id: typeof record.id === 'string' ? record.id : '',
      tenantId: typeof record.tenant_id === 'string' ? record.tenant_id : '',
      agentId: typeof record.agent_id === 'string' ? record.agent_id : '',
      type: this.toMemoryType(record.type),
      content: typeof record.content === 'string' ? record.content : '',
      embedding: Array.isArray(record.embedding)
        ? record.embedding.map((item) => Number(item)).filter((item) => Number.isFinite(item))
        : undefined,
      importance: Number(record.importance ?? 0),
      createdAt: this.toDate(record.created_at),
      updatedAt: this.toDate(record.updated_at),
      metadata: isRecord(metadata) ? metadata : undefined,
    };
  }

  private toScoreBreakdown(value: unknown): {
    semanticScore: number;
    temporalScore: number;
    importanceScore: number;
    reinforcementScore: number;
    layer: 'working' | 'episodic' | 'long_term' | 'archive';
  } | undefined {
    if (!isRecord(value)) {
      return undefined;
    }

    const layer = typeof value.layer === 'string' ? value.layer : 'episodic';
    const normalizedLayer = (['working', 'episodic', 'long_term', 'archive'] as const).includes(
      layer as 'working' | 'episodic' | 'long_term' | 'archive'
    )
      ? (layer as 'working' | 'episodic' | 'long_term' | 'archive')
      : 'episodic';

    return {
      semanticScore: Number(value.semantic_score ?? value.semanticScore ?? 0),
      temporalScore: Number(value.temporal_score ?? value.temporalScore ?? 0),
      importanceScore: Number(value.importance_score ?? value.importanceScore ?? 0),
      reinforcementScore: Number(value.reinforcement_score ?? value.reinforcementScore ?? 0),
      layer: normalizedLayer,
    };
  }

  private toScoredMemory(value: unknown): ScoredMemory {
    const record = isRecord(value) ? value : {};
    return {
      memory: this.toMemory(record.memory),
      score: Number(record.score ?? 0),
      breakdown: this.toScoreBreakdown(record.breakdown),
    };
  }

  private parsePrometheusTelemetry(text: string): TelemetrySnapshot {
    const getMetric = (name: string): number => {
      const line = text.match(new RegExp(`^${name}\\s+([0-9.]+)$`, 'm'));
      return line ? Number(line[1]) : 0;
    };

    const storeCount = getMetric('memos_store_requests_total');
    const retrieveCount = getMetric('memos_retrieve_requests_total');
    const cacheHitCount = getMetric('memos_cache_hits_total');
    const cacheMissCount = getMetric('memos_cache_misses_total');
    const totalCache = cacheHitCount + cacheMissCount;

    return {
      storeCount,
      retrieveCount,
      auditWriteCount: getMetric('memos_audit_writes_total'),
      auditReadCount: getMetric('memos_audit_reads_total'),
      authDeniedCount: getMetric('memos_auth_denied_total'),
      cacheHitCount,
      cacheMissCount,
      cacheHitRate: totalCache > 0 ? (cacheHitCount / totalCache) * 100 : 0,
      replicationLagAvgMs: getMetric('memos_replication_lag_ms_avg'),
      replicationLagMaxMs: getMetric('memos_replication_lag_ms_max'),
      storeLatencyAvgMs: getMetric('memos_store_latency_ms_avg'),
      retrieveLatencyAvgMs: getMetric('memos_retrieve_latency_ms_avg'),
      totalRequests: storeCount + retrieveCount,
    };
  }
}

/**
 * Create a MemOS client instance
 */
export function createClient(config: MemOSConfig): MemOSClient {
  return new MemOSClient(config);
}
