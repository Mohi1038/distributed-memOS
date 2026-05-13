#!/usr/bin/env bash
set -euo pipefail

BASE_DIR="$(cd "$(dirname "$0")/.." && pwd)"
COMPOSE_FILE="$BASE_DIR/deployments/docker-compose.cluster.yml"
RELEASE_BIN="$BASE_DIR/releases/memos-server-darwin-arm64"
LOG_DIR="$BASE_DIR/.demo-logs"
mkdir -p "$LOG_DIR"
rm -f "$LOG_DIR"/node-*.log "$LOG_DIR"/.startup-pids

NODE_MODE="release"
NODE_PIDS=""

echo
echo "=== MemOS Phase 3 Visual Demo ==="
echo

if ! command -v docker >/dev/null 2>&1; then
  echo "ERROR: Docker is required to start Postgres, NATS, and Qdrant." >&2
  exit 1
fi

if [[ ! -x "$RELEASE_BIN" ]]; then
  echo "WARNING: Release binary not found at $RELEASE_BIN. Will use source mode (go run)." >&2
fi

if command -v docker compose >/dev/null 2>&1; then
  DC_CMD=(docker compose -f "$COMPOSE_FILE")
elif command -v docker-compose >/dev/null 2>&1; then
  DC_CMD=(docker-compose -f "$COMPOSE_FILE")
else
  echo "ERROR: Neither 'docker compose' nor 'docker-compose' is available." >&2
  exit 1
fi

cleanup() {
  echo ""
  echo "[CLEANUP] Terminating MemOS nodes..."
  if [[ -f "$LOG_DIR/.startup-pids" ]]; then
    while read -r pid; do
      kill "$pid" 2>/dev/null || true
    done < "$LOG_DIR/.startup-pids"
    rm -f "$LOG_DIR/.startup-pids"
  fi
  if [[ -n "${NODE_PIDS:-}" ]]; then
    for pid in $NODE_PIDS; do
      kill "$pid" 2>/dev/null || true
    done
  fi
  echo "[CLEANUP] Bringing down Docker containers..."
  "${DC_CMD[@]}" down -v --remove-orphans >/dev/null 2>&1 || true
}

trap cleanup EXIT INT TERM

echo "[INFRA] Starting infrastructure: Postgres, NATS, and Qdrant..."
pkill -f "$RELEASE_BIN" >/dev/null 2>&1 || true
pkill -f "go run.*memos/main.go" >/dev/null 2>&1 || true
"${DC_CMD[@]}" down -v --remove-orphans >/dev/null 2>&1 || true
"${DC_CMD[@]}" up -d nats postgres qdrant

echo "[INFRA] Waiting for infrastructure ports..."
for attempt in {1..30}; do
  if nc -z -w 1 127.0.0.1 5432 && nc -z -w 1 127.0.0.1 4222 && nc -z -w 1 127.0.0.1 6333 2>/dev/null; then
    echo "[INFRA] All ports available"
    break
  fi
  if [[ $attempt -eq 30 ]]; then
    echo "ERROR: Infrastructure ports did not become available" >&2
    exit 1
  fi
  sleep 1
done

echo "[INFRA] Waiting for Postgres to accept connections..."
for attempt in {1..30}; do
  if "${DC_CMD[@]}" exec -T postgres pg_isready -U user -d memos >/dev/null 2>&1; then
    echo "[INFRA] Postgres is ready"
    break
  fi
  if [[ $attempt -eq 30 ]]; then
    echo "ERROR: Postgres did not become ready" >&2
    exit 1
  fi
  sleep 1
done

echo "[INFRA] Ensuring Postgres uuid-ossp extension..."
"${DC_CMD[@]}" exec -T postgres psql -U user -d memos -c 'CREATE EXTENSION IF NOT EXISTS "uuid-ossp";' >/dev/null 2>&1
echo "[INFRA] ✓ uuid-ossp extension ready"

echo "[INFRA] Initializing database schema..."
POSTGRES_ID=$("${DC_CMD[@]}" ps -q postgres)
if [[ -z "$POSTGRES_ID" ]]; then
  echo "[INFRA] ✗ Could not find postgres container" >&2
  exit 1
fi

docker cp "$BASE_DIR/scripts/init.sql" "$POSTGRES_ID:/tmp/init.sql"
docker exec "$POSTGRES_ID" psql -U user -d memos -f /tmp/init.sql >/dev/null 2>&1

if docker exec "$POSTGRES_ID" psql -U user -d memos -Atc "select to_regclass('public.tenants');" | grep -qx "tenants"; then
  echo "[INFRA] ✓ Schema initialized"
else
  echo "[INFRA] ✗ Failed to initialize schema" >&2
  exit 1
fi

start_node_release() {
  local node_id="$1"
  local grpc_port="$2"
  local log_file="$LOG_DIR/${node_id}.log"

  NODE_ID="$node_id" \
  NODE_HOST="127.0.0.1" \
  NATS_URL="nats://127.0.0.1:4222" \
  POSTGRES_URL="postgres://user:password@127.0.0.1:5432/memos?sslmode=disable" \
  QDRANT_URL="http://127.0.0.1:6333" \
  GRPC_PORT="$grpc_port" \
    "$RELEASE_BIN" 2>&1 | tee -a "$log_file" &

  local pid=$!
  echo $pid >> "$LOG_DIR/.startup-pids"
  echo $pid
}

start_node_source() {
  local node_id="$1"
  local grpc_port="$2"
  local log_file="$LOG_DIR/${node_id}.log"

  NODE_ID="$node_id" \
  NODE_HOST="127.0.0.1" \
  NATS_URL="nats://127.0.0.1:4222" \
  POSTGRES_URL="postgres://user:password@127.0.0.1:5432/memos?sslmode=disable" \
  QDRANT_URL="http://127.0.0.1:6333" \
  GRPC_PORT="$grpc_port" \
    go run "$BASE_DIR/cmd/memos/main.go" 2>&1 | tee -a "$log_file" &

  local pid=$!
  echo $pid >> "$LOG_DIR/.startup-pids"
  echo $pid
}

wait_for_all_ports() {
  local max_attempts="${1:-30}"
  local attempt=0
  
  while [[ $attempt -lt $max_attempts ]]; do
    if nc -z -w 1 127.0.0.1 50051 && nc -z -w 1 127.0.0.1 50052 && nc -z -w 1 127.0.0.1 50053 2>/dev/null; then
      echo "[PORTS] All three MemOS gRPC ports are open and listening"
      return 0
    fi
    attempt=$((attempt + 1))
    if [[ $((attempt % 5)) -eq 0 ]]; then
      echo "[PORTS] Waiting... ($attempt/$max_attempts)"
    fi
    sleep 1
  done
  return 1
}

is_port_open() {
  local port="$1"
  nc -z -w 1 127.0.0.1 "$port" 2>/dev/null
}

verify_node_ready() {
  echo "[VERIFY] Confirming at least node-1 is listening on port 50051..."
  if is_port_open 50051; then
    echo "[VERIFY] ✓ Node-1 is ready"
    return 0
  fi
  echo "[VERIFY] ✗ Node-1 is not ready" >&2
  return 1
}

echo
echo "[NODES] Launching MemOS cluster (3 nodes)..."
echo

# Try release mode first if binary exists
if [[ -x "$RELEASE_BIN" ]]; then
  echo "[NODES] Attempting release binary mode..."
  NODE_PIDS="$(start_node_release node-1 50051) $(start_node_release node-2 50052) $(start_node_release node-3 50053)"
  
  echo "[NODES] Release node PIDs: $NODE_PIDS"
  echo "[NODES] Waiting up to 30 seconds for nodes to bind ports (release mode)..."
  
  if wait_for_all_ports 30; then
    echo "[NODES] ✓ Release mode successful"
    NODE_MODE="release"
  else
    echo "[NODES] ✗ Release mode timeout - no ports open after 30 seconds"
    echo "[NODES] Killing release mode nodes and falling back to source mode..."
    for pid in $NODE_PIDS; do
      kill "$pid" 2>/dev/null || true
    done
    rm -f "$LOG_DIR/.startup-pids"
    sleep 2
    NODE_MODE="source"
  fi
else
  NODE_MODE="source"
fi

# If release failed or doesn't exist, use source mode
if [[ "$NODE_MODE" == "source" ]]; then
  echo "[NODES] Starting nodes via 'go run' (source mode)..."
  NODE_PIDS="$(start_node_source node-1 50051) $(start_node_source node-2 50052) $(start_node_source node-3 50053)"
  
  echo "[NODES] Source node PIDs: $NODE_PIDS"
  echo "[NODES] Waiting up to 90 seconds for nodes to compile and bind ports (source mode)..."
  
  if ! wait_for_all_ports 90; then
    echo "[NODES] ✗ Source mode timeout - nodes did not open ports after 90 seconds" >&2
    echo "[ERROR] Node logs are in: $LOG_DIR" >&2
    echo "[ERROR] Last 20 lines of node logs:" >&2
    tail -n 20 "$LOG_DIR"/node-*.log 2>/dev/null || true
    exit 1
  fi
  
  echo "[NODES] ✓ Source mode successful"
fi

echo
echo "[VERIFY] Validating node readiness before seeding..."
if ! verify_node_ready; then
  echo "[ERROR] Node validation failed. Cannot proceed with seeding." >&2
  echo "[ERROR] Check logs: tail -f $LOG_DIR/node-*.log" >&2
  exit 1
fi

echo
echo "[SDK] Seeding sample memory via Python SDK..."
SDK_PYTHON="$BASE_DIR/.venv/bin/python"
if [[ ! -x "$SDK_PYTHON" ]]; then
  SDK_PYTHON="python3"
fi

echo "[SDK] Using Python: $SDK_PYTHON ($NODE_MODE mode)"

if (cd "$BASE_DIR/sdk/python" && "$SDK_PYTHON" example.py 2>&1 | tee -a "$LOG_DIR/sdk.log"); then
  echo "[SDK] ✓ Memory seeding successful"
else
  echo "[SDK] ⚠ Memory seeding returned non-zero exit (see $LOG_DIR/sdk.log)" >&2
fi

echo
echo "[DEMO] Cluster is running in $NODE_MODE mode"
echo "[DEMO] Opening Grafana dashboard..."
if command -v open >/dev/null 2>&1; then
  open "http://localhost:3000" 2>/dev/null || echo "[DEMO] Could not auto-open Grafana; visit http://localhost:3000 manually"
fi

echo
echo "=========================================="
echo "[LIVE] Tailing node logs (Ctrl+C to stop)"
echo "=========================================="
echo
