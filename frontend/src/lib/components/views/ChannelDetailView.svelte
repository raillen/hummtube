<script lang="ts">
  import { activePlaylistId, toast } from '../../stores/uiStores';
  import { activeChannelSelection, closeChannel, type ChannelSection } from '../../stores/channelRouteStore';
  import { selectRemotePlaylist } from '../../stores/searchQueryStore';
  import { playerStore } from '../../stores/playerStore';
  import { onMount } from 'svelte';
  import { CatalogService, SearchService, SettingsService } from '../../wailsjs/services';
  import type { ContentViewMode } from '../../stores/uiStores';
  import type { Video } from '../../types';
  import VideoGrid from '../video/VideoGrid.svelte';
  import Button from '../ui/Button.svelte';
  import ContextMenu from '../ui/ContextMenu.svelte';
  import ViewModeToggle from '../ui/ViewModeToggle.svelte';
  import RemotePlaylistSaveModal from '../playlist/RemotePlaylistSaveModal.svelte';
  import AlertCircle from 'lucide-svelte/icons/circle-alert';
  import ArrowLeft from 'lucide-svelte/icons/arrow-left';
  import ListPlus from 'lucide-svelte/icons/list-plus';
  import ListVideo from 'lucide-svelte/icons/list-video';
  import Loader2 from 'lucide-svelte/icons/loader-circle';
  import Play from 'lucide-svelte/icons/play';
  import RefreshCw from 'lucide-svelte/icons/refresh-cw';
  import UserCheck from 'lucide-svelte/icons/user-check';
  import UserPlus from 'lucide-svelte/icons/user-plus';
  import UserRound from 'lucide-svelte/icons/user-round';

  let activeSection: ChannelSection = 'videos';
  let videos: Video[] = [];
  let playlists: Video[] = [];
  let nextVideoPageToken = '';
  let nextPlaylistPageToken = '';
  let isSubscribed = false;
  let isLoading = false;
  let isLoadingMore = false;
  let isUpdatingSubscription = false;
  let viewError: string | null = null;
  let loadedRouteKey = '';
  let loadGeneration = 0;
  let viewMode: ContentViewMode = 'grid';
  let contextPlaylist: Video | null = null;
  let contextX = 0;
  let contextY = 0;
  let contextMenuTrigger: HTMLElement | null = null;
  let playlistToSave: Video | null = null;
  let showPlaylistSave = false;

  $: routeKey = $activeChannelSelection
    ? `${$activeChannelSelection.id}:${$activeChannelSelection.initialSection || 'videos'}`
    : '';
  $: if ($activeChannelSelection && routeKey !== loadedRouteKey) {
    loadedRouteKey = routeKey;
    activeSection = $activeChannelSelection.initialSection || 'videos';
    void loadChannel();
  }

  function errorMessage(error: unknown): string {
    return error instanceof Error && error.message ? error.message : 'Falha ao carregar o canal.';
  }

  async function loadChannel(): Promise<void> {
    const selection = $activeChannelSelection;
    if (!selection) return;
    const generation = ++loadGeneration;
    videos = [];
    playlists = [];
    nextVideoPageToken = '';
    nextPlaylistPageToken = '';
    viewError = null;
    isLoading = true;
    void loadSubscriptionState(selection.id, generation);
    try {
      await loadSectionPage(selection.id, '', false, generation);
    } catch (error: unknown) {
      if (generation === loadGeneration) viewError = errorMessage(error);
    } finally {
      if (generation === loadGeneration) isLoading = false;
    }
  }

  async function loadSubscriptionState(channelID: string, generation: number): Promise<void> {
    try {
      const channels = (await CatalogService.getChannels()) || [];
      if (generation === loadGeneration) isSubscribed = channels.some((channel) => channel.id === channelID && channel.subscribed);
    } catch {
      if (generation === loadGeneration) isSubscribed = false;
    }
  }

  async function loadSectionPage(channelID: string, pageToken: string, append: boolean, generation: number): Promise<void> {
    if (activeSection === 'videos') {
      const page = await CatalogService.getRemoteChannelVideosPage(channelID, pageToken, 24);
      if (generation !== loadGeneration) return;
      videos = mergeUnique(append ? videos : [], page.videos || []);
      nextVideoPageToken = page.next_page_token || '';
      return;
    }
    const page = await SearchService.search({
      query: '', channel_id: channelID, resource_types: ['playlist'],
      order: 'date', max_results: 24, page_token: pageToken || undefined,
    });
    if (generation !== loadGeneration) return;
    playlists = mergeUnique(append ? playlists : [], (page.items || []).filter((item) => item.resource_type === 'playlist'));
    nextPlaylistPageToken = page.next_page_token || '';
  }

  function mergeUnique(existing: Video[], incoming: Video[]): Video[] {
    const known = new Set(existing.map((item) => item.id));
    return [...existing, ...incoming.filter((item) => item.id && !known.has(item.id))];
  }

  async function changeSection(section: ChannelSection): Promise<void> {
    if (section === activeSection || !$activeChannelSelection) return;
    const selection = $activeChannelSelection;
    activeSection = section;
    // Keep the selected section in route state so opening a playlist and
    // returning recreates the channel page in the same context.
    loadedRouteKey = `${selection.id}:${section}`;
    activeChannelSelection.set({ ...selection, initialSection: section });
    const generation = ++loadGeneration;
    viewError = null;
    isLoading = true;
    try {
      await loadSectionPage(selection.id, '', false, generation);
    } catch (error: unknown) {
      if (generation === loadGeneration) viewError = errorMessage(error);
    } finally {
      if (generation === loadGeneration) isLoading = false;
    }
  }

  async function loadMore(): Promise<void> {
    const selection = $activeChannelSelection;
    const token = activeSection === 'videos' ? nextVideoPageToken : nextPlaylistPageToken;
    if (!selection || !token || isLoadingMore) return;
    const generation = loadGeneration;
    isLoadingMore = true;
    try {
      await loadSectionPage(selection.id, token, true, generation);
    } catch (error: unknown) {
      if (generation === loadGeneration) toast.add(errorMessage(error), 'error');
    } finally {
      if (generation === loadGeneration) isLoadingMore = false;
    }
  }

  async function toggleSubscription(): Promise<void> {
    const selection = $activeChannelSelection;
    if (!selection || isUpdatingSubscription) return;
    isUpdatingSubscription = true;
    try {
      if (isSubscribed) {
        await CatalogService.unsubscribeChannel(selection.id);
        isSubscribed = false;
        toast.add('Inscrição local removida.', 'success');
      } else {
        await CatalogService.subscribeChannel(selection.id, selection.title || selection.id);
        isSubscribed = true;
        toast.add('Canal adicionado às inscrições locais.', 'success');
      }
    } catch (error: unknown) {
      toast.add(errorMessage(error), 'error');
    } finally {
      isUpdatingSubscription = false;
    }
  }

  function playAll(): void {
    const [first, ...remaining] = videos;
    if (!first) return;
    for (const video of remaining) void playerStore.addToQueue(video);
    void playerStore.loadVideo(first);
  }

  function addAllToQueue(): void {
    if (videos.length === 0) return;
    for (const video of videos) void playerStore.addToQueue(video);
    toast.add(`${videos.length} vídeos enviados para a fila.`, 'success');
  }

  function openPlaylist(playlist: Video): void {
    const routeID = selectRemotePlaylist(playlist);
    if (routeID) $activePlaylistId = routeID;
  }

  function changeViewMode(mode: ContentViewMode): void {
    viewMode = mode;
    void SettingsService.saveSetting('channel_view_mode', mode).catch(() => undefined);
  }

  function openPlaylistContext(event: MouseEvent | KeyboardEvent, playlist: Video): void {
    event.preventDefault();
    contextPlaylist = playlist;
    contextMenuTrigger = event.currentTarget as HTMLElement;
    if (event instanceof MouseEvent) {
      contextX = event.clientX;
      contextY = event.clientY;
    } else {
      const bounds = contextMenuTrigger.getBoundingClientRect();
      contextX = bounds.left + 24;
      contextY = bounds.top + 24;
    }
  }

  function handlePlaylistContext(event: CustomEvent<string>): void {
    const playlist = contextPlaylist;
    contextPlaylist = null;
    if (!playlist) return;
    if (event.detail === 'save') {
      playlistToSave = playlist;
      showPlaylistSave = true;
      return;
    }
    openPlaylist(playlist);
  }

  onMount(() => {
    void SettingsService.getSettings().then((settings) => {
      if (settings.channel_view_mode === 'list' || settings.channel_view_mode === 'compact') viewMode = settings.channel_view_mode;
    }).catch(() => undefined);
  });
</script>

{#if $activeChannelSelection}
  <section class="mx-auto flex w-full max-w-6xl flex-col gap-6 p-6" aria-labelledby="channel-detail-title">
    <header class="flex flex-col gap-4 rounded-2xl border border-border bg-surface/60 p-5">
      <div class="flex flex-wrap items-center gap-4">
        <button type="button" on:click={closeChannel} aria-label="Voltar à tela anterior" class="rounded-lg p-2 text-muted hover:bg-surfaceHover hover:text-foreground tv-focusable">
          <ArrowLeft size={20} />
        </button>
        {#if $activeChannelSelection.thumbnailUrl}
          <img src={$activeChannelSelection.thumbnailUrl} alt="" class="h-16 w-16 rounded-full border border-border object-cover" />
        {:else}
          <div class="flex h-16 w-16 items-center justify-center rounded-full bg-primary/10 text-primary"><UserRound size={30} /></div>
        {/if}
        <div class="min-w-0 flex-1">
          <h1 id="channel-detail-title" class="truncate text-xl font-bold text-foreground">{$activeChannelSelection.title || 'Canal do YouTube'}</h1>
          <p class="truncate text-xs text-muted">{$activeChannelSelection.id}</p>
        </div>
        <Button variant={isSubscribed ? 'secondary' : 'primary'} size="sm" disabled={isUpdatingSubscription} on:click={toggleSubscription}>
          {#if isUpdatingSubscription}<Loader2 size={15} class="animate-spin" />{:else if isSubscribed}<UserCheck size={15} />{:else}<UserPlus size={15} />{/if}
          {isSubscribed ? 'Inscrito localmente' : 'Adicionar às inscrições'}
        </Button>
      </div>

      <nav class="flex flex-wrap items-center gap-2 border-t border-border pt-4" aria-label="Conteúdo do canal">
        <button type="button" aria-current={activeSection === 'videos' ? 'page' : undefined} on:click={() => changeSection('videos')}
          class="rounded-lg px-3 py-2 text-xs font-semibold tv-focusable {activeSection === 'videos' ? 'bg-primary text-white' : 'bg-surfaceHover text-muted hover:text-foreground'}">
          Vídeos
        </button>
        <button type="button" aria-current={activeSection === 'playlists' ? 'page' : undefined} on:click={() => changeSection('playlists')}
          class="rounded-lg px-3 py-2 text-xs font-semibold tv-focusable {activeSection === 'playlists' ? 'bg-primary text-white' : 'bg-surfaceHover text-muted hover:text-foreground'}">
          Playlists
        </button>
        {#if activeSection === 'videos' && videos.length > 0}
          <div class="ml-auto flex flex-wrap gap-2">
            <Button variant="secondary" size="sm" on:click={addAllToQueue}><ListPlus size={14} /> Adicionar tudo à fila</Button>
            <Button variant="primary" size="sm" on:click={playAll}><Play size={14} /> Reproduzir tudo</Button>
          </div>
        {/if}
        <ViewModeToggle value={viewMode} onChange={changeViewMode} label={`Visualização de ${activeSection === 'videos' ? 'vídeos' : 'playlists'} do canal`} />
      </nav>
    </header>

    {#if viewError}
      <section role="alert" class="flex flex-wrap items-center gap-3 rounded-xl border border-red-500/30 bg-red-500/10 p-4 text-xs text-red-200">
        <AlertCircle size={18} /><span class="flex-1">{viewError}</span>
        <Button variant="secondary" size="sm" on:click={loadChannel}><RefreshCw size={14} /> Tentar novamente</Button>
      </section>
    {/if}

    {#if isLoading}
      <div class="flex items-center justify-center gap-2 py-20 text-xs text-muted" aria-live="polite"><Loader2 size={24} class="animate-spin text-primary" /> Carregando canal…</div>
    {:else if activeSection === 'videos' && videos.length > 0}
      <VideoGrid {videos} {viewMode} />
    {:else if activeSection === 'playlists' && playlists.length > 0}
      <div class={viewMode === 'grid' ? 'grid grid-cols-1 gap-3 md:grid-cols-2 xl:grid-cols-3' : 'flex flex-col gap-2'}>
        {#each playlists as playlist (playlist.id)}
          <button type="button" aria-haspopup="menu" on:click={() => openPlaylist(playlist)} on:contextmenu={(event) => openPlaylistContext(event, playlist)} on:keydown={(event) => { if (event.key === 'ContextMenu' || event.key === 'Apps' || (event.key === 'F10' && event.shiftKey)) openPlaylistContext(event, playlist); }} class="flex items-center gap-3 rounded-xl border border-border bg-surface text-left hover:border-primary/50 hover:bg-surfaceHover tv-focusable {viewMode === 'compact' ? 'p-2' : 'p-4'}">
            <div class="flex shrink-0 items-center justify-center overflow-hidden rounded-lg bg-primary/10 text-primary {viewMode === 'compact' ? 'h-9 w-9' : 'h-12 w-12'}">
              {#if playlist.thumbnail_url}<img src={playlist.thumbnail_url} alt="" class="h-full w-full object-cover" />{:else}<ListVideo size={22} />{/if}
            </div>
            <div class="min-w-0"><h2 class="line-clamp-2 text-sm font-semibold text-foreground">{playlist.title}</h2><p class="mt-1 text-[10px] uppercase tracking-wide text-muted">Playlist do YouTube{playlist.resource_item_count ? ` · ${playlist.resource_item_count} vídeos` : ''}</p></div>
          </button>
        {/each}
      </div>
    {:else if !viewError}
      <div class="flex flex-col items-center justify-center gap-2 rounded-xl border border-dashed border-border py-16 text-center text-muted">
        {#if activeSection === 'videos'}<Play size={24} />{:else}<ListVideo size={24} />{/if}
        <p class="text-sm font-semibold text-foreground">Nenhum conteúdo encontrado</p>
        <p class="text-xs">{activeSection === 'playlists' ? 'Conecte uma conta para consultar playlists oficiais do canal.' : 'O canal não retornou vídeos públicos.'}</p>
      </div>
    {/if}

    {#if (activeSection === 'videos' && nextVideoPageToken) || (activeSection === 'playlists' && nextPlaylistPageToken)}
      <div class="flex justify-center border-t border-border pt-5">
        <Button variant="secondary" disabled={isLoadingMore} on:click={loadMore}>{#if isLoadingMore}<Loader2 size={15} class="animate-spin" />{/if} Carregar mais</Button>
      </div>
    {/if}
  </section>
{/if}

<ContextMenu open={contextPlaylist !== null} x={contextX} y={contextY} actions={[{ id: 'open', label: 'Abrir playlist' }, { id: 'save', label: 'Salvar ou adicionar à playlist existente' }]} on:select={handlePlaylistContext} on:close={() => { contextPlaylist = null; contextMenuTrigger?.focus(); }} />
<RemotePlaylistSaveModal bind:open={showPlaylistSave} remoteID={playlistToSave?.id || ''} suggestedName={playlistToSave?.title || ''} suggestedDescription={playlistToSave?.description || playlistToSave?.description_excerpt || ''} />
