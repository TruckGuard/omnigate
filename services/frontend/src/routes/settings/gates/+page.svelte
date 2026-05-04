<script lang="ts">
  import { goto } from '$app/navigation';
  import { toast } from 'svelte-sonner';
  import TopBar from '$lib/components/TopBar.svelte';
  import GateBadge from '$lib/components/GateBadge.svelte';
  import Field from '$lib/components/Field.svelte';
  import { Button } from '$lib/components/ui/button/index.js';
  import { Badge } from '$lib/components/ui/badge/index.js';
  import { Input } from '$lib/components/ui/input/index.js';
  import { Textarea } from '$lib/components/ui/textarea/index.js';
  import { Switch } from '$lib/components/ui/switch/index.js';
  import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '$lib/components/ui/table/index.js';
  import {
    Dialog, DialogContent, DialogHeader, DialogTitle,
    DialogFooter, DialogDescription,
  } from '$lib/components/ui/dialog/index.js';
  import { api } from '$lib/api.js';
  import type { Gate } from '$lib/types.js';
  import { Plus, Pencil, Trash2 } from 'lucide-svelte';

  let gates   = $state<Gate[]>([]);
  let loading = $state(true);
  let saving  = $state(false);

  let editOpen   = $state(false);
  let deleteOpen = $state(false);
  let isNew      = $state(false);
  let selected   = $state<Gate | null>(null);

  // Form
  let fGateId      = $state('');
  let fName        = $state('');
  let fLocation    = $state('');
  let fDescription = $state('');
  let fActive      = $state(true);

  async function load() {
    try { gates = await api.gates.list(); }
    catch { toast.error('Failed to load gates'); }
    finally { loading = false; }
  }

  $effect(() => { load(); });

  function openCreate() {
    isNew = true; selected = null;
    fGateId = ''; fName = ''; fLocation = ''; fDescription = ''; fActive = true;
    editOpen = true;
  }

  function openEdit(g: Gate) {
    isNew = false; selected = g;
    fGateId = g.gate_id; fName = g.name; fLocation = g.location;
    fDescription = g.description; fActive = g.status === 'active';
    editOpen = true;
  }

  async function handleSave() {
    if (!fGateId || !fName) { toast.error('Gate ID and name are required'); return; }
    saving = true;
    try {
      if (isNew) {
        await api.gates.create({ gate_id: fGateId, name: fName, location: fLocation, description: fDescription });
        toast.success('Gate created');
      } else if (selected) {
        await api.gates.update(selected.id, {
          name: fName, location: fLocation,
          description: fDescription, status: fActive ? 'active' : 'inactive',
        });
        toast.success('Gate saved');
      }
      editOpen = false;
      await load();
    } catch {
      toast.error('Save failed');
    } finally {
      saving = false;
    }
  }

  async function handleDelete() {
    if (!selected) return;
    try {
      await api.gates.delete(selected.id);
      toast.success('Gate deleted');
      deleteOpen = false;
      await load();
    } catch {
      toast.error('Failed to delete gate');
    }
  }
</script>

<TopBar crumbs={['OmniGate', 'Gates']} title="Gates">
  {#snippet actions()}
    <Button size="sm" onclick={openCreate}>
      <Plus size={14} /> New gate
    </Button>
  {/snippet}
</TopBar>

<main class="flex-1 p-6">
  <div class="rounded-md border border-border overflow-hidden">
    <Table>
      <TableHeader>
        <TableRow>
          <TableHead class="w-[160px]">Gate ID</TableHead>
          <TableHead>Name</TableHead>
          <TableHead>Location</TableHead>
          <TableHead class="w-[90px]">Status</TableHead>
          <TableHead class="w-[80px]"></TableHead>
        </TableRow>
      </TableHeader>
      <TableBody>
        {#each gates as g (g.id)}
          <TableRow class="cursor-pointer" onclick={() => goto(`/settings/gates/${g.id}`)}>
            <TableCell><GateBadge gateId={g.gate_id} /></TableCell>
            <TableCell class="font-medium">{g.name}</TableCell>
            <TableCell class="text-[12px] text-muted-foreground">{g.location || '—'}</TableCell>
            <TableCell>
              <Badge variant={g.status === 'active' ? 'default' : 'secondary'}>
                {g.status === 'active' ? 'Active' : 'Inactive'}
              </Badge>
            </TableCell>
            <TableCell>
              <div role="presentation" class="flex gap-1" onclick={(e) => e.stopPropagation()} onkeydown={(e) => e.stopPropagation()}>
                <Button variant="ghost" size="icon-sm" onclick={() => openEdit(g)}>
                  <Pencil size={13} />
                </Button>
                <Button variant="ghost" size="icon-sm" class="hover:text-destructive"
                  onclick={() => { selected = g; deleteOpen = true; }}>
                  <Trash2 size={13} />
                </Button>
              </div>
            </TableCell>
          </TableRow>
        {/each}
        {#if !loading && gates.length === 0}
          <TableRow>
            <TableCell colspan={5} class="py-10 text-center text-muted-foreground">No gates configured.</TableCell>
          </TableRow>
        {/if}
      </TableBody>
    </Table>
  </div>
</main>

<!-- Edit / Create dialog -->
<Dialog bind:open={editOpen}>
  <DialogContent class="max-w-md">
    <DialogHeader>
      <DialogTitle>{isNew ? 'New gate' : `Edit gate — ${selected?.gate_id}`}</DialogTitle>
    </DialogHeader>
    <div class="space-y-4 py-2">
      <div class="grid grid-cols-2 gap-4">
        <Field label="Gate ID" hint="Short unique identifier, e.g. gate-north">
          <Input bind:value={fGateId} placeholder="gate-north" disabled={!isNew} class="font-mono" />
        </Field>
        <Field label="Name">
          <Input bind:value={fName} placeholder="North Gate" />
        </Field>
      </div>
      <Field label="Location">
        <Input bind:value={fLocation} placeholder="Building A, Entrance 1" />
      </Field>
      <Field label="Description">
        <Textarea bind:value={fDescription} rows={2} placeholder="Optional notes…" />
      </Field>
      {#if !isNew}
        <div class="flex items-center justify-between">
          <span class="text-[13px] font-medium">Active</span>
          <Switch bind:checked={fActive} />
        </div>
      {/if}
    </div>
    <DialogFooter>
      <Button variant="outline" onclick={() => (editOpen = false)}>Cancel</Button>
      <Button onclick={handleSave} disabled={saving || !fGateId || !fName}>
        {saving ? 'Saving…' : isNew ? 'Create gate' : 'Save'}
      </Button>
    </DialogFooter>
  </DialogContent>
</Dialog>

<!-- Delete dialog -->
<Dialog bind:open={deleteOpen}>
  <DialogContent class="max-w-sm">
    <DialogHeader>
      <DialogTitle>Delete gate?</DialogTitle>
      <DialogDescription>
        Gate <span class="font-mono">{selected?.gate_id}</span> will be permanently removed.
        Any devices or transactions referencing it will retain the gate ID string.
      </DialogDescription>
    </DialogHeader>
    <DialogFooter>
      <Button variant="outline" onclick={() => (deleteOpen = false)}>Cancel</Button>
      <Button variant="destructive" onclick={handleDelete}>Delete</Button>
    </DialogFooter>
  </DialogContent>
</Dialog>
