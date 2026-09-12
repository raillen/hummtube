<script lang="ts">
  import { onDestroy } from 'svelte';
  import type { Video } from '../../types';
  import { playerStore, playerQueue } from '../../stores/playerStore';
  import Check from 'lucide-svelte/icons/check';
  import Film from 'lucide-svelte/icons/film';
  import ListPlus from 'lucide-svelte/icons/list-plus';
  import MoreVertical from 'lucide-svelte/icons/ellipsis-vertical';
  import Play from 'lucide-svelte/icons/play';
  import { toast } from '../../stores/uiStores';
  import { CatalogService, LibraryService } from '../../wailsjs/services';
  import ContextMenu from '../ui/ContextMenu.svelte';
  import AddToPlaylistModal from '../player/AddToPlaylistModal.svelte';
  import { displayPreferences } from '../../stores/displayPreferences';
  import { openChannel } from '../../stores/channelRouteStore';
  import type { ContentViewMode } from '../../stores/uiStores';
  import { cachedThumbnailURL as loadCachedThumbnail, releaseThumbnailURL } from '../../media/thumbnailCache';

  export let video: Video;
  export let viewMode: ContentViewMode = 'grid';
  let imageError = false;
  let thumbnailFallbackApplied = false;
  let cachedThumbnail = '';
  let thumbnailRequestGeneration = 0;
  let isContextMenuOpen = false;
  let contextMenuX = 0;
  let contextMenuY = 0;
  let contextMenuTrigger: HTMLElement | null = null;
  let showPlaylistModal = false;

  $: isQueued = $playerQueue.some((queued) => queued.id === video.id);

  $: contextActions = [
    { id: 'play', label: 'Reproduzir agora' },
    {
      id: 'queue',
      label: isQueued ? 'Já está na fila' : 'Adicionar à fila',
      disabled: isQueued
    },
    { id: 'favorite', label: 'Adicionar ou remover dos favoritos' },
    { id: 'playlist', label: 'Adicionar à playlist ou criar nova' },
    { id: 'channel', label: 'Abrir canal', disabled: !video.channel_id || video.channel_id === 'direct' }
  ];
  $: thumbnailURL = preferredThumbnailURL(video.thumbnail_url, $displayPreferences.thumbnailQuality);
  $: if (thumbnailURL) void hydrateThumbnail(thumbnailURL);

  async function hydrateThumbnail(source: string): Promise<void> {
    const generation = ++thumbnailRequestGeneration;
    const resolved = await loadCachedThumbnail(source);
    if (generation !== thumbnailRequestGeneration) {
      releaseThumbnailURL(resolved);
      return;
    }
    if (cachedThumbnail && cachedThumbnail !== resolved) releaseThumbnailURL(cachedThumbnail);
    cachedThumbnail = resolved;
  }

  function preferredThumbnailURL(url: string, quality: 'data_saver' | 'balanced' | 'high'): string {
    if (!url || !/(?:i\.ytimg\.com|img\.youtube\.com)/i.test(url)) return url;
    const filename = quality === 'data_saver' ? 'mqdefault.jpg' : quality === 'high' ? 'maxresdefault.jpg' : 'hqdefault.jpg';
    return url.replace(/(?:maxresdefault|sddefault|hqdefault|mqdefault|default)\.jpg(?:\?.*)?$/i, filename);
  }

  function handleThumbnailError(): void {
    if (!thumbnailFallbackApplied && $displayPreferences.thumbnailQuality === 'high') {
      thumbnailFallbackApplied = true;
      thumbnailURL = preferredThumbnailURL(video.thumbnail_url, 'balanced');
      return;
    }
    imageError = true;
  }

  onDestroy(() => {
    thumbnailRequestGeneration += 1;
    releaseThumbnailURL(cachedThumbnail);
  });

  function formatDuration(ns: number) {
    if (!ns) return '';
    const secs = Math.floor(ns / 1e9);
    const m = Math.floor(secs / 60);
    const s = secs % 60;
    return `${m}:${s < 10 ? '0' : ''}${s}`;
  }

  function handlePlay() {
    playerStore.loadVideo(video);
  }

  let prefetchTimeout: ReturnType<typeof setTimeout> | null = null;

  function schedulePrefetch(): void {
    if (prefetchTimeout) return;
    prefetchTimeout = setTimeout(() => {
      playerStore.prefetchVideo(video, video.external_url);
      prefetchTimeout = null;
    }, 300);
  }

  function cancelPrefetch(): void {
    if (prefetchTimeout) {
      clearTimeout(prefetchTimeout);
      prefetchTimeout = null;
    }
  }

  function openVideoChannel(): void {
    if (!video.channel_id || video.channel_id === 'direct') return;
    openChannel({ id: video.channel_id, title: video.channel_title, initialSection: 'videos' });
  }

  function handleAddToQueue(event: MouseEvent) {
    event.stopPropagation();
    const alreadyQueued = isQueued;
    if (alreadyQueued) {
      toast.add('Esse vídeo já está na fila.', 'info');
      return;
    }
    playerStore.addToQueue(video);
    toast.add('Vídeo adicionado à fila.', 'success');
  }

  function openContextMenu(event: MouseEvent | KeyboardEvent): void {
    event.preventDefault();
    contextMenuTrigger = event instanceof KeyboardEvent
      ? document.activeElement as HTMLElement
      : event.currentTarget as HTMLElement;
    if (event instanceof MouseEvent && event.type === 'contextmenu') {
      contextMenuX = event.clientX;
      contextMenuY = event.clientY;
    } else {
      const bounds = contextMenuTrigger.getBoundingClientRect();
      contextMenuX = bounds.left + 16;
      contextMenuY = bounds.top + 16;
    }
    isContextMenuOpen = true;
  }

  async function runContextAction(event: CustomEvent<string>): Promise<void> {
    switch (event.detail) {
      case 'play':
        handlePlay();
        break;
      case 'queue':
        playerStore.addToQueue(video);
        toast.add('Vídeo adicionado à fila.', 'success');
        break;
      case 'favorite':
        try {
          const favorites = await LibraryService.getFavorites();
          const isFavorite = (favorites || []).some((favorite) => favorite.id === video.id);
          if (isFavorite) {
            await LibraryService.removeFavorite(video.id);
            toast.add('Vídeo removido dos favoritos.', 'info');
          } else {
            await CatalogService.rememberVideo(video);
            await LibraryService.addFavorite(video.id);
            toast.add('Vídeo adicionado aos favoritos.', 'success');
          }
        } catch (error: unknown) {
          toast.add(error instanceof Error ? error.message : 'Falha ao favoritar vídeo.', 'error');
        }
        break;
      case 'playlist':
        showPlaylistModal = true;
        break;
      case 'channel':
        openVideoChannel();
        break;
    }
  }

  function registerContextMenu(node: HTMLElement): { destroy: () => void } {
    const handleContextMenu = (event: MouseEvent) => openContextMenu(event);
    const handleKeyDown = (event: KeyboardEvent) => {
      if (event.key === 'ContextMenu' || event.key === 'Apps' || (event.shiftKey && event.key === 'F10')) openContextMenu(event);
    };
    node.addEventListener('contextmenu', handleContextMenu);
    node.addEventListener('keydown', handleKeyDown);
    return {
      destroy: () => {
        node.removeEventListener('contextmenu', handleContextMenu);
        node.removeEventListener('keydown', handleKeyDown);
      },
    };
  }
</script>

<div
  role="group"
  aria-label={`Ações para ${video.title}`}
  use:registerContextMenu
  class="group rounded-xl p-1 transition-transform text-left w-full {viewMode === 'list' ? 'grid grid-cols-[minmax(10rem,16rem)_1fr] items-center gap-3' : 'flex flex-col gap-2'}"
  on:mouseenter={schedulePrefetch}
  on:mouseleave={cancelPrefetch}
  on:focus={schedulePrefetch}
  on:blur={cancelPrefetch}
>
  <!-- Thumbnail Container -->
  <div class="relative w-full aspect-video rounded-xl overflow-hidden bg-surface border border-border flex items-center justify-center">
    <button
      type="button"
      aria-label={`Reproduzir ${video.title} de ${video.channel_title || 'Canal'}`}
      class="absolute inset-0 z-[1] cursor-pointer tv-focusable focus:outline-none"
      on:click={handlePlay}
    />
    {#if video.thumbnail_url && !imageError}
      <img
        src={cachedThumbnail || thumbnailURL}
        alt={video.title}
        loading="lazy"
        on:error={handleThumbnailError}
        class="w-full h-full object-cover group-hover:scale-105 transition-transform duration-200"
      />
    {:else}
      <div class="flex flex-col items-center justify-center text-muted">
        <Film size={28} class="opacity-40" />
      </div>
    {/if}
    
    <!-- Duration / Live Badge -->
    {#if video.live_status === 'live' || video.live_status === 'is_live'}
      <div class="absolute bottom-2 right-2 px-1.5 py-0.5 rounded bg-red-600 text-white font-bold text-[10px] uppercase flex items-center gap-1">
        <span class="w-1.5 h-1.5 rounded-full bg-white animate-pulse"></span>
        Ao Vivo
      </div>
    {:else if video.live_status === 'upcoming' || video.live_status === 'is_upcoming'}
      <div class="absolute bottom-2 right-2 rounded bg-amber-500 px-1.5 py-0.5 text-[10px] font-bold uppercase text-black">Em breve</div>
    {:else if video.duration > 0}
      <div class="absolute bottom-2 right-2 px-1.5 py-0.5 rounded bg-black/80 text-white font-medium text-[11px]">
        {formatDuration(video.duration)}
      </div>
    {/if}
    <div class="absolute bottom-2 left-2 z-[3] flex gap-1">
      {#if video.duration > 0 && video.duration <= 180 * 1e9}<span class="rounded bg-fuchsia-600 px-1.5 py-0.5 text-[9px] font-bold uppercase text-white">Short</span>{/if}
      {#if /membros|members[ -]?only|só para membros/i.test(`${video.title} ${video.description_excerpt || ''}`)}<span class="rounded bg-emerald-600 px-1.5 py-0.5 text-[9px] font-bold uppercase text-white">Membros</span>{/if}
      {#if /\bmix(?:es)?\b|mixagem/i.test(video.title)}<span class="rounded bg-indigo-600 px-1.5 py-0.5 text-[9px] font-bold uppercase text-white">Mix</span>{/if}
    </div>

    <!-- Play Overlay on Hover -->
    <div class="absolute inset-0 z-[2] pointer-events-none bg-black/40 opacity-0 group-hover:opacity-100 transition-opacity flex items-center justify-center">
      <div class="w-10 h-10 rounded-full bg-primary text-white flex items-center justify-center shadow-lg transform scale-90 group-hover:scale-100 transition-transform">
        <Play size={18} fill="currentColor" class="ml-0.5" />
      </div>
    </div>

    <button
      type="button"
      on:click={handleAddToQueue}
      aria-label={`Adicionar ${video.title} à fila`}
      title={isQueued ? 'Já está na fila' : 'Adicionar à fila'}
      data-component="video-card-action"
      class="absolute right-2 top-2 z-[3] rounded-lg bg-black/70 p-1.5 text-white opacity-0 shadow transition-opacity hover:bg-primary group-hover:opacity-100 focus:opacity-100 focus:outline-none tv-focusable"
    >
      {#if isQueued}
        <Check size={15} />
      {:else}
        <ListPlus size={15} />
      {/if}
    </button>
    <button
      type="button"
      aria-label={`Mais ações para ${video.title}`}
      aria-haspopup="menu"
      title="Mais ações"
      data-component="video-card-action"
      class="absolute right-11 top-2 z-[3] rounded-lg bg-black/70 p-1.5 text-white opacity-0 shadow transition-opacity hover:bg-primary group-hover:opacity-100 focus:opacity-100 focus:outline-none tv-focusable"
      on:click={openContextMenu}
    >
      <MoreVertical size={15} />
    </button>
  </div>

  <!-- Meta Info -->
  <div class="flex gap-2 px-0.5 w-full">
    <div class="flex-1 min-w-0">
      <h3 data-component="video-card-title" class="text-sm font-semibold text-foreground line-clamp-2 leading-snug group-hover:text-primary transition-colors">
        {video.title}
      </h3>
      <button type="button" on:click={openVideoChannel} disabled={!video.channel_id || video.channel_id === 'direct'} data-component="video-card-channel" class="mt-1 block max-w-full truncate text-left text-xs text-muted hover:text-primary disabled:pointer-events-none tv-focusable">
        {video.channel_title || 'Canal'}
      </button>
    </div>
  </div>
</div>

<ContextMenu
  bind:open={isContextMenuOpen}
  x={contextMenuX}
  y={contextMenuY}
  actions={contextActions}
  on:select={runContextAction}
  on:close={() => contextMenuTrigger?.focus()}
/>
<AddToPlaylistModal bind:open={showPlaylistModal} {video} />
