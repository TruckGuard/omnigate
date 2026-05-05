# OmniGate API Documentation

This document describes the API endpoints exposed by the OmniGate microservices architecture. All external requests should be routed through the NGINX API Gateway, which handles authentication via the Auth service.

## API Gateway Routing

The API Gateway (NGINX) routes requests to the internal microservices based on the URL prefix. Protected routes are automatically validated against the Auth service (`/validate`) before being proxied.

*   `/api/auth/*` ➔ Routes to **Auth Service**
*   `/ingest/*` ➔ Routes to **Ingestor Service** (Protected)
*   `/api/v1/*` ➔ Routes to **Core Service** (Protected)

---

## 1. Auth Service (`/api/auth/`)

Handles user authentication, session management, and API key generation.

### Public Endpoints
*   `POST /api/auth/login` - Authenticate a user and receive a token/session.
*   `GET /api/auth/validate` - Internal endpoint used by NGINX to validate sessions and API keys.

### Session Management (Requires Authentication)
*   `POST /api/auth/logout` - Logout the current user.
*   `GET /api/auth/sessions` - List active sessions for the current user.
*   `POST /api/auth/sessions/revoke` - Revoke a specific session.
*   `POST /api/auth/sessions/revoke-all` - Revoke all active sessions for the user.
*   `POST /api/auth/change-password` - Update the user's password.
*   `GET /api/auth/hierarchy` - Get permission hierarchy.
*   `GET /api/auth/permissions` - List all available permissions.

### Admin/Management Endpoints

#### Users
*   `POST /api/auth/register` - Register a new user. **[Requires: `manage:users`]**
*   `GET /api/auth/admin/users` - List all users. **[Requires: `read:users`]**
*   `GET /api/auth/admin/users/:id` - Get a specific user. **[Requires: `read:users`]**
*   `PUT /api/auth/admin/users/:id/role` - Update a user's role. **[Requires: `manage:users`]**
*   `DELETE /api/auth/admin/users/:id` - Delete a user. **[Requires: `manage:users`]**
*   `POST /api/auth/admin/users/:id/reset-password` - Admin reset for user password. **[Requires: `manage:users`]**

#### Roles
*   `GET /api/auth/admin/roles` - List roles. **[Requires: `read:roles`]**
*   `POST /api/auth/admin/roles` - Create a new role. **[Requires: `manage:roles`]**
*   `PUT /api/auth/admin/roles/:id` - Update a role. **[Requires: `manage:roles`]**
*   `DELETE /api/auth/admin/roles/:id` - Delete a role. **[Requires: `manage:roles`]**
*   `POST /api/auth/admin/roles/:id/permissions` - Assign permissions to a role. **[Requires: `manage:roles`]**

#### API Keys (Devices)
*   `GET /api/auth/admin/keys` - List active API keys. **[Requires: `read:keys`]**
*   `POST /api/auth/admin/keys` - Create a new API key with specific permissions. **[Requires: `manage:keys`]**
*   `PUT /api/auth/admin/keys/:id` - Update an API key. **[Requires: `manage:keys`]**
*   `DELETE /api/auth/admin/keys/:id` - Revoke/Delete an API key. **[Requires: `manage:keys`]**
*   `PUT /api/auth/admin/keys/:id/permissions` - Modify permissions of a key. **[Requires: `manage:keys`]**

#### Audit
*   `GET /api/auth/audit` - View the system audit log. **[Requires: `read:audit`]**

---

## 2. Ingestor Service (`/ingest/`)

Handles incoming raw data from peripheral devices (cameras, scales). This service requires a valid `X-Gate-ID` and `X-Source-ID` injected by the Auth gateway.

*   `POST /ingest/camera` - Ingest ANPR camera data. Supports `multipart/form-data` to handle raw JSON payload alongside image binaries. Images are uploaded directly to Garage (S3 compatible) and metadata is published to Valkey. **[Requires: `ingest:events`]**
*   `POST /ingest/weight` - Ingest scale weight data. Accepts JSON payload and publishes to Valkey. **[Requires: `ingest:events`]**

---

## 3. Core Service (`/api/v1/`)

The central orchestrator and data layer for the system. It manages structured events, transactions, and system configurations. **Note:** All Core Service endpoints can alternatively be accessed with their respective `:all` permission variant (e.g., `read:events:all`) which grants access regardless of ownership or specific row-level restrictions.

### Events
*   `GET /api/v1/events` - List and filter events. **[Requires: `read:events` or `read:events:all`]**
*   `POST /api/v1/events` - Create a structured event manually (usually done by Adapter). **[Requires: `create:events` or `create:events:all`]**
*   `GET /api/v1/events/:id` - Get details of a specific event. **[Requires: `read:events` or `read:events:all`]**
*   `PUT /api/v1/events/:id` - Update an event. **[Requires: `update:events` or `update:events:all`]**
*   `DELETE /api/v1/events/:id` - Delete an event. **[Requires: `delete:events` or `delete:events:all`]**

### Transactions (Sticky Sessions)
*   `GET /api/v1/transactions` - List active and historical transactions. **[Requires: `read:transactions` or `read:transactions:all`]**
*   `POST /api/v1/transactions` - Manually open a transaction. **[Requires: `create:transactions` or `create:transactions:all`]**
*   `GET /api/v1/transactions/:id` - Get details of a transaction including associated events. **[Requires: `read:transactions` or `read:transactions:all`]**
*   `PUT /api/v1/transactions/:id` - Update transaction status (e.g., closing it). **[Requires: `update:transactions` or `update:transactions:all`]**
*   `DELETE /api/v1/transactions/:id` - Delete a transaction. **[Requires: `delete:transactions` or `delete:transactions:all`]**

### Device Configurations
*   `GET /api/v1/configs/device` - List device configurations. **[Requires: `read:configs` or `read:configs:all`]**
*   `GET /api/v1/configs/device/:source_id` - Fetch configuration mapping for a specific device. **[Requires: `read:configs` or `read:configs:all`]**
*   `POST /api/v1/configs/device` - Create a new device configuration. **[Requires: `create:configs` or `create:configs:all`]**
*   `PUT /api/v1/configs/device/:id` - Update an existing configuration. **[Requires: `update:configs` or `update:configs:all`]**
*   `DELETE /api/v1/configs/device/:id` - Remove a device configuration. **[Requires: `delete:configs` or `delete:configs:all`]**

### Event Types (Schemas)
*   `GET /api/v1/types` - List all event type schemas. **[Requires: `read:types` or `read:types:all`]**
*   `GET /api/v1/types/:id` - Get schema details. **[Requires: `read:types` or `read:types:all`]**
*   `POST /api/v1/types` - Create a new event schema to dynamically validate payloads. **[Requires: `create:types` or `create:types:all`]**
*   `PUT /api/v1/types/:id` - Update an event schema. **[Requires: `update:types` or `update:types:all`]**
*   `DELETE /api/v1/types/:id` - Delete an event schema. **[Requires: `delete:types` or `delete:types:all`]**

### Gates
*   `GET /api/v1/gates` - List all gates. **[Requires: `read:gates` or `read:gates:all`]**
*   `GET /api/v1/gates/:id` - Get gate details. **[Requires: `read:gates` or `read:gates:all`]**
*   `POST /api/v1/gates` - Create a new gate. **[Requires: `create:gates` or `create:gates:all`]**
*   `PUT /api/v1/gates/:id` - Update a gate. **[Requires: `update:gates` or `update:gates:all`]**
*   `DELETE /api/v1/gates/:id` - Delete a gate. **[Requires: `delete:gates` or `delete:gates:all`]**

### User Profiles
*   `GET /api/v1/profiles` - List user profiles. **[Requires: `read:profiles` or `read:profiles:all`]**
*   `GET /api/v1/profiles/:id` - Get profile details. **[Requires: `read:profiles` or `read:profiles:all`]**
*   `POST /api/v1/profiles` - Create a user profile. **[Requires: `create:profiles` or `create:profiles:all`]**
*   `PUT /api/v1/profiles/:id` - Update a user profile. **[Requires: `update:profiles` or `update:profiles:all`]**
*   `DELETE /api/v1/profiles/:id` - Delete a user profile. **[Requires: `delete:profiles` or `delete:profiles:all`]**

---

## 4. Puller Service (Internal)

An internal worker service responsible for polling external APIs when triggered by the Adapter.

*   `POST /pull` (Internal) - Triggers a pull request to an external `trigger_url` and ferries the received data back into the Ingestor pipeline linked via a `transaction_id`.

---

## Health Checks

Every service exposes a basic health check endpoint used by Docker Compose and the Gateway:
*   Auth: `GET /api/auth/health` (Internal: `:8080/health`)
*   Core: `GET /api/v1/health`
*   Ingestor: `GET /ingest/health` (Internal: `:8080/health`)
*   Puller: `GET /health` (Internal: `:8000/health`)
