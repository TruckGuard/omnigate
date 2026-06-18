<script lang="ts">
  import { toast } from "svelte-sonner";
  import { Button } from "$lib/components/ui/button/index.js";
  import { Textarea } from "$lib/components/ui/textarea/index.js";
  import { api } from "$lib/api.js";
  import { untrack } from "svelte";
  import { Save } from "lucide-svelte";

  let {
    txId,
    initialNote,
  }: {
    txId: string;
    initialNote: string;
  } = $props();

  // untrack avoids the "captures initial value" warning while still seeding the state
  let noteText = $state(untrack(() => initialNote));
  let savedNote = $state(untrack(() => initialNote));
  let savingNote = $state(false);

  const isDirty = $derived(noteText !== savedNote);

  // Sync both buffers when the transaction changes (parent updates initialNote)
  $effect(() => {
    noteText = initialNote;
    savedNote = initialNote;
  });

  async function saveNote() {
    savingNote = true;
    try {
      await api.transactions.update(txId, { note: noteText });
      savedNote = noteText;
      toast.success("Нотатку збережено");
    } catch {
      toast.error("Помилка збереження нотатки");
    } finally {
      savingNote = false;
    }
  }
</script>

<div class="space-y-2.5">
  <div class="flex items-center justify-between gap-2">
    <div class="flex items-center gap-2 min-w-0">
      <h2 class="text-sm font-semibold uppercase tracking-wide text-muted-foreground">Нотатки</h2>
      {#if isDirty}
        <span
          class="inline-block w-2 h-2 rounded-full bg-amber-500 shrink-0 animate-pulse"
          title="Є незбережені зміни"
        ></span>
      {/if}
    </div>
    <Button
      size="sm"
      onclick={saveNote}
      disabled={savingNote || !isDirty}
      class="gap-1.5 shrink-0"
    >
      <Save size={13} />
      {savingNote ? "Збереження…" : "Зберегти"}
    </Button>
  </div>

  <Textarea
    bind:value={noteText}
    rows={5}
    placeholder="Додати нотатку про цю транзакцію…"
    class="resize-none text-sm"
  />

  {#if isDirty}
    <p class="text-[11px] text-muted-foreground">Натисніть «Зберегти», щоб записати зміни до бази даних.</p>
  {/if}
</div>
