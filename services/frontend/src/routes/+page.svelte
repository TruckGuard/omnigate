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
  import type { APIKey, DeviceConfig, Gate, Transaction } from '$lib/types.js';
  import DateRangePicker from '$lib/components/DateRangePicker.svelte';
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
  let startAt = $state(_initialPage.url.searchParams.get('start_at') ?? '');
  let endAt   = $state(_initialPage.url.searchParams.get('end_at') ?? '');
  let gates             = $state<Gate[]>([]);
  let devices           = $state<DeviceConfig[]>([]);
  let apiKeys           = $state<APIKey[]>([]);
  let selectedSourceIDs = $state<string[]>(
    _initialPage.url.searchParams.getAll('source_ids')
  );
  let loading           = $state(false);
  let selectedId      = $state('');
  let prevTotal       = $state(-1);

  // ─── Стан drawer'а з історією ───
  // null = закрито; рядок = номер авто, для якого показується історія
  let historyPlate = $state<string | null>(null);

  const totalPages = $derived(Math.ceil(total / PAGE_LIMIT) || 1);

  // Серед усіх searchable_value подій транзакції повертає те,
  // яке найімовірніше спричинило знаходження цієї транзакції при пошуку:
  //   1. точне включення query як підрядку — найкращий збіг
  //   2. найбільша кількість символів query, що зустрічаються в значенні по порядку (для нечіткого)
  function getSearchValue(tx: Transaction, query: string): string | null {
    const vals = (tx.events ?? [])
      .map(ev => ev.searchable_value)
      .filter((v): v is string => !!v);
    if (!vals.length) return null;
    const nq = query.toUpperCase().replace(/\s/g, '');
    if (!nq) return vals[0];
    const exact = vals.find(v => v.includes(nq));
    if (exact) return exact;
    // Greedy subsequence score: count how many chars of nq appear in v in order
    let best = vals[0], bestScore = 0;
    for (const v of vals) {
      let score = 0, j = 0;
      for (const ch of nq) { const i = v.indexOf(ch, j); if (i >= 0) { score++; j = i + 1; } }
      if (score > bestScore) { bestScore = score; best = v; }
    }
    return best;
  }

  // For fuzzy matches (hi === -1): returns char-level segments marking which
  // characters from value appear in query (greedy subsequence).
  function fuzzyParts(value: string, query: string): { text: string; matched: boolean }[] {
    const parts: { text: string; matched: boolean }[] = [];
    let qi = 0;
    for (let vi = 0; vi < value.length; vi++) {
      if (qi < query.length && value[vi] === query[qi]) {
        parts.push({ text: value[vi], matched: true });
        qi++;
      } else {
        parts.push({ text: value[vi], matched: false });
      }
    }
    return parts;
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

  async function loadDevices() {
    try { devices = await api.configs.list(); } catch {}
    // API-ключі потрібні лише для відображення назви пристрою.
    // Якщо немає дозволу read:api-keys — мовчки залишаємо порожнім; fallback → source_id.
    try { apiKeys = await api.auth.keys.list(); } catch {}
  }

  function deviceLabel(cfg: DeviceConfig): string {
    const key = apiKeys.find(k => String(k.id) === cfg.source_id);
    return key?.owner_name ?? cfg.source_id;
  }

  async function loadTransactions() {
    loading = true;
    try {
      const res = await api.transactions.list({
        page, limit: PAGE_LIMIT,
        ...(gateFilter                  && { gate_id: gateFilter }),
        ...(openOnly                    && { open: 'true' }),
        ...(debouncedSearch             && { search: debouncedSearch }),
        ...(startAt                     && { start_at: startAt }),
        ...(endAt                       && { end_at: endAt }),
        ...(selectedSourceIDs.length    && { source_ids: selectedSourceIDs }),
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

  $effect(() => { loadGates(); loadDevices(); });

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
    if (startAt) params.set('start_at', startAt);
    if (endAt)   params.set('end_at',   endAt);
    selectedSourceIDs.forEach(id => params.append('source_ids', id));
    const qs = params.toString();
    if (window.location.search !== (qs ? `?${qs}` : '')) {
      history.replaceState(null, '', qs ? `/?${qs}` : '/');
    }
    loadTransactions();
    const id = setInterval(loadTransactions, 10_000);
    return () => clearInterval(id);
  });
</script>

<TopBar crumbs={[{label:'OmniGate',href:'/'}]} title="Транзакції">
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
        {gateFilter ? gates.find(g => g.gate_id === gateFilter)?.name ?? gateFilter : 'Всі КПП'}
      </SelectTrigger>
      <SelectContent>
        <SelectItem value="">Всі КПП</SelectItem>
        {#each gates as g}
          <SelectItem value={g.gate_id}>{g.name}</SelectItem>
        {/each}
      </SelectContent>
    </Select>

    <DateRangePicker bind:startAt bind:endAt />

    {#if devices.length > 0}
      <Select type="multiple" bind:value={selectedSourceIDs} onValueChange={() => { page = 1; }}>
        <SelectTrigger class="w-[180px]">
          {#if selectedSourceIDs.length === 0}
            Всі пристрої
          {:else}
            {selectedSourceIDs.length} {selectedSourceIDs.length === 1 ? 'пристрій' : 'пристроїв'}
          {/if}
        </SelectTrigger>
        <SelectContent>
          {#each devices as d}
            <SelectItem value={d.source_id}>{deviceLabel(d)}</SelectItem>
          {/each}
        </SelectContent>
      </Select>
    {/if}

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
          <TableHead class="hidden sm:table-cell w-[160px]">КПП</TableHead>
          {#if debouncedSearch}
            <TableHead class="hidden md:table-cell w-[160px]">Знайдено</TableHead>
          {/if}
          <TableHead class="hidden sm:table-cell">Події</TableHead>
          <TableHead class="w-[90px]"></TableHead>
          <TableHead class="w-[80px]"></TableHead>
        </TableRow>
      </TableHeader>
      <TableBody>
        {#each transactions as tx (tx.id)}
          {@const sel      = tx.id === selectedId}
          {@const matchVal = debouncedSearch ? getSearchValue(tx, debouncedSearch) : null}
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

            {#if debouncedSearch}
              <TableCell class="hidden md:table-cell">
                {#if matchVal}
                  {@const nq = debouncedSearch.toUpperCase().replace(/\s/g, '')}
                  {@const hi = matchVal.indexOf(nq)}
                  <div class="flex items-center gap-1.5 font-mono text-sm font-medium tracking-wide">
                    {#if hi >= 0}
                      {matchVal.substring(0, hi)}<span class="text-primary bg-primary/10 rounded px-0.5">{matchVal.substring(hi, hi + nq.length)}</span>{matchVal.substring(hi + nq.length)}
                    {:else}
                      {@const parts = fuzzyParts(matchVal, nq)}
                      {#each parts as p}{#if p.matched}<span class="text-primary">{p.text}</span>{:else}<span class="text-muted-foreground">{p.text}</span>{/if}{/each}
                    {/if}
                    <button
                      type="button"
                      title="Переглянути історію проїздів"
                      class="inline-flex items-center justify-center w-6 h-6 rounded text-muted-foreground hover:text-primary hover:bg-primary/10 transition-colors"
                      onclick={(e) => openHistory(matchVal, e)}
                    >
                      <History size={13} />
                    </button>
                  </div>
                {:else}
                  <span class="text-muted-foreground text-xs">—</span>
                {/if}
              </TableCell>
            {/if}

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
            <TableCell colspan={debouncedSearch ? 7 : 6} class="py-10 text-center text-muted-foreground">
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
