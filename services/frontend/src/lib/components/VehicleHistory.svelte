<script lang="ts">
  import { api } from '$lib/api.js';
  import { fmtDate, fmtTime } from '$lib/utils.js';
  import type { Transaction } from '$lib/types.js';
  import AuthImg from '$lib/components/AuthImg.svelte';
  import GateBadge from '$lib/components/GateBadge.svelte';
  import { goto } from '$app/navigation';
  import { AlertCircle, Clock, Inbox } from 'lucide-svelte';

  let { plate }: { plate: string } = $props();

  let history  = $state<Transaction[]>([]);
  let isLoading = $state(false);
  let error    = $state<string | null>(null);

  $effect(() => {
    const p = plate.trim();
    if (!p) return;

    isLoading = true;
    error     = null;
    history   = [];

    api.transactions.history(p)
      .then(res  => { history = res.data ?? []; })
      .catch(err => { error = err instanceof Error ? err.message : String(err); })
      .finally(()  => { isLoading = false; });
  });

  // Повертає перший image_key з усіх подій транзакції.
  function firstImage(tx: Transaction): string | null {
    for (const ev of tx.events ?? []) {
      if (ev.image_keys?.length) return ev.image_keys[0];
    }
    return null;
  }

  // Знаходить розпізнаний номер з PlateEvent (для відображення у таймлайні).
  function detectedPlate(tx: Transaction): string | null {
    for (const ev of tx.events ?? []) {
      if (ev.type_code === 'PlateEvent' && typeof ev.data?.plate === 'string') {
        return ev.data.plate as string;
      }
    }
    return null;
  }
</script>

<!-- ─── Стан завантаження ─── -->
{#if isLoading}
  <div class="space-y-3 p-1">
    {#each { length: 4 } as _}
      <div class="flex gap-3 animate-pulse">
        <div class="mt-1 shrink-0 w-2.5 h-2.5 rounded-full bg-muted-foreground/20"></div>
        <div class="flex-1 space-y-2">
          <div class="h-3 w-1/3 rounded bg-muted-foreground/20"></div>
          <div class="h-3 w-2/3 rounded bg-muted-foreground/10"></div>
        </div>
        <div class="shrink-0 w-16 h-12 rounded-md bg-muted-foreground/10"></div>
      </div>
    {/each}
  </div>

<!-- ─── Помилка ─── -->
{:else if error}
  <div class="flex flex-col items-center gap-2 py-10 text-destructive">
    <AlertCircle size={28} />
    <p class="text-sm text-center">{error}</p>
  </div>

<!-- ─── Порожній результат ─── -->
{:else if history.length === 0}
  <div class="flex flex-col items-center gap-3 py-12 text-muted-foreground">
    <Inbox size={32} strokeWidth={1.5} />
    <div class="text-center">
      <p class="text-sm font-medium">Історії не знайдено</p>
      <p class="text-xs mt-0.5">Попередніх проїздів з номером <span class="font-mono">{plate}</span> не зафіксовано</p>
    </div>
  </div>

<!-- ─── Таймлайн ─── -->
{:else}
  <p class="text-xs text-muted-foreground mb-4">
    Знайдено {history.length} {history.length === 1 ? 'проїзд' : 'проїзди(ів)'} для <span class="font-mono font-medium text-foreground">{plate}</span>
  </p>

  <ol class="relative space-y-0">
    {#each history as tx, i (tx.id)}
      {@const img  = firstImage(tx)}
      {@const seen = detectedPlate(tx)}

      <!-- Вертикальна лінія між точками -->
      <li class="relative flex gap-4 pb-6 last:pb-0">
        <!-- Маркер + лінія -->
        <div class="flex flex-col items-center shrink-0">
          <div class="mt-0.5 w-2.5 h-2.5 rounded-full border-2 border-primary bg-background z-10 shrink-0"></div>
          {#if i < history.length - 1}
            <div class="w-px flex-1 bg-border mt-1"></div>
          {/if}
        </div>

        <!-- Контент -->
        <button
          type="button"
          class="group flex flex-1 gap-3 text-left rounded-lg p-2 -ml-2 hover:bg-accent transition-colors min-w-0"
          onclick={() => goto(`/transactions/${tx.id}`)}
        >
          <div class="flex-1 min-w-0 space-y-1">
            <!-- Час та дата -->
            <div class="flex items-center gap-2">
              <Clock size={12} class="text-muted-foreground shrink-0" />
              <span class="text-sm font-semibold tabular-nums">{fmtTime(tx.created_at)}</span>
              <span class="text-xs text-muted-foreground">{fmtDate(tx.created_at)}</span>
            </div>

            <!-- Шлагбаум -->
            <GateBadge gateId={tx.gate_id} />

            <!-- Розпізнаний номер (якщо відрізняється від шуканого — показуємо) -->
            {#if seen && seen.toUpperCase() !== plate.toUpperCase()}
              <p class="text-xs text-muted-foreground">
                Зчитано: <span class="font-mono">{seen}</span>
              </p>
            {/if}

            <!-- Кількість подій -->
            <p class="text-xs text-muted-foreground">
              {tx.events?.length ?? 0} {(tx.events?.length ?? 0) === 1 ? 'подія' : 'подій'}
            </p>
          </div>

          <!-- Мініатюра -->
          {#if img}
            <div class="shrink-0">
              <AuthImg
                src={api.imageUrl(img)}
                alt="Фото проїзду"
                class="w-20 h-14 object-cover rounded-md border border-border group-hover:opacity-90 transition-opacity"
              />
            </div>
          {/if}
        </button>
      </li>
    {/each}
  </ol>
{/if}
