<script lang="ts">
  import { onMount } from 'svelte';
  import { PlaylistService, SettingsService } from '../../wailsjs/services';
  import { activePlaylistId, toast, type ContentViewMode } from '../../stores/uiStores';
  import {
    playlistIdFromRemoteRoute,
    remotePlaylistSelection,
  } from '../../stores/searchQueryStore';
  import { playerStore } from '../../stores/playerStore';
  import { activeChannelSelection } from '../../stores/channelRouteStore';
  import type { PlaylistDetail, Video } from '../../types';
  import VideoGrid from '../video/VideoGrid.svelte';
  import ViewModeToggle from '../ui/ViewModeToggle.svelte';
  import RemotePlaylistSaveModal from '../playlist/RemotePlaylistSaveModal.svelte';
  import Button from '../ui/Button.svelte';
  import ConfirmDialog from '../ui/ConfirmDialog.svelte';
  import ArrowLeft from 'lucide-svelte/icons/arrow-left';
  import Download from 'lucide-svelte/icons/download';
  import ListPlus from 'lucide-svelte/icons/list-plus';
  import Loader2 from 'lucide-svelte/icons/loader-circle';
  import Play from 'lucide-svelte/icons/play';
  import RefreshCw from 'lucide-svelte/icons/refresh-cw';
  import Trash2 from 'lucide-svelte/icons/trash-2';

  let localDetail: PlaylistDetail | null = null;
  let remoteVideos: Video[] = [];
  let remoteNextPageToken = '';
  let loadedRouteID = '';
  let isLoading = false;
  let isLoadingMore = false;
  let showRemoteSave = false;
  let pendingConfirm: { kind: 'delete'; trigger: HTMLElement | null } | null = null;
  let viewMode: ContentViewMode = 'grid';
  let viewError: string | null = null;
  let loadGeneration = 0;

  $: remotePlaylistID = $activePlaylistId ? playlistIdFromRemoteRoute($activePlaylistId) : null;
  $: displayedVideos = remotePlaylistID ? remoteVideos : (localDetail?.videos || []);
  $: displayedDuration = displayedVideos.reduce((total, video) => total + Math.max(0, video.duration || 0), 0);
  $: playlistTitle = remotePlaylistID
    ? ($remotePlaylistSelection?.id === remotePlaylistID ? $remotePlaylistSelection.title : 'Playlist do YouTube')
    : (localDetail?.playlist.name || 'Playlist');
  $: playlistDescription = remotePlaylistID
    ? ($remotePlaylistSelection?.id === remotePlaylistID ? $remotePlaylistSelection.description || '' : '')
    : (localDetail?.playlist.description || '');
  $: if ($activePlaylistId && $activePlaylistId !== loadedRouteID) {
    loadedRouteID = $activePlaylistId;
    void loadPlaylistRoute($activePlaylistId);
  }

  function errorMessage(error: unknown): string {
    return error instanceof Error && error.message ? error.message : 'Falha ao carregar playlist.';
  }

  function formatDuration(totalNanoseconds: number): string {
    const totalMinutes = Math.floor(totalNanoseconds / 1e9 / 60);
    if (totalMinutes <= 0) return '';
    const hours = Math.floor(totalMinutes / 60);
    const minutes = totalMinutes % 60;
    return hours > 0 ? `${hours} h ${minutes} min` : `${minutes} min`;
  }

  async function loadPlaylistRoute(routeID: string): Promise<void> {
    const generation = ++loadGeneration;
    isLoading = true;
    viewError = null;
    localDetail = null;
    remoteVideos = [];
    remoteNextPageToken = '';
    try {
      const remoteID = playlistIdFromRemoteRoute(routeID);
      if (remoteID) {
        const page = await PlaylistService.getRemotePlaylistPage(remoteID, '', 24);
        if (generation !== loadGeneration) return;
        remoteVideos = page.videos || [];
        remoteNextPageToken = page.next_page_token || '';
      } else {
        const detail = await PlaylistService.getPlaylist(routeID);
        if (generation !== loadGeneration) return;
        localDetail = detail;
      }
    } catch (error: unknown) {
      if (generation !== loadGeneration) return;
      viewError = errorMessage(error);
    } finally {
      if (generation === loadGeneration) isLoading = false;
    }
  }

  async function loadMoreRemoteVideos(): Promise<void> {
    if (!remotePlaylistID || !remoteNextPageToken || isLoadingMore) return;
    isLoadingMore = true;
    try {
      const page = await PlaylistService.getRemotePlaylistPage(remotePlaylistID, remoteNextPageToken, 24);
      const knownIDs = new Set(remoteVideos.map((video) => video.id));
      remoteVideos = [...remoteVideos, ...(page.videos || []).filter((video) => !knownIDs.has(video.id))];
      remoteNextPageToken = page.next_page_token || '';
    } catch (error: unknown) {
      toast.add(errorMessage(error), 'error');
    } finally {
      isLoadingMore = false;
    }
  }

  function closePlaylist(): void {
    loadGeneration += 1;
    if ($activeChannelSelection) {
      $activeChannelSelection = { ...$activeChannelSelection, initialSection: 'playlists' };
    }
    $activePlaylistId = null;
    remotePlaylistSelection.set(null);
  }

  async function deleteLocalPlaylist(): Promise<void> {
    if (!$activePlaylistId || remotePlaylistID) return;
    pendingConfirm = { kind: 'delete', trigger: document.activeElement instanceof HTMLElement ? document.activeElement : null };
  }

  async function applyDeletePlaylist(): Promise<void> {
    try {
      await PlaylistService.deletePlaylist($activePlaylistId || '');
      toast.add('Playlist local excluída.', 'success');
      closePlaylist();
    } catch (error: unknown) {
      toast.add(errorMessage(error), 'error');
    }
  }

  function addAllToQueue(): void {
    const queueLengthBefore = $playerStore.queue.length;
    for (const video of displayedVideos) playerStore.addToQueue(video);
    const added = $playerStore.queue.length - queueLengthBefore;
    toast.add(added > 0 ? `${added} vídeos adicionados à fila.` : 'Todos os vídeos já estavam na fila.', added > 0 ? 'success' : 'info');
  }

  function playAll(): void {
    const [firstVideo, ...remainingVideos] = displayedVideos;
    if (!firstVideo) return;
    for (const video of remainingVideos) playerStore.addToQueue(video);
    void playerStore.loadVideo(firstVideo);
  }

  function changeViewMode(mode: ContentViewMode): void {
    viewMode = mode;
    void SettingsService.saveSetting('playlist_view_mode', mode).catch(() => undefined);
  }

  function openSavedPlaylist(event: CustomEvent<PlaylistDetail>): void {
    remotePlaylistSelection.set(null);
    loadedRouteID = '';
    $activePlaylistId = event.detail.playlist.id;
  }

  onMount(() => {
    void SettingsService.getSettings().then((settings) => {
      if (settings.playlist_view_mode === 'list' || settings.playlist_view_mode === 'compact') viewMode = settings.playlist_view_mode;
    }).catch(() => undefined);
  });
</script>

<section class="flex flex-col gap-6 p-6 max-w-6xl mx-auto w-full" aria-labelledby="playlist-detail-title">
  <header class="flex flex-wrap items-start justify-between gap-3">
    <div class="flex min-w-0 items-start gap-3">
      <button type="button" on:click={closePlaylist} aria-label="Voltar para playlists" class="p-2 text-muted hover:text-foreground rounded-lg hover:bg-surfaceHover transition-colors tv-focusable">
        <ArrowLeft size={18} />
      </button>
      <div class="min-w-0">
        <div class="flex items-center gap-2">
          <h1 id="playlist-detail-title" class="truncate text-xl font-bold text-foreground">{playlistTitle}</h1>
          {#if remotePlaylistID}<span class="rounded bg-primary/10 px-2 py-0.5 text-[10px] font-semibold uppercase text-primary">Remota</span>{/if}
        </div>
        <p class="text-xs text-muted">{displayedVideos.length} vídeos {remoteNextPageToken ? 'carregados' : ''}{displayedDuration > 0 ? ` · ${formatDuration(displayedDuration)}${remoteNextPageToken ? ' carregadas' : ''}` : ''}</p>
        {#if playlistDescription}<p class="mt-1 max-w-2xl text-xs text-muted line-clamp-2">{playlistDescription}</p>{/if}
      </div>
    </div>

    <div class="flex flex-wrap items-center gap-2">
      <ViewModeToggle value={viewMode} onChange={changeViewMode} label="Visualização dos vídeos da playlist" />
      <Button variant="primary" size="sm" disabled={displayedVideos.length === 0 || isLoading} on:click={playAll}><Play size={15} /> Reproduzir tudo</Button>
      <Button variant="secondary" size="sm" disabled={displayedVideos.length === 0 || isLoading} on:click={addAllToQueue}><ListPlus size={15} /> Adicionar tudo à fila</Button>
      {#if remotePlaylistID}
        <Button variant="secondary" size="sm" disabled={isLoading} on:click={() => (showRemoteSave = true)}>
          <Download size={15} /> Salvar ou combinar
        </Button>
      {:else}
        <Button variant="danger" size="sm" on:click={deleteLocalPlaylist}><Trash2 size={15} /> Excluir playlist</Button>
      {/if}
    </div>
  </header>

  {#if isLoading}
    <div class="flex items-center justify-center gap-2 py-20 text-xs text-muted" aria-live="polite"><Loader2 size={22} class="animate-spin text-primary" /> Carregando playlist…</div>
  {:else if viewError}
    <div role="alert" class="flex flex-col items-center gap-3 rounded-xl border border-red-500/30 bg-red-500/10 p-8 text-center text-xs text-red-300">
      <strong class="text-foreground">Não foi possível abrir a playlist</strong><span>{viewError}</span>
      {#if $activePlaylistId}<Button size="sm" variant="secondary" on:click={() => loadPlaylistRoute($activePlaylistId || '')}><RefreshCw size={14} /> Tentar novamente</Button>{/if}
    </div>
  {:else if displayedVideos.length > 0}
    <VideoGrid videos={displayedVideos} {viewMode} />
    {#if remotePlaylistID && remoteNextPageToken}
      <div class="flex justify-center border-t border-border pt-4"><Button variant="secondary" disabled={isLoadingMore} on:click={loadMoreRemoteVideos}>{#if isLoadingMore}<Loader2 size={15} class="animate-spin" />{/if} Carregar mais</Button></div>
    {/if}
  {:else}
    <div class="text-center py-20 text-muted text-sm bg-surface/30 rounded-xl border border-dashed border-border">Esta playlist ainda não contém nenhum vídeo.</div>
  {/if}
</section>

<RemotePlaylistSaveModal bind:open={showRemoteSave} remoteID={remotePlaylistID || ''} suggestedName={playlistTitle} suggestedDescription={playlistDescription} on:saved={openSavedPlaylist} />

<ConfirmDialog
  open={pendingConfirm !== null}
  trigger={pendingConfirm?.trigger ?? null}
  danger
  title="Excluir playlist"
  description={`Excluir a playlist “${playlistTitle}”? Os vídeos permanecerão na biblioteca.`}
  confirmLabel="Excluir"
  cancelLabel="Cancelar"
  on:confirm={async () => {
    pendingConfirm = null;
    await applyDeletePlaylist();
  }}
  on:close={() => (pendingConfirm = null)}
/>
