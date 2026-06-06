<script lang="ts">
  import { Popover, RangeCalendar } from 'bits-ui';
  import { CalendarDate } from '@internationalized/date';
  import type { DateRange } from 'bits-ui';
  import { CalendarIcon, ChevronLeft, ChevronRight, X } from 'lucide-svelte';
  import { cn } from '$lib/utils.js';

  const HOURS   = Array.from({ length: 24 }, (_, i) => String(i).padStart(2, '0'));
  const MINUTES = Array.from({ length: 60 }, (_, i) => String(i).padStart(2, '0'));

  interface Props {
    startAt?: string; // ISO UTC string or ''
    endAt?: string;
    class?: string;
  }

  let { startAt = $bindable(''), endAt = $bindable(''), class: className = '' }: Props = $props();

  // ─── Helpers ──────────────────────────────────────────────
  function isoToCalDate(iso: string): CalendarDate | undefined {
    if (!iso) return undefined;
    const d = new Date(iso);
    return isNaN(d.getTime()) ? undefined : new CalendarDate(d.getFullYear(), d.getMonth() + 1, d.getDate());
  }

  function isoToTime(iso: string, fallback: string): string {
    if (!iso) return fallback;
    const d = new Date(iso);
    return isNaN(d.getTime())
      ? fallback
      : `${String(d.getHours()).padStart(2, '0')}:${String(d.getMinutes()).padStart(2, '0')}`;
  }

  function buildISO(date: CalendarDate | undefined, time: string): string {
    if (!date) return '';
    const [h, m] = time.split(':').map(Number);
    return new Date(date.year, date.month - 1, date.day, h || 0, m || 0).toISOString();
  }

  // ─── Popover state ────────────────────────────────────────
  let open = $state(false);

  // Working state — only written to props on "Apply"
  let rangeValue = $state<DateRange>({ start: isoToCalDate(startAt), end: isoToCalDate(endAt) });
  let startTime  = $state(isoToTime(startAt, '00:00'));
  let endTime    = $state(isoToTime(endAt, '23:59'));

  function syncFromProps() {
    rangeValue = { start: isoToCalDate(startAt), end: isoToCalDate(endAt) };
    startTime  = isoToTime(startAt, '00:00');
    endTime    = isoToTime(endAt, '23:59');
  }

  function apply() {
    startAt = buildISO(rangeValue.start as CalendarDate | undefined, startTime);
    endAt   = buildISO(rangeValue.end   as CalendarDate | undefined, endTime);
    open = false;
  }

  function selectSingleDay(d: CalendarDate) {
    rangeValue = { start: d, end: d };
    startTime = '00:00';
    endTime = '23:59';
    apply();
  }

  function clear(e?: Event) {
    e?.stopPropagation();
    rangeValue = { start: undefined, end: undefined };
    startTime  = '00:00';
    endTime    = '23:59';
    startAt = '';
    endAt   = '';
    open = false;
  }

  // ─── Trigger label ────────────────────────────────────────
  const hasValue = $derived(!!(startAt || endAt));

  const triggerLabel = $derived.by(() => {
    const date = (iso: string) => {
      const d = new Date(iso);
      return `${String(d.getDate()).padStart(2, '0')}.${String(d.getMonth() + 1).padStart(2, '0')}.${d.getFullYear()}`;
    };
    const time = (iso: string) => {
      const d = new Date(iso);
      return `${String(d.getHours()).padStart(2, '0')}:${String(d.getMinutes()).padStart(2, '0')}`;
    };
    if (!startAt && !endAt) return 'Діапазон дат';
    if (startAt && !endAt)  return `від ${date(startAt)} ${time(startAt)}`;
    if (!startAt && endAt)  return `до ${date(endAt)} ${time(endAt)}`;
    return `${date(startAt)} ${time(startAt)} — ${date(endAt)} ${time(endAt)}`;
  });
</script>

<Popover.Root bind:open onOpenChange={(v) => v && syncFromProps()}>
  <Popover.Trigger>
    {#snippet child({ props })}
      <button
        {...props}
        class={cn(
          "inline-flex items-center gap-1.5 h-10 rounded-md border border-input bg-transparent px-2.5 text-sm shadow-xs",
          "hover:bg-muted focus-visible:ring-2 focus-visible:ring-ring focus-visible:outline-none transition-colors whitespace-nowrap",
          hasValue
            ? "border-primary/50 bg-primary/5 dark:bg-primary/10 text-foreground"
            : "text-muted-foreground",
          className
        )}
      >
        <CalendarIcon size={13} class="shrink-0 {hasValue ? 'text-primary' : ''}" />
        <span>{triggerLabel}</span>
        {#if hasValue}
          <span
            role="button"
            tabindex="0"
            aria-label="Скинути"
            class="ml-0.5 text-muted-foreground hover:text-foreground transition-colors"
            onclick={(e) => clear(e)}
            onkeydown={(e) => e.key === 'Enter' && clear(e)}
          >
            <X size={12} />
          </span>
        {/if}
      </button>
    {/snippet}
  </Popover.Trigger>

  <Popover.Portal>
    <Popover.Content
      sideOffset={6}
      align="start"
      class="bg-popover text-popover-foreground ring-foreground/10 z-50 rounded-lg shadow-lg ring-1 p-4 w-auto"
    >
      <RangeCalendar.Root
        bind:value={rangeValue}
        locale="uk"
        weekdayFormat="short"
        fixedWeeks
        class="select-none"
      >
        {#snippet children({ months, weekdays })}
          <!-- Nav header -->
          <div class="flex items-center justify-between mb-3">
            <RangeCalendar.PrevButton
              class="size-7 rounded-md border border-input bg-transparent hover:bg-muted flex items-center justify-center transition-colors disabled:opacity-40 cursor-pointer"
            >
              <ChevronLeft size={14} />
            </RangeCalendar.PrevButton>
            <RangeCalendar.Heading class="text-sm font-semibold capitalize" />
            <RangeCalendar.NextButton
              class="size-7 rounded-md border border-input bg-transparent hover:bg-muted flex items-center justify-center transition-colors disabled:opacity-40 cursor-pointer"
            >
              <ChevronRight size={14} />
            </RangeCalendar.NextButton>
          </div>

          {#each months as month}
            <RangeCalendar.Grid class="w-full border-collapse">
              <!-- Weekday labels -->
              <RangeCalendar.GridHead>
                <RangeCalendar.GridRow class="flex mb-1">
                  {#each weekdays as wd}
                    <RangeCalendar.HeadCell class="w-9 h-7 flex items-center justify-center text-[11px] font-normal text-muted-foreground">
                      {wd.slice(0, 2)}
                    </RangeCalendar.HeadCell>
                  {/each}
                </RangeCalendar.GridRow>
              </RangeCalendar.GridHead>

              <!-- Day cells -->
              <RangeCalendar.GridBody>
                {#each month.weeks as weekDates}
                  <RangeCalendar.GridRow class="flex">
                    {#each weekDates as date}
                      <RangeCalendar.Cell {date} month={month.value} class="relative p-0"
                        ondblclick={() => selectSingleDay(date as CalendarDate)}>
                        <RangeCalendar.Day
                          class={cn(
                            "size-9 text-sm flex items-center justify-center cursor-pointer transition-colors rounded-md",
                            "hover:bg-muted focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring",
                            // Start & end of selection
                            "[&[data-selection-start]]:bg-primary [&[data-selection-start]]:text-primary-foreground [&[data-selection-start]]:hover:bg-primary/90",
                            "[&[data-selection-end]]:bg-primary [&[data-selection-end]]:text-primary-foreground [&[data-selection-end]]:hover:bg-primary/90",
                            // Middle of range
                            "[&[data-range-middle]]:bg-primary/12 [&[data-range-middle]]:rounded-none [&[data-range-middle]]:hover:bg-primary/20",
                            // Round only the outer corners at start/end of a multi-day range.
                            // When start === end (single day), both attrs are on the same element —
                            // the :not() guards keep all four corners rounded (base rounded-md applies).
                            "[&[data-range-start]:not([data-range-end])]:rounded-l-md [&[data-range-start]:not([data-range-end])]:rounded-r-none",
                            "[&[data-range-end]:not([data-range-start])]:rounded-r-md [&[data-range-end]:not([data-range-start])]:rounded-l-none",
                            // Today
                            "[&[data-today]:not([data-selection-start]):not([data-selection-end])]:font-semibold [&[data-today]:not([data-selection-start]):not([data-selection-end])]:text-primary",
                            // Outside current month
                            "[&[data-outside-month]]:opacity-30 [&[data-outside-month]]:pointer-events-none",
                            // Disabled
                            "[&[data-disabled]]:opacity-30 [&[data-disabled]]:pointer-events-none"
                          )}
                        />
                      </RangeCalendar.Cell>
                    {/each}
                  </RangeCalendar.GridRow>
                {/each}
              </RangeCalendar.GridBody>
            </RangeCalendar.Grid>
          {/each}
        {/snippet}
      </RangeCalendar.Root>

      <!-- Time inputs (24 h) -->
      <!-- Custom 24h selects — bypasses browser AM/PM locale entirely -->
      <div class="mt-3 pt-3 border-t border-border flex items-center gap-3">
        <div class="flex items-center gap-1.5 min-w-0">
          <span class="text-xs text-muted-foreground shrink-0">від</span>
          <div class="flex items-center gap-0.5">
            <select
              value={startTime.split(':')[0] ?? '00'}
              onchange={(e) => { startTime = `${(e.target as HTMLSelectElement).value}:${startTime.split(':')[1] ?? '00'}`; }}
              class="h-8 w-[52px] rounded-md border border-input bg-background text-center text-xs outline-none focus:ring-2 focus:ring-ring cursor-pointer"
            >
              {#each HOURS as h}<option value={h}>{h}</option>{/each}
            </select>
            <span class="text-muted-foreground text-sm font-medium select-none">:</span>
            <select
              value={startTime.split(':')[1] ?? '00'}
              onchange={(e) => { startTime = `${startTime.split(':')[0] ?? '00'}:${(e.target as HTMLSelectElement).value}`; }}
              class="h-8 w-[52px] rounded-md border border-input bg-background text-center text-xs outline-none focus:ring-2 focus:ring-ring cursor-pointer"
            >
              {#each MINUTES as m}<option value={m}>{m}</option>{/each}
            </select>
          </div>
        </div>
        <div class="flex items-center gap-1.5 min-w-0">
          <span class="text-xs text-muted-foreground shrink-0">до</span>
          <div class="flex items-center gap-0.5">
            <select
              value={endTime.split(':')[0] ?? '23'}
              onchange={(e) => { endTime = `${(e.target as HTMLSelectElement).value}:${endTime.split(':')[1] ?? '59'}`; }}
              class="h-8 w-[52px] rounded-md border border-input bg-background text-center text-xs outline-none focus:ring-2 focus:ring-ring cursor-pointer"
            >
              {#each HOURS as h}<option value={h}>{h}</option>{/each}
            </select>
            <span class="text-muted-foreground text-sm font-medium select-none">:</span>
            <select
              value={endTime.split(':')[1] ?? '59'}
              onchange={(e) => { endTime = `${endTime.split(':')[0] ?? '23'}:${(e.target as HTMLSelectElement).value}`; }}
              class="h-8 w-[52px] rounded-md border border-input bg-background text-center text-xs outline-none focus:ring-2 focus:ring-ring cursor-pointer"
            >
              {#each MINUTES as m}<option value={m}>{m}</option>{/each}
            </select>
          </div>
        </div>
      </div>

      <!-- Actions -->
      <div class="mt-3 flex items-center justify-end gap-2">
        <button
          type="button"
          onclick={() => clear()}
          class="h-8 px-3 rounded-md text-xs text-muted-foreground hover:text-foreground hover:bg-muted transition-colors"
        >
          Скинути
        </button>
        <button
          type="button"
          onclick={apply}
          class="h-8 px-3 rounded-md text-xs font-medium bg-primary text-primary-foreground hover:bg-primary/90 transition-colors"
        >
          Застосувати
        </button>
      </div>
    </Popover.Content>
  </Popover.Portal>
</Popover.Root>
