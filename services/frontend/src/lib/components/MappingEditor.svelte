<script lang="ts">
  import { Button } from '$lib/components/ui/button/index.js';
  import { Input } from '$lib/components/ui/input/index.js';
  import {
    Select, SelectContent, SelectItem, SelectTrigger,
  } from '$lib/components/ui/select/index.js';
  import type { EventTypeField, Event } from '$lib/types.js';
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

  const rawDataPreview = $derived.by(() => {
    if (!rawEvent) return null;
    const d = rawEvent.data;
    if (d && typeof d === 'object' && Object.keys(d).length > 0) return d as Record<string, unknown>;
    return null;
  });
</script>

<div class="space-y-2">
  <div class="flex items-center justify-between mb-1">
    <p class="text-[12px] font-medium text-muted-foreground">Mapping rows — JSONPath → field key</p>
    <Button variant="outline" size="sm" onclick={addRow}>
      <Plus size={12} /> Add row
    </Button>
  </div>

  {#if rows.length === 0}
    <p class="text-[12px] text-muted-foreground py-2">No mapping rules. Add a row to map device data fields.</p>
  {/if}

  {#each rows as row, i}
    <div class="flex items-center gap-2">
      {#if schemaKeys.length > 0}
        <div class="w-[180px] shrink-0">
          <Select type="single" bind:value={row.key}>
            <SelectTrigger class="font-mono text-[12px] h-8">
              {row.key || 'Field key…'}
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
          class="font-mono text-[12px] h-8 w-[180px] shrink-0"
        />
      {/if}
      <span class="text-muted-foreground text-[12px] shrink-0">→</span>
      <Input
        bind:value={row.path}
        placeholder="$.path.to.value"
        class="font-mono text-[12px] h-8 flex-1"
      />
      <Button variant="ghost" size="icon-sm" class="hover:text-destructive shrink-0" onclick={() => removeRow(i)}>
        <Trash2 size={12} />
      </Button>
    </div>
  {/each}

  {#if rawEvent}
    <div class="mt-3 border border-border rounded-md overflow-hidden">
      <button
        type="button"
        class="w-full flex items-center gap-2 px-3 py-2 text-[12px] font-medium text-muted-foreground hover:bg-muted transition-colors"
        onclick={() => (rawOpen = !rawOpen)}
      >
        {#if rawOpen}<ChevronDown size={13} />{:else}<ChevronRight size={13} />{/if}
        Latest event sample
        <span class="text-[11px] font-mono opacity-60">{rawEvent.source_id}</span>
      </button>
      {#if rawOpen}
        <div class="px-3 py-2 bg-muted/30 border-t border-border">
          {#if rawDataPreview}
            <pre class="text-[11px] font-mono text-foreground overflow-auto max-h-[300px] p-1">{JSON.stringify(rawDataPreview, null, 2)}</pre>
          {:else}
            <p class="text-[12px] text-muted-foreground">No structured data available for this event.</p>
          {/if}
        </div>
      {/if}
    </div>
  {/if}
</div>
