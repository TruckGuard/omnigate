<script lang="ts">
  import { page } from '$app/stores';
  import { get } from 'svelte/store';
  import { goto } from '$app/navigation';
  import { toast } from 'svelte-sonner';
  import TopBar from '$lib/components/TopBar.svelte';
  import Field from '$lib/components/Field.svelte';
  import PermGuard from '$lib/components/PermGuard.svelte';
  import { Button } from '$lib/components/ui/button/index.js';
  import { Input } from '$lib/components/ui/input/index.js';
  import { Textarea } from '$lib/components/ui/textarea/index.js';
  import { Switch } from '$lib/components/ui/switch/index.js';
  import { Badge } from '$lib/components/ui/badge/index.js';
  import { Card, CardContent, CardHeader, CardTitle } from '$lib/components/ui/card/index.js';
  import {
    Dialog, DialogContent, DialogHeader, DialogTitle,
    DialogFooter, DialogDescription,
  } from '$lib/components/ui/dialog/index.js';
  import {
    Select, SelectContent, SelectItem, SelectTrigger,
  } from '$lib/components/ui/select/index.js';
  import { api } from '$lib/api.js';
  import type { DeviceConfig, EventType, Gate, APIKey } from '$lib/types.js';
  import { ChevronLeft, Plus } from 'lucide-svelte';

  const deviceId = $derived($page.params.id ?? '');
  const isNew    = $derived(deviceId === 'new');

  let gates      = $state<Gate[]>([]);
  let eventTypes = $state<EventType[]>([]);
  let apiKeys    = $state<APIKey[]>([]);
  let loading    = $state(get(page).params.id !== 'new');
  let saving     = $state(false);
  let confirmDelete = $state(false);

  // Form state
  let sourceId       = $state('');
  let gateId         = $state('');
  let eventTypeId    = $state('');
  let dataType       = $state('');
  let mapping        = $state('{\n  "field": "$.path.to.value"\n}');
  let triggerEnabled = $state(false);
  let triggerUrl     = $state('');

  // New key inline form
  let creatingKey  = $state(false);
  let newKeyName   = $state('');
  let newKeyGateId = $state('');
  let savingKey    = $state(false);

  // New gate inline form (shared inputs, separate open flags per context)
  let creatingGate    = $state(false);
  let creatingKeyGate = $state(false);
  let newGateId       = $state('');
  let newGateName     = $state('');
  let savingGate      = $state(false);

  const selectedType = $derived(eventTypes.find(t => t.id === eventTypeId));

  // Inherit gate_id from the selected API key when on a new device
  $effect(() => {
    if (!isNew) return;
    const key = apiKeys.find(k => String(k.id) === sourceId);
    if (key?.gate_id) gateId = key.gate_id;
  });

  $effect(() => {
    Promise.all([api.gates.list(), api.types.list(), api.auth.keys.list()])
      .then(([g, t, k]) => { gates = g; eventTypes = t; apiKeys = k; })
      .catch(() => {});
  });

  $effect(() => {
    if (!isNew) {
      (async () => {
        try {
          const cfg = await api.configs.get(deviceId);
          sourceId       = cfg.source_id;
          gateId         = cfg.gate_id;
          eventTypeId    = cfg.event_type_id;
          dataType       = cfg.data_type;
          mapping        = JSON.stringify(cfg.data_mapping, null, 2);
          triggerEnabled = cfg.trigger_enabled;
          triggerUrl     = cfg.trigger_url ?? '';
        } catch {
          toast.error('Device not found');
          goto('/settings/devices');
        } finally {
          loading = false;
        }
      })();
    }
  });

  async function createNewKey() {
    if (!newKeyName) return;
    savingKey = true;
    try {
      const res = await api.auth.keys.create({
        name: newKeyName,
        gate_id: newKeyGateId || gateId,
        permission_ids: [],
      });
      const refreshed = await api.auth.keys.list();
      apiKeys = refreshed;
      sourceId = String(res.id);
      creatingKey = false;
      newKeyName = '';
      newKeyGateId = '';
      toast.success(`Key #${res.id} created — key shown once: ${res.api_key}`);
    } catch {
      toast.error('Failed to create key');
    } finally {
      savingKey = false;
    }
  }

  async function createNewGate(onCreated: (id: string) => void) {
    if (!newGateId || !newGateName) return;
    savingGate = true;
    try {
      await api.gates.create({ gate_id: newGateId, name: newGateName, location: '', description: '' });
      const refreshed = await api.gates.list();
      gates = refreshed;
      onCreated(newGateId);
      creatingGate = false;
      creatingKeyGate = false;
      newGateId = '';
      newGateName = '';
      toast.success('Gate created');
    } catch {
      toast.error('Failed to create gate');
    } finally {
      savingGate = false;
    }
  }

  async function handleSave() {
    let dataMapping: Record<string, string>;
    try { dataMapping = JSON.parse(mapping); }
    catch { toast.error('Invalid JSON in mapping'); return; }

    saving = true;
    try {
      if (isNew) {
        await api.configs.create({
          source_id: sourceId, gate_id: gateId,
          event_type_id: eventTypeId, data_type: dataType,
          data_mapping: dataMapping, trigger_enabled: triggerEnabled,
          trigger_url: triggerEnabled ? triggerUrl || null : null,
          trigger_source_id: null,
        });
        toast.success('Device created');
      } else {
        await api.configs.update(deviceId, {
          data_mapping: dataMapping, trigger_enabled: triggerEnabled,
          trigger_url: triggerEnabled ? triggerUrl || null : null,
        });
        toast.success('Device saved');
      }
      goto('/settings/devices');
    } catch {
      toast.error('Save failed');
    } finally {
      saving = false;
    }
  }

  async function handleDelete() {
    try {
      await api.configs.delete(deviceId);
      toast.success('Device deleted');
      goto('/settings/devices');
    } catch {
      toast.error('Delete failed');
    }
  }
</script>

<TopBar
  crumbs={['OmniGate', 'Devices', isNew ? 'New device' : sourceId]}
  title={isNew ? 'New device' : 'Edit device'}
>
  {#snippet actions()}
    {#if !isNew}
      <PermGuard permission="manage:keys">
        <Button variant="destructive" size="sm" onclick={() => (confirmDelete = true)}>Delete</Button>
      </PermGuard>
    {/if}
    <Button variant="outline" size="sm" onclick={() => goto('/settings/devices')}>Cancel</Button>
    <PermGuard permission="manage:keys">
      <Button size="sm" onclick={handleSave} disabled={saving}>
        {saving ? 'Saving…' : 'Save'}
      </Button>
    </PermGuard>
  {/snippet}
</TopBar>

{#if loading}
  <div class="flex-1 flex items-center justify-center text-muted-foreground">Loading…</div>
{:else}
  <main class="flex-1 p-6 max-w-[920px] space-y-5">
    <Button variant="ghost" size="sm" onclick={() => goto('/settings/devices')}>
      <ChevronLeft size={14} /> Devices
    </Button>

    <!-- Identity -->
    <Card>
      <CardHeader><CardTitle>Identity</CardTitle></CardHeader>
      <CardContent class="space-y-4">
        <div class="grid grid-cols-2 gap-4">
          <!-- Source ID = API Key -->
          <Field label="Source (API Key)" hint="The API key whose ID identifies this device.">
            {#if isNew}
              <div class="space-y-2">
                <Select type="single" bind:value={sourceId}>
                  <SelectTrigger>
                    {#if sourceId}
                      {apiKeys.find(k => String(k.id) === sourceId)?.owner_name ?? `Key #${sourceId}`}
                    {:else}
                      Select API key…
                    {/if}
                  </SelectTrigger>
                  <SelectContent>
                    {#each apiKeys.filter(k => k.is_active) as k}
                      <SelectItem value={String(k.id)}>
                        {k.owner_name} (#{k.id})
                      </SelectItem>
                    {/each}
                  </SelectContent>
                </Select>
                {#if !creatingKey}
                  <Button variant="outline" size="sm" onclick={() => (creatingKey = true)}>
                    <Plus size={12} /> Create new key
                  </Button>
                {:else}
                  <div class="rounded-md border border-border p-3 space-y-2 bg-muted/30">
                    <p class="text-[12px] font-medium">New API key</p>
                    <Input placeholder="Owner / device name" bind:value={newKeyName} />
                    <Select type="single" bind:value={newKeyGateId}>
                      <SelectTrigger>
                        {gates.find(g => g.gate_id === newKeyGateId)?.name ?? (newKeyGateId || 'Select gate (optional)…')}
                      </SelectTrigger>
                      <SelectContent>
                        <SelectItem value="">None</SelectItem>
                        {#each gates as g}
                          <SelectItem value={g.gate_id}>{g.name} ({g.gate_id})</SelectItem>
                        {/each}
                      </SelectContent>
                    </Select>
                    {#if !creatingKeyGate}
                      <Button variant="outline" size="sm" onclick={() => (creatingKeyGate = true)}>
                        <Plus size={12} /> Create new gate
                      </Button>
                    {:else}
                      <div class="rounded-md border border-border p-3 space-y-2 bg-background">
                        <p class="text-[12px] font-medium">New gate</p>
                        <Input placeholder="gate-north (ID)" bind:value={newGateId} class="font-mono" />
                        <Input placeholder="North Gate (display name)" bind:value={newGateName} />
                        <div class="flex gap-2">
                          <Button size="sm" onclick={() => createNewGate(id => newKeyGateId = id)} disabled={savingGate || !newGateId || !newGateName}>
                            {savingGate ? 'Creating…' : 'Create'}
                          </Button>
                          <Button variant="ghost" size="sm" onclick={() => (creatingKeyGate = false)}>Cancel</Button>
                        </div>
                      </div>
                    {/if}
                    <div class="flex gap-2 pt-1">
                      <Button size="sm" onclick={createNewKey} disabled={savingKey || !newKeyName}>
                        {savingKey ? 'Creating…' : 'Create key'}
                      </Button>
                      <Button variant="ghost" size="sm" onclick={() => { creatingKey = false; creatingKeyGate = false; }}>Cancel</Button>
                    </div>
                  </div>
                {/if}
              </div>
            {:else}
              <Input class="font-mono" disabled value={sourceId} />
            {/if}
          </Field>

          <Field label="Gate" hint="Which gate this device serves.">
            <div class="space-y-2">
              <Select type="single" bind:value={gateId}>
                <SelectTrigger>
                  {gates.find(g => g.gate_id === gateId)?.name ?? (gateId || 'Select gate…')}
                </SelectTrigger>
                <SelectContent>
                  {#each gates as g}
                    <SelectItem value={g.gate_id}>{g.name} ({g.gate_id})</SelectItem>
                  {/each}
                </SelectContent>
              </Select>
              {#if !creatingGate}
                <Button variant="outline" size="sm" onclick={() => (creatingGate = true)}>
                  <Plus size={12} /> Create new gate
                </Button>
              {:else}
                <div class="rounded-md border border-border p-3 space-y-2 bg-muted/30">
                  <p class="text-[12px] font-medium">New gate</p>
                  <Input placeholder="gate-north (ID)" bind:value={newGateId} class="font-mono" />
                  <Input placeholder="North Gate (display name)" bind:value={newGateName} />
                  <div class="flex gap-2">
                    <Button size="sm" onclick={() => createNewGate(id => gateId = id)} disabled={savingGate || !newGateId || !newGateName}>
                      {savingGate ? 'Creating…' : 'Create'}
                    </Button>
                    <Button variant="ghost" size="sm" onclick={() => (creatingGate = false)}>Cancel</Button>
                  </div>
                </div>
              {/if}
            </div>
          </Field>
        </div>
      </CardContent>
    </Card>

    <!-- Payload -->
    <Card>
      <CardHeader>
        <div class="flex items-baseline justify-between">
          <CardTitle>Payload</CardTitle>
          <span class="text-[12px] text-muted-foreground">JSONPath · evaluated by Adapter</span>
        </div>
      </CardHeader>
      <CardContent class="space-y-4">
        <Field label="Event type" hint="Schema the Adapter will validate payloads against.">
          <Select type="single" bind:value={eventTypeId}>
            <SelectTrigger>
              {eventTypes.find(t => t.id === eventTypeId)?.name ?? (eventTypeId || 'Select type…')}
            </SelectTrigger>
            <SelectContent>
              {#each eventTypes as t}
                <SelectItem value={t.id}>{t.name} ({t.code})</SelectItem>
              {/each}
            </SelectContent>
          </Select>
        </Field>

        {#if selectedType}
          <div class="rounded-md border border-border bg-muted/30 p-3 space-y-1">
            <p class="text-[11px] font-semibold text-muted-foreground uppercase tracking-wide mb-2">
              Fields from {selectedType.code}
            </p>
            {#each Object.entries(selectedType.fields) as [key, field]}
              <div class="flex items-baseline gap-3 text-[12px]">
                <span class="font-mono text-foreground w-[160px] shrink-0">{key}</span>
                <Badge variant="outline" class="text-[10px]">{field.type}</Badge>
                {#if field.required}<Badge class="text-[10px]">required</Badge>{/if}
                <span class="text-muted-foreground">{field.description}</span>
              </div>
            {/each}
          </div>
        {/if}

        <Field label="Data type code">
          <Input bind:value={dataType} placeholder="e.g. ANPR, WEIGHT" />
        </Field>

        <Field label="Mapping overrides" hint="Optional — override individual fields from the type schema.">
          <Textarea class="font-mono text-[12px]" rows={6} bind:value={mapping} />
        </Field>
      </CardContent>
    </Card>

    <!-- Pull triggers -->
    <Card>
      <CardHeader>
        <div class="flex items-baseline justify-between">
          <CardTitle>Pull triggers</CardTitle>
          <span class="text-[12px] text-muted-foreground">Adapter → Puller behavior</span>
        </div>
      </CardHeader>
      <CardContent class="space-y-0">
        <div class="flex items-start justify-between gap-4 py-3 border-b border-border">
          <div>
            <p class="text-[13px] font-medium">Enable trigger</p>
            <p class="text-[11px] text-muted-foreground mt-0.5">Adapter publishes to events:puller and Puller fetches Trigger URL.</p>
          </div>
          <Switch bind:checked={triggerEnabled} />
        </div>
        <div class="pt-3 {triggerEnabled ? '' : 'opacity-50 pointer-events-none'}">
          <Field label="Trigger URL" hint="Puller will GET this URL when an event from this device fires.">
            <Input bind:value={triggerUrl} placeholder="https://device.local/snapshot" />
          </Field>
        </div>
      </CardContent>
    </Card>
  </main>
{/if}

<!-- Delete confirmation -->
<Dialog bind:open={confirmDelete}>
  <DialogContent>
    <DialogHeader>
      <DialogTitle>Delete this device?</DialogTitle>
      <DialogDescription>
        Source <span class="font-mono">{sourceId}</span> will be removed and any active mappings revoked. This cannot be undone.
      </DialogDescription>
    </DialogHeader>
    <DialogFooter>
      <Button variant="outline" onclick={() => (confirmDelete = false)}>Cancel</Button>
      <Button variant="destructive" onclick={handleDelete}>Delete</Button>
    </DialogFooter>
  </DialogContent>
</Dialog>
