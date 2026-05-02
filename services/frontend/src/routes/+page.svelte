<script lang="ts">
  import { goto } from '$app/navigation';
  import { toast } from 'svelte-sonner';
  import TopBar from '$lib/components/TopBar.svelte';
  import GateBadge from '$lib/components/GateBadge.svelte';
  import { Button } from '$lib/components/ui/button/index.js';
  import { Input } from '$lib/components/ui/input/index.js';
  import { Badge } from '$lib/components/ui/badge/index.js';
  import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '$lib/components/ui/table/index.js';
  import {
    Select, SelectContent, SelectItem, SelectTrigger,
  } from '$lib/components/ui/select/index.js';
  import { api } from '$lib/api.js';
  import { fmtDate } from '$lib/utils.js';
  import type { Gate, Transaction } from '$lib/types.js';
  import { Search, RefreshCw, Eye } from 'lucide-svelte';

  const PAGE_LIMIT = 20;

  let transactions = $state<Transaction[]>([]);
  let total        = $state(0);
  let page         = $state(1);
  let search       = $state('');
  let gateFilter   = $state('');
  let statusFilter = $state('');
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
        ...(gateFilter   && { gate_id: gateFilter }),
        ...(statusFilter && { status: statusFilter }),
        ...(search       && { search }),
      });
      if (prevTotal >= 0 && res.total > prevTotal) toast.success('New transaction started');
      prevTotal = res.total;
      transactions = res.data ?? [];
      total = res.total;
    } catch {
      toast.error('Failed to load transactions');
    } finally {
      loading = false;
    }
  }

  $effect(() => { loadGates(); });

  $effect(() => {
    const _deps = [page, search, gateFilter, statusFilter];
    loadTransactions();
    const id = setInterval(loadTransactions, 10_000);
    return () => clearInterval(id);
  });

  const statusVariant = (s: string): 'default' | 'secondary' | 'destructive' | 'outline' =>
    s === 'active' ? 'default' : s === 'cancelled' ? 'destructive' : 'secondary';

  const statusLabel: Record<string, string> = {
    active: 'Active', completed: 'Closed', cancelled: 'Cancelled',
  };
</script>

<TopBar crumbs={['OmniGate', 'Transactions']} title="Transactions">
  {#snippet actions()}
    <Button variant="outline" size="sm" onclick={loadTransactions} disabled={loading}>
      <RefreshCw size={14} class={loading ? 'animate-spin' : ''} />
      Refresh
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
        placeholder="Search ID or plate…"
        bind:value={search}
        oninput={() => { page = 1; }}
      />
    </div>

    <Select type="single" bind:value={gateFilter} onValueChange={() => { page = 1; }}>
      <SelectTrigger class="w-[180px]">
        {gateFilter ? gates.find(g => g.gate_id === gateFilter)?.name ?? gateFilter : 'All gates'}
      </SelectTrigger>
      <SelectContent>
        <SelectItem value="">All gates</SelectItem>
        {#each gates as g}
          <SelectItem value={g.gate_id}>{g.name}</SelectItem>
        {/each}
      </SelectContent>
    </Select>

    <Select type="single" bind:value={statusFilter} onValueChange={() => { page = 1; }}>
      <SelectTrigger class="w-[160px]">
        {statusLabel[statusFilter] ?? 'All statuses'}
      </SelectTrigger>
      <SelectContent>
        <SelectItem value="">All statuses</SelectItem>
        <SelectItem value="active">Active</SelectItem>
        <SelectItem value="completed">Completed</SelectItem>
        <SelectItem value="cancelled">Cancelled</SelectItem>
      </SelectContent>
    </Select>
  </div>

  <!-- Table -->
  <div class="rounded-md border border-border overflow-hidden">
    <Table>
      <TableHeader>
        <TableRow>
          <TableHead class="w-[110px]">Code</TableHead>
          <TableHead class="w-[120px]">Time</TableHead>
          <TableHead class="w-[160px]">Gate</TableHead>
          <TableHead>Events</TableHead>
          <TableHead class="w-[110px]">Status</TableHead>
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
            <TableCell class="font-mono text-[12px]">{tx.code}</TableCell>
            <TableCell>
              <div class="leading-none">
                <div class="font-semibold tabular-nums">
                  {new Date(tx.created_at).toLocaleTimeString('en-GB', { hour: '2-digit', minute: '2-digit' })}
                </div>
                <div class="text-[11px] text-muted-foreground mt-0.5">{fmtDate(tx.created_at)}</div>
              </div>
            </TableCell>
            <TableCell><GateBadge gateId={tx.gate_id} dot /></TableCell>
            <TableCell class="text-muted-foreground text-[12px]">
              {tx.events?.length ?? 0} event{tx.events?.length === 1 ? '' : 's'}
            </TableCell>
            <TableCell>
              <Badge variant={statusVariant(tx.status)}>
                {statusLabel[tx.status] ?? tx.status}
              </Badge>
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
              {loading ? 'Loading…' : 'No transactions found'}
            </TableCell>
          </TableRow>
        {/if}
      </TableBody>
    </Table>
  </div>

  <!-- Pagination -->
  <div class="flex items-center justify-between text-[12px] text-muted-foreground">
    <span>{transactions.length} of {total} transactions</span>
    <div class="flex items-center gap-2">
      <Button variant="outline" size="sm" disabled={page <= 1 || loading} onclick={() => page--}>
        Previous
      </Button>
      <span class="px-2">Page {page} of {totalPages}</span>
      <Button variant="outline" size="sm" disabled={page >= totalPages || loading} onclick={() => page++}>
        Next
      </Button>
    </div>
  </div>
</main>
