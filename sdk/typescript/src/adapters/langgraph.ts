/**
 * MemOS LangGraph Adapter
 * Integration for LangGraph/LangChain workflows
 */

import { MemOSClient } from '../client';
import { MemoryType, ScoredMemory } from '../types';

/**
 * LangGraph Tool Configuration
 * Defines how MemOS integrates with LangGraph as a tool
 */
export interface LangGraphTool {
  name: string;
  description: string;
  func: (input: Record<string, unknown>) => Promise<unknown>;
  schema: Record<string, unknown>;
}

/**
 * Create a LangGraph-compatible MemOS store tool
 * @param client MemOS client instance
 * @param tenantId Tenant ID for the tool
 * @param agentId Agent ID for the tool
 */
export function createStoreTool(
  client: MemOSClient,
  tenantId: string,
  agentId: string
): LangGraphTool {
  return {
    name: 'memos_store',
    description: 'Store a memory for later retrieval. Use this to save important information that the agent should remember.',
    func: async (input: Record<string, unknown>) => {
      const content = input.content as string;
      const importance = (input.importance as number) || 0.7;
      const type = (input.type as string) || 'MEMORY_TYPE_EPISODIC';

      const memoryId = await client.store(content, {
        tenantId,
        agentId,
        type: type as MemoryType,
        importance,
      });

      return {
        success: true,
        memoryId,
        message: `Memory stored with ID: ${memoryId}`,
      };
    },
    schema: {
      type: 'object',
      properties: {
        content: {
          type: 'string',
          description: 'The content to store in memory',
        },
        importance: {
          type: 'number',
          minimum: 0,
          maximum: 1,
          description: 'How important this memory is (0-1). Default: 0.7',
        },
        type: {
          type: 'string',
          enum: ['MEMORY_TYPE_EPISODIC', 'MEMORY_TYPE_SEMANTIC', 'MEMORY_TYPE_PROCEDURAL'],
          description: 'Type of memory to store. Default: MEMORY_TYPE_EPISODIC',
        },
      },
      required: ['content'],
    },
  };
}

/**
 * Create a LangGraph-compatible MemOS retrieve tool
 * @param client MemOS client instance
 * @param tenantId Tenant ID for the tool
 * @param agentId Agent ID for the tool
 */
export function createRetrieveTool(
  client: MemOSClient,
  tenantId: string,
  agentId: string
): LangGraphTool {
  return {
    name: 'memos_retrieve',
    description: 'Search for relevant memories. Use this to recall important information related to the current context.',
    func: async (input: Record<string, unknown>) => {
      const query = input.query as string;
      const limit = (input.limit as number) || 5;
      const threshold = (input.threshold as number) || 0.5;

      const memories = await client.retrieve(query, {
        tenantId,
        agentId,
        limit,
        similarityThreshold: threshold,
      });

      // Format for LangGraph/LLM consumption
      const formatted = memories.map((m: ScoredMemory) => ({
        content: m.memory.content,
        score: m.score.toFixed(3),
        layer: m.breakdown?.layer,
        semanticMatch: `${((m.breakdown?.semanticScore || 0) * 100).toFixed(0)}%`,
        recency: `${((m.breakdown?.temporalScore || 0) * 100).toFixed(0)}%`,
        reinforcement: `${((m.breakdown?.reinforcementScore || 0) * 100).toFixed(0)}%`,
      }));

      return {
        success: true,
        count: formatted.length,
        memories: formatted,
        message: `Retrieved ${formatted.length} relevant memories`,
      };
    },
    schema: {
      type: 'object',
      properties: {
        query: {
          type: 'string',
          description: 'Search query to find relevant memories',
        },
        limit: {
          type: 'integer',
          minimum: 1,
          maximum: 50,
          description: 'Maximum number of memories to retrieve. Default: 5',
        },
        threshold: {
          type: 'number',
          minimum: 0,
          maximum: 1,
          description: 'Minimum similarity threshold (0-1). Default: 0.5',
        },
      },
      required: ['query'],
    },
  };
}

/**
 * Create LangGraph MemOS toolkit
 * @param client MemOS client instance
 * @param tenantId Tenant ID
 * @param agentId Agent ID
 * @returns Array of LangGraph-compatible tools
 */
export function createMemOSToolkit(
  client: MemOSClient,
  tenantId: string,
  agentId: string
): LangGraphTool[] {
  return [
    createStoreTool(client, tenantId, agentId),
    createRetrieveTool(client, tenantId, agentId),
  ];
}

/**
 * Example: How to use MemOS with LangGraph
 *
 * @example
 * ```typescript
 * import { createClient } from '@memos/sdk';
 * import { createMemOSToolkit } from '@memos/sdk/adapters/langgraph';
 *
 * const client = createClient({
 *   endpoint: 'localhost',
 *   port: 50051,
 * });
 *
 * const tools = createMemOSToolkit(client, 'tenant-123', 'agent-456');
 *
 * // In LangGraph StateGraph:
 * graph.add_tool_calls(tools.map(t => ({
 *   name: t.name,
 *   description: t.description,
 *   func: t.func,
 *   schema: t.schema,
 * })));
 * ```
 */
