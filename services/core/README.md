# 📦 TruckGuard Core Service

### 1. What is it?

The **Core Service** is the central orchestrator and persistent data layer of the TruckGuard ecosystem. Written in **Go**, it manages the structured business entities: events, sticky gate transactions, configurations, gates, and schemas.

### 2. Purpose & How it Works

- **Event Storage & Querying**:
  - Receives structured events from the Adapter and stores them in PostgreSQL.
  - Supports full event log filtering, querying, and fuzzy vehicle plate search using PostgreSQL `pg_trgm` indexes.
- **Sticky Gate Transactions**:
  - Groups sequential events at a gate into logical transactions (sticky sessions).
  - Evaluates transaction state (active/inactive) using Valkey cache.
- **Device Configuration Management**:
  - Manages mappings, schemas, and trigger definitions for IoT devices (plural `/api/v1/configs/devices`).
  - Supports manually triggering downstream pullers (`POST /api/v1/configs/devices/:id/trigger`).
- **Entity CRUDs**:
  - Provides REST endpoints for managing Gates, Event Types (validation schemas), and User Profiles.

### 3. Tech Stack

- **Language**: [Go 1.25+](https://go.dev/)
- **Database**: PostgreSQL 16
- **Cache**: Valkey 8.0
- **Observability**: [OpenTelemetry](https://opentelemetry.io/)

### 4. Getting Started

#### **Prerequisites**

- Go (v1.25 or higher)
- PostgreSQL 16
- Valkey 8.0

#### **Run Commands**

1.  **Install dependencies:**
    ```bash
    go mod tidy
    ```
2.  **Start the service:**
    ```bash
    go run .
    ```

### 5. Configuration (Environment Variables)

```env
PORT=8080
DATABASE_URL=postgres://core_user:core_pass@localhost:5433/omnigate_core
VALKEY_ADDR=localhost:6380
STORAGE_ENDPOINT=localhost:3900
STORAGE_ACCESS_KEY=your_access_key
STORAGE_SECRET_KEY=your_secret_key
STORAGE_BUCKET=truckguard-images
OTEL_EXPORTER_OTLP_ENDPOINT=localhost:4317
```
