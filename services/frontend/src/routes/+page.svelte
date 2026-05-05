<script lang="ts">
  import { goto } from '$app/navigation';
  import { page as pageStore } from '$app/stores';
  import { get } from 'svelte/store';
  import { toast } from 'svelte-sonner';
  import TopBar from '$lib/components/TopBar.svelte';
  import GateBadge from '$lib/components/GateBadge.svelte';
  import { Button } from '$lib/components/ui/button/index.js';
  import { Input } from '$lib/components/ui/input/index.js';
  import { Badge } from '$lib/components/ui/badge/index.js';
  import { Switch } from '$lib/components/ui/switch/index.js';
  import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '$lib/components/ui/table/index.js';
  import {
    Select, SelectContent, SelectItem, SelectTrigger,
  } from '$lib/components/ui/select/index.js';
  import { api } from '$lib/api.js';
  import { fmtDate } from '$lib/utils.js';
  import type { Gate, Transaction } from '$lib/types.js';
  import { Search, RefreshCw, Eye } from 'lucide-svelte';

  const PAGE_LIMIT = 20;

  const _initialPage = get(pageStore);
  let transactions = $state<Transaction[]>([]);
  let total        = $state(0);
  let page         = $state(1);
  let search       = $state(_initialPage.url.searchParams.get('search') ?? '');
  let gateFilter   = $state(_initialPage.url.searchParams.get('gate_id') ?? '');
  let openOnly     = $state(_initialPage.url.searchParams.get('open') === 'true');
  let gates        = $state<Gate[]>([]);
  let loading      = $state(false);
  let selectedId   = $state('');
  let prevTotal    = $state(-1);

  const totalPages = $derived(Math.ceil(total / PAGE_LIMIT) || 1);

  async function loadGates() {
    try { gates = await api.gates.list(); } catch {}
  }

  async function loadTransactions() {
    loading = true;
    try {
      const res = await api.transactions.list({
        page, limit: PAGE_LIMIT,
        ...(gateFilter && { gate_id: gateFilter }),
        ...(openOnly   && { open: 'true' }),
        ...(search     && { search }),
      });
      if (prevTotal >= 0 && res.total > prevTotal) toast.success('Нова транзакція розпочата');
      prevTotal = res.total;
      transactions = res.data ?? [];
      total = res.total;
    } catch {
      toast.error('Помилка завантаження транзакцій');
    } finally {
      loading = false;
    }
  }

  $effect(() => { loadGates(); });

  $effect(() => {
    const _deps = [page, search, gateFilter, openOnly];
    const params = new URLSearchParams();
    if (gateFilter) params.set('gate_id', gateFilter);
    if (openOnly)   params.set('open', 'true');
    if (search)     params.set('search', search);
    const qs = params.toString();
    const newUrl = qs ? `/?${qs}` : '/';
    if (window.location.search !== (qs ? `?${qs}` : '')) {
      history.replaceState(null, '', newUrl);
    }
    loadTransactions();
    const id = setInterval(loadTransactions, 10_000);
    return () => clearInterval(id);
  });

</script>

<TopBar crumbs={['OmniGate', 'Транзакції']} title="Транзакції">
  {#snippet actions()}
    <Button variant="outline" size="sm" onclick={loadTransactions} disabled={loading}>
      <RefreshCw size={14} class={loading ? 'animate-spin' : ''} />
      Оновити
    </Button>
  {/snippet}
</TopBar>

<main class="flex-1 p-6 space-y-4">
  <!-- Toolbar -->
  <div class="flex items-center gap-2 flex-wrap">
    <div class="relative flex-1 max-w-[360px]">
      <Search size={14} class="absolute left-2.5 top-1/2 -translate-y-1/2 text-muted-foreground pointer-events-none" />
      <Input
        class="pl-8"
        placeholder="Пошук ID або номеру…"
        bind:value={search}
        oninput={() => { page = 1; }}
      />
    </div>

    <Select type="single" bind:value={gateFilter} onValueChange={() => { page = 1; }}>
      <SelectTrigger class="w-[200px]">
        {gateFilter ? gates.find(g => g.gate_id === gateFilter)?.name ?? gateFilter : 'Всі шлагбауми'}
      </SelectTrigger>
      <SelectContent>
        <SelectItem value="">Всі шлагбауми</SelectItem>
        {#each gates as g}
          <SelectItem value={g.gate_id}>{g.name}</SelectItem>
        {/each}
      </SelectContent>
    </Select>

    <label class="flex items-center gap-2 text-sm cursor-pointer select-none">
      <Switch bind:checked={openOnly} onCheckedChange={() => { page = 1; }} />
      Тільки відкриті
    </label>
  </div>

  <!-- Table -->
  <div class="rounded-md border border-border overflow-hidden overflow-x-auto">
    <Table>
      <TableHeader>
        <TableRow>
          <TableHead class="w-[110px]">Код</TableHead>
          <TableHead class="w-[130px]">Час</TableHead>
          <TableHead class="hidden sm:table-cell w-[160px]">Шлагбаум</TableHead>
          <TableHead class="hidden sm:table-cell">Події</TableHead>
          <TableHead class="w-[90px]"></TableHead>
          <TableHead class="w-[48px]"></TableHead>
        </TableRow>
      </TableHeader>
      <TableBody>
        {#each transactions as tx (tx.id)}
          {@const sel = tx.id === selectedId}
          <TableRow
            onclick={() => selectedId = tx.id}
            ondblclick={() => goto(`/transactions/${tx.id}`)}
            class="cursor-pointer {sel ? 'bg-primary/5' : ''}"
            style={sel ? 'box-shadow: inset 2px 0 0 hsl(var(--primary))' : undefined}
          >
            <TableCell class="font-mono text-sm">{tx.code}</TableCell>
            <TableCell>
              <div class="leading-none">
                <div class="font-semibold tabular-nums">
                  {new Date(tx.created_at).toLocaleTimeString('uk-UA', { hour: '2-digit', minute: '2-digit' })}
                </div>
                <div class="text-xs text-muted-foreground mt-0.5">{fmtDate(tx.created_at)}</div>
              </div>
            </TableCell>
            <TableCell class="hidden sm:table-cell"><GateBadge gateId={tx.gate_id} dot /></TableCell>
            <TableCell class="hidden sm:table-cell text-muted-foreground text-sm">
              {tx.events?.length ?? 0} {(tx.events?.length ?? 0) === 1 ? 'подія' : 'подій'}
            </TableCell>
            <TableCell>
              {#if tx.is_open}
                <Badge>Активна</Badge>
              {/if}
            </TableCell>
            <TableCell>
              <Button
                variant="ghost"
                size="icon-sm"
                onclick={(e: MouseEvent) => { e.stopPropagation(); goto(`/transactions/${tx.id}`); }}
              >
                <Eye size={15} />
              </Button>
            </TableCell>
          </TableRow>
        {/each}
        {#if transactions.length === 0}
          <TableRow>
            <TableCell colspan={6} class="py-10 text-center text-muted-foreground">
              {loading ? 'Завантаження…' : 'Транзакцій не знайдено'}
            </TableCell>
          </TableRow>
        {/if}
      </TableBody>
    </Table>
  </div>

  <!-- Pagination -->
  <div class="flex items-center justify-between text-sm text-muted-foreground">
    <span>{transactions.length} з {total} транзакцій</span>
    <div class="flex items-center gap-2">
      <Button variant="outline" size="sm" disabled={page <= 1 || loading} onclick={() => page--}>
        Назад
      </Button>
      <span class="px-2">Сторінка {page} з {totalPages}</span>
      <Button variant="outline" size="sm" disabled={page >= totalPages || loading} onclick={() => page++}>
        Далі
      </Button>
    </div>
  </div>
</main>
