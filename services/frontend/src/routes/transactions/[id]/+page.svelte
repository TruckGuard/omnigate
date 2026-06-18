<script lang="ts">
  import { page } from "$app/stores";
  import { goto } from "$app/navigation";
  import { onMount } from "svelte";
  import { toast } from "svelte-sonner";
  import TopBar from "$lib/components/TopBar.svelte";
  import GateBadge from "$lib/components/GateBadge.svelte";
  import AuthImg from "$lib/components/AuthImg.svelte";
  import PermGuard from "$lib/components/PermGuard.svelte";
  import TransactionImageViewer from "$lib/components/TransactionImageViewer.svelte";
  import TransactionNotes from "$lib/components/TransactionNotes.svelte";
  import { Button } from "$lib/components/ui/button/index.js";
  import { Badge } from "$lib/components/ui/badge/index.js";
  import { Card, CardContent } from "$lib/components/ui/card/index.js";
  import { Separator } from "$lib/components/ui/separator/index.js";
  import { ScrollArea } from "$lib/components/ui/scroll-area/index.js";
  import { ResizablePaneGroup, ResizablePane, ResizableHandle } from "$lib/components/ui/resizable/index.js";
  import { api } from "$lib/api.js";
  import { fmtDate, fmtTime, fmtDateTime } from "$lib/utils.js";
  import type { Transaction, DeviceConfig, APIKey } from "$lib/types.js";
  import { authStore } from "$lib/stores/auth.svelte.js";
  import { ChevronLeft, ChevronRight, ExternalLink, StopCircle } from "lucide-svelte";

  const txId = $derived($page.params.id ?? "");

  let tx = $state<Transaction | null>(null);
  let prevId = $state<string | null>(null);
  let nextId = $state<string | null>(null);
  let loading = $state(true);
  let closing = $state(false);

  // Controlled by TransactionImageViewer via $bindable
  let galleryIdx = $state<number | null>(null);

  // ── Resizable layout persistence ──
  const LAYOUT_KEY = 'transaction-page-layout';
  const DEFAULT_SIZES: [number, number] = [55, 45];

  // Defer pane rendering until after onMount reads localStorage, so defaultSize
  // is already correct on first render and there is no layout shift.
  let panesMounted = $state(false);
  let paneSizes = $state<[number, number]>(DEFAULT_SIZES);

  onMount(() => {
    try {
      const raw = localStorage.getItem(LAYOUT_KEY);
      if (raw) {
        const parsed = JSON.parse(raw) as unknown[];
        if (
          Array.isArray(parsed) &&
          parsed.length === 2 &&
          typeof parsed[0] === 'number' &&
          typeof parsed[1] === 'number'
        ) {
          paneSizes = [parsed[0], parsed[1]];
        }
      }
    } catch { /* ignore corrupt/missing storage */ }
    panesMounted = true;
  });

  function saveLayout(sizes: number[]) {
    if (sizes.length === 2 && typeof sizes[0] === 'number' && typeof sizes[1] === 'number') {
      paneSizes = [sizes[0], sizes[1]];
    }
    try {
      localStorage.setItem(LAYOUT_KEY, JSON.stringify(sizes));
    } catch { /* ignore quota errors */ }
  }

  $effect(() => {
    const id = txId;
    (async () => {
      loading = true;
      galleryIdx = null;
      try {
        const res = await api.transactions.get(id);
        const txData = res.transaction ?? (res as unknown as Transaction);
        tx = txData;
        prevId = res.prev_id ?? null;
        nextId = res.next_id ?? null;
      } catch (err) {
        const msg = err instanceof Error ? err.message : String(err);
        const is404 = msg.startsWith('404');
        toast.error(is404 ? "Транзакцію не знайдено" : `Помилка завантаження: ${msg}`);
        if (is404) goto("/");
      } finally {
        loading = false;
      }
    })();
  });

  async function closeTransaction() {
    if (!tx) return;
    closing = true;
    try {
      await api.transactions.close(tx.id);
      tx = { ...tx, is_open: false };
      toast.success("Транзакцію закрито");
    } catch (err) {
      const raw = err instanceof Error ? err.message : String(err);
      const body = raw.replace(/^\d+:\s*/, '');
      let msg = 'Помилка закриття транзакції';
      try { msg = JSON.parse(body).error ?? msg; } catch { /* */ }
      toast.error(msg);
    } finally {
      closing = false;
    }
  }

  const sortedEvents = $derived(
    [...(tx?.events ?? [])].sort(
      (a, b) => new Date(b.created_at).getTime() - new Date(a.created_at).getTime(),
    ),
  );

  const allImages = $derived(
    sortedEvents.flatMap((ev) =>
      (ev.image_keys ?? []).map((key) => ({
        key,
        label: `${deviceFor(ev.source_id).name} · ${fmtTime(ev.created_at)}`,
      })),
    ),
  );

  const summaryGroups = $derived.by(() => {
    type Group = {
      sourceName: string;
      sourceLink: string | null;
      eventName: string;
      eventCode: string;
      createdAt: string;
      fields: { label: string; value: string }[];
    };
    const groups: Group[] = [];
    for (const ev of sortedEvents) {
      const evFields = ev.event_type?.fields;
      if (!evFields) continue;
      const required: { label: string; value: string }[] = [];
      for (const [key, field] of Object.entries(evFields)) {
        if (field.required && ev.data[key] !== undefined && ev.data[key] !== null && ev.data[key] !== '') {
          required.push({ label: field.name || key, value: String(ev.data[key]) });
        }
      }
      if (!required.length) continue;
      const dev = deviceFor(ev.source_id);
      groups.push({
        sourceName: dev.name,
        sourceLink: dev.config_id ? `/settings/devices/${dev.config_id}` : null,
        eventName: ev.event_type?.name ?? ev.type_code,
        eventCode: ev.event_type?.code ?? ev.type_code,
        createdAt: ev.created_at,
        fields: required,
      });
    }
    return groups;
  });

  // Device lookup helpers
  let allConfigs = $state<DeviceConfig[]>([]);
  let apiKeys    = $state<APIKey[]>([]);

  $effect(() => {
    api.configs.list().then(c => { allConfigs = c; }).catch(() => {});
    api.auth.keys.list().then(k => { apiKeys = k; }).catch(() => {});
  });

  function deviceFor(sourceId: string): { name: string; config_id: string | null } {
    const key = apiKeys.find(k => String(k.id) === sourceId);
    const cfg = allConfigs.find(c => c.source_id === sourceId);
    return { name: key?.owner_name ?? sourceId, config_id: cfg?.id ?? null };
  }

  // Lazy raw payload loading from S3
  let rawCache = $state<Record<string, string | null>>({});

  async function loadRaw(eventId: string) {
    if (eventId in rawCache) return;
    rawCache[eventId] = null;
    try {
      rawCache[eventId] = await api.events.raw(eventId);
    } catch {
      rawCache[eventId] = '(помилка завантаження)';
    }
  }

  function tryFormatJson(s: string): string {
    try { return JSON.stringify(JSON.parse(s), null, 2); }
    catch { return s; }
  }

  // Keyboard: Shift+Arrow navigates between transactions (lightbox keys are in TransactionImageViewer)
  function handleKeydown(e: KeyboardEvent) {
    if (e.shiftKey && e.key === 'ArrowLeft' && prevId) goto(`/transactions/${prevId}`);
    if (e.shiftKey && e.key === 'ArrowRight' && nextId) goto(`/transactions/${nextId}`);
  }
</script>

<svelte:window onkeydown={handleKeydown} />

<TopBar crumbs={[{ label: 'OmniGate', href: '/' }, { label: 'Транзакції', href: '/' }, { label: tx?.code ?? '…' }]}>
  {#snippet actions()}
    <Button size="sm" variant="outline">Експорт</Button>
  {/snippet}
</TopBar>

{#if loading}
  <div class="flex-1 flex items-center justify-center text-muted-foreground">
    Завантаження…
  </div>
{:else if tx}
  <!-- Full-height flex column, constrained to viewport minus the 52px TopBar -->
  <div class="flex flex-col overflow-hidden" style="height: calc(100dvh - 52px)">

    <!-- ── Header row ── -->
    <div class="shrink-0 flex items-center gap-2 flex-wrap px-4 sm:px-6 pt-4 pb-2">
      <Button variant="ghost" size="sm" onclick={() => goto("/")}>
        <ChevronLeft size={14} /> Назад
      </Button>
      <span class="font-mono text-sm font-semibold">{tx.code}</span>
      {#if tx.is_open}
        <Badge>Активна</Badge>
      {:else}
        <Badge variant="secondary">Закрита</Badge>
      {/if}
      <GateBadge gateId={tx.gate_id} dot />

      <div class="ml-auto flex items-center gap-2 flex-wrap">
        {#if tx.is_open}
          <PermGuard permission="close:transactions">
            <Button
              variant="outline"
              size="sm"
              class="text-destructive hover:text-destructive hover:bg-destructive/10 border-destructive/30"
              onclick={closeTransaction}
              disabled={closing}
            >
              <StopCircle size={14} />
              {closing ? 'Закриття…' : 'Закрити транзакцію'}
            </Button>
          </PermGuard>
        {/if}
        <div class="flex items-center gap-1">
          <Button
            variant="outline"
            size="sm"
            href={`/transactions/${prevId}`}
            disabled={!prevId}
            title="Попередня (Shift+←)"
          >
            <ChevronLeft size={14} /> Попередня
          </Button>
          <Button
            variant="outline"
            size="sm"
            href={`/transactions/${nextId}`}
            disabled={!nextId}
            title="Наступна (Shift+→)"
          >
            Наступна <ChevronRight size={14} />
          </Button>
        </div>
      </div>
    </div>

    <!-- ── Meta strip ── -->
    <div class="shrink-0 flex flex-wrap items-center gap-x-5 gap-y-1 mx-4 sm:mx-6 mb-3 rounded-lg border border-border bg-card px-4 py-2.5 text-xs">
      <div class="flex items-center gap-1.5">
        <span class="text-muted-foreground">ID</span>
        <span class="font-mono">{tx.id}</span>
      </div>
      <div class="flex items-center gap-1.5">
        <span class="text-muted-foreground">Відкрито</span>
        <span>{fmtDateTime(tx.created_at)}</span>
      </div>
      <div class="flex items-center gap-1.5">
        <span class="text-muted-foreground">КПП</span>
        <GateBadge gateId={tx.gate_id} />
      </div>
      <span class="text-muted-foreground">
        {sortedEvents.length} {sortedEvents.length === 1 ? 'подія' : 'подій'}
        {#if allImages.length} · {allImages.length} фото{/if}
      </span>
    </div>

    <!-- ── Resizable split layout ── -->
    <div class="flex-1 min-h-0 px-4 sm:px-6 pb-4">
      {#if panesMounted}
      <ResizablePaneGroup
        direction="horizontal"
        class="h-full rounded-lg border border-border overflow-hidden"
        onLayoutChange={saveLayout}
      >

        <!-- LEFT PANE: Events timeline -->
        <ResizablePane defaultSize={paneSizes[0]} minSize={30}>
          <div class="flex flex-col h-full">
            <!-- Pane header (non-scrolling) -->
            <div class="shrink-0 flex items-center gap-2 px-4 py-3 border-b border-border bg-card/50">
              <h2 class="text-sm font-semibold uppercase tracking-wide text-muted-foreground">
                Події
              </h2>
              <span class="text-[11px] font-mono text-muted-foreground/60">
                {sortedEvents.length > 0 ? `${sortedEvents.length} · найновіші зверху` : ''}
              </span>
            </div>

            <!-- Scrollable events list -->
            <div class="flex-1 min-h-0">
              <ScrollArea class="h-full">
                <div class="px-4 py-3">
                  {#if sortedEvents.length}
                    {#each sortedEvents as ev, i (ev.id)}
                      <div class="flex gap-3 sm:gap-5">

                        <!-- Time column (desktop) -->
                        <div class="hidden sm:flex flex-col items-end w-[52px] shrink-0 pt-[13px]">
                          <span class="text-[11px] font-mono font-semibold tabular-nums leading-none">
                            {fmtTime(ev.created_at)}
                          </span>
                          <span class="text-[10px] text-muted-foreground mt-0.5">
                            {fmtDate(ev.created_at)}
                          </span>
                        </div>

                        <!-- Timeline dot + connector line -->
                        <div class="flex flex-col items-center w-4 shrink-0">
                          <div class="mt-[13px] w-3 h-3 rounded-full border-2 border-primary bg-background shrink-0"></div>
                          {#if i < sortedEvents.length - 1}
                            <div class="w-px flex-1 bg-border mt-1 min-h-[8px]"></div>
                          {/if}
                        </div>

                        <!-- Event card -->
                        <div class="flex-1 pb-3 min-w-0">
                          <Card class="overflow-hidden">
                            <CardContent class="p-3 sm:p-4">

                              <div class="flex items-start justify-between gap-2 mb-2">
                                <div class="min-w-0 flex-1">
                                  <div class="flex items-center gap-x-1.5 flex-wrap">
                                    <span class="text-sm font-semibold leading-tight">{ev.event_type?.name ?? 'Подія'}</span>
                                    <span class="text-muted-foreground/40 text-xs leading-tight">·</span>
                                    {#if deviceFor(ev.source_id).config_id}
                                      <a
                                        href="/settings/devices/{deviceFor(ev.source_id).config_id}"
                                        class="inline-flex items-center gap-0.5 text-xs text-muted-foreground hover:text-primary transition-colors leading-tight"
                                      >
                                        {deviceFor(ev.source_id).name}
                                        <ExternalLink size={10} class="shrink-0 opacity-60" />
                                      </a>
                                    {:else}
                                      <span class="text-xs text-muted-foreground leading-tight">{deviceFor(ev.source_id).name}</span>
                                    {/if}
                                  </div>
                                  <div class="text-[10px] font-mono text-muted-foreground/50 mt-0.5 truncate">{ev.id}</div>
                                </div>
                                <!-- Mobile timestamp -->
                                <div class="sm:hidden shrink-0 text-right leading-none">
                                  <div class="text-[11px] font-mono font-semibold tabular-nums">{fmtTime(ev.created_at)}</div>
                                  <div class="text-[10px] text-muted-foreground mt-0.5">{fmtDate(ev.created_at)}</div>
                                </div>
                              </div>

                              <!-- Payload data grid -->
                              {#if ev.data && Object.keys(ev.data).length > 0}
                                <div class="grid grid-cols-[auto_1fr] gap-x-4 gap-y-0.5 mb-3">
                                  {#each Object.entries(ev.data) as [k, v]}
                                    <span class="text-[12px] text-muted-foreground leading-5">{k}</span>
                                    <span class="text-[12px] font-mono font-medium leading-5 break-all">{String(v)}</span>
                                  {/each}
                                </div>
                              {/if}

                              <!-- Inline image thumbnails — click to open lightbox -->
                              {#if ev.image_keys?.length}
                                <div class="flex gap-2 overflow-x-auto snap-x snap-mandatory pb-1.5 -mx-3 px-3 sm:-mx-4 sm:px-4">
                                  {#each ev.image_keys as key (key)}
                                    <button
                                      type="button"
                                      onclick={() => { galleryIdx = allImages.findIndex(img => img.key === key); }}
                                      class="h-36 sm:h-52 aspect-[4/3] shrink-0 rounded-md overflow-hidden border border-border bg-muted snap-start hover:opacity-90 transition-opacity focus:outline-none focus-visible:ring-2 focus-visible:ring-primary/50"
                                    >
                                      <AuthImg
                                        src={api.imageUrl(key)}
                                        alt="фото події"
                                        class="w-full h-full object-cover"
                                      />
                                    </button>
                                  {/each}
                                </div>
                              {/if}

                              <!-- Raw payload (lazy-loaded from S3) -->
                              {#if ev.raw_data_key}
                                <details class="mt-2" ontoggle={(e) => { if ((e.currentTarget as HTMLDetailsElement).open) loadRaw(ev.id); }}>
                                  <summary class="text-[11px] text-muted-foreground cursor-pointer select-none hover:text-foreground transition-colors inline-flex items-center gap-1">
                                    Сирі дані
                                  </summary>
                                  {#if ev.id in rawCache}
                                    {#if rawCache[ev.id] === null}
                                      <p class="mt-1.5 text-[10px] text-muted-foreground pl-1">Завантаження…</p>
                                    {:else}
                                      <pre class="mt-1.5 text-[10px] font-mono bg-muted rounded-md p-2.5 overflow-auto max-h-[160px] leading-relaxed">{tryFormatJson(rawCache[ev.id]!)}</pre>
                                    {/if}
                                  {/if}
                                </details>
                              {/if}

                            </CardContent>
                          </Card>
                        </div>

                      </div>
                    {/each}
                  {:else}
                    <p class="text-sm text-muted-foreground py-4">Подій ще немає.</p>
                  {/if}
                </div>
              </ScrollArea>
            </div>
          </div>
        </ResizablePane>

        <ResizableHandle withHandle />

        <!-- RIGHT PANE: Notes → Resume → Photos -->
        <ResizablePane defaultSize={paneSizes[1]} minSize={25}>
          <ScrollArea class="h-full">
            <div class="p-4 space-y-5">

              <!-- 1. Notes (top) -->
              <TransactionNotes txId={tx.id} initialNote={tx.note ?? ''} />

              <!-- 2. Resume summary -->
              {#if summaryGroups.length}
                <Separator />
                <div class="space-y-2">
                  <h2 class="text-sm font-semibold uppercase tracking-wide text-muted-foreground">Резюме</h2>
                  <Card>
                    <CardContent class="p-0 divide-y divide-border">
                      {#each summaryGroups as g}
                        <div class="py-3.5 px-4 first:pt-3.5 last:pb-3.5 space-y-1">
                          <div class="flex items-center justify-between gap-2 flex-wrap">
                            <div class="flex items-center gap-1 text-[11px] text-muted-foreground flex-wrap">
                              {#if g.sourceLink}
                                <a
                                  href={g.sourceLink}
                                  class="font-medium hover:text-primary transition-colors inline-flex items-center gap-0.5"
                                >
                                  {g.sourceName}<ExternalLink size={9} class="opacity-60 shrink-0" />
                                </a>
                              {:else}
                                <span class="font-medium">{g.sourceName}</span>
                              {/if}
                              <span class="text-muted-foreground/40">·</span>
                              <span class="font-mono">{g.eventCode}</span>
                            </div>
                            <span class="font-mono text-[10px] tabular-nums text-muted-foreground/60 shrink-0">
                              {fmtTime(g.createdAt)}
                              <span class="text-muted-foreground/40">{fmtDate(g.createdAt)}</span>
                            </span>
                          </div>
                          {#each g.fields as f}
                            <div class="flex gap-1.5 text-[12px] leading-5">
                              <span class="text-muted-foreground shrink-0">{f.label}:</span>
                              <span class="font-mono font-medium break-all">{f.value}</span>
                            </div>
                          {/each}
                        </div>
                      {/each}
                    </CardContent>
                  </Card>
                </div>
              {/if}

              <!-- 3. Photos (bottom) -->
              {#if allImages.length}
                <Separator />
                <TransactionImageViewer images={allImages} bind:galleryIdx />
              {/if}

            </div>
          </ScrollArea>
        </ResizablePane>

      </ResizablePaneGroup>
      {:else}
        <!-- Matches the pane group's visual footprint during SSR/pre-mount so there is no layout shift -->
        <div class="h-full rounded-lg border border-border" aria-hidden="true"></div>
      {/if}
    </div>

  </div>
{/if}
