# OmniGate Observability

Self-contained SigNoz stack that collects traces, metrics, and logs from all OmniGate services.

## Quick start

```bash
# 1. Copy env (once)
cp observability/.env.example observability/.env

# 2. Start the observability stack (~60s for ClickHouse + migrator)
make obs-up

# 3. Start the main dev stack
make dev-up
```

Open http://localhost:8080 and create an admin account on first visit.

## Ports

| Port | Service        |
|------|----------------|
| 8080 | SigNoz UI      |
| 4317 | OTLP gRPC      |
| 4318 | OTLP HTTP      |

## How the shared network works

The observability stack creates the Docker network `omnigate-observability`. The dev stack joins it as an external network, which means all five app services (`auth`, `ingestor`, `core`, `adapter`, `puller`) can resolve the hostname `otel-collector` to the SigNoz collector container. No changes to `OTEL_EXPORTER_OTLP_ENDPOINT` are needed.

If the observability stack is not running, OTLP export errors are non-fatal — services retry with backoff and remain functional.

## Startup order

The observability stack must be running before `make dev-up`, otherwise the shared network won't exist and Docker Compose will refuse to start the app services. If you already ran `make dev-up` without observability:

```bash
make dev-down-soft
make obs-up
make dev-up
```

## Stopping

```bash
make obs-down        # stop containers, keep volumes
make obs-down-hard   # stop containers + wipe all ClickHouse/SQLite data
```

## histogramQuantile binary

`init-clickhouse` downloads an architecture-specific binary at first startup into the named Docker volume `omnigate-obs-clickhouse-user-scripts`. The binary is never committed to git.

If the machine has no internet access, download the binary manually:

```bash
version=v0.0.1
os=$(uname -s | tr '[:upper:]' '[:lower:]')
arch=$(uname -m | sed s/aarch64/arm64/ | sed s/x86_64/amd64/)
wget -O histogram-quantile.tar.gz \
  "https://github.com/SigNoz/signoz/releases/download/histogram-quantile%2F${version}/histogram-quantile_${os}_${arch}.tar.gz"
tar -xvzf histogram-quantile.tar.gz
docker run --rm -v omnigate-obs-clickhouse-user-scripts:/scripts \
  busybox cp histogram-quantile /scripts/histogramQuantile
```

## Dashboards

Two pre-built dashboards live in `observability/dashboards/`:

| File | Title | Panels |
|------|-------|--------|
| `omnigate-overview.json` | OmniGate — Services Overview | Request rate, error count, P99 latency, P50 latency (all services) |
| `omnigate-pipeline.json` | OmniGate — Event Pipeline | Ingestor throughput, Core API breakdown, error rate, avg latency, stacked span rate |
| `omnigate-health.json` | OmniGate — Service Health | HTTP health check status, uptime ratio, probe duration (auth/ingestor/core) |

### Importing dashboards

1. Open SigNoz UI and create an admin account (first run only).
2. Run the seeder (it logs in and POSTs each file to the API):

```bash
SIGNOZ_EMAIL=admin@example.com SIGNOZ_PASSWORD=yourpassword make obs-seed-dashboards
```

Or equivalently:

```bash
SIGNOZ_EMAIL=admin@example.com SIGNOZ_PASSWORD=yourpassword \
  observability/seed-dashboards.sh
```

### PromQL metric names

The dashboards query metrics produced by the `signozspanmetrics/delta` processor:

| Metric | Description |
|--------|-------------|
| `signoz_calls_total` | Span count (counter, delta). Labels: `service_name`, `operation`, `kind`, `status_code` |
| `signoz_latency_bucket` | Latency histogram (ms). Used for `histogram_quantile()` |
| `signoz_latency_sum` / `signoz_latency_count` | For average latency formulas |
| `probe_success` | 1=up, 0=down. Produced by `blackbox_exporter`. Label: `instance` (service URL) |
| `probe_duration_seconds` | Time for the HTTP probe to complete. Same labels as above |

If panels show "No data", open SigNoz → Metrics Explorer and search for `signoz_` to confirm the exact metric names produced by your collector version.

## Verification

```bash
# All containers healthy
docker ps --filter name=omnigate-obs

# Send a test event
python test-scripts/test.py

# In SigNoz UI → Services → should list omnigate-auth, omnigate-core, etc.
# In SigNoz UI → Traces → drill into a full event ingestion trace
```
