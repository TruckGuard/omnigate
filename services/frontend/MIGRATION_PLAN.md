# Frontend Migration Plan — shadcn-svelte + Full Backend Alignment

## Goals
1. Replace all custom UI primitives with shadcn-svelte components
2. Align every page with actual backend models and API shapes
3. Add missing pages: API Keys, Event Types, Gates, User Profile
4. Fix device creation flow: source_id = APIKey.ID, not a free-text string
5. Use svelte-sonner throughout (no shadcn toast)
6. Enforce permission-based visibility: pages, sidebar items, and action buttons are hidden when the user lacks the required permission

---

## Permission-Based Access Control

### How permissions are fetched

`GET /validate` (via nginx as `GET /api/auth/validate`) returns `{ id, username, role, permissions: string[] }` when called with a valid `Authorization: Bearer <session_id>` header. The permissions array contains expanded permission IDs (e.g. `"manage:keys"`, `"read:roles"`).

**Flow:**
1. User logs in → `POST /api/auth/login` → `{ session_id }`
2. Immediately call `GET /api/auth/validate` → store `permissions[]` in `authStore`
3. On every app boot (layout `$effect`), re-call validate to refresh permissions and catch revoked sessions

### `authStore` additions

```ts
class AuthStore {
  sessionId   = $state<string | null>(...)
  username    = $state<string | null>(...)
  role        = $state<string | null>(...)
  permissions = $state<string[]>([])   // ← new

  can(permission: string): boolean {
    return this.permissions.includes(permission)
  }

  // login() now also accepts permissions[]
  login(sessionId: string, username: string, role: string, permissions: string[]) { ... }
}
```

### `PermGuard` component — `src/lib/components/PermGuard.svelte`

```svelte
<script lang="ts">
  import { authStore } from '$lib/stores/auth.svelte.js'
  let { permission, children } = $props()
</script>
{#if authStore.can(permission)}
  {@render children()}
{/if}
```

Usage: wrap any button, table action column, or section in `<PermGuard permission="manage:keys">`.

### Known permission IDs (from auth service policy)

| Permission ID | Controls |
|---|---|
| `manage:users` | Users: Edit role, Reset password, Delete buttons |
| `read:roles` | Roles: sidebar item visible, page accessible |
| `manage:roles` | Roles: Create, Edit, Delete, Assign permissions buttons |
| `read:keys` | API Keys: sidebar item visible, page accessible |
| `manage:keys` | API Keys: Create, Edit, Delete, Assign permissions buttons |
| `write:events` | Transactions: Close transaction button |

> Additional permission IDs (for gates, types, configs) may exist in the DB. All `GET /api/auth/admin/permissions` results should be used at runtime; the table above lists only the ones derived from observed policy rules.

### Page-level access rules

| Route | Required to see sidebar item | Required to access page | Redirect if denied |
|---|---|---|---|
| `/` | _(always visible)_ | _(always)_ | — |
| `/transactions/[id]` | — | _(always)_ | — |
| `/settings/devices` | _(always visible)_ | _(always)_ | — |
| `/settings/keys` | `read:keys` | `read:keys` | `/` |
| `/settings/types` | _(always visible)_ | _(always)_ | — |
| `/settings/gates` | _(always visible)_ | _(always)_ | — |
| `/settings/users` | `manage:users` | `manage:users` | `/` |
| `/settings/users/[id]` | — | `manage:users` | `/` |
| `/settings/roles` | `read:roles` | `read:roles` | `/` |

### Button-level visibility map

| Page | Button / action | Required permission |
|---|---|---|
| Transactions dashboard | _(no mutating actions)_ | — |
| Transaction detail | Close transaction | `write:events` |
| Transaction detail | Save note | _(always — operators add notes)_ |
| Devices list | New device button | `manage:keys` _(creating a device = binding to a key)_ |
| Device edit | Save, Delete buttons | `manage:keys` |
| API Keys list | Create key button | `manage:keys` |
| API Keys list | Edit / Delete / Permissions buttons per row | `manage:keys` |
| Event Types list | Create type button | `manage:roles` _(admin-level config)_ |
| Gates list | Create / Edit / Delete | `manage:roles` _(admin-level config)_ |
| Users list | Edit role, Reset password, Delete per row | `manage:users` |
| User profile | Save account section | `manage:users` |
| User profile | Save profile section | `manage:users` |
| Roles list | Create role button | `manage:roles` |
| Roles list | Edit / Delete / Assign permissions per card | `manage:roles` |

### Sidebar implementation

Each nav item gets an optional `permission` field. The sidebar renders the item only if `permission` is absent or `authStore.can(permission)` is true:

```ts
const navItems = [
  { id: 'transactions', href: '/',                 label: 'Transactions', icon: LayoutGrid, section: 'Operations' },
  { id: 'devices',      href: '/settings/devices', label: 'Devices',      icon: Cpu,        section: 'Operations' },
  { id: 'keys',         href: '/settings/keys',    label: 'API Keys',     icon: KeySquare,  section: 'Operations',    permission: 'read:keys' },
  { id: 'types',        href: '/settings/types',   label: 'Event Types',  icon: Layers,     section: 'Configuration' },
  { id: 'gates',        href: '/settings/gates',   label: 'Gates',        icon: GitFork,    section: 'Configuration' },
  { id: 'users',        href: '/settings/users',   label: 'Users',        icon: Users,      section: 'Access',        permission: 'manage:users' },
  { id: 'roles',        href: '/settings/roles',   label: 'Roles',        icon: KeyRound,   section: 'Access',        permission: 'read:roles' },
]
```

### Route guard in `+layout.svelte`

```ts
$effect(() => {
  if (isLoginPage) return
  if (!authStore.isAuthenticated) { goto('/login'); return }
  const item = navItems.find(n => $page.url.pathname.startsWith(n.href) && n.href !== '/')
  if (item?.permission && !authStore.can(item.permission)) goto('/')
})
```

---

## shadcn Component Mapping

| Custom file | shadcn replacement | Notes |
|---|---|---|
| `Button.svelte` | `$lib/components/ui/button` | Delete custom after migration |
| `Badge.svelte` | `$lib/components/ui/badge` | Keep gate-color logic in a thin wrapper |
| `Card.svelte` | `$lib/components/ui/card` | Use `Card`, `CardHeader`, `CardContent` |
| `Dialog.svelte` | `$lib/components/ui/dialog` | Use `Dialog`, `DialogContent`, `DialogHeader` |
| `Input.svelte` | `$lib/components/ui/input` | Delete custom |
| `Textarea.svelte` | `$lib/components/ui/textarea` | Delete custom |
| `Select.svelte` | `$lib/components/ui/select` | Use `Select`, `SelectTrigger`, `SelectContent`, `SelectItem` |
| `Switch.svelte` | `$lib/components/ui/switch` | Delete custom |
| `Field.svelte` | `$lib/components/ui/label` | Keep `Field.svelte` as thin wrapper using shadcn Label |
| _(new)_ | `$lib/components/ui/table` | All list pages |
| _(new)_ | `$lib/components/ui/separator` | Sidebar sections |
| `TopBar.svelte` | Keep custom — not replaced | Uses shadcn Button inside |
| Toaster | `svelte-sonner` `<Toaster>` | Already correct — do NOT replace with shadcn toast |

---

## Navigation Changes

Add to sidebar:

| Section | Route | Label |
|---|---|---|
| Operations | `/` | Transactions |
| Operations | `/settings/devices` | Devices |
| Operations | `/settings/keys` | API Keys _(new)_ |
| Configuration | `/settings/types` | Event Types _(new)_ |
| Configuration | `/settings/gates` | Gates _(new)_ |
| Access | `/settings/users` | Users |
| Access | `/settings/users/[id]` | User Profile _(new, no sidebar item — navigated from Users list)_ |
| Access | `/settings/roles` | Roles |

---

## Backend Data Types Reference

### APIKey (`GET /api/auth/admin/keys`)
```ts
interface APIKey {
  id: number;          // string source_id = String(id)
  owner_name: string;
  is_active: boolean;
  gate_id: string;
  permissions: Permission[];
  created_at: string;
}
// Create: POST /api/auth/admin/keys → { name, gate_id, permission_ids[] }
// Response: { api_key: string, id: number }
// Update: PUT /api/auth/admin/keys/:id → { owner_name?, is_active? }
// Permissions: PUT /api/auth/admin/keys/:id/permissions → { permission_ids[] }
// Delete: DELETE /api/auth/admin/keys/:id
```

### EventType (`GET /api/v1/types`)
```ts
interface EventType {
  id: string;           // UUID
  code: string;         // e.g. "ANPR", "WEIGHT"
  name: string;         // e.g. "License Plate Read"
  description: string;
  fields: Record<string, string>; // JSONPath → description
  created_at: string;
}
// Create: POST /api/v1/types → { code, name, description, fields }
```
Fields in EventType:
```typescript
{
  "key": { // key in JSON 
    "name": "display name",
    "description": "",
    "type": "string | number | boolean | datetime | image_url | ..." //some primitive data types to now how to parse data
    "required": true | false
  },
  ...
}
```

### DeviceConfig (`GET /api/v1/configs/device`)
```ts
interface DeviceConfig {
  id: string;
  source_id: string;         // = String(APIKey.id)
  event_type_id: string;     // UUID of EventType
  event_type?: EventType;
  gate_id: string;
  data_mapping: Record<string, string>;
  data_type: string;
  trigger_url: string | null;
  trigger_source_id: string | null;
  trigger_enabled: boolean;
  enabled: boolean;
  created_at: string;
  updated_at: string;
}
```

### Gate (`GET /api/v1/gates`)
```ts
interface Gate {
  id: string;
  gate_id: string;       // short human identifier e.g. "gate-north" - also in API Key as gate_id
  name: string;
  location: string;
  description: string;
  status: string;        // "active" | "inactive"
  created_at: string;
  updated_at: string;
}
```

### Transaction (`GET /api/v1/transactions`)
```ts
interface Transaction {
  id: string;
  code: string;
  gate_id: string;
  status: string;        // "active" | "completed" | "cancelled"
  note: string;
  events?: Event[];
  created_at: string;
  updated_at: string;
  completed_at: string | null;
}
```

---

## Page-by-Page Work

### 1. Login — `/login`
- Shadcn: `Input`, `Button`, `Card`, `CardContent`, `Label`
- No changes to logic; auth flow already correct

### 2. Transactions Dashboard — `/`
- Shadcn: `Button`, `Badge`, `Table`, `TableHead`, `TableBody`, `TableRow`, `TableCell`
- Replace custom `<table>` with shadcn Table primitives
- Gate badges: keep thin wrapper around shadcn `Badge` for categorical color logic
- Polling + sonner toast: unchanged

### 3. Transaction Detail — `/transactions/[id]`
- Shadcn: `Button`, `Badge`, `Card`/`CardContent`, `Dialog`/`DialogContent`/`DialogHeader`/`DialogFooter`, `Textarea`
- No logic changes; replace primitives only

### 4. Devices List — `/settings/devices`
- Shadcn: `Button`, `Badge`, `Table`
- Add "Source ID" column showing `source_id` (= API key ID) and link to that key

### 5. Device Edit/Create — `/settings/devices/[id]`
- Shadcn: `Button`, `Card`/`CardContent`, `Input`, `Textarea`, `Switch`, `Dialog`/`DialogContent`
- **Source ID field** → replace free-text Input with shadcn `Select` populated from `GET /api/auth/admin/keys`
  - Options: `{key.owner_name} (#{key.id})` for each active key
  - Extra option: "Create new key…" → inline mini-form (owner_name + gate_id) → POST → auto-selects result
- **Event Type field** → shadcn `Select` populated from `GET /api/v1/types`
  - When an event type is selected, show its `fields` schema below as read-only hint

### 6. API Keys — `/settings/keys` _(new page)_
Backend: `GET/POST/PUT/DELETE /api/auth/admin/keys`
- List table: owner_name, gate_id, active badge, created_at, permissions count
- Create dialog: owner_name, gate_id, multi-select permissions checkboxes
- On create: show generated key in a one-time reveal dialog (copy button)
- Edit dialog: toggle active, rename owner_name
- Permissions dialog: checkbox list of all permissions from `GET /api/auth/admin/permissions`
- Delete with confirm dialog

### 7. Event Types — `/settings/types` _(new page)_
Backend: `GET/POST /api/v1/types`
- List: code badge, name, description, fields count
- Create dialog: code (uppercase), name, description, fields editor
  - Fields editor: add/remove rows of `{jsonpath: string, description: string}`
  - On save: serialize to `Record<string, string>` JSONB
- Detail view: expand row or side sheet to show full field schema table

### 8. Gates — `/settings/gates` _(new page)_
Backend: `GET/POST/PUT/DELETE /api/v1/gates`
- List table: gate_id, name, location, status badge
- Create/edit dialog: gate_id, name, location, description, status toggle
- Delete confirm dialog

### 9. Users — `/settings/users`
Backend: `GET /api/auth/admin/users`, `PUT /api/auth/admin/users/:id/role`, `DELETE /api/auth/admin/users/:id`, `POST /api/auth/admin/users/:id/reset-password`
- Shadcn: `Button`, `Badge`, `Table`, `Dialog`
- List table: username, role badge, last_login, created_at, actions
- Each row links to their profile page at `/settings/users/:id`
- Edit role dialog: role dropdown from `GET /api/auth/admin/roles`
- Reset password dialog: admin sets new password via `POST /api/auth/admin/users/:id/reset-password`
- Delete with confirm dialog

### 10. User Profile — `/settings/users/[id]`  _(new page)_
Backend: `GET /api/v1/profiles?auth_id=` / `POST /api/v1/profiles` / `PUT /api/v1/profiles/:id`
Auth user: `GET /api/auth/admin/users/:id`
- Header: username, role badge, created_at, last_login
- Two-column layout:
  - Left — **Account**: username (read-only), role (editable dropdown), sessions list from `GET /api/auth/sessions` _(admin view, if supported)_
  - Right — **Profile**: first_name, last_name, phone, gate_id (Select from gates list), notes (Textarea)
- Profile is fetched by `?auth_id={user.id}`; if none exists, show "Create profile" button that POSTs a new one
- Save button per section, sonner toast on success/error
- Shadcn: `Button`, `Card`/`CardContent`, `Input`, `Textarea`, `Select`, `Label`, `Badge`, `Separator`

### 11. Roles — `/settings/roles`
- Shadcn: `Button`, `Badge`, `Card`, `Dialog`, `Table`
- Per-role cards remain; replace primitives
- Permission matrix dialog: shadcn checkbox list

---

## `src/lib/types.ts` Updates Required

```ts
// Add/fix APIKey type
interface APIKey {
  id: number;
  owner_name: string;
  is_active: boolean;
  gate_id: string;
  permissions: Permission[];
  created_at: string;
}

// Fix DeviceConfig: source_id is string version of APIKey.id
// EventType.fields is Record<string,string>
// All types already present — verify json tag alignment
```

---

## `src/lib/api.ts` Updates Required

```ts
// Add to api object:
keys: {
  list: () => req<APIKey[]>('/api/auth/admin/keys'),
  create: (d: { name: string; gate_id: string; permission_ids: string[] }) =>
    req<{ api_key: string; id: number }>('/api/auth/admin/keys', { method: 'POST', ... }),
  update: (id: number, d: { owner_name?: string; is_active?: boolean }) =>
    req<void>(`/api/auth/admin/keys/${id}`, { method: 'PUT', ... }),
  updatePermissions: (id: number, permission_ids: string[]) =>
    req<void>(`/api/auth/admin/keys/${id}/permissions`, { method: 'PUT', ... }),
  delete: (id: number) => req<void>(`/api/auth/admin/keys/${id}`, { method: 'DELETE' }),
},

types: {
  list: () => req<EventType[]>('/api/v1/types'),
  get: (id: string) => req<EventType>(`/api/v1/types/${id}`),
  create: (d: { code: string; name: string; description: string; fields: Record<string,string> }) =>
    req<EventType>('/api/v1/types', { method: 'POST', ... }),
},

// configs: add list endpoint (currently missing)
// GET /api/v1/configs/device — list all device configs
```

---

## Migration Order

1. Update `src/lib/types.ts` — add APIKey, fix existing types
2. Update `src/lib/api.ts` — add keys, fix types endpoint, add configs list
3. Update `+layout.svelte` — new nav items, shadcn Separator for sections
4. Migrate primitive components (Button, Input, etc.) → delete custom, import from ui/
5. Update `TopBar.svelte` to use shadcn Button internally
6. Keep `Field.svelte` as label+hint wrapper using shadcn Label
7. Create `GateBadge.svelte` thin wrapper around shadcn Badge with gate-color logic
8. Migrate pages in order: login → dashboard → transaction detail → devices list → device edit → keys → types → gates → users → user profile → roles
9. Delete obsolete custom component files once all references are replaced
