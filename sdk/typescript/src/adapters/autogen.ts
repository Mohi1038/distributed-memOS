/**
 * MemOS AutoGen Adapter
 * Integration for Microsoft AutoGen multi-agent framework
 */

import { MemOSClient } from '../client';
import { MemoryType } from '../types';

/**
 * AutoGen Tool Definition
 */
export interface AutoGenTool {
  name: string;
  description: string;
  params: Record<string, unknown>;
  callable: (input: Record<string, unknown>) => Promise<string>;
}

/**
 * Create AutoGen-compatible MemOS store function
 * @param client MemOS client instance
 * @param tenantId Tenant ID
 * @param agentId Agent ID
 */
export function createAutoGenStoreTool(
  client: MemOSClient,
  tenantId: string,
  agentId: string
): AutoGenTool {
  return {
    name: 'store_memory',
    description:
      'Store a memory in MemOS. This persists information for future use by the agent.',
    params: {
      type: 'object',
      properties: {
        content: {
          type: 'string',
          description: 'The content to store',
        },
        importance: {
          type: 'number',
          minimum: 0,
          maximum: 1,
          description: 'Importance score (0-1)',
        },
        category: {
          type: 'string',
          enum: ['MEMORY_TYPE_EPISODIC', 'MEMORY_TYPE_SEMANTIC', 'MEMORY_TYPE_PROCEDURAL'],
          description: 'Memory category',
        },
      },
      required: ['content'],
    },
    callable: async (input: Record<string, unknown>) => {
      try {
        const content = input.content as string;
        const importance = (input.importance as number) || 0.6;
        const category = (input.category as string) || 'MEMORY_TYPE_EPISODIC';

        const memoryId = await client.store(content, {
          tenantId,
          agentId,
          type: category as MemoryType,
          importance: Math.min(1, Math.max(0, importance)),
        });

        return JSON.stringify({
          success: true,
          memoryId,
          timestamp: new Date().toISOString(),
        });
      } catch (error) {
        return JSON.stringify({ error: String(error) });
      }
    },
  };
}

/**
 * Create AutoGen-compatible MemOS retrieve function
 * @param client MemOS client instance
 * @param tenantId Tenant ID
 * @param agentId Agent ID
 */
export function createAutoGenRetrieveTool(
  client: MemOSClient,
  tenantId: string,
  agentId: string
): AutoGenTool {
  return {
    name: 'retrieve_memory',
    description:
      'Search for memories in MemOS. Returns the most relevant stored information.',
    params: {
      type: 'object',
      properties: {
        query: {
          type: 'string',
          description: 'Search query',
        },
        limit: {
          type: 'integer',
          minimum: 1,
          maximum: 50,
          description: 'Max results to return',
        },
        threshold: {
          type: 'number',
          minimum: 0,
          maximum: 1,
          description: 'Minimum relevance threshold',
        },
      },
      required: ['query'],
    },
    callable: async (input: Record<string, unknown>) => {
      try {
        const query = input.query as string;
        const limit = (input.limit as number) || 5;
        const threshold = (input.threshold as number) || 0.5;

        const memories = await client.retrieve(query, {
          tenantId,
          agentId,
          limit,
          similarityThreshold: threshold,
        });

        const results = memories.map((m) => ({
          text: m.memory.content,
          relevance: (m.score * 100).toFixed(1) + '%',
          layer: m.breakdown?.layer,
        }));

        return JSON.stringify({
          success: true,
          count: results.length,
          results,
        });
      } catch (error) {
        return JSON.stringify({ error: String(error) });
      }
    },
  };
}

/**
 * Create AutoGen MemOS toolkit
 * @param client MemOS client instance
 * @param tenantId Tenant ID
 * @param agentId Agent ID
 * @returns Array of AutoGen-compatible tools
 */
export function createAutoGenToolkit(
  client: MemOSClient,
  tenantId: string,
  agentId: string
): AutoGenTool[] {
  return [
    createAutoGenStoreTool(client, tenantId, agentId),
    createAutoGenRetrieveTool(client, tenantId, agentId),
  ];
}

/**
 * Example: How to use MemOS with AutoGen
 *
 * @example
 * ```python
 * import autogen
 * from memos_sdk import MemOSClient, create_autogen_toolkit
 *
 * # Initialize MemOS
 * client = MemOSClient(endpoint='localhost', port=50051)
 * toolkit = create_autogen_toolkit(client, 'tenant-123', 'agent-456')
 *
 * # Convert to AutoGen format
 * tools = [
 *     {
 *         'type': 'function',
 *         'function': {
 *             'name': tool.name,
 *             'description': tool.description,
 *             'parameters': tool.params,
 *         }
 *     }
 *     for tool in toolkit
 * ]
 *
 * # Create agent
 * agent = autogen.AssistantAgent(
 *     name='assistant',
 *     llm_config={
 *         'config_list': [...],
 *         'tools': tools
 *     }
 * )
 *
 * # Use in conversation
 * user = autogen.UserProxyAgent()
 * user.initiate_chat(agent, message='What do you remember about our last discussion?')
 * ```
 */
