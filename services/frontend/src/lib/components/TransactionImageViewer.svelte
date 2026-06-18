<script lang="ts">
  import AuthImg from "$lib/components/AuthImg.svelte";
  import { api } from "$lib/api.js";
  import { authStore } from "$lib/stores/auth.svelte.js";
  import { toast } from "svelte-sonner";
  import { Camera, ChevronLeft, ChevronRight, ExternalLink, X } from "lucide-svelte";

  interface ImageItem {
    key: string;
    label: string;
  }

  let {
    images,
    galleryIdx = $bindable<number | null>(null),
  }: {
    images: ImageItem[];
    galleryIdx?: number | null;
  } = $props();

  const openPhoto = $derived(galleryIdx !== null ? (images[galleryIdx] ?? null) : null);

  let imgScale = $state(1);
  let imgX = $state(0);
  let imgY = $state(0);
  let dragging = $state(false);
  let dragStart = $state({ x: 0, y: 0, ox: 0, oy: 0 });
  let openingOriginal = $state(false);

  // Reset zoom/pan whenever the active image changes
  $effect(() => { galleryIdx; imgScale = 1; imgX = 0; imgY = 0; });

  function closePhoto() { galleryIdx = null; }

  function prevImage() {
    if (!images.length) return;
    galleryIdx = galleryIdx === null ? 0 : (galleryIdx - 1 + images.length) % images.length;
  }

  function nextImage() {
    if (!images.length) return;
    galleryIdx = galleryIdx === null ? 0 : (galleryIdx + 1) % images.length;
  }

  async function openOriginal() {
    if (!openPhoto) return;
    openingOriginal = true;
    try {
      const res = await fetch(api.imageUrl(openPhoto.key), {
        headers: authStore.sessionId ? { Authorization: `Bearer ${authStore.sessionId}` } : {},
      });
      if (!res.ok) throw new Error();
      const blob = await res.blob();
      const url = URL.createObjectURL(blob);
      window.open(url, '_blank');
      setTimeout(() => URL.revokeObjectURL(url), 60_000);
    } catch {
      toast.error('Не вдалося відкрити зображення');
    } finally {
      openingOriginal = false;
    }
  }

  function onDblClick() {
    if (imgScale > 1) { imgScale = 1; imgX = 0; imgY = 0; } else imgScale = 2.5;
  }
  function onMouseDown(e: MouseEvent) {
    if (e.button !== 0) return;
    dragging = true;
    dragStart = { x: e.clientX, y: e.clientY, ox: imgX, oy: imgY };
  }
  function onMouseMove(e: MouseEvent) {
    if (!dragging) return;
    imgX = dragStart.ox + (e.clientX - dragStart.x);
    imgY = dragStart.oy + (e.clientY - dragStart.y);
  }
  function onMouseUp() { dragging = false; }

  // Non-passive wheel + pinch (passive:false required for preventDefault)
  function lightboxInteract(node: HTMLElement) {
    let ld = 0, lmx = 0, lmy = 0;
    function wheel(e: WheelEvent) {
      e.preventDefault();
      imgScale = Math.max(1, Math.min(6, imgScale * (e.deltaY > 0 ? 0.85 : 1.15)));
      if (imgScale <= 1) { imgScale = 1; imgX = 0; imgY = 0; }
    }
    function touchStart(e: TouchEvent) {
      if (e.touches.length !== 2) return;
      e.preventDefault();
      ld = Math.hypot(e.touches[0].clientX - e.touches[1].clientX, e.touches[0].clientY - e.touches[1].clientY);
      lmx = (e.touches[0].clientX + e.touches[1].clientX) / 2;
      lmy = (e.touches[0].clientY + e.touches[1].clientY) / 2;
    }
    function touchMove(e: TouchEvent) {
      if (e.touches.length !== 2) return;
      e.preventDefault();
      const d = Math.hypot(e.touches[0].clientX - e.touches[1].clientX, e.touches[0].clientY - e.touches[1].clientY);
      const mx = (e.touches[0].clientX + e.touches[1].clientX) / 2;
      const my = (e.touches[0].clientY + e.touches[1].clientY) / 2;
      if (ld) imgScale = Math.max(1, Math.min(6, imgScale * d / ld));
      if (imgScale <= 1) { imgScale = 1; imgX = 0; imgY = 0; } else { imgX += mx - lmx; imgY += my - lmy; }
      ld = d; lmx = mx; lmy = my;
    }
    function touchEnd(e: TouchEvent) { if (e.touches.length < 2) ld = 0; }
    node.addEventListener('wheel', wheel, { passive: false });
    node.addEventListener('touchstart', touchStart, { passive: false });
    node.addEventListener('touchmove', touchMove, { passive: false });
    node.addEventListener('touchend', touchEnd);
    return {
      destroy() {
        node.removeEventListener('wheel', wheel);
        node.removeEventListener('touchstart', touchStart);
        node.removeEventListener('touchmove', touchMove);
        node.removeEventListener('touchend', touchEnd);
      },
    };
  }

  function handleKeydown(e: KeyboardEvent) {
    if (galleryIdx === null) return;
    if (e.key === 'Escape') { closePhoto(); return; }
    if (e.key === 'ArrowLeft') { prevImage(); return; }
    if (e.key === 'ArrowRight') { nextImage(); return; }
  }
</script>

<svelte:window onkeydown={handleKeydown} />

<!-- Thumbnail grid -->
{#if images.length}
  <div>
    <div class="flex items-center justify-between mb-3">
      <h2 class="text-sm font-semibold uppercase tracking-wide text-muted-foreground">Фото</h2>
      <span class="text-xs text-muted-foreground">{images.length} знімків</span>
    </div>
    <div class="grid grid-cols-2 gap-2">
      {#each images as img, idx (img.key)}
        <button
          onclick={() => (galleryIdx = idx)}
          class="aspect-[4/3] w-full bg-muted rounded-md border border-border overflow-hidden relative hover:opacity-90 transition-opacity focus:outline-none focus-visible:ring-2 focus-visible:ring-primary/50"
        >
          <AuthImg
            src={api.imageUrl(img.key)}
            alt={img.label}
            class="absolute inset-0 w-full h-full object-cover"
          />
          <div class="absolute inset-0 flex items-end p-1.5 pointer-events-none bg-gradient-to-t from-black/40 to-transparent">
            <span class="text-[10px] text-white/90 font-mono leading-tight">{img.label}</span>
          </div>
          <Camera size={13} class="absolute top-1.5 right-1.5 text-white/60 drop-shadow" />
        </button>
      {/each}
    </div>
  </div>
{/if}

<!-- Lightbox overlay -->
{#if openPhoto}
  <div class="fixed inset-0 z-50 bg-black/95 flex flex-col select-none" role="dialog" aria-modal="true">

    <!-- Top bar -->
    <div class="flex items-center justify-between px-4 py-2 shrink-0">
      <span class="font-mono text-xs text-white/50">{openPhoto.label}</span>
      <div class="flex items-center gap-2 text-white/50">
        <span class="text-[11px] tabular-nums">{Math.round(imgScale * 100)}%</span>
        {#if images.length > 1}
          <span class="text-[11px] tabular-nums">Фото {(galleryIdx ?? 0) + 1} з {images.length}</span>
        {/if}
        <button
          onclick={openOriginal}
          disabled={openingOriginal}
          class="flex items-center gap-1.5 text-[11px] px-2.5 py-1.5 rounded-md border border-white/20 hover:border-white/50 hover:text-white transition-colors disabled:opacity-40"
        >
          <ExternalLink size={13} />
          Оригінал
        </button>
        <button onclick={closePhoto} class="hover:text-white transition-colors p-1.5">
          <X size={18} />
        </button>
      </div>
    </div>

    <!-- Image area -->
    <div
      use:lightboxInteract
      role="presentation"
      class="flex-1 overflow-hidden flex items-center justify-center {imgScale > 1 ? (dragging ? 'cursor-grabbing' : 'cursor-grab') : 'cursor-zoom-in'}"
      onmousedown={onMouseDown}
      onmousemove={onMouseMove}
      onmouseup={onMouseUp}
      onmouseleave={onMouseUp}
      ondblclick={onDblClick}
      onclick={(e) => { if (e.target === e.currentTarget && imgScale === 1) closePhoto(); }}
    >
      <div
        class="transform-gpu"
        style="transform: translate({imgX}px, {imgY}px) scale({imgScale}); transition: {dragging ? 'none' : 'transform 0.1s ease'};"
      >
        <AuthImg
          src={api.imageUrl(openPhoto.key)}
          alt={openPhoto.label}
          class="block max-w-[99vw] max-h-[calc(100dvh-80px)] object-contain pointer-events-none"
        />
      </div>
    </div>

    <!-- Prev/Next arrows -->
    {#if images.length > 1}
      <button
        onclick={prevImage}
        class="absolute left-3 top-1/2 -translate-y-1/2 p-2.5 rounded-full bg-white/10 hover:bg-white/25 text-white transition-colors"
        aria-label="Попереднє фото"
      >
        <ChevronLeft size={22} />
      </button>
      <button
        onclick={nextImage}
        class="absolute right-3 top-1/2 -translate-y-1/2 p-2.5 rounded-full bg-white/10 hover:bg-white/25 text-white transition-colors"
        aria-label="Наступне фото"
      >
        <ChevronRight size={22} />
      </button>
    {/if}

    <!-- Hints -->
    <div class="shrink-0 flex items-center justify-center gap-5 py-2 text-[10px] text-white/20">
      <span>scroll / pinch — zoom</span>
      <span>двічі — 2.5×</span>
      <span>тягни — зсув</span>
      {#if images.length > 1}<span>← → — навігація</span>{/if}
      <span>ESC — закрити</span>
    </div>
  </div>
{/if}
