import time
import numpy as np
from memos_sdk import MemOSClient

# Configuration
TENANT_ID = "00000000-0000-0000-0000-000000000001" # Assume this exists or was seeded
AGENT_ID = "00000000-0000-0000-0000-000000000002"
NUM_TRIALS = 100
SERVER_URL = "localhost:50051"

def run_latency_benchmark():
    client = MemOSClient(SERVER_URL)
    latencies = []
    
    print(f"Running latency benchmark for {NUM_TRIALS} trials...")
    
    # Warmup
    for _ in range(5):
        client.retrieve(TENANT_ID, AGENT_ID, "warmup query")
        
    for i in range(NUM_TRIALS):
        start = time.time()
        client.retrieve(TENANT_ID, AGENT_ID, f"test query {i}")
        end = time.time()
        latencies.append((end - start) * 1000) # Convert to ms
        
    print("\n" + "="*30)
    print("BENCHMARK RESULTS (MemOS SDK)")
    print("="*30)
    print(f"Mean Latency: {np.mean(latencies):.2f} ms")
    print(f"Median Latency: {np.median(latencies):.2f} ms")
    print(f"P95 Latency: {np.percentile(latencies, 95):.2f} ms")
    print(f"P99 Latency: {np.percentile(latencies, 99):.2f} ms")
    print("="*30)

if __name__ == "__main__":
    run_latency_benchmark()
