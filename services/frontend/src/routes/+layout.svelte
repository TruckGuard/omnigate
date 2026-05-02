<script lang="ts">
  import '../app.css';
  import { page } from '$app/stores';
  import { goto } from '$app/navigation';
  import { Toaster } from 'svelte-sonner';
  import { LayoutGrid, Cpu, KeySquare, Layers, GitFork, Users, KeyRound, LogOut } from 'lucide-svelte';
  import type { Snippet } from 'svelte';
  import { authStore } from '$lib/stores/auth.svelte.js';
  import { api } from '$lib/api.js';
  import { Separator } from '$lib/components/ui/separator/index.js';
  import { Button } from '$lib/components/ui/button/index.js';

  let { children }: { children: Snippet } = $props();

  interface NavItem {
    id: string;
    href: string;
    label: string;
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    icon: any;
    section: string;
    permission?: string;
  }

  const navItems: NavItem[] = [
    { id: 'transactions', href: '/',                 label: 'Transactions', icon: LayoutGrid, section: 'Operations' },
    { id: 'devices',      href: '/settings/devices', label: 'Devices',      icon: Cpu,        section: 'Operations' },
    { id: 'keys',         href: '/settings/keys',    label: 'API Keys',     icon: KeySquare,  section: 'Operations',    permission: 'read:keys' },
    { id: 'types',        href: '/settings/types',   label: 'Event Types',  icon: Layers,     section: 'Configuration' },
    { id: 'gates',        href: '/settings/gates',   label: 'Gates',        icon: GitFork,    section: 'Configuration' },
    { id: 'users',        href: '/settings/users',   label: 'Users',        icon: Users,      section: 'Access',        permission: 'manage:users' },
    { id: 'roles',        href: '/settings/roles',   label: 'Roles',        icon: KeyRound,   section: 'Access',        permission: 'read:roles' },
  ];

  const isLoginPage = $derived($page.url.pathname === '/login');

  $effect(() => {
    if (isLoginPage) return;
    if (!authStore.isAuthenticated) { goto('/login'); return; }

    // Refresh permissions on every app boot
    api.auth.validate().then((data) => {
      authStore.setPermissions(data.permissions);
    }).catch(() => {
      authStore.logout();
      goto('/login');
    });
  });

  $effect(() => {
    if (isLoginPage) return;
    const path = $page.url.pathname;
    const item = navItems.find(n => {
      if (n.href === '/') return path === '/';
      return path.startsWith(n.href);
    });
    if (item?.permission && !authStore.can(item.permission)) goto('/');
  });

  async function handleLogout() {
    try { await api.auth.logout(); } catch { /* ignore */ }
    authStore.logout();
    goto('/login');
  }

  const initials = $derived(() => {
    const u = authStore.username ?? '';
    return u.slice(0, 2).toUpperCase() || '?';
  });

  const activeId = $derived(() => {
    const path = $page.url.pathname;
    if (path.startsWith('/settings/devices')) return 'devices';
    if (path.startsWith('/settings/keys'))    return 'keys';
    if (path.startsWith('/settings/types'))   return 'types';
    if (path.startsWith('/settings/gates'))   return 'gates';
    if (path.startsWith('/settings/users'))   return 'users';
    if (path.startsWith('/settings/roles'))   return 'roles';
    return 'transactions';
  });

  // Group nav items by section for separator rendering
  const sections = $derived(() => {
    const seen = new Set<string>();
    return navItems.map(item => {
      const firstInSection = !seen.has(item.section);
      seen.add(item.section);
      return { ...item, firstInSection };
    });
  });
</script>

<div class="min-h-screen flex bg-background" style="font-family: 'Inter', system-ui, sans-serif;">
  {#if !isLoginPage}
  <aside class="w-[224px] shrink-0 bg-card border-r border-border h-screen sticky top-0 flex flex-col overflow-hidden">
    <!-- Brand -->
    <div class="h-[48px] flex items-center gap-2 px-4 border-b border-border shrink-0">
      <div class="w-[22px] h-[22px] rounded-md bg-primary flex items-center justify-center shrink-0">
        <svg viewBox="0 0 40 40" width="14" height="14">
          <rect x="4" y="6" width="32" height="28" rx="4" fill="none" stroke="white" stroke-width="3"/>
          <line x1="4" y1="14" x2="36" y2="14" stroke="white" stroke-width="3"/>
          <line x1="14" y1="14" x2="14" y2="34" stroke="white" stroke-width="3"/>
          <line x1="26" y1="14" x2="26" y2="34" stroke="white" stroke-width="3"/>
        </svg>
      </div>
      <span class="font-bold text-[15px] tracking-[-0.01em]">OmniGate</span>
    </div>

    <!-- Nav -->
    <nav class="p-2 flex flex-col flex-1 overflow-y-auto">
      {#each sections() as item}
        {#if !item.permission || authStore.can(item.permission)}
          {#if item.firstInSection}
            <p class="text-[10px] uppercase tracking-[0.06em] text-muted-foreground px-2 pt-3 pb-1 font-semibold">
              {item.section}
            </p>
          {/if}
          {@const active = activeId() === item.id}
          <a
            href={item.href}
            class="flex items-center gap-2.5 h-8 px-2 rounded-md text-[13px] font-medium transition-colors duration-100
              {active
                ? 'bg-primary/10 text-primary'
                : 'text-muted-foreground hover:bg-muted hover:text-foreground'}"
          >
            <item.icon size={15} />
            {item.label}
          </a>
        {/if}
      {/each}
    </nav>

    <!-- User -->
    <div class="p-2 border-t border-border shrink-0">
      <div class="flex items-center gap-2 px-2 h-9">
        <div class="w-6 h-6 rounded-full bg-primary flex items-center justify-center text-[10px] font-semibold text-primary-foreground shrink-0">
          {initials()}
        </div>
        <div class="leading-tight min-w-0 flex-1">
          <div class="text-[12px] font-medium truncate">{authStore.username ?? '—'}</div>
          <div class="text-[11px] text-muted-foreground capitalize">{authStore.role ?? ''}</div>
        </div>
        <Button
          variant="ghost"
          size="icon-sm"
          onclick={handleLogout}
          title="Sign out"
          class="text-muted-foreground hover:text-destructive shrink-0"
        >
          <LogOut size={14} />
        </Button>
      </div>
    </div>
  </aside>
  {/if}

  <div class="flex-1 min-w-0 flex flex-col min-h-screen">
    {@render children()}
  </div>
</div>

<Toaster position="bottom-right" richColors />
