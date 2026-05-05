<script lang="ts">
  import { page } from '$app/stores';
  import { goto } from '$app/navigation';
  import { toast } from 'svelte-sonner';
  import TopBar from '$lib/components/TopBar.svelte';
  import Field from '$lib/components/Field.svelte';
  import GateBadge from '$lib/components/GateBadge.svelte';
  import { Button } from '$lib/components/ui/button/index.js';
  import { Badge } from '$lib/components/ui/badge/index.js';
  import { Input } from '$lib/components/ui/input/index.js';
  import { Switch } from '$lib/components/ui/switch/index.js';
  import { Card, CardContent, CardHeader, CardTitle } from '$lib/components/ui/card/index.js';
  import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '$lib/components/ui/table/index.js';
  import { api } from '$lib/api.js';
  import { fmtDateTime, fmtDate } from '$lib/utils.js';
  import type { Gate, GateSettings, GateStats } from '$lib/types.js';
  import { ChevronLeft, ExternalLink } from 'lucide-svelte';

  const gateId = $derived($page.params.id ?? '');

  let gate     = $state<Gate | null>(null);
  let stats    = $state<GateStats | null>(null);
  let loading  = $state(true);
  let saving   = $state(false);

  let ttlMinutes    = $state(30);
  let autoClose     = $state(true);
  let maxEvents     = $state(100);

  $effect(() => {
    const id = gateId;
    (async () => {
      loading = true;
      try {
        const [g, s] = await Promise.all([api.gates.get(id), api.gates.stats(id)]);
        gate = g;
        stats = s;
        const cfg = g.settings ?? {};
        ttlMinutes = cfg.transaction_ttl_minutes ?? 30;
        autoClose  = cfg.auto_close_transactions ?? true;
        maxEvents  = cfg.max_events_per_transaction ?? 100;
      } catch {
        toast.error('Шлагбаум не знайдено');
        goto('/settings/gates');
      } finally {
        loading = false;
      }
    })();
  });

  async function saveSettings() {
    if (!gate) return;
    saving = true;
    try {
      const settings: GateSettings = {
        transaction_ttl_minutes: ttlMinutes,
        auto_close_transactions: autoClose,
        max_events_per_transaction: maxEvents,
      };
      gate = await api.gates.updateSettings(gate.id, settings);
      toast.success('Налаштування збережено');
    } catch {
      toast.error('Помилка збереження налаштувань');
    } finally {
      saving = false;
    }
  }
</script>

<TopBar crumbs={['OmniGate', 'Шлагбауми', gate?.name ?? '…']}>
  {#snippet actions()}
    <Button variant="outline" size="sm" onclick={() => goto('/settings/gates')}>
      <ChevronLeft size={14} /> Шлагбауми
    </Button>
  {/snippet}
</TopBar>

{#if loading}
  <div class="flex-1 flex items-center justify-center text-muted-foreground">Завантаження…</div>
{:else if gate}
  <main class="flex-1 p-6 max-w-[960px] space-y-5">

    <!-- Header card -->
    <Card>
      <CardContent class="pt-5">
        <div class="flex items-start gap-4 flex-wrap">
          <GateBadge gateId={gate.gate_id} dot />
          <div class="flex-1 min-w-0">
            <div class="flex items-center gap-3 flex-wrap">
              <span class="text-lg font-semibold">{gate.name}</span>
              <Badge variant={gate.status === 'active' ? 'default' : 'secondary'}>
                {gate.status === 'active' ? 'Активний' : 'Неактивний'}
              </Badge>
            </div>
            {#if gate.location}
              <p class="text-sm text-muted-foreground mt-0.5">{gate.location}</p>
            {/if}
            {#if gate.description}
              <p class="text-sm text-muted-foreground mt-1">{gate.description}</p>
            {/if}
          </div>
          <div class="grid grid-cols-3 gap-4 text-center">
            <a href="/?gate_id={gate.gate_id}" class="rounded-md border border-border px-4 py-2 hover:bg-muted transition-colors">
              <div class="text-2xl font-bold">{stats?.open_transactions ?? 0}</div>
              <div class="text-xs text-muted-foreground">Відкриті</div>
            </a>
            <a href="/?gate_id={gate.gate_id}" class="rounded-md border border-border px-4 py-2 hover:bg-muted transition-colors">
              <div class="text-2xl font-bold">{stats?.total_transactions ?? 0}</div>
              <div class="text-xs text-muted-foreground">Всього</div>
            </a>
            <a href="/settings/devices?gate_id={gate.gate_id}" class="rounded-md border border-border px-4 py-2 hover:bg-muted transition-colors">
              <div class="text-2xl font-bold">{stats?.total_devices ?? 0}</div>
              <div class="text-xs text-muted-foreground">Пристрої</div>
            </a>
          </div>
        </div>
      </CardContent>
    </Card>

    <div class="grid grid-cols-[1fr_1.1fr] gap-5 items-start">

      <!-- Settings -->
      <Card>
        <CardHeader class="pb-3">
          <CardTitle class="text-base">Налаштування транзакцій</CardTitle>
        </CardHeader>
        <CardContent class="space-y-4">
          <Field label="TTL транзакції (хвилини)" hint="Автоматично закривати відкриті транзакції після стількох хвилин неактивності.">
            <Input type="number" bind:value={ttlMinutes} min={1} max={1440} />
          </Field>
          <div class="flex items-center justify-between">
            <div>
              <p class="text-sm font-medium">Автозакриття транзакцій</p>
              <p class="text-xs text-muted-foreground mt-0.5">Закривати після закінчення TTL без нових подій.</p>
            </div>
            <Switch bind:checked={autoClose} />
          </div>
          <Field label="Макс. подій на транзакцію" hint="Жорсткий ліміт; додаткові події стартують нову транзакцію.">
            <Input type="number" bind:value={maxEvents} min={1} max={10000} />
          </Field>
          <div class="flex justify-end pt-1">
            <Button size="sm" onclick={saveSettings} disabled={saving}>
              {saving ? 'Збереження…' : 'Зберегти налаштування'}
            </Button>
          </div>
        </CardContent>
      </Card>

      <!-- Recent transactions -->
      <Card>
        <CardHeader class="pb-2">
          <div class="flex items-center justify-between">
            <CardTitle class="text-base">Останні транзакції</CardTitle>
            <a
              href="/?gate_id={gate.gate_id}"
              class="text-sm text-muted-foreground hover:text-foreground flex items-center gap-1"
            >
              Всі <ExternalLink size={11} />
            </a>
          </div>
        </CardHeader>
        <CardContent class="p-0">
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Код</TableHead>
                <TableHead>Статус</TableHead>
                <TableHead>Відкрито</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {#each (stats?.recent_transactions ?? []) as tx (tx.id)}
                <TableRow class="cursor-pointer" onclick={() => goto(`/transactions/${tx.id}`)}>
                  <TableCell class="font-mono text-sm">{tx.code}</TableCell>
                  <TableCell>
                    {#if tx.is_open}
                      <Badge class="text-xs">Активна</Badge>
                    {:else}
                      <Badge variant="secondary" class="text-xs">Закрита</Badge>
                    {/if}
                  </TableCell>
                  <TableCell class="text-sm text-muted-foreground">{fmtDate(tx.created_at)}</TableCell>
                </TableRow>
              {/each}
              {#if !stats?.recent_transactions?.length}
                <TableRow>
                  <TableCell colspan={3} class="py-6 text-center text-muted-foreground">Транзакцій ще немає.</TableCell>
                </TableRow>
              {/if}
            </TableBody>
          </Table>
        </CardContent>
      </Card>
    </div>

  </main>
{/if}
