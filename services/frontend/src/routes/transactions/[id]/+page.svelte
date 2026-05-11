<script lang="ts">
  import { page } from "$app/stores";
  import { goto } from "$app/navigation";
  import { toast } from "svelte-sonner";
  import TopBar from "$lib/components/TopBar.svelte";
  import GateBadge from "$lib/components/GateBadge.svelte";
  import AuthImg from "$lib/components/AuthImg.svelte";
  import { Button } from "$lib/components/ui/button/index.js";
  import { Badge } from "$lib/components/ui/badge/index.js";
  import { Textarea } from "$lib/components/ui/textarea/index.js";
  import { Card, CardContent } from "$lib/components/ui/card/index.js";
  import {
    Dialog,
    DialogContent,
    DialogHeader,
    DialogTitle,
  } from "$lib/components/ui/dialog/index.js";
  import { api } from "$lib/api.js";
  import { fmtDate, fmtTime, fmtDateTime } from "$lib/utils.js";
  import type { Transaction } from "$lib/types.js";
  import { ChevronLeft, Camera } from "lucide-svelte";

  const txId = $derived($page.params.id ?? "");

  let tx = $state<Transaction | null>(null);
  let loading = $state(true);
  let noteText = $state("");
  let savingNote = $state(false);
  let openPhoto = $state<{ key: string; label: string } | null>(null);

  $effect(() => {
    const id = txId;
    (async () => {
      loading = true;
      try {
        const res = await api.transactions.get(id);
        tx = res;
        noteText = res.note ?? "";
      } catch {
        toast.error("Транзакцію не знайдено");
        goto("/");
      } finally {
        loading = false;
      }
    })();
  });

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

  function tryFormatJson(s: string): string {
    try { return JSON.stringify(JSON.parse(s), null, 2); }
    catch { return s; }
  }

  // Lazy raw data: event id → fetched string (or null while loading)
  let rawCache = $state<Record<string, string | null>>({});

  async function loadRaw(eventId: string) {
    if (eventId in rawCache) return;
    rawCache[eventId] = null; // mark as loading
    try {
      rawCache[eventId] = await api.events.raw(eventId);
    } catch {
      rawCache[eventId] = '(помилка завантаження)';
    }
  }
</script>

<TopBar crumbs={["OmniGate", "Транзакції", tx?.code ?? "…"]}>
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

    <!-- Body: flex so photo sidebar only occupies space when photos exist -->
    <div class="flex flex-col lg:flex-row gap-6 items-start">

      <!-- ── LEFT: Timeline ── -->
      <div class="flex-1 min-w-0">
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
                  <div class="mt-[13px] w-3 h-3 rounded-full border-2 border-primary bg-background shrink-0 z-10"></div>
                  {#if i < sortedEvents.length - 1}
                    <div class="w-px flex-1 bg-border mt-1 min-h-[8px]"></div>
                  {/if}
                </div>

                <!-- Event card -->
                <div class="flex-1 pb-3 min-w-0">
                  <Card class="overflow-hidden">
                    <CardContent class="p-3 sm:p-4">

                      <!-- Card header: type name + source badge + time (mobile) -->
                      <div class="flex items-start justify-between gap-2 mb-2">
                        <div class="flex items-center gap-1.5 flex-wrap min-w-0">
                          <span class="text-sm font-semibold leading-tight">
                            {ev.event_type?.name ?? 'Подія'}
                          </span>
                          <Badge variant="outline" class="font-mono text-[11px] shrink-0 px-1.5">
                            {ev.source_id}
                          </Badge>
                        </div>
                        <!-- Mobile-only timestamp -->
                        <div class="sm:hidden shrink-0 text-right leading-none">
                          <div class="text-[11px] font-mono font-semibold tabular-nums">
                            {fmtTime(ev.created_at)}
                          </div>
                          <div class="text-[10px] text-muted-foreground mt-0.5">
                            {fmtDate(ev.created_at)}
                          </div>
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
                              onclick={() => (openPhoto = {
                                key,
                                label: `${ev.source_id} · ${fmtTime(ev.created_at)}`,
                              })}
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

      <!-- ── RIGHT: Photo gallery — only rendered when photos exist ── -->
      {#if allImages.length}
        <div class="w-full lg:w-[300px] shrink-0 lg:sticky lg:top-[52px] space-y-3">
          <div class="flex items-center justify-between">
            <h2 class="text-sm font-semibold uppercase tracking-wide text-muted-foreground">Фотодокази</h2>
            <span class="text-xs text-muted-foreground">{allImages.length} знімків</span>
          </div>
          <div class="grid grid-cols-2 sm:grid-cols-4 lg:grid-cols-2 gap-2">
            {#each allImages as img}
              <button
                onclick={() => (openPhoto = img)}
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
  </main>
{/if}

<!-- Photo lightbox -->
<Dialog
  open={!!openPhoto}
  onOpenChange={(v) => {
    if (!v) openPhoto = null;
  }}
>
  <DialogContent class="max-w-2xl">
    {#if openPhoto}
      <DialogHeader>
        <DialogTitle
          class="font-mono text-xs font-normal text-muted-foreground"
          >{openPhoto.label}</DialogTitle
        >
      </DialogHeader>
      <div
        class="aspect-[4/3] w-full rounded-md border border-border overflow-hidden bg-[#1e293b]"
      >
        <AuthImg
          src={api.imageUrl(openPhoto.key)}
          alt={openPhoto.label}
          class="w-full h-full object-contain"
        />
      </div>
    {/if}
  </DialogContent>
</Dialog>
