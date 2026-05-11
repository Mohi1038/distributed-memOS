import grpc
import uuid
from typing import List, Optional, Dict, Any

from . import memory_pb2
from . import memory_pb2_grpc


class MemOSClient:
    def __init__(self, target: str = "localhost:50051"):
        """
        Initialize the MemOS client.
        
        Args:
            target (str): The address of the MemOS gRPC server (e.g., "localhost:50051").
        """
        self.channel = grpc.insecure_channel(target)
        self.stub = memory_pb2_grpc.MemoryServiceStub(self.channel)

    def store(
        self,
        tenant_id: str,
        agent_id: str,
        content: str,
        memory_type: memory_pb2.MemoryType = memory_pb2.MEMORY_TYPE_EPISODIC,
        importance: float = 0.5,
        metadata: Optional[Dict[str, Any]] = None
    ) -> str:
        """
        Store a new memory.
        
        Args:
            tenant_id (str): The UUID of the tenant.
            agent_id (str): The UUID of the agent.
            content (str): The raw text content of the memory.
            memory_type (MemoryType): The type of memory (default: EPISODIC).
            importance (float): The importance score between 0.0 and 1.0 (default: 0.5).
            metadata (dict, optional): Additional key-value metadata.
            
        Returns:
            str: The UUID of the stored memory.
        """
        # We pass metadata as a JSON string or struct, but here we can keep it simple 
        # and just omit it or pass empty for the basic SDK. Let's pass empty for MVP.
        req = memory_pb2.StoreRequest(
            tenant_id=tenant_id,
            agent_id=agent_id,
            type=memory_type,
            content=content,
            importance=importance,
        )
        # Note: we are ignoring metadata for the python MVP, 
        # normally we'd convert the dict to google.protobuf.Struct
        
        response = self.stub.Store(req)
        return response.memory_id

    def retrieve(
        self,
        tenant_id: str,
        agent_id: str,
        query: str,
        limit: int = 5,
        similarity_threshold: float = 0.5,
        alpha_semantic: float = 0.4,
        beta_temporal: float = 0.2,
        gamma_importance: float = 0.3
    ) -> List[memory_pb2.ScoredMemory]:
        """
        Retrieve memories using the cognitive ranking algorithm.
        
        Args:
            tenant_id (str): The UUID of the tenant.
            agent_id (str): The UUID of the agent.
            query (str): The search query.
            limit (int): Maximum number of results to return.
            similarity_threshold (float): Minimum final score to include in results.
            alpha_semantic (float): Weight for semantic similarity.
            beta_temporal (float): Weight for temporal decay.
            gamma_importance (float): Weight for user-defined importance.
            
        Returns:
            List[ScoredMemory]: A list of retrieved memories with their cognitive scores.
        """
        req = memory_pb2.RetrieveRequest(
            tenant_id=tenant_id,
            agent_id=agent_id,
            query=query,
            limit=limit,
            similarity_threshold=similarity_threshold,
            alpha_semantic=alpha_semantic,
            beta_temporal=beta_temporal,
            gamma_importance=gamma_importance,
        )
        
        response = self.stub.Retrieve(req)
        return list(response.memories)

    def close(self):
        """Close the underlying gRPC channel."""
        self.channel.close()
        
    def __enter__(self):
        return self
        
    def __exit__(self, exc_type, exc_val, exc_tb):
        self.close()
