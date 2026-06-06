<script lang="ts">
  import { page } from "$app/stores";
  import { goto } from "$app/navigation";
  import { toast } from "svelte-sonner";
  import TopBar from "$lib/components/TopBar.svelte";
  import GateBadge from "$lib/components/GateBadge.svelte";
  import AuthImg from "$lib/components/AuthImg.svelte";
  import PermGuard from "$lib/components/PermGuard.svelte";
  import { Button } from "$lib/components/ui/button/index.js";
  import { Badge } from "$lib/components/ui/badge/index.js";
  import { Textarea } from "$lib/components/ui/textarea/index.js";
  import { Card, CardContent } from "$lib/components/ui/card/index.js";
  import { api } from "$lib/api.js";
  import { fmtDate, fmtTime, fmtDateTime } from "$lib/utils.js";
  import type { Transaction, DeviceConfig, APIKey } from "$lib/types.js";
  import { authStore } from "$lib/stores/auth.svelte.js";
  import { ChevronLeft, ChevronRight, Camera, X, ExternalLink, StopCircle } from "lucide-svelte";

  const txId = $derived($page.params.id ?? "");

  let tx = $state<Transaction | null>(null);
  let prevId = $state<string | null>(null);
  let nextId = $state<string | null>(null);
  let loading = $state(true);
  let noteText = $state("");
  let savingNote = $state(false);
  let closing = $state(false);

  // Gallery: index into allImages; openPhoto is derived
  let galleryIdx = $state<number | null>(null);
  const openPhoto = $derived(galleryIdx !== null ? (allImages[galleryIdx] ?? null) : null);

  // Lightbox zoom/pan
  let imgScale = $state(1);
  let imgX = $state(0);
  let imgY = $state(0);
  let dragging = $state(false);
  let dragStart = $state({ x: 0, y: 0, ox: 0, oy: 0 });

  // Reset zoom whenever the active photo changes
  $effect(() => { galleryIdx; imgScale = 1; imgX = 0; imgY = 0; });

  function openImage(key: string) {
    const idx = allImages.findIndex(img => img.key === key);
    if (idx !== -1) galleryIdx = idx;
  }
  function closePhoto() { galleryIdx = null; imgScale = 1; imgX = 0; imgY = 0; }
  function prevImage() {
    if (!allImages.length) return;
    galleryIdx = galleryIdx === null ? 0 : (galleryIdx - 1 + allImages.length) % allImages.length;
  }
  function nextImage() {
    if (!allImages.length) return;
    galleryIdx = galleryIdx === null ? 0 : (galleryIdx + 1) % allImages.length;
  }

  function handleKeydown(e: KeyboardEvent) {
    if (galleryIdx !== null) {
      if (e.key === 'Escape') { closePhoto(); return; }
      if (e.key === 'ArrowLeft') { prevImage(); return; }
      if (e.key === 'ArrowRight') { nextImage(); return; }
    }
    if (e.shiftKey && e.key === 'ArrowLeft' && prevId) goto(`/transactions/${prevId}`);
    if (e.shiftKey && e.key === 'ArrowRight' && nextId) goto(`/transactions/${nextId}`);
  }

  let openingOriginal = $state(false);
  async function openOriginal() {
    if (!openPhoto) return;
    openingOriginal = true;
    try {
      const res = await fetch(api.imageUrl(openPhoto.key), {
        headers: authStore.sessionId ? { Authorization: `Bearer ${authStore.sessionId}` } : {},
      });
      if (!res.ok) throw new Error();
      const blob = await res.blob();
      const url = URL.createObjectURL(blob);
      window.open(url, '_blank');
      setTimeout(() => URL.revokeObjectURL(url), 60_000);
    } catch {
      toast.error('Не вдалося відкрити зображення');
    } finally {
      openingOriginal = false;
    }
  }

  function onDblClick() {
    if (imgScale > 1) { imgScale = 1; imgX = 0; imgY = 0; } else imgScale = 2.5;
  }
  function onMouseDown(e: MouseEvent) {
    if (e.button !== 0) return;
    dragging = true;
    dragStart = { x: e.clientX, y: e.clientY, ox: imgX, oy: imgY };
  }
  function onMouseMove(e: MouseEvent) {
    if (!dragging) return;
    imgX = dragStart.ox + (e.clientX - dragStart.x);
    imgY = dragStart.oy + (e.clientY - dragStart.y);
  }
  function onMouseUp() { dragging = false; }

  // Non-passive wheel + pinch via Svelte action (passive:false required for preventDefault)
  function lightboxInteract(node: HTMLElement) {
    let ld = 0, lmx = 0, lmy = 0;
    function wheel(e: WheelEvent) {
      e.preventDefault();
      imgScale = Math.max(1, Math.min(6, imgScale * (e.deltaY > 0 ? 0.85 : 1.15)));
      if (imgScale <= 1) { imgScale = 1; imgX = 0; imgY = 0; }
    }
    function touchStart(e: TouchEvent) {
      if (e.touches.length !== 2) return;
      e.preventDefault();
      ld = Math.hypot(e.touches[0].clientX - e.touches[1].clientX, e.touches[0].clientY - e.touches[1].clientY);
      lmx = (e.touches[0].clientX + e.touches[1].clientX) / 2;
      lmy = (e.touches[0].clientY + e.touches[1].clientY) / 2;
    }
    function touchMove(e: TouchEvent) {
      if (e.touches.length !== 2) return;
      e.preventDefault();
      const d = Math.hypot(e.touches[0].clientX - e.touches[1].clientX, e.touches[0].clientY - e.touches[1].clientY);
      const mx = (e.touches[0].clientX + e.touches[1].clientX) / 2;
      const my = (e.touches[0].clientY + e.touches[1].clientY) / 2;
      if (ld) imgScale = Math.max(1, Math.min(6, imgScale * d / ld));
      if (imgScale <= 1) { imgScale = 1; imgX = 0; imgY = 0; } else { imgX += mx - lmx; imgY += my - lmy; }
      ld = d; lmx = mx; lmy = my;
    }
    function touchEnd(e: TouchEvent) { if (e.touches.length < 2) ld = 0; }
    node.addEventListener('wheel', wheel, { passive: false });
    node.addEventListener('touchstart', touchStart, { passive: false });
    node.addEventListener('touchmove', touchMove, { passive: false });
    node.addEventListener('touchend', touchEnd);
    return { destroy() {
      node.removeEventListener('wheel', wheel);
      node.removeEventListener('touchstart', touchStart);
      node.removeEventListener('touchmove', touchMove);
      node.removeEventListener('touchend', touchEnd);
    }};
  }

  $effect(() => {
    const id = txId;
    (async () => {
      loading = true;
      galleryIdx = null;
      try {
        const res = await api.transactions.get(id);
        // Handle both new envelope format and legacy bare Transaction
        const txData = res.transaction ?? (res as unknown as Transaction);
        tx = txData;
        prevId = res.prev_id ?? null;
        nextId = res.next_id ?? null;
        noteText = txData.note ?? "";
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

  async function saveNote() {
    if (!tx) return;
    savingNote = true;
    try {
      await api.transactions.update(tx.id, { note: noteText });
      toast.success("Нотатку збережено");
    } catch {
      toast.error("Помилка збереження нотатки");
    } finally {
      savingNote = false;
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
        label: `${ev.source_id} · ${fmtTime(ev.created_at)}`,
      })),
    ),
  );

  // Required fields grouped by event — used for the summary sidebar card
  const summaryGroups = $derived.by(() => {
    type Group = {
      sourceName: string;
      sourceLink: string | null;
      eventName: string;
      eventCode: string;
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
        fields: required,
      });
    }
    return groups;
  });

  function tryFormatJson(s: string): string {
    try { return JSON.stringify(JSON.parse(s), null, 2); }
    catch { return s; }
  }

  // Device lookup
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

  // Lazy raw data: event id → fetched string (or null while loading)
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
</script>

<TopBar crumbs={[{label:'OmniGate',href:'/'},{label:'Транзакції',href:'/'},{label:tx?.code ?? '…'}]}>
  {#snippet actions()}
    <Button size="sm">Експорт</Button>
  {/snippet}
</TopBar>

{#if loading}
  <div class="flex-1 flex items-center justify-center text-muted-foreground">
    Завантаження…
  </div>
{:else if tx}
  <main class="flex-1 p-4 sm:p-6 space-y-4">

    <!-- Header row -->
    <div class="flex items-center gap-2 flex-wrap">
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

      <!-- Right-aligned controls: close + navigation -->
      <div class="ml-auto flex items-center gap-2 flex-wrap">
        {#if tx.is_open}
          <PermGuard permission="transactions:close">
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

    <!-- Transaction meta — compact horizontal strip -->
    <div class="flex flex-wrap items-center gap-x-5 gap-y-1 rounded-lg border border-border bg-card px-4 py-2.5 text-xs">
      <div class="flex items-center gap-1.5">
        <span class="text-muted-foreground">ID</span>
        <span class="font-mono">{tx.id}</span>
      </div>
      <div class="flex items-center gap-1.5">
        <span class="text-muted-foreground">Відкрито</span>
        <span>{fmtDateTime(tx.created_at)}</span>
      </div>
      <div class="flex items-center gap-1.5">
        <span class="text-muted-foreground">Шлагбаум</span>
        <GateBadge gateId={tx.gate_id} />
      </div>
      <span class="text-muted-foreground">
        {sortedEvents.length} {sortedEvents.length === 1 ? 'подія' : 'подій'}
        {#if allImages.length} · {allImages.length} фото{/if}
      </span>
    </div>

    <!-- Body: two-column grid -->
    <div class="grid grid-cols-1 lg:grid-cols-12 gap-6 items-start">

      <!-- ── LEFT: Timeline (8 cols) ── -->
      <div class="lg:col-span-8 min-w-0">
        <h2 class="text-sm font-semibold uppercase tracking-wide text-muted-foreground mb-3">
          Хронологія · {sortedEvents.length > 0 ? 'найновіші зверху' : ''}
        </h2>

        {#if sortedEvents.length}
          <div>
            {#each sortedEvents as ev, i (ev.id)}
              <div class="flex gap-3 sm:gap-5">

                <!-- Time column — desktop only -->
                <div class="hidden sm:flex flex-col items-end w-[52px] shrink-0 pt-[13px]">
                  <span class="text-[11px] font-mono font-semibold tabular-nums leading-none">
                    {fmtTime(ev.created_at)}
                  </span>
                  <span class="text-[10px] text-muted-foreground mt-0.5">
                    {fmtDate(ev.created_at)}
                  </span>
                </div>

                <!-- Dot + vertical line -->
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

                      <!-- Card header -->
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
                        <!-- Mobile-only timestamp -->
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

                      <!-- Image gallery — horizontal scroll with snapping -->
                      {#if ev.image_keys?.length}
                        <div class="flex gap-2 overflow-x-auto snap-x snap-mandatory pb-1.5 -mx-3 px-3 sm:-mx-4 sm:px-4">
                          {#each ev.image_keys as key (key)}
                            <button
                              type="button"
                              onclick={() => openImage(key)}
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

                      <!-- Raw payload collapsible — lazy loaded from Garage/S3 -->
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
          </div>
        {:else}
          <p class="text-sm text-muted-foreground py-4">Подій ще немає.</p>
        {/if}

        <!-- Note -->
        <div class="mt-5">
          <h2 class="text-sm font-semibold uppercase tracking-wide text-muted-foreground mb-2">Нотатка</h2>
          <Textarea bind:value={noteText} rows={3} placeholder="Додати нотатку про цю транзакцію…" />
          <div class="flex justify-end mt-2">
            <Button size="sm" onclick={saveNote} disabled={savingNote}>
              {savingNote ? "Збереження…" : "Зберегти нотатку"}
            </Button>
          </div>
        </div>
      </div>

      <!-- ── RIGHT: Sticky sidebar (4 cols) ── -->
      <div class="lg:col-span-4 space-y-4 lg:sticky lg:top-6">

        <!-- Summary card: required fields grouped by event -->
        {#if summaryGroups.length}
          <div class="space-y-2">
            <h2 class="text-sm font-semibold uppercase tracking-wide text-muted-foreground">Резюме</h2>
            <Card>
              <CardContent class="p-0 divide-y divide-border">
                {#each summaryGroups as g}
                  <div class="py-4 px-4 first:pt-0 last:pb-0 space-y-1">
                    <!-- Header: source link · event code -->
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
                    <!-- Fields -->
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

        <!-- Photo gallery sidebar -->
        {#if allImages.length}
          <div class="space-y-2">
            <div class="flex items-center justify-between">
              <h2 class="text-sm font-semibold uppercase tracking-wide text-muted-foreground">Фотодокази</h2>
              <span class="text-xs text-muted-foreground">{allImages.length} знімків</span>
            </div>
            <div class="grid grid-cols-2 sm:grid-cols-4 lg:grid-cols-2 gap-2">
              {#each allImages as img, idx}
                <button
                  onclick={() => (galleryIdx = idx)}
                  class="aspect-[4/3] w-full bg-muted rounded-md border border-border overflow-hidden relative hover:opacity-90 transition-opacity focus:outline-none focus-visible:ring-2 focus-visible:ring-primary/50"
                >
                  <AuthImg
                    src={api.imageUrl(img.key)}
                    alt={img.label}
                    class="absolute inset-0 w-full h-full object-cover"
                  />
                  <div class="absolute inset-0 flex items-end p-1.5 pointer-events-none bg-gradient-to-t from-black/40 to-transparent">
                    <span class="text-[10px] text-white/90 font-mono leading-tight">{img.label}</span>
                  </div>
                  <Camera size={13} class="absolute top-1.5 right-1.5 text-white/60 drop-shadow" />
                </button>
              {/each}
            </div>
          </div>
        {/if}

      </div>

    </div>
  </main>
{/if}

<svelte:window onkeydown={handleKeydown} />

<!-- Lightbox / Photo gallery modal -->
{#if openPhoto}
  <div class="fixed inset-0 z-50 bg-black/95 flex flex-col select-none" role="dialog" aria-modal="true">

    <!-- Top bar -->
    <div class="flex items-center justify-between px-4 py-2 shrink-0">
      <span class="font-mono text-xs text-white/50">{openPhoto.label}</span>
      <div class="flex items-center gap-2 text-white/50">
        <span class="text-[11px] tabular-nums">{Math.round(imgScale * 100)}%</span>
        {#if allImages.length > 1}
          <span class="text-[11px] tabular-nums">Фото {(galleryIdx ?? 0) + 1} з {allImages.length}</span>
        {/if}
        <button
          onclick={openOriginal}
          disabled={openingOriginal}
          class="flex items-center gap-1.5 text-[11px] px-2.5 py-1.5 rounded-md border border-white/20 hover:border-white/50 hover:text-white transition-colors disabled:opacity-40"
        >
          <ExternalLink size={13} />
          Оригінал
        </button>
        <button onclick={closePhoto} class="hover:text-white transition-colors p-1.5">
          <X size={18} />
        </button>
      </div>
    </div>

    <!-- Image area -->
    <div
      use:lightboxInteract
      role="presentation"
      class="flex-1 overflow-hidden flex items-center justify-center {imgScale > 1 ? (dragging ? 'cursor-grabbing' : 'cursor-grab') : 'cursor-zoom-in'}"
      onmousedown={onMouseDown}
      onmousemove={onMouseMove}
      onmouseup={onMouseUp}
      onmouseleave={onMouseUp}
      ondblclick={onDblClick}
      onclick={(e) => { if (e.target === e.currentTarget && imgScale === 1) closePhoto(); }}
    >
      <div
        class="transform-gpu"
        style="transform: translate({imgX}px, {imgY}px) scale({imgScale}); transition: {dragging ? 'none' : 'transform 0.1s ease'};"
      >
        <AuthImg
          src={api.imageUrl(openPhoto.key)}
          alt={openPhoto.label}
          class="block max-w-[99vw] max-h-[calc(100dvh-80px)] object-contain pointer-events-none"
        />
      </div>
    </div>

    <!-- Prev/Next arrows — only when multiple photos -->
    {#if allImages.length > 1}
      <button
        onclick={prevImage}
        class="absolute left-3 top-1/2 -translate-y-1/2 p-2.5 rounded-full bg-white/10 hover:bg-white/25 text-white transition-colors"
        aria-label="Попереднє фото"
      >
        <ChevronLeft size={22} />
      </button>
      <button
        onclick={nextImage}
        class="absolute right-3 top-1/2 -translate-y-1/2 p-2.5 rounded-full bg-white/10 hover:bg-white/25 text-white transition-colors"
        aria-label="Наступне фото"
      >
        <ChevronRight size={22} />
      </button>
    {/if}

    <!-- Hints -->
    <div class="shrink-0 flex items-center justify-center gap-5 py-2 text-[10px] text-white/20">
      <span>scroll / pinch — zoom</span>
      <span>двічі — 2.5×</span>
      <span>тягни — зсув</span>
      {#if allImages.length > 1}<span>← → — навігація</span>{/if}
      <span>ESC — закрити</span>
    </div>

  </div>
{/if}
