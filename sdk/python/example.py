import uuid
from memos_sdk import MemOSClient, MemoryType

def main():
    # Connect to the local MemOS instance
    client = MemOSClient("localhost:50051")
    
    # Use the dev tenant and agent UUIDs
    tenant_id = "00000000-0000-0000-0000-000000000001"
    agent_id = "00000000-0000-0000-0000-000000000002"
    
    print("--- MemOS Python SDK Example ---")
    
    # 1. Store a memory
    print("\n1. Storing a new memory...")
    content = "The user prefers a high contrast dark mode with large fonts for the UI."
    memory_id = client.store(
        tenant_id=tenant_id,
        agent_id=agent_id,
        content=content,
        memory_type=MemoryType.MEMORY_TYPE_EPISODIC,
        importance=0.8
    )
    print(f"✅ Stored memory with ID: {memory_id}")
    
    # 2. Retrieve memories using cognitive ranking
    print("\n2. Retrieving memories related to 'dark mode'...")
    results = client.retrieve(
        tenant_id=tenant_id,
        agent_id=agent_id,
        query="UI preferences dark mode",
        limit=3
    )
    
    print(f"✅ Found {len(results)} relevant memories:")
    for res in results:
        print(f"   [Score: {res.score:.2f}] {res.memory.content}")

if __name__ == "__main__":
    main()
