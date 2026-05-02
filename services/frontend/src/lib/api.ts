import type {
  APIKey,
  AuthRole,
  AuthUser,
  DeviceConfig,
  EventType,
  Gate,
  Permission,
  Transaction,
  TransactionListResponse,
  UserProfile,
  ValidateResponse,
} from './types.js';
import { authStore } from './stores/auth.svelte.js';

function getToken(): string | null {
  return authStore.sessionId;
}

async function req<T>(path: string, init: RequestInit = {}): Promise<T> {
  const token = getToken();
  const headers: Record<string, string> = {
    'Content-Type': 'application/json',
    ...(token ? { Authorization: `Bearer ${token}` } : {}),
    ...(init.headers as Record<string, string>),
  };
  const res = await fetch(path, { ...init, headers });
  if (!res.ok) throw new Error(`${res.status}: ${await res.text().catch(() => res.statusText)}`);
  if (res.status === 204) return undefined as T;
  return res.json();
}

export interface TxQuery {
  page?: number;
  limit?: number;
  gate_id?: string;
  status?: string;
  search?: string;
}

export const api = {
  auth: {
    login: (username: string, password: string) =>
      req<{ session_id: string }>('/api/auth/login', { method: 'POST', body: JSON.stringify({ username, password }) }),
    logout: () => req<void>('/api/auth/logout', { method: 'POST' }),
    validate: () => req<ValidateResponse>('/api/auth/validate'),
    users: () => req<AuthUser[]>('/api/auth/admin/users'),
    getUser: (id: number) => req<AuthUser>(`/api/auth/admin/users/${id}`),
    deleteUser: (id: number) => req<void>(`/api/auth/admin/users/${id}`, { method: 'DELETE' }),
    updateUserRole: (userId: number, roleId: number) =>
      req<void>(`/api/auth/admin/users/${userId}/role`, { method: 'PUT', body: JSON.stringify({ role_id: roleId }) }),
    resetPassword: (userId: number, password: string) =>
      req<void>(`/api/auth/admin/users/${userId}/reset-password`, { method: 'POST', body: JSON.stringify({ password }) }),
    roles: () => req<AuthRole[]>('/api/auth/admin/roles'),
    createRole: (data: { name: string; description: string }) =>
      req<AuthRole>('/api/auth/admin/roles', { method: 'POST', body: JSON.stringify(data) }),
    updateRole: (id: number, data: { name?: string; description?: string }) =>
      req<AuthRole>(`/api/auth/admin/roles/${id}`, { method: 'PUT', body: JSON.stringify(data) }),
    deleteRole: (id: number) => req<void>(`/api/auth/admin/roles/${id}`, { method: 'DELETE' }),
    assignPermissions: (roleId: number, ids: string[]) =>
      req<void>(`/api/auth/admin/roles/${roleId}/permissions`, { method: 'POST', body: JSON.stringify({ permission_ids: ids }) }),
    permissions: () => req<Permission[]>('/api/auth/admin/permissions'),
    keys: {
      list: () => req<APIKey[]>('/api/auth/admin/keys'),
      create: (d: { name: string; gate_id: string; permission_ids: string[] }) =>
        req<{ api_key: string; id: number }>('/api/auth/admin/keys', { method: 'POST', body: JSON.stringify(d) }),
      update: (id: number, d: { owner_name?: string; is_active?: boolean; gate_id?: string }) =>
        req<void>(`/api/auth/admin/keys/${id}`, { method: 'PUT', body: JSON.stringify(d) }),
      updatePermissions: (id: number, permission_ids: string[]) =>
        req<void>(`/api/auth/admin/keys/${id}/permissions`, { method: 'PUT', body: JSON.stringify({ permission_ids }) }),
      delete: (id: number) => req<void>(`/api/auth/admin/keys/${id}`, { method: 'DELETE' }),
    },
  },

  transactions: {
    list: (q: TxQuery = {}) => {
      const p = new URLSearchParams();
      if (q.page)    p.set('page', String(q.page));
      if (q.limit)   p.set('limit', String(q.limit));
      if (q.gate_id) p.set('gate_id', q.gate_id);
      if (q.status)  p.set('status', q.status);
      if (q.search)  p.set('search', q.search);
      return req<TransactionListResponse>(`/api/v1/transactions?${p}`);
    },
    get: (id: string) => req<Transaction>(`/api/v1/transactions/${id}`),
    update: (id: string, data: { status?: string; note?: string }) =>
      req<Transaction>(`/api/v1/transactions/${id}`, { method: 'PUT', body: JSON.stringify(data) }),
    delete: (id: string) => req<void>(`/api/v1/transactions/${id}`, { method: 'DELETE' }),
  },

  gates: {
    list: () => req<Gate[]>('/api/v1/gates'),
    get: (id: string) => req<Gate>(`/api/v1/gates/${id}`),
    create: (d: { gate_id: string; name: string; location?: string; description?: string }) =>
      req<Gate>('/api/v1/gates', { method: 'POST', body: JSON.stringify(d) }),
    update: (id: string, d: Partial<Gate>) =>
      req<Gate>(`/api/v1/gates/${id}`, { method: 'PUT', body: JSON.stringify(d) }),
    delete: (id: string) => req<void>(`/api/v1/gates/${id}`, { method: 'DELETE' }),
  },

  types: {
    list: () => req<EventType[]>('/api/v1/types'),
    get: (id: string) => req<EventType>(`/api/v1/types/${id}`),
    create: (d: { code: string; name: string; description: string; fields: Record<string, unknown> }) =>
      req<EventType>('/api/v1/types', { method: 'POST', body: JSON.stringify(d) }),
  },

  configs: {
    list: () => req<DeviceConfig[]>('/api/v1/configs/devices'),
    get: (sourceId: string) => req<DeviceConfig>(`/api/v1/configs/devices/${sourceId}`),
    create: (d: Omit<DeviceConfig, 'id' | 'created_at' | 'updated_at' | 'enabled' | 'event_type'>) =>
      req<DeviceConfig>('/api/v1/configs/devices', { method: 'POST', body: JSON.stringify(d) }),
    update: (id: string, d: Partial<Pick<DeviceConfig, 'data_mapping' | 'trigger_enabled' | 'trigger_url'>>) =>
      req<DeviceConfig>(`/api/v1/configs/devices/${id}`, { method: 'PUT', body: JSON.stringify(d) }),
    delete: (id: string) => req<void>(`/api/v1/configs/devices/${id}`, { method: 'DELETE' }),
  },

  profiles: {
    list: (authId?: number) => {
      const p = authId !== undefined ? `?auth_id=${authId}` : '';
      return req<UserProfile[]>(`/api/v1/profiles${p}`);
    },
    get: (id: string) => req<UserProfile>(`/api/v1/profiles/${id}`),
    create: (d: Omit<UserProfile, 'id' | 'created_at' | 'updated_at'>) =>
      req<UserProfile>('/api/v1/profiles', { method: 'POST', body: JSON.stringify(d) }),
    update: (id: string, d: Partial<UserProfile>) =>
      req<UserProfile>(`/api/v1/profiles/${id}`, { method: 'PUT', body: JSON.stringify(d) }),
  },

  imageUrl: (key: string) => `/data/${key}`,
};
