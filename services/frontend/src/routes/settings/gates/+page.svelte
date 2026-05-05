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
  import PermGuard from '$lib/components/PermGuard.svelte';
  import type { Gate } from '$lib/types.js';
  import { Plus, Pencil, Trash2 } from 'lucide-svelte';

  let gates   = $state<Gate[]>([]);
  let loading = $state(true);
  let saving  = $state(false);

  let editOpen   = $state(false);
  let deleteOpen = $state(false);
  let isNew      = $state(false);
  let selected   = $state<Gate | null>(null);

  let fGateId      = $state('');
  let fName        = $state('');
  let fLocation    = $state('');
  let fDescription = $state('');
  let fActive      = $state(true);

  async function load() {
    try { gates = await api.gates.list(); }
    catch { toast.error('Помилка завантаження шлагбаумів'); }
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
    if (!fGateId || !fName) { toast.error('ID та назва шлагбауму обов\'язкові'); return; }
    saving = true;
    try {
      if (isNew) {
        await api.gates.create({ gate_id: fGateId, name: fName, location: fLocation, description: fDescription });
        toast.success('Шлагбаум створено');
      } else if (selected) {
        await api.gates.update(selected.id, {
          name: fName, location: fLocation,
          description: fDescription, status: fActive ? 'active' : 'inactive',
        });
        toast.success('Шлагбаум збережено');
      }
      editOpen = false;
      await load();
    } catch {
      toast.error('Помилка збереження');
    } finally {
      saving = false;
    }
  }

  async function handleDelete() {
    if (!selected) return;
    try {
      await api.gates.delete(selected.id);
      toast.success('Шлагбаум видалено');
      deleteOpen = false;
      await load();
    } catch {
      toast.error('Помилка видалення шлагбауму');
    }
  }
</script>

<TopBar crumbs={['OmniGate', 'Шлагбауми']} title="Шлагбауми">
  {#snippet actions()}
    <PermGuard permission="manage:gates">
      <Button size="sm" onclick={openCreate}>
        <Plus size={14} /> Новий шлагбаум
      </Button>
    </PermGuard>
  {/snippet}
</TopBar>

<main class="flex-1 p-6">
  <div class="rounded-md border border-border overflow-hidden overflow-x-auto">
    <Table>
      <TableHeader>
        <TableRow>
          <TableHead class="w-[160px]">ID шлагбауму</TableHead>
          <TableHead>Назва</TableHead>
          <TableHead class="hidden sm:table-cell">Місцезнаходження</TableHead>
          <TableHead class="w-[90px]">Статус</TableHead>
          <TableHead class="w-[80px]"></TableHead>
        </TableRow>
      </TableHeader>
      <TableBody>
        {#each gates as g (g.id)}
          <TableRow class="cursor-pointer" onclick={() => goto(`/settings/gates/${g.id}`)}>
            <TableCell><GateBadge gateId={g.gate_id} /></TableCell>
            <TableCell class="font-medium">{g.name}</TableCell>
            <TableCell class="hidden sm:table-cell text-sm text-muted-foreground">{g.location || '—'}</TableCell>
            <TableCell>
              <Badge variant={g.status === 'active' ? 'default' : 'secondary'}>
                {g.status === 'active' ? 'Активний' : 'Неактивний'}
              </Badge>
            </TableCell>
            <TableCell>
              <PermGuard permission="manage:gates">
                <div role="presentation" class="flex gap-1" onclick={(e) => e.stopPropagation()} onkeydown={(e) => e.stopPropagation()}>
                  <Button variant="ghost" size="icon-sm" onclick={() => openEdit(g)}>
                    <Pencil size={13} />
                  </Button>
                  <Button variant="ghost" size="icon-sm" class="hover:text-destructive"
                    onclick={() => { selected = g; deleteOpen = true; }}>
                    <Trash2 size={13} />
                  </Button>
                </div>
              </PermGuard>
            </TableCell>
          </TableRow>
        {/each}
        {#if !loading && gates.length === 0}
          <TableRow>
            <TableCell colspan={5} class="py-10 text-center text-muted-foreground">Шлагбауми не налаштовані.</TableCell>
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
      <DialogTitle>{isNew ? 'Новий шлагбаум' : `Редагувати — ${selected?.gate_id}`}</DialogTitle>
    </DialogHeader>
    <div class="space-y-4 py-2">
      <div class="grid grid-cols-1 sm:grid-cols-2 gap-4">
        <Field label="ID шлагбауму" hint="Короткий унікальний ідентифікатор, напр. gate-north">
          <Input bind:value={fGateId} placeholder="gate-north" disabled={!isNew} class="font-mono" />
        </Field>
        <Field label="Назва">
          <Input bind:value={fName} placeholder="Північна брама" />
        </Field>
      </div>
      <Field label="Місцезнаходження">
        <Input bind:value={fLocation} placeholder="Будівля А, Вхід 1" />
      </Field>
      <Field label="Опис">
        <Textarea bind:value={fDescription} rows={2} placeholder="Необов'язкові нотатки…" />
      </Field>
      {#if !isNew}
        <div class="flex items-center justify-between">
          <span class="text-sm font-medium">Активний</span>
          <Switch bind:checked={fActive} />
        </div>
      {/if}
    </div>
    <DialogFooter>
      <Button variant="outline" onclick={() => (editOpen = false)}>Скасувати</Button>
      <Button onclick={handleSave} disabled={saving || !fGateId || !fName}>
        {saving ? 'Збереження…' : isNew ? 'Створити шлагбаум' : 'Зберегти'}
      </Button>
    </DialogFooter>
  </DialogContent>
</Dialog>

<!-- Delete dialog -->
<Dialog bind:open={deleteOpen}>
  <DialogContent class="max-w-sm">
    <DialogHeader>
      <DialogTitle>Видалити шлагбаум?</DialogTitle>
      <DialogDescription>
        Шлагбаум <span class="font-mono">{selected?.gate_id}</span> буде назавжди видалено.
        Пристрої та транзакції збережуть рядок з ID шлагбауму.
      </DialogDescription>
    </DialogHeader>
    <DialogFooter>
      <Button variant="outline" onclick={() => (deleteOpen = false)}>Скасувати</Button>
      <Button variant="destructive" onclick={handleDelete}>Видалити</Button>
    </DialogFooter>
  </DialogContent>
</Dialog>
