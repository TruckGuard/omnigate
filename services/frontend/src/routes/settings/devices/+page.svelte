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
  import { Plus, Settings, Zap } from 'lucide-svelte';

  let configs    = $state<DeviceConfig[]>([]);
  let gates      = $state<Gate[]>([]);
  let apiKeys    = $state<APIKey[]>([]);
  let loading    = $state(true);
  let triggeringId = $state('');

  $effect(() => {
    (async () => {
      try {
        [configs, gates] = await Promise.all([api.configs.list(), api.gates.list()]);
      } catch {
        toast.error('Помилка завантаження пристроїв');
      } finally {
        loading = false;
      }
      // API ключі потрібні лише для відображення назви пристрою.
      // Якщо у користувача немає read:api-keys — мовчки пропускаємо;
      // deviceName() покаже source_id як fallback.
      try {
        apiKeys = await api.auth.keys.list();
      } catch { /* insufficient permissions — fallback to source_id */ }
    })();
  });

  function deviceName(cfg: DeviceConfig): string {
    const key = apiKeys.find(k => String(k.id) === cfg.source_id);
    return key?.owner_name ?? cfg.source_id;
  }

  async function handleTrigger(e: MouseEvent, cfgId: string) {
    e.stopPropagation();
    triggeringId = cfgId;
    try {
      await api.configs.trigger(cfgId);
      toast.success('Тригер(и) запущено');
    } catch {
      toast.error('Помилка запуску тригера');
    } finally {
      triggeringId = '';
    }
  }
</script>

<TopBar crumbs={[{label:'OmniGate',href:'/'},{label:'Пристрої'}]} title="Пристрої">
  {#snippet actions()}
    <PermGuard permission="create:devices">
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
          <TableHead class="hidden sm:table-cell w-[160px]">КПП</TableHead>
          <TableHead class="hidden sm:table-cell w-[150px]">Тип події</TableHead>
          <TableHead class="hidden md:table-cell w-[140px]">Тригери</TableHead>
          <TableHead class="w-[90px]">Статус</TableHead>
          <TableHead class="w-[88px]"></TableHead>
        </TableRow>
      </TableHeader>
      <TableBody>
        {#each configs as cfg (cfg.id)}
          {@const triggerCount = (cfg.triggers ?? []).filter(t => t.source_id).length}
          {@const isTriggering = triggeringId === cfg.id}
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
              {#if cfg.trigger_enabled && triggerCount > 0}
                <span class="text-sm text-muted-foreground font-mono">
                  → {triggerCount} {triggerCount === 1 ? 'пристрій' : 'пристрої'}
                </span>
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
            <TableCell class="flex items-center gap-1">
              {#if cfg.trigger_enabled && triggerCount > 0}
                <PermGuard permission="trigger:devices">
                  <Button
                    variant="ghost"
                    size="icon-sm"
                    title="Запустити тригер"
                    disabled={isTriggering}
                    onclick={(e: MouseEvent) => handleTrigger(e, cfg.id)}
                  >
                    <Zap size={15} class={isTriggering ? 'animate-pulse text-primary' : 'text-muted-foreground'} />
                  </Button>
                </PermGuard>
              {/if}
              <Button
                variant="ghost"
                size="icon-sm"
                onclick={(e: MouseEvent) => { e.stopPropagation(); goto(`/settings/devices/${cfg.id}`); }}
              >
                <Settings size={15} />
              </Button>
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
