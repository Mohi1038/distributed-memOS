import uuid
import time
import random
from memos_sdk import MemOSClient, MemoryType

# Benchmark Configuration
TENANT_ID = str(uuid.uuid4())
AGENT_ID = str(uuid.uuid4())
NUM_MEMORIES = 1000
SERVER_URL = "localhost:50051"

def seed_benchmark_data():
    client = MemOSClient(SERVER_URL)
    print(f"Seeding {NUM_MEMORIES} memories for tenant {TENANT_ID}...")
    
    start_time = time.time()
    for i in range(NUM_MEMORIES):
        content = f"This is benchmark memory number {i}. It contains some random context like {random.randint(0, 10000)}."
        importance = random.random()
        client.store(
            tenant_id=TENANT_ID,
            agent_id=AGENT_ID,
            content=content,
            importance=importance
        )
        if i % 100 == 0:
            print(f"Progress: {i}/{NUM_MEMORIES}")
            
    end_time = time.time()
    print(f"Finished seeding in {end_time - start_time:.2f} seconds.")
    return TENANT_ID, AGENT_ID

if __name__ == "__main__":
    seed_benchmark_data()
