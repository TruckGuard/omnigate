<script lang="ts">
  import type { Snippet } from 'svelte';
  import { Menu } from 'lucide-svelte';
  import { mobileNav } from '$lib/stores/mobileNav.svelte.js';

  type Crumb = string | { label: string; href?: string };

  let {
    crumbs = [] as Crumb[],
    title = '',
    actions,
  }: {
    crumbs?: Crumb[];
    title?: string;
    actions?: Snippet;
  } = $props();

  function crumbLabel(c: Crumb): string {
    return typeof c === 'string' ? c : c.label;
  }

  function crumbHref(c: Crumb): string | undefined {
    return typeof c === 'string' ? undefined : c.href;
  }
</script>

<div class="h-[52px] w-full border-b border-border bg-card flex items-center px-4 gap-3 sticky top-0 z-10 shrink-0">
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
        {@const isLast = i === crumbs.length - 1}
        {@const href = crumbHref(crumb)}
        {@const label = crumbLabel(crumb)}
        {#if href}
          <a
            {href}
            class={isLast
              ? 'text-foreground font-medium hover:underline underline-offset-2'
              : 'hover:text-foreground transition-colors'}
          >{label}</a>
        {:else}
          <span class={isLast ? 'text-foreground font-medium' : ''}>{label}</span>
        {/if}
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
