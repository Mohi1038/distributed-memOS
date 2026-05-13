# MemOS Phase 3 Visual Demo

This folder contains a small demo script to visually showcase the Phase 3 distributed replication and repair features.

Files
- `scripts/demo_visual.sh` - starts Postgres, NATS, and Qdrant with Docker Compose, attempts to run the macOS release binary for three MemOS nodes, automatically falls back to `go run cmd/memos/main.go` if release nodes do not become ready, seeds a sample memory via the Python SDK example, opens Grafana, and tails logs.

Quick start (macOS)
1. Ensure Docker Desktop is running.
2. From the repository root run:

```bash
chmod +x scripts/demo_visual.sh
scripts/demo_visual.sh
```

What the script does
- Boots only the infrastructure services using `deployments/docker-compose.cluster.yml`.
- Runs `releases/memos-server-darwin-arm64` three times with distinct `NODE_ID` and `GRPC_PORT` values.
- If release nodes do not open ports `50051-50053`, automatically falls back to source mode using `go run cmd/memos/main.go` for all three nodes.
- Runs `sdk/python/example.py` to seed a sample memory (connects to `localhost:50051`).
- Opens `http://localhost:3000` so you can view the Grafana dashboards that are provisioned in the repo.
- Tails service logs so you can watch Merkle root exchanges, replication, and conflict-resolution events in real time.

Notes
- The demo uses the repository's Python SDK example to seed content. If your local build exposes gRPC on a different port, update `sdk/python/example.py` or adjust the `GRPC_PORT` values in `scripts/demo_visual.sh`.
- If you prefer not to auto-open Grafana, skip that step and open `http://localhost:3000` manually.

Troubleshooting

```bash
docker compose -f deployments/docker-compose.cluster.yml ps
docker compose -f deployments/docker-compose.cluster.yml logs --tail 200
```
