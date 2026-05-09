<script lang="ts">
  import { goto } from '$app/navigation';
  import { page as pageState } from '$app/state';
  import { toast } from 'svelte-sonner';
  import TopBar from '$lib/components/TopBar.svelte';
  import GateBadge from '$lib/components/GateBadge.svelte';
  import VehicleHistory from '$lib/components/VehicleHistory.svelte';
  import { Button } from '$lib/components/ui/button/index.js';
  import { Input } from '$lib/components/ui/input/index.js';
  import { Badge } from '$lib/components/ui/badge/index.js';
  import { Switch } from '$lib/components/ui/switch/index.js';
  import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '$lib/components/ui/table/index.js';
  import { Select, SelectContent, SelectItem, SelectTrigger } from '$lib/components/ui/select/index.js';
  import { api } from '$lib/api.js';
  import { fmtDate } from '$lib/utils.js';
  import type { Gate, Transaction } from '$lib/types.js';
  import { Search, RefreshCw, Eye, History, X } from 'lucide-svelte';

  const PAGE_LIMIT = 20;

  const _initialPage = pageState;
  let transactions    = $state<Transaction[]>([]);
  let total           = $state(0);
  let page            = $state(1);
  // search — «сирий» стан поля вводу; оновлюється при кожному натисканні
  let search          = $state(_initialPage.url.searchParams.get('search') ?? '');
  // debouncedSearch — значення, яке реально йде в API; оновлюється через 300мс після останньої зміни.
  // Ініціалізуємо з URL напряму (не через search), щоб уникнути Svelte-попередження
  // про захоплення лише початкового значення реактивної змінної.
  let debouncedSearch = $state(_initialPage.url.searchParams.get('search') ?? '');
  let gateFilter      = $state(_initialPage.url.searchParams.get('gate_id') ?? '');
  let openOnly        = $state(_initialPage.url.searchParams.get('open') === 'true');
  let gates           = $state<Gate[]>([]);
  let loading         = $state(false);
  let selectedId      = $state('');
  let prevTotal       = $state(-1);

  // ─── Стан drawer'а з історією ───
  // null = закрито; рядок = номер авто, для якого показується історія
  let historyPlate = $state<string | null>(null);

  const totalPages = $derived(Math.ceil(total / PAGE_LIMIT) || 1);

  // Витягує номер авто з PlateEvent в подіях транзакції.
  // Повертає null, якщо транзакція не містить жодного PlateEvent з номером.
  function getPlate(tx: Transaction): string | null {
    for (const ev of tx.events ?? []) {
      if (ev.type_code === 'PlateEvent' && typeof ev.data?.plate === 'string') {
        return (ev.data.plate as string).trim();
      }
    }
    return null;
  }

  function openHistory(plate: string, e: MouseEvent) {
    e.stopPropagation();
    historyPlate = plate;
  }

  function closeHistory() {
    historyPlate = null;
  }

  async function loadGates() {
    try { gates = await api.gates.list(); } catch {}
  }

  async function loadTransactions() {
    loading = true;
    try {
      const res = await api.transactions.list({
        page, limit: PAGE_LIMIT,
        ...(gateFilter       && { gate_id: gateFilter }),
        ...(openOnly         && { open: 'true' }),
        // Передаємо дебаунсоване значення — не сирий search
        ...(debouncedSearch  && { search: debouncedSearch }),
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

  // Ефект дебаунсу: читає «сирий» search, через 300мс записує в debouncedSearch.
  // Cleanup скидає таймер при кожній новій зміні → запит іде лише після паузи.
  $effect(() => {
    const q = search;
    const t = setTimeout(() => {
      page            = 1;       // скидаємо пагінацію при новому запиті
      debouncedSearch = q;
    }, 300);
    return () => clearTimeout(t);
  });

  // Ефект завантаження: реагує на debouncedSearch (не на search),
  // тому не тригериться при кожному натисканні клавіші.
  // Також оновлює URL і встановлює інтервал авто-оновлення.
  $effect(() => {
    const params = new URLSearchParams();
    if (gateFilter)      params.set('gate_id', gateFilter);
    if (openOnly)        params.set('open', 'true');
    if (debouncedSearch) params.set('search', debouncedSearch);
    const qs = params.toString();
    if (window.location.search !== (qs ? `?${qs}` : '')) {
      history.replaceState(null, '', qs ? `/?${qs}` : '/');
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
        placeholder="Пошук за кодом або номером авто…"
        bind:value={search}
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
          <TableHead class="hidden md:table-cell">Номер авто</TableHead>
          <TableHead class="hidden sm:table-cell">Події</TableHead>
          <TableHead class="w-[90px]"></TableHead>
          <TableHead class="w-[80px]"></TableHead>
        </TableRow>
      </TableHeader>
      <TableBody>
        {#each transactions as tx (tx.id)}
          {@const sel   = tx.id === selectedId}
          {@const plate = getPlate(tx)}
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

            <!-- Колонка з номером авто та кнопкою "Історія" -->
            <TableCell class="hidden md:table-cell">
              {#if plate}
                <div class="flex items-center gap-1.5">
                  <span class="font-mono text-sm font-medium tracking-wide">{plate}</span>
                  <button
                    type="button"
                    title="Переглянути історію проїздів"
                    class="inline-flex items-center justify-center w-6 h-6 rounded text-muted-foreground hover:text-primary hover:bg-primary/10 transition-colors"
                    onclick={(e) => openHistory(plate, e)}
                  >
                    <History size={13} />
                  </button>
                </div>
              {:else}
                <span class="text-muted-foreground text-sm">—</span>
              {/if}
            </TableCell>

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
            <TableCell colspan={7} class="py-10 text-center text-muted-foreground">
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

<!-- ─── Drawer: Історія проїздів ─── -->
<!--
  Використовуємо CSS-переходи замість Svelte-transitions:
  drawer завжди в DOM, але invisible/translate-x-full коли закритий.
  Це дозволяє уникнути миготіння та зберігає стан між відкриттями.
-->
<div
  class="fixed inset-0 z-50 transition-all duration-300 {historyPlate !== null ? 'visible' : 'invisible pointer-events-none'}"
>
  <!-- Фоновий overlay -->
  <div
    role="button"
    tabindex="-1"
    aria-label="Закрити"
    class="absolute inset-0 bg-black/40 backdrop-blur-sm transition-opacity duration-300 {historyPlate !== null ? 'opacity-100' : 'opacity-0'}"
    onclick={closeHistory}
    onkeydown={(e) => e.key === 'Escape' && closeHistory()}
  ></div>

  <!-- Панель -->
  <aside
    class="absolute right-0 top-0 h-full w-[440px] max-w-[95vw] bg-background border-l border-border shadow-2xl
           flex flex-col transition-transform duration-300 ease-in-out
           {historyPlate !== null ? 'translate-x-0' : 'translate-x-full'}"
  >
    <!-- Шапка -->
    <div class="flex items-center justify-between px-5 py-4 border-b border-border shrink-0">
      <div class="flex items-center gap-2">
        <History size={18} class="text-primary" />
        <div>
          <h2 class="text-sm font-semibold leading-none">Історія проїздів</h2>
          {#if historyPlate}
            <p class="text-xs text-muted-foreground mt-1 font-mono">{historyPlate}</p>
          {/if}
        </div>
      </div>
      <button
        type="button"
        class="rounded-md p-1.5 text-muted-foreground hover:text-foreground hover:bg-accent transition-colors"
        onclick={closeHistory}
        aria-label="Закрити"
      >
        <X size={16} />
      </button>
    </div>

    <!-- Тіло (прокручується) -->
    <div class="flex-1 overflow-y-auto px-5 py-4">
      {#if historyPlate}
        <VehicleHistory plate={historyPlate} />
      {/if}
    </div>
  </aside>
</div>
