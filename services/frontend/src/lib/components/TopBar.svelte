<script lang="ts">
  import type { Snippet } from 'svelte';

  let {
    crumbs = [] as string[],
    title = '',
    actions,
  }: {
    crumbs?: string[];
    title?: string;
    actions?: Snippet;
  } = $props();
</script>

<div class="h-[48px] border-b border-border bg-card flex items-center px-6 gap-4 sticky top-0 z-10 shrink-0">
  {#if crumbs.length}
    <div class="flex items-center gap-2 text-[12px] text-muted-foreground">
      {#each crumbs as crumb, i}
        {#if i > 0}
          <span class="text-border">/</span>
        {/if}
        <span class={i === crumbs.length - 1 ? 'text-foreground font-medium' : ''}>{crumb}</span>
      {/each}
    </div>
  {/if}
  {#if title}
    <h1 class="text-[18px] font-semibold tracking-[-0.01em]">{title}</h1>
  {/if}
  <div class="ml-auto flex items-center gap-2">
    {@render actions?.()}
  </div>
</div>
