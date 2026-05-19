<script lang="ts">
  import { authStore } from '$lib/stores/auth.svelte.js';

  let { src, alt = '', class: cls = '', ...rest }: {
    src: string;
    alt?: string;
    class?: string;
    [key: string]: unknown;
  } = $props();

  let blobUrl = $state<string | null>(null);
  let error   = $state(false);

  $effect(() => {
    const url = src;
    if (!url) return;

    let revoked = false;
    error = false;
    blobUrl = null;

    fetch(url, {
      headers: authStore.sessionId
        ? { Authorization: `Bearer ${authStore.sessionId}` }
        : {},
    })
      .then(r => {
        if (!r.ok) throw new Error(String(r.status));
        return r.blob();
      })
      .then(blob => {
        if (revoked) return;
        blobUrl = URL.createObjectURL(blob);
      })
      .catch(() => {
        if (!revoked) error = true;
      });

    return () => {
      revoked = true;
      if (blobUrl) URL.revokeObjectURL(blobUrl);
    };
  });
</script>

{#if blobUrl}
  <img {alt} src={blobUrl} class={cls} {...rest} />
{:else if error}
  <div class="flex items-center justify-center bg-muted text-muted-foreground text-[11px] {cls}">
    Не вдалося завантажити
  </div>
{:else}
  <div class="bg-muted animate-pulse {cls}"></div>
{/if}
