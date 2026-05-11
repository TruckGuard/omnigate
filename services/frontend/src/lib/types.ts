export interface GateSettings {
  transaction_ttl_minutes?: number;
  auto_close_transactions?: boolean;
  max_events_per_transaction?: number;
}

export interface Gate {
  id: string;
  gate_id: string;
  name: string;
  location: string;
  description: string;
  status: 'active' | 'inactive';
  settings?: GateSettings;
  created_at: string;
  updated_at: string;
}

export interface Session {
  session_id: string;
  user_id: number;
  username: string;
  role: string;
  permissions?: string[];
  created_at?: string;
}

export interface EventTypeField {
  name: string;
  description: string;
  type: 'string' | 'number' | 'boolean' | 'datetime' | 'image_url';
  required: boolean;
}

export interface EventType {
  id: string;
  code: string;
  name: string;
  description: string;
  fields: Record<string, EventTypeField>;
  searchable_key: string;
  created_at: string;
}

export interface Event {
  id: string;
  transaction_id: string | null;
  event_type_id: string;
  event_type?: EventType;
  gate_id: string;
  source_id: string;
  data: Record<string, unknown>;
  raw_data_key: string;
  image_keys: string[] | null;
  // Матеріалізовані поля (заповнюються BeforeSave хуком на бекенді)
  type_code: string;
  searchable_value: string;
  created_at: string;
}

export interface Transaction {
  id: string;
  code: string;
  gate_id: string;
  is_open: boolean;
  note: string;
  vehicle_plate: string;
  events?: Event[];
  created_at: string;
  updated_at: string;
}

export interface VehicleHistoryResponse {
  plate: string;
  data: Transaction[];
}

export interface TransactionListResponse {
  data: Transaction[];
  total: number;
  page: number;
  limit: number;
}

export interface GateStats {
  total_transactions: number;
  open_transactions: number;
  total_devices: number;
  recent_transactions: Transaction[];
}

export interface Trigger {
  source_id: string;
}

export interface DeviceConfig {
  id: string;
  source_id: string;
  event_type_id: string;
  event_type?: EventType;
  gate_id: string;
  data_mapping: Record<string, string>;
  data_type: string;
  /** URL Puller calls when THIS device is the pull target of another device's trigger. */
  trigger_url: string | null;
  /** List of target devices this device activates after its own event is processed. */
  triggers: Trigger[];
  trigger_enabled: boolean;
  enabled: boolean;
  created_at: string;
  updated_at: string;
}

export interface UserProfile {
  id: string;
  auth_id: number;
  first_name: string;
  last_name: string;
  phone: string;
  gate_id: string;
  notes: string;
  created_at: string;
  updated_at: string;
}

export interface AuthUser {
  id: number;
  username: string;
  role_id: number;
  role?: AuthRole;
  created_at: string;
  last_login: string | null;
}

export interface AuthRole {
  id: number;
  name: string;
  description: string;
  permissions?: Permission[];
}

export interface Permission {
  id: string;
  name: string;
  description: string;
  module: string;
}

export interface APIKey {
  id: number;
  owner_name: string;
  is_active: boolean;
  gate_id: string;
  permissions: Permission[];
  created_at: string;
}

export interface ValidateResponse {
  id: string;
  username: string;
  role: string;
  permissions: string[];
  session_id: string;
}
