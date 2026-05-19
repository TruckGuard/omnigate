<script lang="ts">
  import type { Snippet } from 'svelte';
  import { Button } from '$lib/components/ui/button/index.js';
  import {
    Dialog, DialogContent, DialogHeader, DialogTitle,
    DialogFooter, DialogDescription,
  } from '$lib/components/ui/dialog/index.js';

  interface Props {
    open: boolean;
    title: string;
    description?: Snippet;
    onconfirm: () => void | Promise<void>;
    loading?: boolean;
  }

  let { open = $bindable(), title, description, onconfirm, loading = false }: Props = $props();
</script>

<Dialog bind:open>
  <DialogContent class="max-w-sm">
    <DialogHeader>
      <DialogTitle>{title}</DialogTitle>
      {#if description}
        <DialogDescription>{@render description()}</DialogDescription>
      {/if}
    </DialogHeader>
    <DialogFooter>
      <Button variant="outline" onclick={() => (open = false)}>Скасувати</Button>
      <Button variant="destructive" onclick={onconfirm} disabled={loading}>
        {loading ? 'Видалення…' : 'Видалити'}
      </Button>
    </DialogFooter>
  </DialogContent>
</Dialog>
