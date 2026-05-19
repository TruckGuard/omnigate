<script lang="ts">
  import { Button } from '$lib/components/ui/button/index.js';
  import { Input } from '$lib/components/ui/input/index.js';
  import {
    Select, SelectContent, SelectItem, SelectTrigger,
  } from '$lib/components/ui/select/index.js';
  import type { EventTypeField, Event } from '$lib/types.js';
  import { api } from '$lib/api.js';
  import { Plus, Trash2, ChevronDown, ChevronRight } from 'lucide-svelte';

  let {
    value = $bindable({}),
    schema = {},
    rawEvent = undefined,
  }: {
    value: Record<string, string>;
    schema?: Record<string, EventTypeField>;
    rawEvent?: Event;
  } = $props();

  type Row = { key: string; path: string };

  let rows = $state<Row[]>([]);
  let rawOpen = $state(false);
  let initialized = $state(false);
  let rawSample = $state<string | null | undefined>(undefined); // undefined=not loaded, null=loading

  async function toggleRaw() {
    rawOpen = !rawOpen;
    if (rawOpen && rawSample === undefined && rawEvent?.raw_data_key) {
      rawSample = null;
      try {
        rawSample = await api.events.raw(rawEvent.id);
      } catch {
        rawSample = '(помилка завантаження)';
      }
    }
  }

  const schemaKeys = $derived(Object.keys(schema));

  // One-time init from incoming value prop
  $effect(() => {
    if (!initialized && Object.keys(value).length > 0) {
      rows = Object.entries(value).map(([key, path]) => ({ key, path }));
      initialized = true;
    }
  });

  // Sync rows → value
  $effect(() => {
    if (!initialized) return;
    const result: Record<string, string> = {};
    for (const r of rows) {
      if (r.key) result[r.key] = r.path;
    }
    value = result;
  });

  function addRow() {
    rows = [...rows, { key: '', path: '' }];
    initialized = true;
  }

  function removeRow(i: number) {
    rows = rows.filter((_, idx) => idx !== i);
  }

</script>

<div class="space-y-2">
  <div class="flex items-center justify-between mb-1">
    <p class="text-[12px] font-medium text-muted-foreground">Рядки маппінгу — JSONPath → поле</p>
    <Button variant="outline" size="sm" onclick={addRow}>
      <Plus size={12} /> Додати рядок
    </Button>
  </div>

  {#if rows.length === 0}
    <p class="text-[12px] text-muted-foreground py-2">Немає правил маппінгу. Додайте рядок для зіставлення полів даних пристрою.</p>
  {/if}

  {#each rows as row, i}
    <div class="flex flex-col sm:flex-row sm:items-center gap-2">
      {#if schemaKeys.length > 0}
        <div class="w-full sm:w-[180px] sm:shrink-0">
          <Select type="single" bind:value={row.key}>
            <SelectTrigger class="font-mono text-[12px] h-8 w-full">
              {row.key || 'Ключ поля…'}
            </SelectTrigger>
            <SelectContent>
              {#each schemaKeys as k}
                <SelectItem value={k} class="font-mono text-[12px]">{k}</SelectItem>
              {/each}
            </SelectContent>
          </Select>
        </div>
      {:else}
        <Input
          bind:value={row.key}
          placeholder="field_key"
          class="font-mono text-[12px] h-8 w-full sm:w-[180px] sm:shrink-0"
        />
      {/if}
      <span class="hidden sm:inline text-muted-foreground text-[12px] shrink-0">→</span>
      <Input
        bind:value={row.path}
        placeholder="$.path.to.value"
        class="font-mono text-[12px] h-8 flex-1"
      />
      <Button variant="ghost" size="icon-sm" class="hover:text-destructive shrink-0 self-end sm:self-auto" onclick={() => removeRow(i)}>
        <Trash2 size={12} />
      </Button>
    </div>
  {/each}

  {#if rawEvent?.raw_data_key}
    <div class="mt-3 border border-border rounded-md overflow-hidden">
      <button
        type="button"
        class="w-full flex items-center gap-2 px-3 py-2 text-[12px] font-medium text-muted-foreground hover:bg-muted transition-colors"
        onclick={toggleRaw}
      >
        {#if rawOpen}<ChevronDown size={13} />{:else}<ChevronRight size={13} />{/if}
        Приклад останньої події
        <span class="text-[11px] font-mono opacity-60">{rawEvent.source_id}</span>
      </button>
      {#if rawOpen}
        <div class="px-3 py-2 bg-muted/30 border-t border-border">
          {#if rawSample === null}
            <p class="text-[12px] text-muted-foreground">Завантаження…</p>
          {:else if rawSample}
            <pre class="text-[11px] font-mono text-foreground overflow-auto max-h-[300px] p-1">{rawSample}</pre>
          {:else}
            <p class="text-[12px] text-muted-foreground">Сирі дані для цієї події відсутні.</p>
          {/if}
        </div>
      {/if}
    </div>
  {/if}
</div>
