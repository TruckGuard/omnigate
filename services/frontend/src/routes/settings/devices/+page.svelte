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
  import type { APIKey, DeviceConfig, Gate } from '$lib/types.js';
  import { Plus, Settings } from 'lucide-svelte';

  let configs = $state<DeviceConfig[]>([]);
  let gates   = $state<Gate[]>([]);
  let apiKeys = $state<APIKey[]>([]);
  let loading = $state(true);

  $effect(() => {
    (async () => {
      try {
        [configs, gates, apiKeys] = await Promise.all([
          api.configs.list(), api.gates.list(), api.auth.keys.list(),
        ]);
      } catch {
        toast.error('Помилка завантаження пристроїв');
      } finally {
        loading = false;
      }
    })();
  });

  function deviceName(cfg: DeviceConfig): string {
    const key = apiKeys.find(k => String(k.id) === cfg.source_id);
    return key?.owner_name ?? cfg.source_id;
  }
</script>

<TopBar crumbs={['OmniGate', 'Пристрої']} title="Пристрої">
  {#snippet actions()}
    <PermGuard permission="manage:keys">
      <Button size="sm" onclick={() => goto('/settings/devices/new')}>
        <Plus size={14} /> Додати пристрій
      </Button>
    </PermGuard>
  {/snippet}
</TopBar>

<main class="flex-1 p-6">
  <div class="rounded-md border border-border overflow-hidden overflow-x-auto">
    <Table>
      <TableHeader>
        <TableRow>
          <TableHead>Пристрій</TableHead>
          <TableHead class="hidden md:table-cell w-[120px] text-muted-foreground text-xs">Source ID</TableHead>
          <TableHead class="hidden sm:table-cell w-[160px]">Шлагбаум</TableHead>
          <TableHead class="hidden sm:table-cell w-[150px]">Тип події</TableHead>
          <TableHead class="hidden md:table-cell w-[120px]">Тригер</TableHead>
          <TableHead class="w-[90px]">Статус</TableHead>
          <TableHead class="w-[48px]"></TableHead>
        </TableRow>
      </TableHeader>
      <TableBody>
        {#each configs as cfg (cfg.id)}
          <TableRow class="cursor-pointer" onclick={() => goto(`/settings/devices/${cfg.id}`)}>
            <TableCell class="font-medium">{deviceName(cfg)}</TableCell>
            <TableCell class="hidden md:table-cell font-mono text-xs text-muted-foreground">{cfg.source_id}</TableCell>
            <TableCell class="hidden sm:table-cell">
              {@const g = gates.find(x => x.gate_id === cfg.gate_id)}
              <GateBadge gateId={cfg.gate_id} name={g?.name ?? ''} href={g ? `/settings/gates/${g.id}` : undefined} />
            </TableCell>
            <TableCell class="hidden sm:table-cell">
              {#if cfg.event_type}
                <Badge variant="outline">{cfg.event_type.code}</Badge>
              {:else}
                <span class="text-muted-foreground text-sm">—</span>
              {/if}
            </TableCell>
            <TableCell class="hidden md:table-cell">
              {@const triggeredBy = configs.find(c => c.trigger_source_id === cfg.source_id && c.source_id !== cfg.source_id)}
              {@const triggers = cfg.trigger_source_id ? configs.find(c => c.source_id === cfg.trigger_source_id) : null}
              {#if triggers}
                <span class="text-sm font-mono text-muted-foreground">→ {deviceName(triggers)}</span>
              {:else if triggeredBy}
                <span class="text-sm font-mono text-muted-foreground">← {deviceName(triggeredBy)}</span>
              {:else if cfg.trigger_enabled}
                <Badge variant="outline" class="text-xs">URL</Badge>
              {:else}
                <span class="text-sm text-muted-foreground">—</span>
              {/if}
            </TableCell>
            <TableCell>
              <Badge variant={cfg.enabled ? 'default' : 'secondary'}>
                {cfg.enabled ? 'Активний' : 'Вимкнений'}
              </Badge>
            </TableCell>
            <TableCell>
              <Button variant="ghost" size="icon-sm"><Settings size={15} /></Button>
            </TableCell>
          </TableRow>
        {/each}
        {#if !loading && configs.length === 0}
          <TableRow>
            <TableCell colspan={7} class="py-10 text-center text-muted-foreground">
              Пристроїв не налаштовано.
            </TableCell>
          </TableRow>
        {/if}
      </TableBody>
    </Table>
  </div>
</main>
