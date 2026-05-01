# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Development Commands

```bash
# Initial setup
make env-up          # Copy .env.example to .env (required before first run)
make dev-up          # Start all services
make dev-up-build    # Start with rebuild
make dev-init        # Initialize storage buckets (Garage/MinIO) — run once after first dev-up
make logs            # Stream all service logs

# Lifecycle
make dev-down        # Stop and remove containers + volumes
make dev-down-soft   # Stop without removing volumes
make dev-rebuild     # Rebuild all service images
make dev-restart     # Full restart

# Production
make build           # Build all Docker images
make push            # Push images to GHCR
```

**Running a single Go service locally (without Docker):**
```bash
cd services/<auth|ingestor|core>
go mod tidy
go run .
```

**Running a single Python service locally:**
```bash
cd services/<adapter|puller>
pip install -r requirements.txt
python main.py
```

There are no unit test suites — integration is tested via `test-scripts/` and manual API calls against the running stack.

## Architecture Overview

OmniGate is an IoT data ingestion and processing platform. The full event flow is:

```
IoT Device → [NGINX Gateway :8090]
                ↓ (auth_request to auth service)
           [Ingestor :8002]
                ├─ Raw payload + images → Garage (S3)
                └─ Event → Valkey Stream "events:adapter"
                                ↓
                       [Adapter] (consumer group: adapter-workers)
                                ├─ Camera events → ANPR service → license plate
                                ├─ Weight events → manufacturer payload parsing
                                ├─ Device config fetch from Core
                                ├─ Structured event → Core REST API
                                └─ If trigger configured → "events:puller"
                                                ↓
                                        [Puller] (consumer group: puller-workers)
                                                ├─ GET trigger_url (external API)
                                                └─ Re-inject into Ingestor REST API
```

**Valkey Streams:**
- `events:adapter` — raw ingested events (Ingestor → Adapter)
- `events:puller` — external fetch triggers (Adapter → Puller)
- `events:dlq` — dead-letter queue (after 3 failed retries)

**NGINX** validates every request via `auth_request` and injects identity headers (`X-User-ID`, `X-Source-ID`, `X-Gate-ID`, `X-Permissions`) before proxying to upstream services. Services trust these headers and do not re-validate tokens. The Ingestor gets `source_id` from these auth-injected headers; when the Puller re-injects data it supplies `source_id` directly in the request body.

**Transactions** ("sticky sessions") tie a sequence of events to a gate. The Ingestor only packages raw data for the Adapter — it does not create transactions. The Adapter processes and forwards structured events to Core. Core creates a new transaction if `transaction_id` is absent, or appends to the existing one if it is present. Core also manages transaction lifecycle and cleanup.

## Services

| Service | Lang | Port | Role |
|---------|------|------|------|
| `auth` | Go 1.25 | 8001 | JWT/API key auth, session management, user/device CRUD |
| `ingestor` | Go 1.25 | 8002 | Multipart upload endpoint, stores to S3, publishes to stream |
| `core` | Go 1.25 | 8003 | Persists structured events, transactions, device configs |
| `adapter` | Python 3.13 | — | Stream consumer, event routing/transformation, ANPR integration |
| `puller` | Python 3.11 | — | Stream consumer, polls external URLs, re-injects data |

**Go services** use Gin (HTTP), GORM (PostgreSQL), go-redis, and OpenTelemetry.  
**Python services** use redis-py for streams, requests for HTTP, minio for S3, and OpenTelemetry.

## Infrastructure

- **PostgreSQL 16** — two separate databases: `omnigate_auth` (auth service) and `omnigate_core` (core service), created by `infra/postgres/init-db.sh`
- **Valkey 8.0** — Redis-compatible; used for event streams and caching
- **Garage** — self-hosted S3-compatible storage; buckets configured in `infra/garage/`
- **OpenTelemetry Collector** — all services export OTLP traces to `otel-collector:4317`

## Key Configuration

All runtime config is in `.env` (copy from `.env.example`). Critical variables:

```env
WORKER_SYSTEM_KEY   # Inter-service auth (adapter ↔ core, puller ↔ ingestor)
PULLER_API_KEY      # Puller authentication against ingestor
JWT_SECRET          # JWT signing key for auth service
CORE_URL            # Used by adapter: http://gateway/api/v1
INGESTOR_URL        # Used by puller: http://gateway/ingest
VALKEY_ADDR         # Redis-compatible address
STORAGE_*           # S3 credentials for Garage
```

## CI/CD

GitHub Actions (`.github/workflows/build.yml`) builds all 5 services in a matrix on push to `main`/`develop` and on PRs. Images are pushed to GHCR with branch, SHA, and `latest` (main only) tags.
