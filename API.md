# OmniGate API Documentation

This document describes the API endpoints exposed by the OmniGate microservices architecture. All external requests should be routed through the NGINX API Gateway, which handles authentication via the `Auth` service.

## API Gateway Routing

The API Gateway (NGINX) routes requests to the internal microservices based on the URL prefix. Protected routes are automatically validated against the Auth service (`/validate`) before being proxied.

*   `/auth/*` ➔ Routes to **Auth Service**
*   `/ingest/*` ➔ Routes to **Ingestor Service** (Protected)
*   `/api/v1/*` ➔ Routes to **Core Service** (Protected)

---

## 1. Auth Service (`/auth/`)

Handles user authentication, session management, and API key generation.

### Public Endpoints
*   `POST /auth/login` - Authenticate a user and receive a token/session.
*   `POST /auth/register` - Register a new user.
*   `GET /auth/validate` - Internal endpoint used by NGINX to validate sessions and API keys.

### Session Management
*   `POST /auth/logout` - Logout the current user.
*   `GET /auth/sessions` - List active sessions for the current user.
*   `POST /auth/sessions/revoke` - Revoke a specific session.
*   `POST /auth/sessions/revoke-all` - Revoke all active sessions for the user.
*   `POST /auth/change-password` - Update the user's password.

### Admin/Management Endpoints (Requires Permissions)
*   `GET /auth/hierarchy` - Get permission hierarchy.
*   `GET /auth/permissions` - List all available permissions.

#### Users
*   `GET /auth/admin/users` - List all users.
*   `GET /auth/admin/users/:id` - Get a specific user.
*   `PUT /auth/admin/users/:id/role` - Update a user's role.
*   `DELETE /auth/admin/users/:id` - Delete a user.
*   `POST /auth/admin/users/:id/reset-password` - Admin reset for user password.

#### Roles
*   `GET /auth/admin/roles` - List roles.
*   `POST /auth/admin/roles` - Create a new role.
*   `PUT /auth/admin/roles/:id` - Update a role.
*   `DELETE /auth/admin/roles/:id` - Delete a role.
*   `POST /auth/admin/roles/:id/permissions` - Assign permissions to a role.

#### API Keys (Devices)
*   `GET /auth/admin/keys` - List active API keys.
*   `POST /auth/admin/keys` - Create a new API key with specific permissions.
*   `PUT /auth/admin/keys/:id` - Update an API key.
*   `DELETE /auth/admin/keys/:id` - Revoke/Delete an API key.
*   `PUT /auth/admin/keys/:id/permissions` - Modify permissions of a key.
*   `PUT /auth/admin/keys/:id` - Update an API key.

---

## 2. Ingestor Service (`/ingest/`)

Handles incoming raw data from peripheral devices (cameras, scales). This service requires a valid `X-Gate-ID` and `X-Source-ID` injected by the Auth gateway.

*   `POST /ingest/camera` - Ingest ANPR camera data. Supports `multipart/form-data` to handle raw JSON payload alongside image binaries. Images are uploaded directly to S3 (Garage) and metadata is published to Valkey.
*   `POST /ingest/weight` - Ingest scale weight data. Accepts JSON payload and publishes to Valkey.

---

## 3. Core Service (`/api/v1/`)

The central orchestrator and data layer for the system. It manages structured events, transactions, and system configurations.

### Events
*   `GET /api/v1/events` - List and filter events.
*   `POST /api/v1/events` - Create a structured event manually (usually done by Adapter).
*   `GET /api/v1/events/:id` - Get details of a specific event.
*   `DELETE /api/v1/events/:id` - Delete an event.

### Transactions (Sticky Sessions)
*   `GET /api/v1/transactions` - List active and historical transactions.
*   `POST /api/v1/transactions` - Manually open a transaction.
*   `GET /api/v1/transactions/:id` - Get details of a transaction including associated events.
*   `PUT /api/v1/transactions/:id` - Update transaction status (e.g., closing it).
*   `DELETE /api/v1/transactions/:id` - Delete a transaction.

### Device Configurations
*   `GET /api/v1/configs/device/:source_id` - Fetch configuration mapping for a specific device (used heavily by Adapter).
*   `POST /api/v1/configs/device` - Create a new device configuration.
*   `PUT /api/v1/configs/device/:id` - Update an existing configuration.
*   `DELETE /api/v1/configs/device/:id` - Remove a device configuration.

### Event Types (Schemas)
*   `GET /api/v1/types` - List all event type schemas.
*   `GET /api/v1/types/:id` - Get schema details.
*   `POST /api/v1/types` - Create a new event schema to dynamically validate payloads.

---

## 4. Puller Service (Internal)

An internal worker service responsible for polling external APIs when triggered by the Adapter.

*   `POST /pull` (Internal) - Triggers a pull request to an external `trigger_url` and ferries the received data back into the Ingestor pipeline linked via a `transaction_id`.

---

## Health Checks

Every service exposes a basic health check endpoint used by Docker Compose and the Gateway:
*   Auth: `GET /auth/health`
*   Core: `GET /api/v1/health`
*   Ingestor: `GET /ingest/health` (Internal: `:8080/health`)
*   Puller: `GET /health` (Internal: `:8000/health`)
