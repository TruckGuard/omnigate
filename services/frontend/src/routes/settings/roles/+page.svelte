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
      toast.error('Помилка завантаження ролей');
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
      toast.success('Дозволи оновлено');
      editRole = null;
      await load();
    } catch {
      toast.error('Помилка збереження дозволів');
    } finally {
      saving = false;
    }
  }

  async function createRole() {
    if (!newName.trim()) return;
    saving = true;
    try {
      await api.auth.createRole({ name: newName, description: newDesc });
      toast.success('Роль створено');
      newRoleOpen = false;
      newName = ''; newDesc = '';
      await load();
    } catch {
      toast.error('Помилка створення ролі');
    } finally {
      saving = false;
    }
  }

  async function handleDelete() {
    if (!deleteTarget) return;
    try {
      await api.auth.deleteRole(deleteTarget.id);
      toast.success('Роль видалено');
      deleteOpen = false;
      await load();
    } catch {
      toast.error('Помилка видалення ролі');
    }
  }
</script>

<TopBar crumbs={['OmniGate', 'Ролі']} title="Ролі">
  {#snippet actions()}
    <PermGuard permission="manage:roles">
      <Button size="sm" onclick={() => (newRoleOpen = true)}>
        <Plus size={14} /> Нова роль
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
              <CardTitle class="text-sm">{role.name}</CardTitle>
              <Badge variant="outline" class="font-mono text-[10px]">id:{role.id}</Badge>
            </div>
            {#if role.description}
              <p class="text-xs text-muted-foreground mt-0.5">{role.description}</p>
            {/if}
          </div>
          <PermGuard permission="manage:roles">
            <div class="flex gap-2 shrink-0">
              <Button variant="outline" size="sm" onclick={() => openEdit(role)}>Редагувати дозволи</Button>
              <Button variant="ghost" size="sm" class="hover:text-destructive"
                onclick={() => { deleteTarget = role; deleteOpen = true; }}>
                Видалити
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
            <span class="text-xs text-muted-foreground">Дозволів не призначено</span>
          {/if}
        </div>
      </CardContent>
    </Card>
  {/each}
  {#if !loading && roles.length === 0}
    <div class="text-center text-muted-foreground py-12">Ролей ще не визначено.</div>
  {/if}
</main>

<!-- Edit permissions dialog -->
<Dialog open={!!editRole} onOpenChange={(v) => { if (!v) editRole = null; }}>
  <DialogContent class="max-w-md">
    <DialogHeader>
      <DialogTitle>Дозволи — {editRole?.name}</DialogTitle>
      <DialogDescription>Оберіть дозволи, які надає ця роль.</DialogDescription>
    </DialogHeader>
    <div class="space-y-1 max-h-[420px] overflow-y-auto py-2">
      {#each [...permsByModule()] as [module, perms]}
        <p class="text-[11px] uppercase tracking-wide text-muted-foreground mt-3 mb-1">{module}</p>
        {#each perms as perm}
          {@const active = selectedPerms.includes(perm.id)}
          <button
            onclick={() => togglePerm(perm.id)}
            class="w-full flex items-center justify-between px-3 py-2 rounded-md border text-sm transition-colors
              {active ? 'bg-primary/10 border-primary/30 text-primary' : 'bg-background border-border text-muted-foreground hover:bg-muted'}"
          >
            <div class="text-left">
              <span class="font-mono font-medium text-xs">{perm.id}</span>
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
      <Button variant="outline" onclick={() => (editRole = null)}>Скасувати</Button>
      <Button onclick={savePermissions} disabled={saving}>
        {saving ? 'Збереження…' : 'Зберегти дозволи'}
      </Button>
    </DialogFooter>
  </DialogContent>
</Dialog>

<!-- New role dialog -->
<Dialog bind:open={newRoleOpen}>
  <DialogContent class="max-w-sm">
    <DialogHeader><DialogTitle>Створити роль</DialogTitle></DialogHeader>
    <div class="space-y-4 py-2">
      <Field label="Назва"><Input bind:value={newName} placeholder="напр. operator" /></Field>
      <Field label="Опис"><Input bind:value={newDesc} placeholder="Необов'язковий опис" /></Field>
    </div>
    <DialogFooter>
      <Button variant="outline" onclick={() => (newRoleOpen = false)}>Скасувати</Button>
      <Button onclick={createRole} disabled={saving || !newName.trim()}>
        {saving ? 'Створення…' : 'Створити'}
      </Button>
    </DialogFooter>
  </DialogContent>
</Dialog>

<!-- Delete dialog -->
<Dialog bind:open={deleteOpen}>
  <DialogContent class="max-w-sm">
    <DialogHeader>
      <DialogTitle>Видалити роль?</DialogTitle>
      <DialogDescription>
        Роль <span class="font-mono">{deleteTarget?.name}</span> буде назавжди видалено.
        Користувачі з цією роллю потребуватимуть переназначення.
      </DialogDescription>
    </DialogHeader>
    <DialogFooter>
      <Button variant="outline" onclick={() => (deleteOpen = false)}>Скасувати</Button>
      <Button variant="destructive" onclick={handleDelete}>Видалити</Button>
    </DialogFooter>
  </DialogContent>
</Dialog>
