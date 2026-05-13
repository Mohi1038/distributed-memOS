/**
 * MemOS CrewAI Adapter
 * Integration for CrewAI agent framework
 */

import { MemOSClient } from '../client';
import { MemoryType, ScoredMemory } from '../types';

/**
 * CrewAI Tool Specification
 */
export interface CrewAITool {
  name: string;
  description: string;
  callback: (input: Record<string, unknown>) => Promise<string>;
}

/**
 * Create a CrewAI-compatible MemOS store tool
 * @param client MemOS client instance
 * @param tenantId Tenant ID for the agent
 * @param agentId Agent ID
 */
export function createCrewAIStoreTool(
  client: MemOSClient,
  tenantId: string,
  agentId: string
): CrewAITool {
  return {
    name: 'memos_store_memory',
    description:
      'Store important information in MemOS for the agent to remember. ' +
      'Use this to persist context, decisions, and knowledge that should be available in future interactions.',
    callback: async (input: Record<string, unknown>) => {
      try {
        const content = input.content as string;
        const importance = (input.importance as number) || 0.7;
        const type = (input.type as string) || 'MEMORY_TYPE_EPISODIC';

        const memoryId = await client.store(content, {
          tenantId,
          agentId,
          type: type as MemoryType,
          importance: Math.min(1, Math.max(0, importance)),
        });

        return JSON.stringify({
          status: 'success',
          memoryId,
          message: `Memory stored successfully with ID: ${memoryId}`,
        });
      } catch (error) {
        return JSON.stringify({
          status: 'error',
          message: `Failed to store memory: ${error}`,
        });
      }
    },
  };
}

/**
 * Create a CrewAI-compatible MemOS retrieve tool
 * @param client MemOS client instance
 * @param tenantId Tenant ID for the agent
 * @param agentId Agent ID
 */
export function createCrewAIRetrieveTool(
  client: MemOSClient,
  tenantId: string,
  agentId: string
): CrewAITool {
  return {
    name: 'memos_retrieve_memories',
    description:
      'Search MemOS for relevant memories and prior knowledge. ' +
      'Use this to recall important information that has been learned or experienced before.',
    callback: async (input: Record<string, unknown>) => {
      try {
        const query = input.query as string;
        const limit = Math.min(20, (input.limit as number) || 5);
        const threshold = (input.threshold as number) || 0.5;

        const memories = await client.retrieve(query, {
          tenantId,
          agentId,
          limit,
          similarityThreshold: Math.min(1, Math.max(0, threshold)),
        });

        // Format response as natural language for CrewAI agent
        if (memories.length === 0) {
          return `No relevant memories found for query: "${query}"`;
        }

        const formatted = memories
          .map((m: ScoredMemory, idx: number) => {
            const score = (m.score * 100).toFixed(0);
            const semantic = ((m.breakdown?.semanticScore || 0) * 100).toFixed(0);
            const layer = m.breakdown?.layer || 'unknown';
            return `[${idx + 1}] (${score}% match, ${layer} layer) ${m.memory.content}`;
          })
          .join('\n');

        return `Found ${memories.length} relevant memories:\n\n${formatted}`;
      } catch (error) {
        return `Error retrieving memories: ${error}`;
      }
    },
  };
}

/**
 * Create CrewAI MemOS toolkit
 * @param client MemOS client instance
 * @param tenantId Tenant ID
 * @param agentId Agent ID
 * @returns Array of CrewAI-compatible tools
 */
export function createCrewAIToolkit(
  client: MemOSClient,
  tenantId: string,
  agentId: string
): CrewAITool[] {
  return [
    createCrewAIStoreTool(client, tenantId, agentId),
    createCrewAIRetrieveTool(client, tenantId, agentId),
  ];
}

/**
 * Example: How to use MemOS with CrewAI
 *
 * @example
 * ```python
 * from crewai import Agent, Task, Crew
 * from memos_sdk import MemOSClient, create_crewai_toolkit
 *
 * # Initialize MemOS client
 * client = MemOSClient(
 *     endpoint='localhost',
 *     port=50051
 * )
 *
 * # Create toolkit
 * toolkit = create_crewai_toolkit(
 *     client,
 *     tenant_id='tenant-123',
 *     agent_id='agent-456'
 * )
 *
 * # Create agent with MemOS tools
 * agent = Agent(
 *     role='Research Assistant',
 *     goal='Answer questions using MemOS memory',
 *     tools=toolkit
 * )
 *
 * task = Task(
 *     description='Answer: What was discussed about topic X?',
 *     agent=agent
 * )
 *
 * crew = Crew(agents=[agent], tasks=[task])
 * crew.kickoff()
 * ```
 */
