# 🔐 TruckGuard Auth Service

### 1. What is it?

The **Auth Service** manages user authentication, role-based access control (RBAC), user sessions, and device API keys for the TruckGuard platform. Written in **Go**, it handles authentication requests from the NGINX API Gateway.

### 2. Purpose & How it Works

- **Authentication & Validation**:
  - Exposes `/api/auth/login` to authenticate users.
  - Exposes `/api/auth/validate` as an internal endpoint for the NGINX API Gateway to verify incoming JWT tokens or API keys.
- **Session Management**:
  - Keeps track of active user sessions in Valkey. Exposes endpoints to view and revoke sessions.
- **RBAC & Policies**:
  - Seeds roles (`admin`, `manager`, `operator`) and CRUD permission hierarchies.
  - Automatically resolves nested permissions via policy engine.
- **API Keys**:
  - Generates secure API keys for external devices (ANPR cameras, scales) and system workers (Adapter, Puller).

### 3. Tech Stack

- **Language**: [Go 1.25+](https://go.dev/)
- **Database**: PostgreSQL (User details, roles, permissions)
- **Session Store / Cache**: Valkey (Active sessions, revoked tokens)
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
DATABASE_URL=postgres://auth_user:auth_pass@localhost:5433/omnigate_auth
VALKEY_ADDR=localhost:6380
WORKER_SYSTEM_KEY=your_worker_key
PULLER_API_KEY=your_puller_key
ADMIN_DEFAULT_PASSWORD=admin_password
OTEL_EXPORTER_OTLP_ENDPOINT=localhost:4317
```
