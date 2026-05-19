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
  import MappingEditor from '$lib/components/MappingEditor.svelte';
  import { api } from '$lib/api.js';
  import type { DeviceConfig, Event, EventType, Gate, APIKey, Trigger } from '$lib/types.js';
  import { ChevronLeft, Plus, Trash2, Zap } from 'lucide-svelte';
  import ConfirmDelete from '$lib/components/ConfirmDelete.svelte';

  const deviceId = $derived($page.params.id ?? '');
  const isNew    = $derived(deviceId === 'new');

  let gates      = $state<Gate[]>([]);
  let eventTypes = $state<EventType[]>([]);
  let apiKeys    = $state<APIKey[]>([]);
  let allConfigs = $state<DeviceConfig[]>([]);
  let loading    = $state(get(page).params.id !== 'new');
  let saving     = $state(false);
  let triggering = $state(false);
  let confirmDelete = $state(false);

  // Form state
  let sourceId       = $state('');
  let gateId         = $state('');
  let eventTypeId    = $state('');
  let dataType       = $state('');
  let mappingObj     = $state<Record<string, string>>({});
  // Pull URL on THIS device (called by Puller when this device is someone else's target)
  let triggerUrl     = $state('');
  // Outgoing triggers: devices THIS device activates after its own event
  let triggerEnabled = $state(false);
  let triggers       = $state<Trigger[]>([]);

  let latestEvent = $state<Event | null>(null);

  // Inline key form
  let creatingKey  = $state(false);
  let newKeyName   = $state('');
  let newKeyGateId = $state('');
  let savingKey    = $state(false);
  let generatedKey    = $state('');
  let showKeyDialog   = $state(false);

  // Inline gate form
  let creatingGate    = $state(false);
  let creatingKeyGate = $state(false);
  let newGateId       = $state('');
  let newGateName     = $state('');
  let savingGate      = $state(false);

  const selectedType = $derived(eventTypes.find(t => t.id === eventTypeId));

  // Devices that trigger THIS device (have this source_id in their triggers[])
  const triggeredByConfigs = $derived(
    allConfigs.filter(c =>
      c.source_id !== sourceId &&
      (c.triggers ?? []).some((t: Trigger) => t.source_id === sourceId)
    )
  );

  $effect(() => {
    if (!isNew) return;
    const key = apiKeys.find(k => String(k.id) === sourceId);
    if (key?.gate_id) gateId = key.gate_id;
  });

  $effect(() => {
    Promise.all([api.gates.list(), api.types.list(), api.configs.list()])
      .then(([g, t, c]) => { gates = g; eventTypes = t; allConfigs = c; })
      .catch(() => {});
    // API ключі — лише для відображення назви пристрою; відсутній read:keys не ламає форму.
    api.auth.keys.list().then(k => { apiKeys = k; }).catch(() => {});
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
          mappingObj     = cfg.data_mapping ?? {};
          triggerUrl     = cfg.trigger_url ?? '';
          triggerEnabled = cfg.trigger_enabled;
          triggers       = cfg.triggers ?? [];
          api.events.latestForSource(cfg.source_id)
            .then(e => { latestEvent = e; })
            .catch(() => {});
        } catch {
          toast.error('Пристрій не знайдено');
          goto('/settings/devices');
        } finally {
          loading = false;
        }
      })();
    }
  });

  function addTrigger() {
    triggers = [...triggers, { source_id: '' }];
  }

  function removeTrigger(i: number) {
    triggers = triggers.filter((_, idx) => idx !== i);
  }

  function setTriggerSourceId(i: number, value: string) {
    triggers = triggers.map((t, idx) => idx === i ? { source_id: value } : t);
  }

  async function createNewKey() {
    if (!newKeyName) return;
    savingKey = true;
    try {
      const res = await api.auth.keys.create({
        name: newKeyName,
        gate_id: newKeyGateId || gateId,
        permission_ids: ['ingest:events'],
      });
      apiKeys = await api.auth.keys.list();
      sourceId = String(res.id);
      creatingKey = false; newKeyName = ''; newKeyGateId = '';
      generatedKey = res.api_key;
      showKeyDialog = true;
    } catch {
      toast.error('Помилка створення ключа');
    } finally {
      savingKey = false;
    }
  }

  async function createNewGate(onCreated: (id: string) => void) {
    if (!newGateId || !newGateName) return;
    savingGate = true;
    try {
      await api.gates.create({ gate_id: newGateId, name: newGateName, location: '', description: '' });
      gates = await api.gates.list();
      onCreated(newGateId);
      creatingGate = false; creatingKeyGate = false; newGateId = ''; newGateName = '';
      toast.success('Шлагбаум створено');
    } catch {
      toast.error('Помилка створення шлагбауму');
    } finally {
      savingGate = false;
    }
  }

  async function handleSave() {
    saving = true;
    const cleanTriggers = triggerEnabled
      ? triggers.filter(t => t.source_id.trim() !== '')
      : [];
    try {
      if (isNew) {
        await api.configs.create({
          source_id: sourceId, gate_id: gateId,
          event_type_id: eventTypeId, data_type: dataType,
          data_mapping: mappingObj,
          trigger_url: triggerUrl || null,
          trigger_enabled: triggerEnabled,
          triggers: cleanTriggers,
        });
        toast.success('Пристрій створено');
      } else {
        await api.configs.update(deviceId, {
          event_type_id: eventTypeId, gate_id: gateId,
          data_type: dataType, data_mapping: mappingObj,
          trigger_url: triggerUrl || null,
          trigger_enabled: triggerEnabled,
          triggers: cleanTriggers,
        });
        toast.success('Пристрій збережено');
      }
      goto('/settings/devices');
    } catch {
      toast.error('Помилка збереження');
    } finally {
      saving = false;
    }
  }

  async function handleDelete() {
    try {
      await api.configs.delete(deviceId);
      toast.success('Пристрій видалено');
      goto('/settings/devices');
    } catch {
      toast.error('Помилка видалення');
    }
  }

  async function handleManualTrigger() {
    triggering = true;
    try {
      await api.configs.trigger(deviceId);
      toast.success('Тригери запущено вручну');
    } catch {
      toast.error('Помилка запуску тригерів');
    } finally {
      triggering = false;
    }
  }
</script>

<TopBar
  crumbs={['OmniGate', 'Пристрої', isNew ? 'Новий пристрій' : sourceId]}
  title={isNew ? 'Новий пристрій' : 'Редагувати пристрій'}
>
  {#snippet actions()}
    {#if !isNew}
      <PermGuard permission="manage:configs">
        <Button variant="destructive" size="sm" onclick={() => (confirmDelete = true)}>Видалити</Button>
      </PermGuard>
    {/if}
    <Button variant="outline" size="sm" onclick={() => goto('/settings/devices')}>Скасувати</Button>
    <PermGuard permission="manage:configs">
      <Button size="sm" onclick={handleSave} disabled={saving}>
        {saving ? 'Збереження…' : 'Зберегти'}
      </Button>
    </PermGuard>
  {/snippet}
</TopBar>

{#if loading}
  <div class="flex-1 flex items-center justify-center text-muted-foreground">Завантаження…</div>
{:else}
  <main class="flex-1 p-4 sm:p-6 max-w-[920px] space-y-5">
    <Button variant="ghost" size="sm" onclick={() => goto('/settings/devices')}>
      <ChevronLeft size={14} /> Пристрої
    </Button>

    <!-- Identity -->
    <Card>
      <CardHeader><CardTitle>Ідентифікація</CardTitle></CardHeader>
      <CardContent class="space-y-4">
        <div class="grid grid-cols-1 md:grid-cols-2 gap-4">
          <Field label="Джерело (API Ключ)" hint="API ключ, ID якого ідентифікує цей пристрій.">
            {#if isNew}
              <div class="space-y-2">
                <Select type="single" bind:value={sourceId}>
                  <SelectTrigger>
                    {#if sourceId}
                      {apiKeys.find(k => String(k.id) === sourceId)?.owner_name ?? `Ключ #${sourceId}`}
                    {:else}
                      Оберіть API ключ…
                    {/if}
                  </SelectTrigger>
                  <SelectContent>
                    {#each apiKeys.filter(k => k.is_active) as k}
                      <SelectItem value={String(k.id)}>{k.owner_name} (#{k.id})</SelectItem>
                    {/each}
                  </SelectContent>
                </Select>
                {#if !creatingKey}
                  <Button variant="outline" size="sm" onclick={() => (creatingKey = true)}>
                    <Plus size={12} /> Створити новий ключ
                  </Button>
                {:else}
                  <div class="rounded-md border border-border p-3 space-y-2 bg-muted/30">
                    <p class="text-sm font-medium">Новий API ключ</p>
                    <Input placeholder="Власник / назва пристрою" bind:value={newKeyName} />
                    <Select type="single" bind:value={newKeyGateId}>
                      <SelectTrigger>
                        {gates.find(g => g.gate_id === newKeyGateId)?.name ?? (newKeyGateId || 'Оберіть шлагбаум (необов\'язково)…')}
                      </SelectTrigger>
                      <SelectContent>
                        <SelectItem value="">Немає</SelectItem>
                        {#each gates as g}
                          <SelectItem value={g.gate_id}>{g.name} ({g.gate_id})</SelectItem>
                        {/each}
                      </SelectContent>
                    </Select>
                    {#if !creatingKeyGate}
                      <Button variant="outline" size="sm" onclick={() => (creatingKeyGate = true)}>
                        <Plus size={12} /> Створити новий шлагбаум
                      </Button>
                    {:else}
                      <div class="rounded-md border border-border p-3 space-y-2 bg-background">
                        <p class="text-sm font-medium">Новий шлагбаум</p>
                        <Input placeholder="gate-north (ID)" bind:value={newGateId} class="font-mono" />
                        <Input placeholder="Північна брама (назва)" bind:value={newGateName} />
                        <div class="flex gap-2">
                          <Button size="sm" onclick={() => createNewGate(id => newKeyGateId = id)} disabled={savingGate || !newGateId || !newGateName}>
                            {savingGate ? 'Створення…' : 'Створити'}
                          </Button>
                          <Button variant="ghost" size="sm" onclick={() => (creatingKeyGate = false)}>Скасувати</Button>
                        </div>
                      </div>
                    {/if}
                    <div class="flex gap-2 pt-1">
                      <Button size="sm" onclick={createNewKey} disabled={savingKey || !newKeyName}>
                        {savingKey ? 'Створення…' : 'Створити ключ'}
                      </Button>
                      <Button variant="ghost" size="sm" onclick={() => { creatingKey = false; creatingKeyGate = false; }}>Скасувати</Button>
                    </div>
                  </div>
                {/if}
              </div>
            {:else}
              <Input class="font-mono" disabled value={sourceId} />
            {/if}
          </Field>

          <Field label="Шлагбаум" hint="Шлагбаум, якому слугує цей пристрій.">
            <div class="space-y-2">
              <Select type="single" bind:value={gateId}>
                <SelectTrigger>
                  {gates.find(g => g.gate_id === gateId)?.name ?? (gateId || 'Оберіть шлагбаум…')}
                </SelectTrigger>
                <SelectContent>
                  {#each gates as g}
                    <SelectItem value={g.gate_id}>{g.name} ({g.gate_id})</SelectItem>
                  {/each}
                </SelectContent>
              </Select>
              {#if !creatingGate}
                <Button variant="outline" size="sm" onclick={() => (creatingGate = true)}>
                  <Plus size={12} /> Створити новий шлагбаум
                </Button>
              {:else}
                <div class="rounded-md border border-border p-3 space-y-2 bg-muted/30">
                  <p class="text-sm font-medium">Новий шлагбаум</p>
                  <Input placeholder="gate-north (ID)" bind:value={newGateId} class="font-mono" />
                  <Input placeholder="Північна брама (назва)" bind:value={newGateName} />
                  <div class="flex gap-2">
                    <Button size="sm" onclick={() => createNewGate(id => gateId = id)} disabled={savingGate || !newGateId || !newGateName}>
                      {savingGate ? 'Створення…' : 'Створити'}
                    </Button>
                    <Button variant="ghost" size="sm" onclick={() => (creatingGate = false)}>Скасувати</Button>
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
          <CardTitle>Корисне навантаження</CardTitle>
          <span class="text-sm text-muted-foreground">JSONPath · обробляється Адаптером</span>
        </div>
      </CardHeader>
      <CardContent class="space-y-4">
        <Field label="Тип події" hint="Схема, яку Адаптер використовує для валідації.">
          <Select type="single" bind:value={eventTypeId}>
            <SelectTrigger>
              {eventTypes.find(t => t.id === eventTypeId)?.name ?? (eventTypeId || 'Оберіть тип…')}
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
            <p class="text-xs font-semibold text-muted-foreground uppercase tracking-wide mb-2">
              Поля з {selectedType.code}
            </p>
            {#each Object.entries(selectedType.fields) as [key, field]}
              <div class="flex items-baseline gap-3 text-sm">
                <span class="font-mono text-foreground w-[160px] shrink-0">{key}</span>
                <Badge variant="outline" class="text-xs">{field.type}</Badge>
                {#if field.required}<Badge class="text-xs">обов'язкове</Badge>{/if}
                <span class="text-muted-foreground">{field.description}</span>
              </div>
            {/each}
          </div>
        {/if}

        <Field label="Код типу даних">
          <Input bind:value={dataType} placeholder="напр. ANPR, WEIGHT" />
        </Field>

        <Field label="Маппінг даних" hint="Зіставте шляхи корисного навантаження з полями типу події.">
          <MappingEditor
            bind:value={mappingObj}
            schema={selectedType?.fields ?? {}}
            rawEvent={latestEvent ?? undefined}
          />
        </Field>
      </CardContent>
    </Card>

    <!-- Puller section -->
    <Card>
      <CardHeader>
        <div class="flex items-baseline justify-between">
          <CardTitle>Тригери Puller</CardTitle>
          <span class="text-sm text-muted-foreground">Поведінка Adapter → Puller</span>
        </div>
      </CardHeader>
      <CardContent class="space-y-0 divide-y divide-border">

        <!-- Own pull URL (this device as a target) -->
        <div class="py-4 space-y-3">
          <div>
            <p class="text-sm font-medium">URL цього пристрою</p>
            <p class="text-xs text-muted-foreground mt-0.5">
              Puller викличе цей URL, коли інший пристрій тригерить цей. Залиште порожнім якщо пристрій не є ціллю.
            </p>
          </div>
          <Input bind:value={triggerUrl} placeholder="https://device.local/snapshot" />
        </div>

        <!-- Outgoing triggers toggle + list -->
        <div class="pt-4 space-y-4">
          <div class="flex items-start justify-between gap-4">
            <div>
              <p class="text-sm font-medium">Ініціювати тригери</p>
              <p class="text-xs text-muted-foreground mt-0.5">
                Після обробки події цього пристрою Adapter поставить задачу в Puller для кожного цільового пристрою.
              </p>
            </div>
            <Switch bind:checked={triggerEnabled} />
          </div>

          <div class="{triggerEnabled ? '' : 'opacity-50 pointer-events-none'} space-y-2">
            {#each triggers as trigger, i (i)}
              <div class="flex items-center gap-2">
                <div class="flex-1">
                  <Select
                    type="single"
                    value={trigger.source_id}
                    onValueChange={v => setTriggerSourceId(i, v)}
                  >
                    <SelectTrigger>
                      {#if trigger.source_id}
                        {@const c = allConfigs.find(x => x.source_id === trigger.source_id)}
                        {c ? `${c.source_id}${c.event_type ? ' — ' + c.event_type.code : ''}` : trigger.source_id}
                      {:else}
                        Оберіть цільовий пристрій…
                      {/if}
                    </SelectTrigger>
                    <SelectContent>
                      {#each allConfigs.filter(c => c.source_id !== sourceId) as c}
                        <SelectItem value={c.source_id}>
                          {c.source_id}{c.event_type ? ` — ${c.event_type.code}` : ''}
                          {#if c.trigger_url}
                            <span class="text-muted-foreground"> · має URL</span>
                          {/if}
                        </SelectItem>
                      {/each}
                    </SelectContent>
                  </Select>
                </div>
                <Button
                  variant="ghost"
                  size="icon-sm"
                  onclick={() => removeTrigger(i)}
                  class="text-destructive hover:text-destructive shrink-0"
                >
                  <Trash2 size={14} />
                </Button>
              </div>
            {/each}

            <Button variant="outline" size="sm" onclick={addTrigger} class="w-full gap-2 mt-1">
              <Plus size={14} /> Додати цільовий пристрій
            </Button>

            {#if !isNew && triggerEnabled && triggers.some(t => t.source_id)}
              <div class="pt-1">
                <Button
                  variant="outline"
                  size="sm"
                  onclick={handleManualTrigger}
                  disabled={triggering}
                  class="gap-2"
                >
                  <Zap size={14} />
                  {triggering ? 'Запуск…' : 'Запустити всі тригери вручну'}
                </Button>
                <p class="text-xs text-muted-foreground mt-1">
                  Негайно поставить задачу в Puller для кожного цільового пристрою.
                </p>
              </div>
            {/if}
          </div>
        </div>

      </CardContent>
    </Card>

    <!-- Trigger relationships -->
    {#if !isNew && (triggeredByConfigs.length > 0 || triggers.length > 0)}
      <Card>
        <CardHeader><CardTitle>Зв'язки тригерів</CardTitle></CardHeader>
        <CardContent class="space-y-3">
          {#each triggers.filter(t => t.source_id) as t}
            {@const targetCfg = allConfigs.find(c => c.source_id === t.source_id)}
            <div class="flex items-center gap-2 text-sm">
              <span class="text-muted-foreground w-[130px] shrink-0">Тригерує →</span>
              {#if targetCfg}
                <a href="/settings/devices/{targetCfg.id}" class="font-mono hover:underline text-primary">
                  {targetCfg.source_id}
                </a>
                {#if targetCfg.event_type}
                  <Badge variant="outline" class="text-xs">{targetCfg.event_type.code}</Badge>
                {/if}
                {#if targetCfg.trigger_url}
                  <span class="text-xs text-muted-foreground truncate max-w-[200px]">{targetCfg.trigger_url}</span>
                {:else}
                  <Badge variant="secondary" class="text-xs">без URL</Badge>
                {/if}
              {:else}
                <span class="font-mono text-muted-foreground">{t.source_id}</span>
              {/if}
            </div>
          {/each}
          {#each triggeredByConfigs as trig (trig.id)}
            <div class="flex items-center gap-2 text-sm">
              <span class="text-muted-foreground w-[130px] shrink-0">← Викликається</span>
              <a href="/settings/devices/{trig.id}" class="font-mono hover:underline text-primary">
                {trig.source_id}
              </a>
              {#if trig.event_type}
                <Badge variant="outline" class="text-xs">{trig.event_type.code}</Badge>
              {/if}
            </div>
          {/each}
        </CardContent>
      </Card>
    {/if}

  </main>
{/if}

<ConfirmDelete bind:open={confirmDelete} title="Видалити цей пристрій?" onconfirm={handleDelete}>
  {#snippet description()}
    Джерело <span class="font-mono">{sourceId}</span> буде видалено, всі активні маппінги анульовані.
    Дію неможливо скасувати.
  {/snippet}
</ConfirmDelete>

<Dialog bind:open={showKeyDialog}>
  <DialogContent class="max-w-md">
    <DialogHeader>
      <DialogTitle>API ключ створено</DialogTitle>
      <DialogDescription>Скопіюйте ключ зараз — він більше не буде показаний.</DialogDescription>
    </DialogHeader>
    <div class="rounded-md bg-muted p-3 font-mono text-sm break-all select-all">{generatedKey}</div>
    <DialogFooter>
      <Button onclick={() => { navigator.clipboard.writeText(generatedKey); toast.success('Скопійовано'); }}>
        Копіювати
      </Button>
      <Button variant="outline" onclick={() => (showKeyDialog = false)}>Закрити</Button>
    </DialogFooter>
  </DialogContent>
</Dialog>
