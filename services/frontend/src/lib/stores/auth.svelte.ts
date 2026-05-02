const SESSION_KEY     = 'omnigate_session';
const USERNAME_KEY    = 'omnigate_username';
const ROLE_KEY        = 'omnigate_role';
const PERMISSIONS_KEY = 'omnigate_permissions';

function read(key: string): string | null {
  if (typeof localStorage === 'undefined') return null;
  return localStorage.getItem(key);
}

function readPermissions(): string[] {
  if (typeof localStorage === 'undefined') return [];
  try {
    return JSON.parse(localStorage.getItem(PERMISSIONS_KEY) ?? '[]');
  } catch {
    return [];
  }
}

class AuthStore {
  sessionId   = $state<string | null>(read(SESSION_KEY));
  username    = $state<string | null>(read(USERNAME_KEY));
  role        = $state<string | null>(read(ROLE_KEY));
  permissions = $state<string[]>(readPermissions());

  get isAuthenticated(): boolean {
    return !!this.sessionId;
  }

  can(permission: string): boolean {
    return this.permissions.includes(permission);
  }

  login(sessionId: string, username: string, role: string, permissions: string[] = []) {
    this.sessionId   = sessionId;
    this.username    = username;
    this.role        = role;
    this.permissions = permissions;
    localStorage.setItem(SESSION_KEY,     sessionId);
    localStorage.setItem(USERNAME_KEY,    username);
    localStorage.setItem(ROLE_KEY,        role);
    localStorage.setItem(PERMISSIONS_KEY, JSON.stringify(permissions));
  }

  setPermissions(permissions: string[]) {
    this.permissions = permissions;
    localStorage.setItem(PERMISSIONS_KEY, JSON.stringify(permissions));
  }

  logout() {
    this.sessionId   = null;
    this.username    = null;
    this.role        = null;
    this.permissions = [];
    localStorage.removeItem(SESSION_KEY);
    localStorage.removeItem(USERNAME_KEY);
    localStorage.removeItem(ROLE_KEY);
    localStorage.removeItem(PERMISSIONS_KEY);
  }
}

export const authStore = new AuthStore();
