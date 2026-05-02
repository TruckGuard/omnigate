<script lang="ts">
  import type { Snippet } from 'svelte';
  import { Label } from '$lib/components/ui/label/index.js';

  let {
    label = '',
    hint = '',
    error = '',
    children,
  }: {
    label?: string;
    hint?: string;
    error?: string;
    children?: Snippet;
  } = $props();
</script>

<div class="space-y-1.5">
  {#if label}
    <Label class="text-[12px] font-medium text-muted-foreground">{label}</Label>
  {/if}
  {@render children?.()}
  {#if error}
    <p class="text-[11px] text-destructive">{error}</p>
  {:else if hint}
    <p class="text-[11px] text-muted-foreground">{hint}</p>
  {/if}
</div>
