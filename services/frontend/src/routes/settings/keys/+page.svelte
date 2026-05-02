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
  import { Plus, Trash2, KeyRound, ShieldCheck } from 'lucide-svelte';

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
      toast.error('Failed to load keys');
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
      toast.error('Failed to create key');
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
      toast.success('Key updated');
      editOpen = false;
      await load();
    } catch {
      toast.error('Failed to update key');
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
      toast.success('Permissions updated');
      permsOpen = false;
      await load();
    } catch {
      toast.error('Failed to update permissions');
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
      toast.success('Key deleted');
      deleteOpen = false;
      await load();
    } catch {
      toast.error('Failed to delete key');
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

  // Group permissions by module
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

<TopBar crumbs={['OmniGate', 'API Keys']} title="API Keys">
  {#snippet actions()}
    <Button size="sm" onclick={openCreate}>
      <Plus size={14} /> New key
    </Button>
  {/snippet}
</TopBar>

<main class="flex-1 p-6">
  <div class="rounded-md border border-border overflow-hidden">
    <Table>
      <TableHeader>
        <TableRow>
          <TableHead class="w-[50px]">ID</TableHead>
          <TableHead>Owner name</TableHead>
          <TableHead class="w-[160px]">Gate</TableHead>
          <TableHead class="w-[80px]">Status</TableHead>
          <TableHead class="w-[100px]">Permissions</TableHead>
          <TableHead class="w-[110px]">Created</TableHead>
          <TableHead class="w-[100px]"></TableHead>
        </TableRow>
      </TableHeader>
      <TableBody>
        {#each keys as k (k.id)}
          <TableRow>
            <TableCell class="font-mono text-[12px] text-muted-foreground">#{k.id}</TableCell>
            <TableCell class="font-medium">{k.owner_name}</TableCell>
            <TableCell>
              {#if k.gate_id}
                {@const g = gates.find(x => x.gate_id === k.gate_id)}
                <GateBadge gateId={k.gate_id} name={g?.name ?? ''} href="/settings/gates" />
              {:else}
                <span class="text-muted-foreground text-[12px]">—</span>
              {/if}
            </TableCell>
            <TableCell>
              <Badge variant={k.is_active ? 'default' : 'secondary'}>
                {k.is_active ? 'Active' : 'Inactive'}
              </Badge>
            </TableCell>
            <TableCell class="text-[12px] text-muted-foreground">
              {k.permissions.length} permission{k.permissions.length === 1 ? '' : 's'}
            </TableCell>
            <TableCell class="text-[12px] text-muted-foreground">{fmtDate(k.created_at)}</TableCell>
            <TableCell>
              <div class="flex gap-1">
                <Button variant="ghost" size="icon-sm" title="Permissions" onclick={() => openPerms(k)}>
                  <ShieldCheck size={14} />
                </Button>
                <Button variant="ghost" size="icon-sm" title="Edit" onclick={() => openEdit(k)}>
                  <KeyRound size={14} />
                </Button>
                <Button variant="ghost" size="icon-sm" title="Delete" class="hover:text-destructive" onclick={() => openDelete(k)}>
                  <Trash2 size={14} />
                </Button>
              </div>
            </TableCell>
          </TableRow>
        {/each}
        {#if !loading && keys.length === 0}
          <TableRow>
            <TableCell colspan={7} class="py-10 text-center text-muted-foreground">No API keys yet.</TableCell>
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
      <DialogTitle>New API key</DialogTitle>
      <DialogDescription>The raw key is shown only once after creation.</DialogDescription>
    </DialogHeader>
    <div class="space-y-4 py-2">
      <Field label="Owner / device name">
        <Input bind:value={newName} placeholder="cam-north-01" />
      </Field>
      <Field label="Gate ID">
        <Input bind:value={newGateId} placeholder="gate-north (optional)" />
      </Field>
      <div>
        <p class="text-[12px] font-medium mb-2">Permissions</p>
        {#each [...permsByModule()] as [module, perms]}
          <p class="text-[11px] uppercase tracking-wide text-muted-foreground mt-2 mb-1">{module}</p>
          {#each perms as p}
            <label class="flex items-center gap-2 py-0.5 cursor-pointer">
              <input type="checkbox" checked={newPermIds.includes(p.id)} onchange={() => toggleNewPerm(p.id)} />
              <span class="text-[13px]">{p.name}</span>
              <span class="text-[11px] text-muted-foreground">{p.description}</span>
            </label>
          {/each}
        {/each}
      </div>
    </div>
    <DialogFooter>
      <Button variant="outline" onclick={() => (createOpen = false)}>Cancel</Button>
      <Button onclick={handleCreate} disabled={saving || !newName}>
        {saving ? 'Creating…' : 'Create key'}
      </Button>
    </DialogFooter>
  </DialogContent>
</Dialog>

<!-- Reveal key dialog -->
<Dialog bind:open={revealOpen}>
  <DialogContent class="max-w-md">
    <DialogHeader>
      <DialogTitle>API key created</DialogTitle>
      <DialogDescription>Copy this key now — it will not be shown again.</DialogDescription>
    </DialogHeader>
    <div class="rounded-md bg-muted p-3 font-mono text-[13px] break-all select-all">{newKeyValue}</div>
    <DialogFooter>
      <Button onclick={() => { navigator.clipboard.writeText(newKeyValue); toast.success('Copied'); }}>
        Copy
      </Button>
      <Button variant="outline" onclick={() => (revealOpen = false)}>Close</Button>
    </DialogFooter>
  </DialogContent>
</Dialog>

<!-- Edit dialog -->
<Dialog bind:open={editOpen}>
  <DialogContent class="max-w-sm">
    <DialogHeader><DialogTitle>Edit key #{selected?.id}</DialogTitle></DialogHeader>
    <div class="space-y-4 py-2">
      <Field label="Owner name">
        <Input bind:value={editName} />
      </Field>
      <Field label="Gate">
        <Select type="single" bind:value={editGateId}>
          <SelectTrigger>
            {gates.find(g => g.gate_id === editGateId)?.name ?? (editGateId || 'None')}
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="">None</SelectItem>
            {#each gates as g}
              <SelectItem value={g.gate_id}>{g.name} ({g.gate_id})</SelectItem>
            {/each}
          </SelectContent>
        </Select>
      </Field>
      <div class="flex items-center justify-between">
        <span class="text-[13px] font-medium">Active</span>
        <Switch bind:checked={editActive} />
      </div>
    </div>
    <DialogFooter>
      <Button variant="outline" onclick={() => (editOpen = false)}>Cancel</Button>
      <Button onclick={handleEdit} disabled={saving}>
        {saving ? 'Saving…' : 'Save'}
      </Button>
    </DialogFooter>
  </DialogContent>
</Dialog>

<!-- Permissions dialog -->
<Dialog bind:open={permsOpen}>
  <DialogContent class="max-w-md">
    <DialogHeader>
      <DialogTitle>Permissions — {selected?.owner_name}</DialogTitle>
    </DialogHeader>
    <div class="space-y-1 max-h-[400px] overflow-y-auto py-2">
      {#each [...permsByModule()] as [module, perms]}
        <p class="text-[11px] uppercase tracking-wide text-muted-foreground mt-3 mb-1">{module}</p>
        {#each perms as p}
          <label class="flex items-center gap-2 py-0.5 cursor-pointer">
            <input type="checkbox" checked={editPermIds.includes(p.id)} onchange={() => togglePerm(p.id)} />
            <span class="text-[13px]">{p.name}</span>
            <span class="text-[11px] text-muted-foreground ml-auto">{p.id}</span>
          </label>
        {/each}
      {/each}
    </div>
    <DialogFooter>
      <Button variant="outline" onclick={() => (permsOpen = false)}>Cancel</Button>
      <Button onclick={handlePerms} disabled={saving}>
        {saving ? 'Saving…' : 'Update permissions'}
      </Button>
    </DialogFooter>
  </DialogContent>
</Dialog>

<!-- Delete dialog -->
<Dialog bind:open={deleteOpen}>
  <DialogContent class="max-w-sm">
    <DialogHeader>
      <DialogTitle>Delete key #{selected?.id}?</DialogTitle>
      <DialogDescription>
        Key for <span class="font-medium">{selected?.owner_name}</span> will be permanently revoked. Any device using it will lose access.
      </DialogDescription>
    </DialogHeader>
    <DialogFooter>
      <Button variant="outline" onclick={() => (deleteOpen = false)}>Cancel</Button>
      <Button variant="destructive" onclick={handleDelete}>Delete</Button>
    </DialogFooter>
  </DialogContent>
</Dialog>
