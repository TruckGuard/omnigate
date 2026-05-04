<script lang="ts">
  import { goto } from '$app/navigation';
  import { toast } from 'svelte-sonner';
  import TopBar from '$lib/components/TopBar.svelte';
  import GateBadge from '$lib/components/GateBadge.svelte';
  import PermGuard from '$lib/components/PermGuard.svelte';
  import { Button } from '$lib/components/ui/button/index.js';
  import { Badge } from '$lib/components/ui/badge/index.js';
  import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '$lib/components/ui/table/index.js';
  import { api } from '$lib/api.js';
  import type { DeviceConfig, Gate } from '$lib/types.js';
  import { Plus, Settings } from 'lucide-svelte';

  let configs = $state<DeviceConfig[]>([]);
  let gates   = $state<Gate[]>([]);
  let loading = $state(true);

  $effect(() => {
    (async () => {
      try { [configs, gates] = await Promise.all([api.configs.list(), api.gates.list()]); }
      catch { toast.error('Failed to load devices'); }
      finally { loading = false; }
    })();
  });
</script>

<TopBar crumbs={['OmniGate', 'Devices']} title="Devices">
  {#snippet actions()}
    <PermGuard permission="manage:keys">
      <Button size="sm" onclick={() => goto('/settings/devices/new')}>
        <Plus size={14} /> Add device
      </Button>
    </PermGuard>
  {/snippet}
</TopBar>

<main class="flex-1 p-6">
  <div class="rounded-md border border-border overflow-hidden">
    <Table>
      <TableHeader>
        <TableRow>
          <TableHead>Source ID</TableHead>
          <TableHead class="w-[160px]">Gate</TableHead>
          <TableHead class="w-[150px]">Event type</TableHead>
          <TableHead class="w-[100px]">Trigger</TableHead>
          <TableHead class="w-[90px]">Status</TableHead>
          <TableHead class="w-[48px]"></TableHead>
        </TableRow>
      </TableHeader>
      <TableBody>
        {#each configs as cfg (cfg.id)}
          <TableRow class="cursor-pointer" onclick={() => goto(`/settings/devices/${cfg.id}`)}>
            <TableCell class="font-mono text-[12px]">{cfg.source_id}</TableCell>
            <TableCell>
              {@const g = gates.find(x => x.gate_id === cfg.gate_id)}
              <GateBadge gateId={cfg.gate_id} name={g?.name ?? ''} href={g ? `/settings/gates/${g.id}` : undefined} />
            </TableCell>
            <TableCell>
              {#if cfg.event_type}
                <Badge variant="outline">{cfg.event_type.code}</Badge>
              {:else}
                <span class="text-muted-foreground text-[12px]">—</span>
              {/if}
            </TableCell>
            <TableCell>
              {@const triggeredBy = configs.find(c => c.trigger_source_id === cfg.source_id && c.source_id !== cfg.source_id)}
              {@const triggers = cfg.trigger_source_id ? configs.find(c => c.source_id === cfg.trigger_source_id) : null}
              {#if triggers}
                <span class="text-[11px] font-mono text-muted-foreground">→ {triggers.source_id}</span>
              {:else if triggeredBy}
                <span class="text-[11px] font-mono text-muted-foreground">← {triggeredBy.source_id}</span>
              {:else if cfg.trigger_enabled}
                <Badge variant="outline" class="text-[10px]">URL</Badge>
              {:else}
                <span class="text-[12px] text-muted-foreground">—</span>
              {/if}
            </TableCell>
            <TableCell>
              <Badge variant={cfg.enabled ? 'default' : 'secondary'}>
                {cfg.enabled ? 'Active' : 'Disabled'}
              </Badge>
            </TableCell>
            <TableCell>
              <Button variant="ghost" size="icon-sm"><Settings size={15} /></Button>
            </TableCell>
          </TableRow>
        {/each}
        {#if !loading && configs.length === 0}
          <TableRow>
            <TableCell colspan={6} class="py-10 text-center text-muted-foreground">
              No devices configured.
            </TableCell>
          </TableRow>
        {/if}
      </TableBody>
    </Table>
  </div>
</main>
