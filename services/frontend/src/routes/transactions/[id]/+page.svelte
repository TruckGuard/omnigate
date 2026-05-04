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
  import {
    Card,
    CardContent,
    CardHeader,
    CardTitle,
  } from "$lib/components/ui/card/index.js";
  import {
    Dialog,
    DialogContent,
    DialogHeader,
    DialogTitle,
  } from "$lib/components/ui/dialog/index.js";
  import { api } from "$lib/api.js";
  import { fmtDate, fmtTime, fmtDateTime } from "$lib/utils.js";
  import type { Transaction } from "$lib/types.js";
  import { ChevronLeft, Camera, ChevronsRight } from "lucide-svelte";

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
        toast.error("Transaction not found");
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
      toast.success("Note saved");
    } catch {
      toast.error("Failed to save note");
    } finally {
      savingNote = false;
    }
  }

  const allImages = $derived(
    (tx?.events ?? []).flatMap((ev) =>
      (ev.image_keys ?? []).map((key) => ({
        key,
        label: `${ev.source_id} · ${fmtTime(ev.created_at)}`,
      })),
    ),
  );
</script>

<TopBar crumbs={["OmniGate", "Transactions", tx?.code ?? "…"]}>
  {#snippet actions()}
    <Button size="sm">Export</Button>
  {/snippet}
</TopBar>

{#if loading}
  <div class="flex-1 flex items-center justify-center text-muted-foreground">
    Loading…
  </div>
{:else if tx}
  <main class="flex-1 p-6 space-y-5">
    <div class="flex items-center gap-3 flex-wrap">
      <Button variant="ghost" size="sm" onclick={() => goto("/")}>
        <ChevronLeft size={14} /> Back
      </Button>
      <span class="font-mono text-[14px] font-semibold">{tx.code}</span>
      {#if tx.is_open}
        <Badge>Active</Badge>
      {:else}
        <Badge variant="secondary">Closed</Badge>
      {/if}
      <GateBadge gateId={tx.gate_id} dot />
      <span class="text-[12px] text-muted-foreground">
        opened {fmtTime(tx.created_at)} · {fmtDate(tx.created_at)} · {tx.events
          ?.length ?? 0} events · {allImages.length} photos
      </span>
    </div>

    <div class="grid grid-cols-[minmax(0,1fr)_minmax(0,1.1fr)] gap-6">
      <!-- Timeline -->
      <div>
        <h2 class="text-[15px] font-semibold mb-3">Timeline</h2>
        {#if tx.events?.length}
          <div class="relative pl-5">
            <div
              class="absolute left-1.5 top-1.5 bottom-1.5 w-px bg-border"
            ></div>
            <div class="space-y-3">
              {#each tx.events as ev (ev.id)}
                <div class="relative">
                  <span
                    class="absolute -left-[18px] top-3 w-2.5 h-2.5 rounded-full bg-background border-2 border-primary"
                  ></span>
                  <Card>
                    <CardContent>
                      <div class="flex items-baseline justify-between">
                        <div class="flex items-center gap-2">
                          <ChevronsRight
                            size={14}
                            class="text-muted-foreground shrink-0"
                          />
                          <span class="text-[13px] font-semibold"
                            >{ev.source_id}</span
                          >
                        </div>
                        <span
                          class="font-mono text-[11px] text-muted-foreground"
                          >{fmtTime(ev.created_at)}</span
                        >
                      </div>
                      <div class="mb-2 flex items-center gap-2">
                        <GateBadge gateId={ev.gate_id} />
                        <span
                          class="font-mono text-[11px] text-muted-foreground"
                          >{ev.id.slice(0, 8)}…</span
                        >
                        {#if ev.event_type}
                          <Badge variant="outline" class="text-[10px]"
                            >{ev.event_type.name}</Badge
                          >
                        {/if}
                      </div>
                      <div class="flex gap-4">
                        {#if Object.keys(ev.data).length > 0}
                          <div
                            class="grid grid-cols-[80px_1fr] gap-y-1 gap-x-3 text-[12px]"
                          >
                            {#each Object.entries(ev.data) as [k, v]}
                              <div class="text-muted-foreground">{k}</div>
                              <div class="font-mono">{String(v)}</div>
                            {/each}
                          </div>
                        {/if}
                        {#if ev.image_keys && ev.image_keys.length > 0}
                          <div class="flex gap-2 col-span-2">
                            {#each ev.image_keys as key (key)}
                              <button
                                type="button"
                                onclick={() => {
                                  openPhoto = {
                                    key,
                                    label: `${ev.source_id} · ${fmtTime(ev.created_at)}`,
                                  };
                                }}
                                class="w-20 h-20 rounded-lg overflow-hidden border shrink-0 bg-muted hover:bg-muted/80 transition-colors"
                              >
                                <AuthImg
                                  class="w-full h-full object-cover"
                                  src={`/data/${key}`}
                                  alt="photo"
                                />
                              </button>
                            {/each}
                          </div>
                        {/if}
                      </div>
                    </CardContent>
                  </Card>
                </div>
              {/each}
            </div>
          </div>
        {:else}
          <p class="text-[13px] text-muted-foreground">No events yet.</p>
        {/if}

        <!-- Note -->
        <div class="mt-5">
          <h2 class="text-[15px] font-semibold mb-2">Note</h2>
          <Textarea
            bind:value={noteText}
            rows={3}
            placeholder="Add a note about this transaction…"
          />
          <div class="flex justify-end mt-2">
            <Button size="sm" onclick={saveNote} disabled={savingNote}>
              {savingNote ? "Saving…" : "Save note"}
            </Button>
          </div>
        </div>
      </div>

      <!-- Photos + meta -->
      <div class="space-y-5">
        <div>
          <div class="flex items-center justify-between mb-3">
            <h2 class="text-[15px] font-semibold">Photo evidence</h2>
            <span class="text-[12px] text-muted-foreground"
              >{allImages.length} captures</span
            >
          </div>
          {#if allImages.length}
            <div class="grid grid-cols-4 gap-2.5">
              {#each allImages as img}
                <button
                  onclick={() => (openPhoto = img)}
                  class="aspect-[4/3] w-full bg-[#1e293b] rounded-md border border-border overflow-hidden relative focus:outline-none focus-visible:ring-2 focus-visible:ring-primary/50"
                >
                  <AuthImg
                    src={api.imageUrl(img.key)}
                    alt={img.label}
                    class="absolute inset-0 w-full h-full object-cover"
                  />
                  <div
                    class="absolute inset-0 flex items-end p-2 pointer-events-none"
                  >
                    <span class="text-[10px] text-white/75 font-mono"
                      >{img.label}</span
                    >
                  </div>
                  <Camera
                    size={18}
                    class="absolute top-1.5 right-1.5 text-white/60 drop-shadow"
                  />
                </button>
              {/each}
            </div>
          {:else}
            <p class="text-[13px] text-muted-foreground">No photos captured.</p>
          {/if}
        </div>

        <Card>
          <CardHeader class="pb-2">
            <CardTitle class="text-[15px]">Transaction info</CardTitle>
          </CardHeader>
          <CardContent>
            <div
              class="grid grid-cols-[100px_1fr] gap-y-1.5 gap-x-3 text-[13px]"
            >
              <span class="text-muted-foreground">ID</span>
              <span class="font-mono text-[12px]">{tx.id}</span>
              <span class="text-muted-foreground">Code</span>
              <span class="font-mono text-[12px]">{tx.code}</span>
              <span class="text-muted-foreground">Gate</span>
              <GateBadge gateId={tx.gate_id} />
              <span class="text-muted-foreground">Status</span>
              {#if tx.is_open}
                <Badge class="w-fit">Active</Badge>
              {:else}
                <Badge variant="secondary" class="w-fit">Closed</Badge>
              {/if}
              <span class="text-muted-foreground">Opened</span>
              <span>{fmtDateTime(tx.created_at)}</span>
            </div>
          </CardContent>
        </Card>
      </div>
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
          class="font-mono text-[12px] font-normal text-muted-foreground"
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

