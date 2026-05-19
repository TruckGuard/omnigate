<script lang="ts">
  import { toast } from 'svelte-sonner';
  import TopBar from '$lib/components/TopBar.svelte';
  import GateBadge from '$lib/components/GateBadge.svelte';
  import Field from '$lib/components/Field.svelte';
  import { Button } from '$lib/components/ui/button/index.js';
  import { Badge } from '$lib/components/ui/badge/index.js';
  import { Input } from '$lib/components/ui/input/index.js';
  import { Switch } from '$lib/components/ui/switch/index.js';
  import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '$lib/components/ui/table/index.js';
  import {
    Dialog, DialogContent, DialogHeader, DialogTitle,
    DialogFooter, DialogDescription,
  } from '$lib/components/ui/dialog/index.js';
  import { api } from '$lib/api.js';
  import { fmtDate } from '$lib/utils.js';
  import type { APIKey, Gate, Permission } from '$lib/types.js';
  import {
    Select, SelectContent, SelectItem, SelectTrigger,
  } from '$lib/components/ui/select/index.js';
  import { Plus, Trash2, KeyRound, ShieldCheck, Check } from 'lucide-svelte';
  import PermGuard from '$lib/components/PermGuard.svelte';

  let keys        = $state<APIKey[]>([]);
  let gates       = $state<Gate[]>([]);
  let allPerms    = $state<Permission[]>([]);
  let loading     = $state(true);

  let createOpen   = $state(false);
  let revealOpen   = $state(false);
  let editOpen     = $state(false);
  let permsOpen    = $state(false);
  let deleteOpen   = $state(false);

  let selected     = $state<APIKey | null>(null);
  let newKeyValue  = $state('');

  // Create form
  let newName      = $state('');
  let newGateId    = $state('');
  let newPermIds   = $state<string[]>([]);
  let saving       = $state(false);

  // Edit form
  let editName     = $state('');
  let editGateId   = $state('');
  let editActive   = $state(true);

  // Permissions form
  let editPermIds  = $state<string[]>([]);

  async function load() {
    try {
      [keys, gates, allPerms] = await Promise.all([
        api.auth.keys.list(),
        api.gates.list(),
        api.auth.permissions(),
      ]);
    } catch {
      toast.error('Помилка завантаження ключів');
    } finally {
      loading = false;
    }
  }

  $effect(() => { load(); });

  function openCreate() {
    newName = ''; newGateId = ''; newPermIds = [];
    createOpen = true;
  }

  async function handleCreate() {
    saving = true;
    try {
      const res = await api.auth.keys.create({ name: newName, gate_id: newGateId, permission_ids: newPermIds });
      newKeyValue = res.api_key;
      createOpen = false;
      revealOpen = true;
      await load();
    } catch {
      toast.error('Помилка створення ключа');
    } finally {
      saving = false;
    }
  }

  function openEdit(k: APIKey) {
    selected = k; editName = k.owner_name; editGateId = k.gate_id ?? ''; editActive = k.is_active;
    editOpen = true;
  }

  async function handleEdit() {
    if (!selected) return;
    saving = true;
    try {
      await api.auth.keys.update(selected.id, { owner_name: editName, gate_id: editGateId, is_active: editActive });
      toast.success('Ключ оновлено');
      editOpen = false;
      await load();
    } catch {
      toast.error('Помилка оновлення ключа');
    } finally {
      saving = false;
    }
  }

  function openPerms(k: APIKey) {
    selected = k;
    editPermIds = k.permissions.map(p => p.id);
    permsOpen = true;
  }

  async function handlePerms() {
    if (!selected) return;
    saving = true;
    try {
      await api.auth.keys.updatePermissions(selected.id, editPermIds);
      toast.success('Дозволи оновлено');
      permsOpen = false;
      await load();
    } catch {
      toast.error('Помилка оновлення дозволів');
    } finally {
      saving = false;
    }
  }

  function openDelete(k: APIKey) {
    selected = k; deleteOpen = true;
  }

  async function handleDelete() {
    if (!selected) return;
    try {
      await api.auth.keys.delete(selected.id);
      toast.success('Ключ видалено');
      deleteOpen = false;
      await load();
    } catch {
      toast.error('Помилка видалення ключа');
    }
  }

  function togglePerm(id: string) {
    if (editPermIds.includes(id)) {
      editPermIds = editPermIds.filter(p => p !== id);
    } else {
      editPermIds = [...editPermIds, id];
    }
  }

  function toggleNewPerm(id: string) {
    if (newPermIds.includes(id)) {
      newPermIds = newPermIds.filter(p => p !== id);
    } else {
      newPermIds = [...newPermIds, id];
    }
  }

  const permsByModule = $derived(() => {
    const map = new Map<string, Permission[]>();
    for (const p of allPerms) {
      const g = map.get(p.module) ?? [];
      g.push(p);
      map.set(p.module, g);
    }
    return map;
  });
</script>

<TopBar crumbs={['OmniGate', 'API ключі']} title="API ключі">
  {#snippet actions()}
    <PermGuard permission="manage:keys">
      <Button size="sm" onclick={openCreate}>
        <Plus size={14} /> Новий ключ
      </Button>
    </PermGuard>
  {/snippet}
</TopBar>

<main class="flex-1 p-6">
  <div class="rounded-md border border-border overflow-hidden">
    <Table>
      <TableHeader>
        <TableRow>
          <TableHead class="w-[50px]">ID</TableHead>
          <TableHead>Власник</TableHead>
          <TableHead class="w-[160px]">Шлагбаум</TableHead>
          <TableHead class="w-[80px]">Статус</TableHead>
          <TableHead class="w-[100px]">Дозволи</TableHead>
          <TableHead class="w-[110px]">Створено</TableHead>
          <TableHead class="w-[100px]"></TableHead>
        </TableRow>
      </TableHeader>
      <TableBody>
        {#each keys as k (k.id)}
          <TableRow>
            <TableCell class="font-mono text-xs text-muted-foreground">#{k.id}</TableCell>
            <TableCell class="font-medium">{k.owner_name}</TableCell>
            <TableCell>
              {#if k.gate_id}
                {@const g = gates.find(x => x.gate_id === k.gate_id)}
                <GateBadge gateId={k.gate_id} name={g?.name ?? ''} href="/settings/gates" />
              {:else}
                <span class="text-muted-foreground text-xs">—</span>
              {/if}
            </TableCell>
            <TableCell>
              <Badge variant={k.is_active ? 'default' : 'secondary'}>
                {k.is_active ? 'Активний' : 'Неактивний'}
              </Badge>
            </TableCell>
            <TableCell class="text-xs text-muted-foreground">
              {k.permissions.length} {k.permissions.length === 1 ? 'дозвіл' : 'дозволів'}
            </TableCell>
            <TableCell class="text-xs text-muted-foreground">{fmtDate(k.created_at)}</TableCell>
            <TableCell>
              <PermGuard permission="manage:keys">
                <div class="flex gap-1">
                  <Button variant="ghost" size="icon-sm" title="Дозволи" onclick={() => openPerms(k)}>
                    <ShieldCheck size={14} />
                  </Button>
                  <Button variant="ghost" size="icon-sm" title="Редагувати" onclick={() => openEdit(k)}>
                    <KeyRound size={14} />
                  </Button>
                  <Button variant="ghost" size="icon-sm" title="Видалити" class="hover:text-destructive" onclick={() => openDelete(k)}>
                    <Trash2 size={14} />
                  </Button>
                </div>
              </PermGuard>
            </TableCell>
          </TableRow>
        {/each}
        {#if !loading && keys.length === 0}
          <TableRow>
            <TableCell colspan={7} class="py-10 text-center text-muted-foreground">API ключів ще немає.</TableCell>
          </TableRow>
        {/if}
      </TableBody>
    </Table>
  </div>
</main>

<!-- Create dialog -->
<Dialog bind:open={createOpen}>
  <DialogContent class="max-w-md">
    <DialogHeader>
      <DialogTitle>Новий API ключ</DialogTitle>
      <DialogDescription>Ключ буде показано лише один раз після створення.</DialogDescription>
    </DialogHeader>
    <div class="space-y-4 py-2">
      <Field label="Ім'я власника / пристрою">
        <Input bind:value={newName} placeholder="cam-north-01" />
      </Field>
      <Field label="ID шлагбауму">
        <Input bind:value={newGateId} placeholder="gate-north (необов'язково)" />
      </Field>
      <div>
        <p class="text-xs font-medium mb-2">Дозволи</p>
        <div class="space-y-1 max-h-[220px] overflow-y-auto">
          {#each [...permsByModule()] as [module, perms]}
            <p class="text-[11px] uppercase tracking-wide text-muted-foreground mt-3 mb-1">{module}</p>
            {#each perms as p}
              {@const active = newPermIds.includes(p.id)}
              <button
                type="button"
                onclick={() => toggleNewPerm(p.id)}
                class="w-full flex items-center justify-between px-3 py-2 rounded-md border text-sm transition-colors
                  {active ? 'bg-primary/10 border-primary/30 text-primary' : 'bg-background border-border text-muted-foreground hover:bg-muted'}"
              >
                <div class="text-left">
                  <span class="font-mono font-medium text-xs">{p.id}</span>
                  {#if p.description}
                    <span class="block text-[11px] mt-0.5 opacity-70">{p.description}</span>
                  {/if}
                </div>
                {#if active}<Check size={14} class="shrink-0" />{/if}
              </button>
            {/each}
          {/each}
        </div>
      </div>
    </div>
    <DialogFooter>
      <Button variant="outline" onclick={() => (createOpen = false)}>Скасувати</Button>
      <PermGuard permission="manage:keys">
        <Button onclick={handleCreate} disabled={saving || !newName}>
          {saving ? 'Створення…' : 'Створити ключ'}
        </Button>
      </PermGuard>
    </DialogFooter>
  </DialogContent>
</Dialog>

<!-- Reveal key dialog -->
<Dialog bind:open={revealOpen}>
  <DialogContent class="max-w-md">
    <DialogHeader>
      <DialogTitle>API ключ створено</DialogTitle>
      <DialogDescription>Скопіюйте ключ зараз — він більше не буде показаний.</DialogDescription>
    </DialogHeader>
    <div class="rounded-md bg-muted p-3 font-mono text-sm break-all select-all">{newKeyValue}</div>
    <DialogFooter>
      <Button onclick={() => { navigator.clipboard.writeText(newKeyValue); toast.success('Скопійовано'); }}>
        Копіювати
      </Button>
      <Button variant="outline" onclick={() => (revealOpen = false)}>Закрити</Button>
    </DialogFooter>
  </DialogContent>
</Dialog>

<!-- Edit dialog -->
<Dialog bind:open={editOpen}>
  <DialogContent class="max-w-sm">
    <DialogHeader><DialogTitle>Редагувати ключ #{selected?.id}</DialogTitle></DialogHeader>
    <div class="space-y-4 py-2">
      <Field label="Ім'я власника">
        <Input bind:value={editName} />
      </Field>
      <Field label="Шлагбаум">
        <Select type="single" bind:value={editGateId}>
          <SelectTrigger>
            {gates.find(g => g.gate_id === editGateId)?.name ?? (editGateId || 'Не обрано')}
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="">Не обрано</SelectItem>
            {#each gates as g}
              <SelectItem value={g.gate_id}>{g.name} ({g.gate_id})</SelectItem>
            {/each}
          </SelectContent>
        </Select>
      </Field>
      <div class="flex items-center justify-between">
        <span class="text-sm font-medium">Активний</span>
        <Switch bind:checked={editActive} />
      </div>
    </div>
    <DialogFooter>
      <Button variant="outline" onclick={() => (editOpen = false)}>Скасувати</Button>
      <PermGuard permission="manage:keys">
        <Button onclick={handleEdit} disabled={saving}>
          {saving ? 'Збереження…' : 'Зберегти'}
        </Button>
      </PermGuard>
    </DialogFooter>
  </DialogContent>
</Dialog>

<!-- Permissions dialog -->
<Dialog bind:open={permsOpen}>
  <DialogContent class="max-w-md">
    <DialogHeader>
      <DialogTitle>Дозволи — {selected?.owner_name}</DialogTitle>
    </DialogHeader>
    <div class="space-y-1 max-h-[400px] overflow-y-auto py-2">
      {#each [...permsByModule()] as [module, perms]}
        <p class="text-[11px] uppercase tracking-wide text-muted-foreground mt-3 mb-1">{module}</p>
        {#each perms as p}
          {@const active = editPermIds.includes(p.id)}
          <button
            type="button"
            onclick={() => togglePerm(p.id)}
            class="w-full flex items-center justify-between px-3 py-2 rounded-md border text-sm transition-colors
              {active ? 'bg-primary/10 border-primary/30 text-primary' : 'bg-background border-border text-muted-foreground hover:bg-muted'}"
          >
            <div class="text-left">
              <span class="font-mono font-medium text-xs">{p.id}</span>
              {#if p.description}
                <span class="block text-[11px] mt-0.5 opacity-70">{p.description}</span>
              {/if}
            </div>
            {#if active}<Check size={14} class="shrink-0" />{/if}
          </button>
        {/each}
      {/each}
    </div>
    <DialogFooter>
      <Button variant="outline" onclick={() => (permsOpen = false)}>Скасувати</Button>
      <PermGuard permission="manage:keys">
        <Button onclick={handlePerms} disabled={saving}>
          {saving ? 'Збереження…' : 'Оновити дозволи'}
        </Button>
      </PermGuard>
    </DialogFooter>
  </DialogContent>
</Dialog>

<!-- Delete dialog -->
<Dialog bind:open={deleteOpen}>
  <DialogContent class="max-w-sm">
    <DialogHeader>
      <DialogTitle>Видалити ключ #{selected?.id}?</DialogTitle>
      <DialogDescription>
        Ключ для <span class="font-medium">{selected?.owner_name}</span> буде назавжди відкликано. Пристрій, що використовує його, втратить доступ.
      </DialogDescription>
    </DialogHeader>
    <DialogFooter>
      <Button variant="outline" onclick={() => (deleteOpen = false)}>Скасувати</Button>
      <PermGuard permission="manage:keys">
        <Button variant="destructive" onclick={handleDelete}>Видалити</Button>
      </PermGuard>
    </DialogFooter>
  </DialogContent>
</Dialog>
