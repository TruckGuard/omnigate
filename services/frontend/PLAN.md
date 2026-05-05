# OmniGate — Feature Plan

## Priority order
1. Fix edit device (broken PUT)
2. Event type editing
3. Device trigger relationships display
4. User registration page
5. Profile / session management page
6. Gate detail pages + gate settings
7. Data mapping visual editor

---

## 1. Fix: Edit device

**Problem:** `HandleUpdateDeviceConfig` uses `*datatypes.JSON` for `DataMapping`, which gin's
`ShouldBindJSON` cannot unmarshal into. Also the frontend sends `data_mapping` as a plain JS
object (already parsed), so the round-trip needs to survive JSON→[]byte.

**Backend fix — `services/core/src/api/handlers/configs.go`:**
- Change `DataMapping *datatypes.JSON` binding field to `*json.RawMessage`
- Assign via `config.DataMapping = datatypes.JSON(*req.DataMapping)`

**Frontend fix — already correct** (sends `data_mapping` as parsed object in body).

---

## 2. Event type editing

**No update endpoint exists.** Need field-level editing: rename fields, change descriptions,
change types, add/remove fields.

### 2a. Backend — `services/core`

**New model change:** `EventType.Fields` is stored as `datatypes.JSON` (JSONB). The update
replaces the whole fields map.

**New handler** `HandleUpdateEventType` in `handlers/types.go`:
```
PUT /api/v1/types/:id
Body: { name?, description?, fields? }
```
- Partial update: only provided keys are changed.
- `fields` replaces the entire fields map when provided.

**Register route in `main.go`:**
```go
api.PUT("/types/:id", handlers.HandleUpdateEventType)
```

### 2b. Frontend — `settings/types/+page.svelte`

Current page has expand-to-show-fields rows. Add an **edit dialog**:
- Opens with current name, description, and fields array (same editor as create)
- Can add/remove/edit individual fields
- Calls `api.types.update(id, data)` → `PUT /api/v1/types/:id`

**New api.ts method:**
```ts
api.types.update(id: string, d: { name?: string; description?: string; fields?: Record<string,…> })
```

---

## 3. Device trigger relationships

**Data already exists:** `DeviceConfig.trigger_source_id` is the source_id the Puller uses when
re-injecting. No backend changes needed.

### Frontend — `settings/devices/[id]/+page.svelte`

When loading a device config, also:
1. Load **all configs** to resolve relationships.
2. **"Triggered by"**: find configs where `cfg.trigger_source_id === currentDevice.source_id`
   → "This device is triggered by: [Device name / source_id]" with a link.
3. **"Triggers"**: if current device has `trigger_source_id`, find the config whose
   `source_id === currentDevice.trigger_source_id`
   → "This device triggers: [Device name / source_id]" with a link.

Add a **Trigger Relationships** card in the device detail page showing both directions.

### Frontend — `settings/devices/+page.svelte` (list)

Add a "Trigger" column that shows a small badge / arrow when a device is part of a trigger chain.

### Puller source_id/gate_id verification

Check `services/puller/src/worker/puller_worker.py`:
- It sends `assume_source_id = trigger_source_id` from the event message.
- Ingestor should read `source_id` from body when provided.
- **Verify**: does `ingestor` accept `source_id` in the POST body (non-auth path) for puller re-injection?
  - If yes: no change needed.
  - If no: add body `source_id` override in ingestor for requests authenticated with `PULLER_API_KEY`.

---

## 4. User registration page

**Backend:** `POST /register` already exists in auth service (line 20 `HandleRegister`).
It's protected by nginx `auth_request` under `/api/auth/` → admin-only, correct.

### Frontend — new dialog in `settings/users/+page.svelte`

Add **"New user"** button → dialog with:
- Username (required)
- Password (required)
- Role select (list from `api.auth.roles()`)

Calls `api.auth.createUser({ username, password, role_id })` → `POST /api/auth/register`

**New api.ts method:**
```ts
api.auth.createUser(d: { username: string; password: string; role_id?: number })
  → req<AuthUser>('/api/auth/register', { method: 'POST', body: ... })
```

No backend change required.

---

## 5. Profile / session management page

### 5a. Backend — `services/auth`

**New endpoints needed:**

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/me` | Current user info (id, username, role, permissions) |
| `GET` | `/sessions` | Already exists — list own active sessions |
| `DELETE` | `/sessions/:session_id` | Logout a specific session |
| `DELETE` | `/sessions` | Logout all sessions (except current) |
| `PUT` | `/me/password` | Change own password (requires `current_password` + `new_password`) |

**Implementation notes:**
- `GET /sessions` already exists in handlers.go:226, just needs nginx exposure.
- `DELETE /sessions/:id` and `DELETE /sessions` need new handlers that delete Redis keys.
- `PUT /me/password` needs bcrypt verify of `current_password` before update.
- All `/me/*` and `/sessions/*` routes are protected (auth_request in nginx).

**Register routes in `auth/main.go`:**
```go
r.GET("/me", handlers.HandleGetMe) // in auth service stors only information to login - profile sotes in core serivece
r.PUT("/me/password", handlers.HandleChangePassword)
r.DELETE("/sessions/:id", handlers.HandleLogoutSession)
r.DELETE("/sessions", handlers.HandleLogoutAllSessions)
```

**Session model** (in Redis, key `auth:{hash}`):
- Need to store session_id alongside user data so individual sessions can be targeted.
- Check current session storage format in `repository.go`.

### 5b. Frontend — new route `/profile`

New file: `src/routes/profile/+page.svelte`

Layout: two columns
- **Left — Account info:**
  - Username (read-only)
  - Role badge
  - Change password form (current + new + confirm)

- **Right — Active sessions:**
  - Table of sessions: created_at, last_used, IP (if stored)
  - "Logout this session" button per row
  - "Logout all other sessions" button at top

Also link to full profile edit (`/settings/users/[id]`) for name/phone/gate.

**Navigation:** add "Profile" link in sidebar bottom section (next to username).

**New api.ts methods:**
```ts
api.auth.me()                              → GET /api/auth/me
api.auth.sessions()                        → GET /api/auth/sessions
api.auth.logoutSession(id: string)         → DELETE /api/auth/sessions/:id
api.auth.logoutAllSessions()               → DELETE /api/auth/sessions
api.auth.changePassword(current, next)     → PUT /api/auth/me/password
```

---

## 6. Gate detail pages

### 6a. Backend — `services/core`

**New: gate settings** (stored as JSONB on the Gate model itself — no separate table needed):

Alter Gate model to add `settings datatypes.JSON` field:
```go
Settings datatypes.JSON `gorm:"type:jsonb;default:'{}'" json:"settings"`
```

Add migration (GORM auto-migrate will add the column).

**Gate settings schema** (frontend-defined, stored as JSON):
```json
{
  "transaction_ttl_minutes": 30,
  "auto_close_transactions": true,
  "max_events_per_transaction": 100
}
```

**New handlers:**
```
GET  /api/v1/gates/:id           → HandleGetGate (single gate by numeric id)
PUT  /api/v1/gates/:id/settings  → HandleUpdateGateSettings
GET  /api/v1/gates/:id/stats     → HandleGetGateStats
```

`HandleGetGateStats` returns:
```json
{
  "total_transactions": 42,
  "open_transactions": 3,
  "total_devices": 5,
  "recent_transactions": [/* last 5 */]
}
```

**New api.ts methods:**
```ts
api.gates.get(id: string)              → GET /api/v1/gates/:id
api.gates.updateSettings(id, settings) → PUT /api/v1/gates/:id/settings
api.gates.stats(id: string)            → GET /api/v1/gates/:id/stats
```

### 6b. Frontend — new route `settings/gates/[id]/+page.svelte`

**Layout: three sections**

1. **Header card** — gate_id badge, name, location, status toggle, description

2. **Settings card** — form fields for settings JSON:
   - Transaction TTL (minutes) — number input
   - Auto-close transactions — switch
   - Max events per transaction — number input
   - Save settings button

3. **Stats card** — summary tiles:
   - Open transactions count (links to filtered transactions view)
   - Total transactions count
   - Active devices count (links to filtered devices view)
   - Recent transactions mini-table (last 5)

**Update GateBadge links:** change `href="/settings/gates"` to `href="/settings/gates/{gate.id}"` wherever a gate object is available.

**Update `settings/gates/+page.svelte`:** make rows clickable → navigate to `/settings/gates/${g.id}`.

---

## 7. Data mapping visual editor

### 7a. Backend — `services/core`

**New endpoint:**
```
GET /api/v1/events/latest?source_id=:source_id
```
Returns the most recent `Event` for a given source_id (for use as a mapping reference).

Implementation: `ORDER BY created_at DESC LIMIT 1 WHERE source_id = ?`

**New api.ts method:**
```ts
api.events.latestForSource(sourceId: string) → GET /api/v1/events/latest?source_id=...
```

### 7b. Frontend — mapping editor component

New file: `src/lib/components/MappingEditor.svelte`

Props:
- `bind:value: Record<string, string>` — the mapping object
- `schema: Record<string, EventTypeField>` — event type fields (for knowing what keys to expect)
- `rawEvent?: Event` — latest raw event (if available) for reference

**UI:**
- Table of rows: `[field key (select from schema)] → [JSONPath expression (text input)]`
- "Add row" button for custom fields not in schema
- Delete row button
- **Raw event panel** (collapsible): shows `rawEvent.raw_data_key` content or a JSON preview
  if the event has structured payload data stored

**Integration in `devices/[id]/+page.svelte`:**
- Replace the raw `<Textarea>` for mapping with `<MappingEditor>`
- On load (edit mode): fetch `api.events.latestForSource(sourceId)` as reference
- Parse existing mapping JSON into the row array

---

## Implementation order

| # | Task | Backend | Frontend | Complexity |
|---|------|---------|----------|------------|
| 1 | Fix edit device | core: fix JSON binding | — | Low |
| 2 | Event type edit | core: PUT /types/:id | types page: edit dialog | Low |
| 3 | Trigger relationships | — (data exists) | devices detail + list | Low |
| 4 | User registration | — (endpoint exists) | users page: new user dialog | Low |
| 5 | Profile page | auth: 4 new endpoints | new /profile route | Medium |
| 6 | Gate detail + settings | core: settings field + 3 endpoints | new gates/[id] page | Medium |
| 7 | Mapping editor | core: latest event endpoint | new component + integration | High |
