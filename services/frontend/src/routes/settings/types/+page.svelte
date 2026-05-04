<script lang="ts">
  import { goto } from '$app/navigation';
  import { toast } from 'svelte-sonner';
  import TopBar from '$lib/components/TopBar.svelte';
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
  import {
    Select, SelectContent, SelectItem, SelectTrigger,
  } from '$lib/components/ui/select/index.js';
  import { api } from '$lib/api.js';
  import { fmtDate } from '$lib/utils.js';
  import type { EventType, EventTypeField } from '$lib/types.js';
  import { Plus, Trash2, ChevronDown, ChevronRight, Pencil } from 'lucide-svelte';

  const FIELD_TYPES = ['string', 'number', 'boolean', 'datetime', 'image_url'] as const;

  let types   = $state<EventType[]>([]);
  let loading = $state(true);
  let createOpen  = $state(false);
  let editOpen    = $state(false);
  let detailId    = $state<string | null>(null);
  let saving      = $state(false);

  // Create form
  let newCode        = $state('');
  let newName        = $state('');
  let newDescription = $state('');
  let newFields      = $state<Array<{ key: string; name: string; description: string; type: string; required: boolean }>>([]);

  // Edit form
  let editType        = $state<EventType | null>(null);
  let editName        = $state('');
  let editDescription = $state('');
  let editFields      = $state<Array<{ key: string; name: string; description: string; type: string; required: boolean }>>([]);

  async function load() {
    try { types = await api.types.list(); }
    catch { toast.error('Failed to load event types'); }
    finally { loading = false; }
  }

  $effect(() => { load(); });

  function openCreate() {
    newCode = ''; newName = ''; newDescription = ''; newFields = [];
    createOpen = true;
  }

  function addField() {
    newFields = [...newFields, { key: '', name: '', description: '', type: 'string', required: false }];
  }

  function removeField(i: number) {
    newFields = newFields.filter((_, idx) => idx !== i);
  }

  async function handleCreate() {
    if (!newCode || !newName) { toast.error('Code and name are required'); return; }
    saving = true;
    try {
      const fields: Record<string, unknown> = {};
      for (const f of newFields) {
        if (f.key) fields[f.key] = { name: f.name, description: f.description, type: f.type, required: f.required };
      }
      await api.types.create({ code: newCode.toUpperCase(), name: newName, description: newDescription, fields });
      toast.success('Event type created');
      createOpen = false;
      await load();
    } catch {
      toast.error('Failed to create event type');
    } finally {
      saving = false;
    }
  }

  function openEdit(t: EventType) {
    editType = t;
    editName = t.name;
    editDescription = t.description;
    editFields = Object.entries(t.fields).map(([key, f]) => ({
      key, name: f.name, description: f.description, type: f.type, required: f.required,
    }));
    editOpen = true;
  }

  function addEditField() {
    editFields = [...editFields, { key: '', name: '', description: '', type: 'string', required: false }];
  }

  function removeEditField(i: number) {
    editFields = editFields.filter((_, idx) => idx !== i);
  }

  async function handleEdit() {
    if (!editType || !editName) { toast.error('Name is required'); return; }
    saving = true;
    try {
      const fields: Record<string, unknown> = {};
      for (const f of editFields) {
        if (f.key) fields[f.key] = { name: f.name, description: f.description, type: f.type, required: f.required };
      }
      await api.types.update(editType.id, { name: editName, description: editDescription, fields });
      toast.success('Event type updated');
      editOpen = false;
      await load();
    } catch {
      toast.error('Failed to update event type');
    } finally {
      saving = false;
    }
  }

  const detailType = $derived(types.find(t => t.id === detailId));
</script>

<TopBar crumbs={['OmniGate', 'Event Types']} title="Event Types">
  {#snippet actions()}
    <Button size="sm" onclick={openCreate}>
      <Plus size={14} /> New type
    </Button>
  {/snippet}
</TopBar>

<main class="flex-1 p-6">
  <div class="rounded-md border border-border overflow-hidden">
    <Table>
      <TableHeader>
        <TableRow>
          <TableHead class="w-[32px]"></TableHead>
          <TableHead class="w-[120px]">Code</TableHead>
          <TableHead>Name</TableHead>
          <TableHead>Description</TableHead>
          <TableHead class="w-[80px]">Fields</TableHead>
          <TableHead class="w-[110px]">Created</TableHead>
          <TableHead class="w-[50px]"></TableHead>
        </TableRow>
      </TableHeader>
      <TableBody>
        {#each types as t (t.id)}
          <TableRow
            class="cursor-pointer"
            onclick={() => detailId = detailId === t.id ? null : t.id}
          >
            <TableCell class="text-muted-foreground">
              {#if detailId === t.id}
                <ChevronDown size={14} />
              {:else}
                <ChevronRight size={14} />
              {/if}
            </TableCell>
            <TableCell><Badge variant="outline" class="font-mono">{t.code}</Badge></TableCell>
            <TableCell class="font-medium">{t.name}</TableCell>
            <TableCell class="text-muted-foreground text-[12px]">{t.description}</TableCell>
            <TableCell class="text-[12px] text-muted-foreground">{Object.keys(t.fields).length}</TableCell>
            <TableCell class="text-[12px] text-muted-foreground">{fmtDate(t.created_at)}</TableCell>
            <TableCell>
              <div role="presentation" onclick={(e) => e.stopPropagation()} onkeydown={(e) => e.stopPropagation()}>
                <Button variant="ghost" size="icon-sm" onclick={() => openEdit(t)}>
                  <Pencil size={13} />
                </Button>
              </div>
            </TableCell>
          </TableRow>
          {#if detailId === t.id}
            <TableRow class="bg-muted/30 hover:bg-muted/30">
              <TableCell colspan={7} class="p-0">
                <div class="px-6 py-3">
                  <p class="text-[11px] font-semibold uppercase tracking-wide text-muted-foreground mb-2">Field schema</p>
                  <div class="space-y-1">
                    {#each Object.entries(t.fields) as [key, field]}
                      <div class="flex items-baseline gap-3 text-[12px]">
                        <span class="font-mono w-[200px] shrink-0">{key}</span>
                        <Badge variant="outline" class="text-[10px] shrink-0">{field.type}</Badge>
                        {#if field.required}<Badge class="text-[10px] shrink-0">required</Badge>{/if}
                        <span class="text-muted-foreground">{field.name}{field.description ? ` — ${field.description}` : ''}</span>
                      </div>
                    {/each}
                  </div>
                </div>
              </TableCell>
            </TableRow>
          {/if}
        {/each}
        {#if !loading && types.length === 0}
          <TableRow>
            <TableCell colspan={7} class="py-10 text-center text-muted-foreground">No event types defined yet.</TableCell>
          </TableRow>
        {/if}
      </TableBody>
    </Table>
  </div>
</main>

<!-- Create dialog -->
<Dialog bind:open={createOpen}>
  <DialogContent class="max-w-2xl">
    <DialogHeader>
      <DialogTitle>New event type</DialogTitle>
      <DialogDescription>Define the schema for a new type of IoT event.</DialogDescription>
    </DialogHeader>
    <div class="space-y-4 py-2 max-h-[60vh] overflow-y-auto pr-1">
      <div class="grid grid-cols-2 gap-4">
        <Field label="Code" hint="Short uppercase identifier, e.g. ANPR">
          <Input bind:value={newCode} placeholder="ANPR" oninput={() => newCode = newCode.toUpperCase()} />
        </Field>
        <Field label="Name">
          <Input bind:value={newName} placeholder="License Plate Read" />
        </Field>
      </div>
      <Field label="Description">
        <Textarea bind:value={newDescription} rows={2} placeholder="Describe what this event type captures…" />
      </Field>

      <div>
        <div class="flex items-center justify-between mb-2">
          <p class="text-[12px] font-medium">Fields</p>
          <Button variant="outline" size="sm" onclick={addField}>
            <Plus size={12} /> Add field
          </Button>
        </div>
        {#if newFields.length === 0}
          <p class="text-[12px] text-muted-foreground">No fields yet.</p>
        {/if}
        {#each newFields as f, i}
          <div class="rounded-md border border-border p-3 space-y-2 mb-2">
            <div class="grid grid-cols-[1fr_1fr_auto] gap-2 items-end">
              <Field label="Key (JSON field name)">
                <Input bind:value={f.key} placeholder="plate_number" class="font-mono text-[12px]" />
              </Field>
              <Field label="Display name">
                <Input bind:value={f.name} placeholder="Plate Number" />
              </Field>
              <Button variant="ghost" size="icon-sm" class="mb-0.5 hover:text-destructive" onclick={() => removeField(i)}>
                <Trash2 size={13} />
              </Button>
            </div>
            <div class="grid grid-cols-[1fr_120px_auto] gap-2 items-end">
              <Field label="Description">
                <Input bind:value={f.description} placeholder="Vehicle license plate" />
              </Field>
              <Field label="Type">
                <Select type="single" bind:value={f.type}>
                  <SelectTrigger>{f.type}</SelectTrigger>
                  <SelectContent>
                    {#each FIELD_TYPES as ft}
                      <SelectItem value={ft}>{ft}</SelectItem>
                    {/each}
                  </SelectContent>
                </Select>
              </Field>
              <div class="flex items-center gap-2 pb-1">
                <span class="text-[12px]">Required</span>
                <Switch bind:checked={f.required} />
              </div>
            </div>
          </div>
        {/each}
      </div>
    </div>
    <DialogFooter>
      <Button variant="outline" onclick={() => (createOpen = false)}>Cancel</Button>
      <Button onclick={handleCreate} disabled={saving || !newCode || !newName}>
        {saving ? 'Creating…' : 'Create type'}
      </Button>
    </DialogFooter>
  </DialogContent>
</Dialog>

<!-- Edit dialog -->
<Dialog bind:open={editOpen}>
  <DialogContent class="max-w-2xl">
    <DialogHeader>
      <DialogTitle>Edit event type — <span class="font-mono font-normal">{editType?.code}</span></DialogTitle>
      <DialogDescription>Update the name, description, or field schema.</DialogDescription>
    </DialogHeader>
    <div class="space-y-4 py-2 max-h-[60vh] overflow-y-auto pr-1">
      <div class="grid grid-cols-2 gap-4">
        <Field label="Name">
          <Input bind:value={editName} placeholder="License Plate Read" />
        </Field>
        <Field label="Description">
          <Input bind:value={editDescription} placeholder="What this type captures…" />
        </Field>
      </div>

      <div>
        <div class="flex items-center justify-between mb-2">
          <p class="text-[12px] font-medium">Fields</p>
          <Button variant="outline" size="sm" onclick={addEditField}>
            <Plus size={12} /> Add field
          </Button>
        </div>
        {#if editFields.length === 0}
          <p class="text-[12px] text-muted-foreground">No fields yet.</p>
        {/if}
        {#each editFields as f, i}
          <div class="rounded-md border border-border p-3 space-y-2 mb-2">
            <div class="grid grid-cols-[1fr_1fr_auto] gap-2 items-end">
              <Field label="Key (JSON field name)">
                <Input bind:value={f.key} placeholder="plate_number" class="font-mono text-[12px]" />
              </Field>
              <Field label="Display name">
                <Input bind:value={f.name} placeholder="Plate Number" />
              </Field>
              <Button variant="ghost" size="icon-sm" class="mb-0.5 hover:text-destructive" onclick={() => removeEditField(i)}>
                <Trash2 size={13} />
              </Button>
            </div>
            <div class="grid grid-cols-[1fr_120px_auto] gap-2 items-end">
              <Field label="Description">
                <Input bind:value={f.description} placeholder="Vehicle license plate" />
              </Field>
              <Field label="Type">
                <Select type="single" bind:value={f.type}>
                  <SelectTrigger>{f.type}</SelectTrigger>
                  <SelectContent>
                    {#each FIELD_TYPES as ft}
                      <SelectItem value={ft}>{ft}</SelectItem>
                    {/each}
                  </SelectContent>
                </Select>
              </Field>
              <div class="flex items-center gap-2 pb-1">
                <span class="text-[12px]">Required</span>
                <Switch bind:checked={f.required} />
              </div>
            </div>
          </div>
        {/each}
      </div>
    </div>
    <DialogFooter>
      <Button variant="outline" onclick={() => (editOpen = false)}>Cancel</Button>
      <Button onclick={handleEdit} disabled={saving || !editName}>
        {saving ? 'Saving…' : 'Save changes'}
      </Button>
    </DialogFooter>
  </DialogContent>
</Dialog>
