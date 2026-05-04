<script lang="ts">
  import { page } from '$app/stores';
  import { goto } from '$app/navigation';
  import { toast } from 'svelte-sonner';
  import TopBar from '$lib/components/TopBar.svelte';
  import Field from '$lib/components/Field.svelte';
  import GateBadge from '$lib/components/GateBadge.svelte';
  import { Button } from '$lib/components/ui/button/index.js';
  import { Badge } from '$lib/components/ui/badge/index.js';
  import { Input } from '$lib/components/ui/input/index.js';
  import { Switch } from '$lib/components/ui/switch/index.js';
  import { Card, CardContent, CardHeader, CardTitle } from '$lib/components/ui/card/index.js';
  import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '$lib/components/ui/table/index.js';
  import { api } from '$lib/api.js';
  import { fmtDateTime, fmtDate } from '$lib/utils.js';
  import type { Gate, GateSettings, GateStats, Transaction } from '$lib/types.js';
  import { ChevronLeft, ExternalLink } from 'lucide-svelte';

  const gateId = $derived($page.params.id ?? '');

  let gate     = $state<Gate | null>(null);
  let stats    = $state<GateStats | null>(null);
  let loading  = $state(true);
  let saving   = $state(false);

  // Settings form
  let ttlMinutes    = $state(30);
  let autoClose     = $state(true);
  let maxEvents     = $state(100);

  $effect(() => {
    const id = gateId;
    (async () => {
      loading = true;
      try {
        const [g, s] = await Promise.all([api.gates.get(id), api.gates.stats(id)]);
        gate = g;
        stats = s;
        const cfg = g.settings ?? {};
        ttlMinutes = cfg.transaction_ttl_minutes ?? 30;
        autoClose  = cfg.auto_close_transactions ?? true;
        maxEvents  = cfg.max_events_per_transaction ?? 100;
      } catch {
        toast.error('Gate not found');
        goto('/settings/gates');
      } finally {
        loading = false;
      }
    })();
  });

  async function saveSettings() {
    if (!gate) return;
    saving = true;
    try {
      const settings: GateSettings = {
        transaction_ttl_minutes: ttlMinutes,
        auto_close_transactions: autoClose,
        max_events_per_transaction: maxEvents,
      };
      gate = await api.gates.updateSettings(gate.id, settings);
      toast.success('Settings saved');
    } catch {
      toast.error('Failed to save settings');
    } finally {
      saving = false;
    }
  }

  function statusVariant(s: string): 'default' | 'secondary' | 'destructive' | 'outline' {
    return s === 'active' ? 'default' : s === 'completed' ? 'secondary' : 'destructive';
  }
</script>

<TopBar crumbs={['OmniGate', 'Gates', gate?.name ?? '…']}>
  {#snippet actions()}
    <Button variant="outline" size="sm" onclick={() => goto('/settings/gates')}>
      <ChevronLeft size={14} /> Gates
    </Button>
  {/snippet}
</TopBar>

{#if loading}
  <div class="flex-1 flex items-center justify-center text-muted-foreground">Loading…</div>
{:else if gate}
  <main class="flex-1 p-6 max-w-[960px] space-y-5">

    <!-- Header card -->
    <Card>
      <CardContent class="pt-5">
        <div class="flex items-start gap-4 flex-wrap">
          <GateBadge gateId={gate.gate_id} dot />
          <div class="flex-1 min-w-0">
            <div class="flex items-center gap-3 flex-wrap">
              <span class="text-[18px] font-semibold">{gate.name}</span>
              <Badge variant={gate.status === 'active' ? 'default' : 'secondary'}>
                {gate.status === 'active' ? 'Active' : 'Inactive'}
              </Badge>
            </div>
            {#if gate.location}
              <p class="text-[13px] text-muted-foreground mt-0.5">{gate.location}</p>
            {/if}
            {#if gate.description}
              <p class="text-[12px] text-muted-foreground mt-1">{gate.description}</p>
            {/if}
          </div>
          <div class="grid grid-cols-3 gap-4 text-center">
            <a href="/?gate_id={gate.gate_id}" class="rounded-md border border-border px-4 py-2 hover:bg-muted transition-colors">
              <div class="text-[22px] font-bold">{stats?.open_transactions ?? 0}</div>
              <div class="text-[11px] text-muted-foreground">Open</div>
            </a>
            <a href="/?gate_id={gate.gate_id}" class="rounded-md border border-border px-4 py-2 hover:bg-muted transition-colors">
              <div class="text-[22px] font-bold">{stats?.total_transactions ?? 0}</div>
              <div class="text-[11px] text-muted-foreground">Total tx</div>
            </a>
            <a href="/settings/devices?gate_id={gate.gate_id}" class="rounded-md border border-border px-4 py-2 hover:bg-muted transition-colors">
              <div class="text-[22px] font-bold">{stats?.total_devices ?? 0}</div>
              <div class="text-[11px] text-muted-foreground">Devices</div>
            </a>
          </div>
        </div>
      </CardContent>
    </Card>

    <div class="grid grid-cols-[1fr_1.1fr] gap-5 items-start">

      <!-- Settings -->
      <Card>
        <CardHeader class="pb-3">
          <CardTitle class="text-[15px]">Transaction settings</CardTitle>
        </CardHeader>
        <CardContent class="space-y-4">
          <Field label="Transaction TTL (minutes)" hint="Automatically close open transactions after this many minutes of inactivity.">
            <Input type="number" bind:value={ttlMinutes} min={1} max={1440} />
          </Field>
          <div class="flex items-center justify-between">
            <div>
              <p class="text-[13px] font-medium">Auto-close transactions</p>
              <p class="text-[11px] text-muted-foreground mt-0.5">Close when TTL expires with no new events.</p>
            </div>
            <Switch bind:checked={autoClose} />
          </div>
          <Field label="Max events per transaction" hint="Hard cap; additional events will start a new transaction.">
            <Input type="number" bind:value={maxEvents} min={1} max={10000} />
          </Field>
          <div class="flex justify-end pt-1">
            <Button size="sm" onclick={saveSettings} disabled={saving}>
              {saving ? 'Saving…' : 'Save settings'}
            </Button>
          </div>
        </CardContent>
      </Card>

      <!-- Recent transactions -->
      <Card>
        <CardHeader class="pb-2">
          <div class="flex items-center justify-between">
            <CardTitle class="text-[15px]">Recent transactions</CardTitle>
            <a
              href="/?gate_id={gate.gate_id}"
              class="text-[12px] text-muted-foreground hover:text-foreground flex items-center gap-1"
            >
              View all <ExternalLink size={11} />
            </a>
          </div>
        </CardHeader>
        <CardContent class="p-0">
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Code</TableHead>
                <TableHead>Status</TableHead>
                <TableHead>Opened</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {#each (stats?.recent_transactions ?? []) as tx (tx.id)}
                <TableRow class="cursor-pointer" onclick={() => goto(`/transactions/${tx.id}`)}>
                  <TableCell class="font-mono text-[12px]">{tx.code}</TableCell>
                  <TableCell>
                    <Badge variant={statusVariant(tx.status)} class="text-[10px]">
                      {tx.status === 'active' ? 'Open' : tx.status === 'completed' ? 'Closed' : 'Cancelled'}
                    </Badge>
                  </TableCell>
                  <TableCell class="text-[12px] text-muted-foreground">{fmtDate(tx.created_at)}</TableCell>
                </TableRow>
              {/each}
              {#if !stats?.recent_transactions?.length}
                <TableRow>
                  <TableCell colspan={3} class="py-6 text-center text-muted-foreground">No transactions yet.</TableCell>
                </TableRow>
              {/if}
            </TableBody>
          </Table>
        </CardContent>
      </Card>
    </div>

  </main>
{/if}
