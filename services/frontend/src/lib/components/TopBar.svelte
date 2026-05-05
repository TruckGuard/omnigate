<script lang="ts">
  import type { Snippet } from 'svelte';
  import { Menu } from 'lucide-svelte';
  import { mobileNav } from '$lib/stores/mobileNav.svelte.js';

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

<div class="h-[48px] border-b border-border bg-card flex items-center px-4 gap-3 sticky top-0 z-10 shrink-0">
  <!-- Hamburger: mobile only -->
  <button
    class="md:hidden flex items-center justify-center w-8 h-8 rounded-md hover:bg-muted text-muted-foreground shrink-0"
    onclick={() => mobileNav.toggle()}
    aria-label="Відкрити меню"
  >
    <Menu size={18} />
  </button>

  {#if crumbs.length}
    <div class="hidden sm:flex items-center gap-2 text-[12px] text-muted-foreground">
      {#each crumbs as crumb, i}
        {#if i > 0}
          <span class="text-border">/</span>
        {/if}
        <span class={i === crumbs.length - 1 ? 'text-foreground font-medium' : ''}>{crumb}</span>
      {/each}
    </div>
  {/if}
  {#if title}
    <h1 class="text-[16px] sm:text-[18px] font-semibold tracking-[-0.01em] truncate">{title}</h1>
  {/if}
  <div class="ml-auto flex items-center gap-2">
    {@render actions?.()}
  </div>
</div>
