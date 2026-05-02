<script lang="ts">
  import { toast } from 'svelte-sonner';
  import TopBar from '$lib/components/TopBar.svelte';
  import Field from '$lib/components/Field.svelte';
  import PermGuard from '$lib/components/PermGuard.svelte';
  import { Button } from '$lib/components/ui/button/index.js';
  import { Badge } from '$lib/components/ui/badge/index.js';
  import { Input } from '$lib/components/ui/input/index.js';
  import { Card, CardContent, CardHeader, CardTitle } from '$lib/components/ui/card/index.js';
  import {
    Dialog, DialogContent, DialogHeader, DialogTitle,
    DialogFooter, DialogDescription,
  } from '$lib/components/ui/dialog/index.js';
  import { api } from '$lib/api.js';
  import type { AuthRole, Permission } from '$lib/types.js';
  import { Plus, Check } from 'lucide-svelte';

  let roles       = $state<AuthRole[]>([]);
  let permissions = $state<Permission[]>([]);
  let loading     = $state(true);
  let saving      = $state(false);

  let editRole     = $state<AuthRole | null>(null);
  let selectedPerms = $state<string[]>([]);

  let newRoleOpen = $state(false);
  let newName     = $state('');
  let newDesc     = $state('');

  let deleteOpen  = $state(false);
  let deleteTarget = $state<AuthRole | null>(null);

  async function load() {
    try {
      [roles, permissions] = await Promise.all([api.auth.roles(), api.auth.permissions()]);
    } catch {
      toast.error('Failed to load roles');
    } finally {
      loading = false;
    }
  }

  $effect(() => { load(); });

  const permsByModule = $derived(() => {
    const map = new Map<string, Permission[]>();
    for (const p of permissions) {
      const list = map.get(p.module) ?? [];
      list.push(p);
      map.set(p.module, list);
    }
    return map;
  });

  function openEdit(role: AuthRole) {
    editRole = role;
    selectedPerms = (role.permissions ?? []).map(p => p.id);
  }

  function togglePerm(id: string) {
    if (selectedPerms.includes(id)) {
      selectedPerms = selectedPerms.filter(p => p !== id);
    } else {
      selectedPerms = [...selectedPerms, id];
    }
  }

  async function savePermissions() {
    if (!editRole) return;
    saving = true;
    try {
      await api.auth.assignPermissions(editRole.id, selectedPerms);
      toast.success('Permissions updated');
      editRole = null;
      await load();
    } catch {
      toast.error('Failed to save permissions');
    } finally {
      saving = false;
    }
  }

  async function createRole() {
    if (!newName.trim()) return;
    saving = true;
    try {
      await api.auth.createRole({ name: newName, description: newDesc });
      toast.success('Role created');
      newRoleOpen = false;
      newName = ''; newDesc = '';
      await load();
    } catch {
      toast.error('Failed to create role');
    } finally {
      saving = false;
    }
  }

  async function handleDelete() {
    if (!deleteTarget) return;
    try {
      await api.auth.deleteRole(deleteTarget.id);
      toast.success('Role deleted');
      deleteOpen = false;
      await load();
    } catch {
      toast.error('Failed to delete role');
    }
  }
</script>

<TopBar crumbs={['OmniGate', 'Roles']} title="Roles">
  {#snippet actions()}
    <PermGuard permission="manage:roles">
      <Button size="sm" onclick={() => (newRoleOpen = true)}>
        <Plus size={14} /> New role
      </Button>
    </PermGuard>
  {/snippet}
</TopBar>

<main class="flex-1 p-6 space-y-4">
  {#each roles as role (role.id)}
    <Card>
      <CardHeader class="pb-2">
        <div class="flex items-start justify-between gap-4">
          <div>
            <div class="flex items-center gap-2">
              <CardTitle class="text-[14px]">{role.name}</CardTitle>
              <Badge variant="outline" class="font-mono text-[10px]">id:{role.id}</Badge>
            </div>
            {#if role.description}
              <p class="text-[12px] text-muted-foreground mt-0.5">{role.description}</p>
            {/if}
          </div>
          <PermGuard permission="manage:roles">
            <div class="flex gap-2 shrink-0">
              <Button variant="outline" size="sm" onclick={() => openEdit(role)}>Edit permissions</Button>
              <Button variant="ghost" size="sm" class="hover:text-destructive"
                onclick={() => { deleteTarget = role; deleteOpen = true; }}>
                Delete
              </Button>
            </div>
          </PermGuard>
        </div>
      </CardHeader>
      <CardContent>
        <div class="flex flex-wrap gap-1.5">
          {#each role.permissions ?? [] as perm}
            <Badge variant="secondary" class="font-mono text-[11px]">{perm.id}</Badge>
          {/each}
          {#if !role.permissions?.length}
            <span class="text-[12px] text-muted-foreground">No permissions assigned</span>
          {/if}
        </div>
      </CardContent>
    </Card>
  {/each}
  {#if !loading && roles.length === 0}
    <div class="text-center text-muted-foreground py-12">No roles defined.</div>
  {/if}
</main>

<!-- Edit permissions dialog -->
<Dialog open={!!editRole} onOpenChange={(v) => { if (!v) editRole = null; }}>
  <DialogContent class="max-w-md">
    <DialogHeader>
      <DialogTitle>Permissions — {editRole?.name}</DialogTitle>
      <DialogDescription>Toggle which permissions this role grants.</DialogDescription>
    </DialogHeader>
    <div class="space-y-1 max-h-[420px] overflow-y-auto py-2">
      {#each [...permsByModule()] as [module, perms]}
        <p class="text-[11px] uppercase tracking-wide text-muted-foreground mt-3 mb-1">{module}</p>
        {#each perms as perm}
          {@const active = selectedPerms.includes(perm.id)}
          <button
            onclick={() => togglePerm(perm.id)}
            class="w-full flex items-center justify-between px-3 py-2 rounded-md border text-[13px] transition-colors
              {active ? 'bg-primary/10 border-primary/30 text-primary' : 'bg-background border-border text-muted-foreground hover:bg-muted'}"
          >
            <div class="text-left">
              <span class="font-mono font-medium text-[12px]">{perm.id}</span>
              {#if perm.description}
                <span class="block text-[11px] mt-0.5 opacity-70">{perm.description}</span>
              {/if}
            </div>
            {#if active}<Check size={14} class="shrink-0" />{/if}
          </button>
        {/each}
      {/each}
    </div>
    <DialogFooter>
      <Button variant="outline" onclick={() => (editRole = null)}>Cancel</Button>
      <Button onclick={savePermissions} disabled={saving}>
        {saving ? 'Saving…' : 'Save permissions'}
      </Button>
    </DialogFooter>
  </DialogContent>
</Dialog>

<!-- New role dialog -->
<Dialog bind:open={newRoleOpen}>
  <DialogContent class="max-w-sm">
    <DialogHeader><DialogTitle>Create role</DialogTitle></DialogHeader>
    <div class="space-y-4 py-2">
      <Field label="Name"><Input bind:value={newName} placeholder="e.g. operator" /></Field>
      <Field label="Description"><Input bind:value={newDesc} placeholder="Optional description" /></Field>
    </div>
    <DialogFooter>
      <Button variant="outline" onclick={() => (newRoleOpen = false)}>Cancel</Button>
      <Button onclick={createRole} disabled={saving || !newName.trim()}>
        {saving ? 'Creating…' : 'Create'}
      </Button>
    </DialogFooter>
  </DialogContent>
</Dialog>

<!-- Delete dialog -->
<Dialog bind:open={deleteOpen}>
  <DialogContent class="max-w-sm">
    <DialogHeader>
      <DialogTitle>Delete role?</DialogTitle>
      <DialogDescription>
        Role <span class="font-mono">{deleteTarget?.name}</span> will be permanently removed.
        Users with this role will need to be reassigned.
      </DialogDescription>
    </DialogHeader>
    <DialogFooter>
      <Button variant="outline" onclick={() => (deleteOpen = false)}>Cancel</Button>
      <Button variant="destructive" onclick={handleDelete}>Delete</Button>
    </DialogFooter>
  </DialogContent>
</Dialog>
