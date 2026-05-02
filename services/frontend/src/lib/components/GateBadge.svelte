<script lang="ts">
  import { gateColor, gateIndex } from '$lib/gateColors.js';

  let { gateId, name = '', dot = false, href = '' }: {
    gateId: string;
    name?: string;
    dot?: boolean;
    href?: string;
  } = $props();

  const c = $derived(gateColor(gateIndex(gateId)));
</script>

{#if href}
  <a
    {href}
    class="inline-flex items-center gap-1 rounded-full px-2 py-0.5 text-[11px] font-medium hover:opacity-80 transition-opacity"
    style="background:{c.bg}; color:{c.fg};"
  >
    {#if dot}
      <span class="w-1.5 h-1.5 rounded-full shrink-0" style="background:{c.dot};"></span>
    {/if}
    {gateId}{#if name}&nbsp;<span class="opacity-70">·</span>&nbsp;{name}{/if}
  </a>
{:else}
  <span
    class="inline-flex items-center gap-1 rounded-full px-2 py-0.5 text-[11px] font-medium"
    style="background:{c.bg}; color:{c.fg};"
  >
    {#if dot}
      <span class="w-1.5 h-1.5 rounded-full shrink-0" style="background:{c.dot};"></span>
    {/if}
    {gateId}{#if name}&nbsp;<span class="opacity-70">·</span>&nbsp;{name}{/if}
  </span>
{/if}
