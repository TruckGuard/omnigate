<script lang="ts">
  import type { Snippet } from 'svelte';
  import { authStore } from '$lib/stores/auth.svelte.js';

  let { permission, children }: { permission: string; children: Snippet } = $props();
</script>

{#if authStore.can(permission)}
  {@render children()}
{/if}
