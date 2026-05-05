<script lang="ts">
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
  import type { EventType } from '$lib/types.js';
  import { Plus, Trash2, ChevronDown, ChevronRight, Pencil } from 'lucide-svelte';

  const FIELD_TYPES = ['string', 'number', 'boolean', 'datetime', 'image_url'] as const;

  let types   = $state<EventType[]>([]);
  let loading = $state(true);
  let createOpen  = $state(false);
  let editOpen    = $state(false);
  let detailId    = $state<string | null>(null);
  let saving      = $state(false);

  let newCode        = $state('');
  let newName        = $state('');
  let newDescription = $state('');
  let newFields      = $state<Array<{ key: string; name: string; description: string; type: string; required: boolean }>>([]);

  let editType        = $state<EventType | null>(null);
  let editName        = $state('');
  let editDescription = $state('');
  let editFields      = $state<Array<{ key: string; name: string; description: string; type: string; required: boolean }>>([]);

  async function load() {
    try { types = await api.types.list(); }
    catch { toast.error('Помилка завантаження типів подій'); }
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
    if (!newCode || !newName) { toast.error('Код та назва обов\'язкові'); return; }
    saving = true;
    try {
      const fields: Record<string, unknown> = {};
      for (const f of newFields) {
        if (f.key) fields[f.key] = { name: f.name, description: f.description, type: f.type, required: f.required };
      }
      await api.types.create({ code: newCode.toUpperCase(), name: newName, description: newDescription, fields });
      toast.success('Тип події створено');
      createOpen = false;
      await load();
    } catch {
      toast.error('Помилка створення типу події');
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
    if (!editType || !editName) { toast.error('Назва обов\'язкова'); return; }
    saving = true;
    try {
      const fields: Record<string, unknown> = {};
      for (const f of editFields) {
        if (f.key) fields[f.key] = { name: f.name, description: f.description, type: f.type, required: f.required };
      }
      await api.types.update(editType.id, { name: editName, description: editDescription, fields });
      toast.success('Тип події оновлено');
      editOpen = false;
      await load();
    } catch {
      toast.error('Помилка оновлення типу події');
    } finally {
      saving = false;
    }
  }

  const detailType = $derived(types.find(t => t.id === detailId));
</script>

<TopBar crumbs={['OmniGate', 'Типи подій']} title="Типи подій">
  {#snippet actions()}
    <Button size="sm" onclick={openCreate}>
      <Plus size={14} /> Новий тип
    </Button>
  {/snippet}
</TopBar>

<main class="flex-1 p-6">
  <div class="rounded-md border border-border overflow-hidden">
    <Table>
      <TableHeader>
        <TableRow>
          <TableHead class="w-[32px]"></TableHead>
          <TableHead class="w-[120px]">Код</TableHead>
          <TableHead>Назва</TableHead>
          <TableHead>Опис</TableHead>
          <TableHead class="w-[80px]">Поля</TableHead>
          <TableHead class="w-[110px]">Створено</TableHead>
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
            <TableCell class="text-muted-foreground text-sm">{t.description}</TableCell>
            <TableCell class="text-sm text-muted-foreground">{Object.keys(t.fields).length}</TableCell>
            <TableCell class="text-sm text-muted-foreground">{fmtDate(t.created_at)}</TableCell>
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
                  <p class="text-xs font-semibold uppercase tracking-wide text-muted-foreground mb-2">Схема полів</p>
                  <div class="space-y-1">
                    {#each Object.entries(t.fields) as [key, field]}
                      <div class="flex items-baseline gap-3 text-sm">
                        <span class="font-mono w-[200px] shrink-0">{key}</span>
                        <Badge variant="outline" class="text-xs shrink-0">{field.type}</Badge>
                        {#if field.required}<Badge class="text-xs shrink-0">обов'язкове</Badge>{/if}
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
            <TableCell colspan={7} class="py-10 text-center text-muted-foreground">Типи подій ще не визначені.</TableCell>
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
      <DialogTitle>Новий тип події</DialogTitle>
      <DialogDescription>Визначте схему для нового типу IoT-події.</DialogDescription>
    </DialogHeader>
    <div class="space-y-4 py-2 max-h-[60vh] overflow-y-auto pr-1">
      <div class="grid grid-cols-2 gap-4">
        <Field label="Код" hint="Короткий ідентифікатор у верхньому регістрі, напр. ANPR">
          <Input bind:value={newCode} placeholder="ANPR" oninput={() => newCode = newCode.toUpperCase()} />
        </Field>
        <Field label="Назва">
          <Input bind:value={newName} placeholder="Розпізнавання номерного знаку" />
        </Field>
      </div>
      <Field label="Опис">
        <Textarea bind:value={newDescription} rows={2} placeholder="Опишіть, що фіксує цей тип події…" />
      </Field>

      <div>
        <div class="flex items-center justify-between mb-2">
          <p class="text-sm font-medium">Поля</p>
          <Button variant="outline" size="sm" onclick={addField}>
            <Plus size={12} /> Додати поле
          </Button>
        </div>
        {#if newFields.length === 0}
          <p class="text-sm text-muted-foreground">Полів ще немає.</p>
        {/if}
        {#each newFields as f, i}
          <div class="rounded-md border border-border p-3 space-y-2 mb-2">
            <div class="grid grid-cols-[1fr_1fr_auto] gap-2 items-end">
              <Field label="Ключ (ім'я JSON поля)">
                <Input bind:value={f.key} placeholder="plate_number" class="font-mono text-sm" />
              </Field>
              <Field label="Відображувана назва">
                <Input bind:value={f.name} placeholder="Номерний знак" />
              </Field>
              <Button variant="ghost" size="icon-sm" class="mb-0.5 hover:text-destructive" onclick={() => removeField(i)}>
                <Trash2 size={13} />
              </Button>
            </div>
            <div class="grid grid-cols-[1fr_120px_auto] gap-2 items-end">
              <Field label="Опис">
                <Input bind:value={f.description} placeholder="Номерний знак транспортного засобу" />
              </Field>
              <Field label="Тип">
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
                <span class="text-sm">Обов'язкове</span>
                <Switch bind:checked={f.required} />
              </div>
            </div>
          </div>
        {/each}
      </div>
    </div>
    <DialogFooter>
      <Button variant="outline" onclick={() => (createOpen = false)}>Скасувати</Button>
      <Button onclick={handleCreate} disabled={saving || !newCode || !newName}>
        {saving ? 'Створення…' : 'Створити тип'}
      </Button>
    </DialogFooter>
  </DialogContent>
</Dialog>

<!-- Edit dialog -->
<Dialog bind:open={editOpen}>
  <DialogContent class="max-w-2xl">
    <DialogHeader>
      <DialogTitle>Редагувати тип події — <span class="font-mono font-normal">{editType?.code}</span></DialogTitle>
      <DialogDescription>Оновіть назву, опис або схему полів.</DialogDescription>
    </DialogHeader>
    <div class="space-y-4 py-2 max-h-[60vh] overflow-y-auto pr-1">
      <div class="grid grid-cols-2 gap-4">
        <Field label="Назва">
          <Input bind:value={editName} placeholder="Розпізнавання номерного знаку" />
        </Field>
        <Field label="Опис">
          <Input bind:value={editDescription} placeholder="Що фіксує цей тип…" />
        </Field>
      </div>

      <div>
        <div class="flex items-center justify-between mb-2">
          <p class="text-sm font-medium">Поля</p>
          <Button variant="outline" size="sm" onclick={addEditField}>
            <Plus size={12} /> Додати поле
          </Button>
        </div>
        {#if editFields.length === 0}
          <p class="text-sm text-muted-foreground">Полів ще немає.</p>
        {/if}
        {#each editFields as f, i}
          <div class="rounded-md border border-border p-3 space-y-2 mb-2">
            <div class="grid grid-cols-[1fr_1fr_auto] gap-2 items-end">
              <Field label="Ключ (ім'я JSON поля)">
                <Input bind:value={f.key} placeholder="plate_number" class="font-mono text-sm" />
              </Field>
              <Field label="Відображувана назва">
                <Input bind:value={f.name} placeholder="Номерний знак" />
              </Field>
              <Button variant="ghost" size="icon-sm" class="mb-0.5 hover:text-destructive" onclick={() => removeEditField(i)}>
                <Trash2 size={13} />
              </Button>
            </div>
            <div class="grid grid-cols-[1fr_120px_auto] gap-2 items-end">
              <Field label="Опис">
                <Input bind:value={f.description} placeholder="Номерний знак транспортного засобу" />
              </Field>
              <Field label="Тип">
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
                <span class="text-sm">Обов'язкове</span>
                <Switch bind:checked={f.required} />
              </div>
            </div>
          </div>
        {/each}
      </div>
    </div>
    <DialogFooter>
      <Button variant="outline" onclick={() => (editOpen = false)}>Скасувати</Button>
      <Button onclick={handleEdit} disabled={saving || !editName}>
        {saving ? 'Збереження…' : 'Зберегти зміни'}
      </Button>
    </DialogFooter>
  </DialogContent>
</Dialog>
