-- migrate-permissions.sql
-- Migrates the permission system from the old naming scheme to the new one.
--
-- What changes:
--   keys/*         → api-keys/*       (resource renamed)
--   configs/*      → devices/*        (resource renamed)
--   *:all suffix   → base permission  (scope concept dropped)
--   transactions:close → close:transactions  (action:resource order fixed)
--
-- How to run (against omnigate_auth database):
--   psql $DATABASE_URL -f scripts/migrate-permissions.sql
--   or
--   docker exec -i <postgres-container> psql -U postgres omnigate_auth -f - < scripts/migrate-permissions.sql
--
-- After running this script:
--   1. Restart the auth service — seed will recreate policy_rules, hierarchy,
--      and reset admin/manager/operator role permissions to the new defaults.
--   2. Custom roles and all API keys are handled entirely by this script.
--
-- The script is idempotent: safe to run multiple times.

BEGIN;

-- ─────────────────────────────────────────────────────────────────────────────
-- 1. Insert all new permissions that don't exist yet
-- ─────────────────────────────────────────────────────────────────────────────

INSERT INTO permissions (id, name, module) VALUES
  -- users
  ('read:users',           'Users: Read',           'auth'),
  ('create:users',         'Users: Create',         'auth'),
  ('update:users',         'Users: Update',         'auth'),
  ('delete:users',         'Users: Delete',         'auth'),
  ('change-role:users',    'Users: Change Role',    'auth'),
  ('reset-password:users', 'Users: Reset Password', 'auth'),
  ('manage:users',         'Users: Full Access',    'auth'),
  -- roles
  ('read:roles',               'Roles: Read',               'auth'),
  ('create:roles',             'Roles: Create',             'auth'),
  ('update:roles',             'Roles: Update',             'auth'),
  ('delete:roles',             'Roles: Delete',             'auth'),
  ('update-permissions:roles', 'Roles: Update Permissions', 'auth'),
  ('manage:roles',             'Roles: Full Access',        'auth'),
  -- api-keys
  ('read:api-keys',               'API Keys: Read',               'auth'),
  ('create:api-keys',             'API Keys: Create',             'auth'),
  ('update:api-keys',             'API Keys: Update',             'auth'),
  ('delete:api-keys',             'API Keys: Delete',             'auth'),
  ('update-permissions:api-keys', 'API Keys: Update Permissions', 'auth'),
  ('create-digest:api-keys',      'API Keys: Set Digest',         'auth'),
  ('delete-digest:api-keys',      'API Keys: Clear Digest',       'auth'),
  ('manage:api-keys',             'API Keys: Full Access',        'auth'),
  -- audit
  ('read:audit', 'Audit: View', 'auth'),
  -- ingest
  ('ingest:events',        'Ingest: Create Event',           'ingestor'),
  ('ingest:assume-source', 'Ingest: Assume Source Identity', 'ingestor'),
  -- events
  ('read:events',   'Events: Read',        'core'),
  ('create:events', 'Events: Create',      'core'),
  ('delete:events', 'Events: Delete',      'core'),
  ('manage:events', 'Events: Full Access', 'core'),
  -- transactions
  ('read:transactions',   'Transactions: Read',        'core'),
  ('create:transactions', 'Transactions: Create',      'core'),
  ('update:transactions', 'Transactions: Update',      'core'),
  ('delete:transactions', 'Transactions: Delete',      'core'),
  ('close:transactions',  'Transactions: Close',       'core'),
  ('manage:transactions', 'Transactions: Full Access', 'core'),
  -- devices
  ('read:devices',    'Devices: Read',        'core'),
  ('create:devices',  'Devices: Create',      'core'),
  ('update:devices',  'Devices: Update',      'core'),
  ('delete:devices',  'Devices: Delete',      'core'),
  ('trigger:devices', 'Devices: Trigger',     'core'),
  ('manage:devices',  'Devices: Full Access', 'core'),
  -- types
  ('read:types',   'Event Types: Read',        'core'),
  ('create:types', 'Event Types: Create',      'core'),
  ('update:types', 'Event Types: Update',      'core'),
  ('delete:types', 'Event Types: Delete',      'core'),
  ('manage:types', 'Event Types: Full Access', 'core'),
  -- gates
  ('read:gates',   'Gates: Read',        'core'),
  ('create:gates', 'Gates: Create',      'core'),
  ('update:gates', 'Gates: Update',      'core'),
  ('delete:gates', 'Gates: Delete',      'core'),
  ('manage:gates', 'Gates: Full Access', 'core'),
  -- profiles
  ('read:profiles',   'Profiles: Read',        'core'),
  ('create:profiles', 'Profiles: Create',      'core'),
  ('update:profiles', 'Profiles: Update',      'core'),
  ('delete:profiles', 'Profiles: Delete',      'core'),
  ('manage:profiles', 'Profiles: Full Access', 'core')
ON CONFLICT (id) DO UPDATE
  SET name   = EXCLUDED.name,
      module = EXCLUDED.module;

-- ─────────────────────────────────────────────────────────────────────────────
-- 2. Define old → new permission mapping
-- ─────────────────────────────────────────────────────────────────────────────

CREATE TEMP TABLE _perm_map (old_id text PRIMARY KEY, new_id text NOT NULL);

INSERT INTO _perm_map (old_id, new_id) VALUES
  -- keys → api-keys
  ('read:keys',         'read:api-keys'),
  ('create:keys',       'create:api-keys'),
  ('update:keys',       'update:api-keys'),
  ('delete:keys',       'delete:api-keys'),
  ('manage:keys',       'manage:api-keys'),
  -- configs → devices
  ('read:configs',      'read:devices'),
  ('read:configs:all',  'read:devices'),
  ('create:configs',    'create:devices'),
  ('update:configs',    'update:devices'),
  ('delete:configs',    'delete:devices'),
  ('manage:configs',    'manage:devices'),
  ('manage:configs:all','manage:devices'),
  -- transactions:close → close:transactions
  ('transactions:close','close:transactions'),
  -- drop :all scope (map to base permission)
  ('read:events:all',        'read:events'),
  ('manage:events:all',      'manage:events'),
  ('read:transactions:all',  'read:transactions'),
  ('manage:transactions:all','manage:transactions'),
  ('read:types:all',         'read:types'),
  ('manage:types:all',       'manage:types'),
  ('read:gates:all',         'read:gates'),
  ('manage:gates:all',       'manage:gates'),
  ('read:profiles:all',      'read:profiles'),
  ('manage:profiles:all',    'manage:profiles'),
  ('read:users:all',         'read:users'),
  ('manage:users:all',       'manage:users'),
  ('read:roles:all',         'read:roles'),
  ('manage:roles:all',       'manage:roles');

-- ─────────────────────────────────────────────────────────────────────────────
-- 3. Migrate role_permissions
--    (admin/manager/operator will be reset by seed on restart;
--     this covers any custom roles the user may have created)
-- ─────────────────────────────────────────────────────────────────────────────

-- Add new permissions where roles had old ones
INSERT INTO role_permissions (role_id, permission_id)
  SELECT rp.role_id, m.new_id
  FROM role_permissions rp
  JOIN _perm_map m ON m.old_id = rp.permission_id
ON CONFLICT DO NOTHING;

-- Remove old permissions from all roles
DELETE FROM role_permissions
WHERE permission_id IN (SELECT old_id FROM _perm_map);

-- ─────────────────────────────────────────────────────────────────────────────
-- 4. Migrate apikey_permissions
--    (covers all keys including system worker and puller,
--     which seed does not update if they already exist)
-- ─────────────────────────────────────────────────────────────────────────────

INSERT INTO apikey_permissions (api_key_id, permission_id)
  SELECT ap.api_key_id, m.new_id
  FROM apikey_permissions ap
  JOIN _perm_map m ON m.old_id = ap.permission_id
ON CONFLICT DO NOTHING;

DELETE FROM apikey_permissions
WHERE permission_id IN (SELECT old_id FROM _perm_map);

-- ─────────────────────────────────────────────────────────────────────────────
-- 5. Clear policy_rules and permission_hierarchies
--    Seed will recreate both correctly on the next service restart.
-- ─────────────────────────────────────────────────────────────────────────────

DELETE FROM policy_rules;
DELETE FROM permission_hierarchies;

-- ─────────────────────────────────────────────────────────────────────────────
-- 6. Delete old permissions
--    Safe to remove now — no role or key references them anymore.
--    Keeps the permissions table clean.
-- ─────────────────────────────────────────────────────────────────────────────

DELETE FROM permissions WHERE id IN (SELECT old_id FROM _perm_map);

-- Also remove any leftover :all permissions not covered by the map above
DELETE FROM permissions WHERE id LIKE '%:all';

COMMIT;
