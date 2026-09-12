<script lang="ts">
  import { onDestroy, onMount, tick } from 'svelte';
  import Play from 'lucide-svelte/icons/play';
  import Pause from 'lucide-svelte/icons/pause';
  import Volume2 from 'lucide-svelte/icons/volume-2';
  import VolumeX from 'lucide-svelte/icons/volume-x';
  import Maximize from 'lucide-svelte/icons/maximize';
  import Minimize from 'lucide-svelte/icons/minimize';
  import RotateCcw from 'lucide-svelte/icons/rotate-ccw';
  import RotateCw from 'lucide-svelte/icons/rotate-cw';
  import Sliders from 'lucide-svelte/icons/sliders-vertical';
  import Moon from 'lucide-svelte/icons/moon';
  import Minimize2 from 'lucide-svelte/icons/minimize-2';
  import StickyNote from 'lucide-svelte/icons/sticky-note';
  import ListPlus from 'lucide-svelte/icons/list-plus';
  import Heart from 'lucide-svelte/icons/heart';
  import ListMusic from 'lucide-svelte/icons/list-music';
  import SkipBack from 'lucide-svelte/icons/skip-back';
  import SkipForward from 'lucide-svelte/icons/skip-forward';
  import ArrowLeft from 'lucide-svelte/icons/arrow-left';
  import Info from 'lucide-svelte/icons/info';
  import ListVideo from 'lucide-svelte/icons/list-video';
  import Check from 'lucide-svelte/icons/check';
  import MonitorCog from 'lucide-svelte/icons/monitor-cog';
  import Music from 'lucide-svelte/icons/music-2';
  import FileText from 'lucide-svelte/icons/file-text';
  import type { PlayerQualityOption } from '../../player/quality';
  import { playerStore } from '../../stores/playerStore';
  import { get } from 'svelte/store';
  import { LibraryService } from '../../wailsjs/services';
  import { toast } from '../../stores/uiStores';
  import { openChannel } from '../../stores/channelRouteStore';
  import AudioDSPModal from './AudioDSPModal.svelte';
  import SleepTimerModal from './SleepTimerModal.svelte';
  import VideoNotesModal from './VideoNotesModal.svelte';
  import AddToPlaylistModal from './AddToPlaylistModal.svelte';
  import QueueModal from './QueueModal.svelte';

  export let videoElement: HTMLVideoElement;
  export let qualityOptions: PlayerQualityOption[] = [];
  export let selectedQualityID = 'auto';
  export let activeQualityLabel: string = 'Automática';
  export let audioTracks: { id: string; label: string; language?: string }[] = [];
  export let activeAudioTrackID: string = '';
  export let onAudioTrackChange: (trackID: string) => void = () => undefined;
  export let subtitles: { language: string; label?: string; url: string; format?: string }[] = [];
  export let activeSubtitleURL: string = '';
  export let onSubtitleChange: (subtitleURL: string) => void = () => undefined;
  export let onQualityChange: (optionID: string) => void = () => undefined;
  export let isFullscreen = false;
  export let onToggleFullscreen: () => void = () => undefined;
  export let isDSPAvailable = true;
  export let dspUnavailableReason = '';

  let showDSPModal = false;
  let showSleepModal = false;
  let showNotesModal = false;
  let showPlaylistModal = false;
  let showQueueModal = false;
  let isFavorite = false;
  let lastFavoriteCheckID = '';
  $: if ($playerStore.currentVideo && $playerStore.currentVideo.id !== lastFavoriteCheckID) {
    lastFavoriteCheckID = $playerStore.currentVideo.id;
    const currentID = $playerStore.currentVideo.id;
    isFavorite = false;
    LibraryService.getFavorites()
      .then((favorites) => {
        if (lastFavoriteCheckID === currentID) {
          isFavorite = Array.isArray(favorites) && favorites.some((entry) => entry.id === currentID);
        }
      })
      .catch(() => undefined);
  }
  let showVideoInfo = false;
  let showQualityMenu = false;
  let showAudioMenu = false;
  let showSubtitleMenu = false;
  let qualityButtonElement: HTMLButtonElement;
  let qualityMenuElement: HTMLDivElement;
  let audioButtonElement: HTMLButtonElement;
  let audioMenuElement: HTMLDivElement;
  let subtitleButtonElement: HTMLButtonElement;
  let subtitleMenuElement: HTMLDivElement;
  let infoTab: 'about' | 'queue' = 'about';

  $: selectedQuality = qualityOptions.find((option) => option.id === selectedQualityID) ?? qualityOptions[0];
  $: isQualityAvailable = qualityOptions.length > 0;
  $: requestedQualityLabel = selectedQuality?.label || ($playerStore.isLoading ? 'Carregando' : 'Indisponível');
  $: qualityButtonLabel = activeQualityLabel && activeQualityLabel !== 'Automática' && activeQualityLabel !== requestedQualityLabel
    ? `Reproduzindo ${activeQualityLabel} (pediu ${requestedQualityLabel})`
    : requestedQualityLabel;
  $: videoQualityCount = qualityOptions.filter((option) => option.source === 'plan' || option.source === 'hls').length;
  $: audioButtonLabel = audioTracks.find((track) => track.id === activeAudioTrackID)?.label
    || audioTracks[0]?.label
    || 'Áudio original';
  $: subtitleButtonLabel = activeSubtitleURL === ''
    ? 'Legendas'
    : 'Legendas ativas';

  function formatTime(secs: number) {
    if (isNaN(secs)) return '0:00';
    const m = Math.floor(secs / 60);
    const s = Math.floor(secs % 60);
    return `${m}:${s < 10 ? '0' : ''}${s}`;
  }

  function handleSeek(e: Event) {
    const target = e.target as HTMLInputElement;
    const time = parseFloat(target.value);
    if (videoElement) {
      videoElement.currentTime = time;
    }
  }

  function handleSeekBarKey(event: KeyboardEvent) {
    if (!videoElement) return;
    if (event.key === 'ArrowLeft') {
      event.preventDefault();
      videoElement.currentTime = Math.max(0, videoElement.currentTime - 5);
    } else if (event.key === 'ArrowRight') {
      event.preventDefault();
      videoElement.currentTime = Math.min(videoElement.duration || 0, videoElement.currentTime + 5);
    } else if (event.key === 'ArrowUp') {
      event.preventDefault();
      const current = get(playerStore) as { volume: number };
      playerStore.setVolume(Math.min(1, current.volume + 0.05));
    } else if (event.key === 'ArrowDown') {
      event.preventDefault();
      const current = get(playerStore) as { volume: number };
      playerStore.setVolume(Math.max(0, current.volume - 0.05));
    }
  }

  function handleVolumeBarKey(event: KeyboardEvent) {
    if (event.key === 'ArrowLeft') {
      event.preventDefault();
      const current = get(playerStore) as { volume: number };
      playerStore.setVolume(Math.max(0, current.volume - 0.05));
    } else if (event.key === 'ArrowRight') {
      event.preventDefault();
      const current = get(playerStore) as { volume: number };
      playerStore.setVolume(Math.min(1, current.volume + 0.05));
    }
  }

  function seekRelative(delta: number) {
    if (!videoElement) return;
    videoElement.currentTime = Math.max(0, Math.min(videoElement.duration || 0, videoElement.currentTime + delta));
  }

  function toggleSpeed() {
    const speeds = [0.5, 0.75, 1.0, 1.25, 1.5, 1.75, 2.0];
    const currentIndex = speeds.indexOf($playerStore.speed);
    const nextSpeed = speeds[(currentIndex + 1) % speeds.length];
    playerStore.setSpeed(nextSpeed);
  }

  async function toggleQualityMenu(): Promise<void> {
    if (!isQualityAvailable) return;
    showQualityMenu = !showQualityMenu;
    if (!showQualityMenu) return;
    await tick();
    qualityMenuElement?.querySelector<HTMLElement>('[aria-checked="true"]')?.focus();
  }

  function selectQuality(optionID: string): void {
    onQualityChange(optionID);
    showQualityMenu = false;
    qualityButtonElement?.focus();
  }

  function closeQualityMenuFromDocument(event: MouseEvent): void {
    if (!showQualityMenu) return;
    const target = event.target;
    if (!(target instanceof Node) || qualityMenuElement?.contains(target) || qualityButtonElement?.contains(target)) return;
    showQualityMenu = false;
  }

  function closeQualityMenuWithEscape(event: KeyboardEvent): void {
    if (event.key !== 'Escape' || !showQualityMenu) return;
    event.stopPropagation();
    showQualityMenu = false;
    qualityButtonElement?.focus();
  }

  function navigateQualityMenu(event: KeyboardEvent): void {
    if (!showQualityMenu || !qualityMenuElement) return;
    const options = Array.from(qualityMenuElement.querySelectorAll<HTMLButtonElement>('[role="menuitemradio"]'));
    if (options.length === 0) return;

    const currentIndex = Math.max(0, options.indexOf(document.activeElement as HTMLButtonElement));
    let nextIndex: number | null = null;
    if (event.key === 'ArrowDown') nextIndex = (currentIndex + 1) % options.length;
    if (event.key === 'ArrowUp') nextIndex = (currentIndex - 1 + options.length) % options.length;
    if (event.key === 'Home') nextIndex = 0;
    if (event.key === 'End') nextIndex = options.length - 1;
    if (nextIndex === null) return;

    event.preventDefault();
    options[nextIndex]?.focus();
  }

  function closeSecondaryMenus(): void {
    showAudioMenu = false;
    showSubtitleMenu = false;
  }

  async function toggleAudioMenu(): Promise<void> {
    if (audioTracks.length === 0) return;
    showQualityMenu = false;
    showAudioMenu = !showAudioMenu;
    if (!showAudioMenu) return;
    await tick();
    audioMenuElement?.querySelector<HTMLElement>('[aria-checked="true"]')?.focus();
  }

  async function toggleSubtitleMenu(): Promise<void> {
    if (subtitles.length === 0) return;
    showQualityMenu = false;
    showSubtitleMenu = !showSubtitleMenu;
    if (!showSubtitleMenu) return;
    await tick();
    subtitleMenuElement?.querySelector<HTMLElement>('[aria-checked="true"]')?.focus();
  }

  function navigateAudioMenu(event: KeyboardEvent): void {
    if (!showAudioMenu || !audioMenuElement) return;
    const options = Array.from(audioMenuElement.querySelectorAll<HTMLButtonElement>('[role="menuitemradio"]'));
    if (options.length === 0) return;
    const currentIndex = Math.max(0, options.indexOf(document.activeElement as HTMLButtonElement));
    let nextIndex: number | null = null;
    if (event.key === 'ArrowDown') nextIndex = (currentIndex + 1) % options.length;
    if (event.key === 'ArrowUp') nextIndex = (currentIndex - 1 + options.length) % options.length;
    if (event.key === 'Home') nextIndex = 0;
    if (event.key === 'End') nextIndex = options.length - 1;
    if (nextIndex === null) return;
    event.preventDefault();
    options[nextIndex]?.focus();
  }

  function navigateSubtitleMenu(event: KeyboardEvent): void {
    if (!showSubtitleMenu || !subtitleMenuElement) return;
    const options = Array.from(subtitleMenuElement.querySelectorAll<HTMLButtonElement>('[role="menuitemradio"]'));
    if (options.length === 0) return;
    const currentIndex = Math.max(0, options.indexOf(document.activeElement as HTMLButtonElement));
    let nextIndex: number | null = null;
    if (event.key === 'ArrowDown') nextIndex = (currentIndex + 1) % options.length;
    if (event.key === 'ArrowUp') nextIndex = (currentIndex - 1 + options.length) % options.length;
    if (event.key === 'Home') nextIndex = 0;
    if (event.key === 'End') nextIndex = options.length - 1;
    if (nextIndex === null) return;
    event.preventDefault();
    options[nextIndex]?.focus();
  }

  async function toggleFavorite() {
    if (!$playerStore.currentVideo?.id) return;
    const vid = $playerStore.currentVideo.id;
    try {
      if (isFavorite) {
        await LibraryService.removeFavorite(vid);
        isFavorite = false;
        toast.add('Vídeo removido dos favoritos', 'info');
      } else {
        await LibraryService.addFavorite(vid);
        isFavorite = true;
        toast.add('Vídeo adicionado aos favoritos!', 'success');
      }
    } catch (e) {
      toast.add('Erro ao atualizar favorito', 'error');
    }
  }

  function openChannelContent(resource: 'video' | 'playlist'): void {
    const video = $playerStore.currentVideo;
    if (!video?.channel_id) return;
    openChannel({
      id: video.channel_id,
      title: video.channel_title,
      initialSection: resource === 'playlist' ? 'playlists' : 'videos',
    });
    playerStore.setMiniplayer(true);
  }

  onMount(() => {
    document.addEventListener('pointerdown', closeQualityMenuFromDocument);
    document.addEventListener('keydown', closeQualityMenuWithEscape);
  });

  onDestroy(() => {
    document.removeEventListener('pointerdown', closeQualityMenuFromDocument);
    document.removeEventListener('keydown', closeQualityMenuWithEscape);
  });
</script>

<button
  type="button"
  on:click={() => playerStore.setMiniplayer(true)}
  class="pointer-events-auto absolute left-4 top-4 z-30 flex items-center gap-1.5 rounded-lg bg-black/70 px-3 py-2 text-xs font-semibold text-white shadow-lg hover:bg-black/90 focus:outline-none focus:ring-2 focus:ring-white/70"
  aria-label="Voltar à interface e manter o vídeo no miniplayer"
>
  <ArrowLeft size={17} />
  <span>Voltar à interface</span>
</button>

<div class="pointer-events-none absolute inset-0 z-20 flex flex-col justify-between bg-gradient-to-t from-black/85 via-transparent to-transparent p-4" data-testid="player-controls-layer">
  <!-- Top Bar -->
  <div class="pointer-events-none flex items-center justify-between opacity-0 transition-opacity group-hover:opacity-100 focus-within:opacity-100">
    <div class="pointer-events-auto ml-48 flex min-w-0 max-w-xl items-center gap-2">
      <h3 class="text-sm font-semibold text-white truncate">
        {$playerStore.currentVideo?.title || ''}
      </h3>
      {#if $playerStore.currentVideo?.channel_title}
        <span class="text-xs text-white/60 truncate shrink-0">• {$playerStore.currentVideo.channel_title}</span>
      {/if}
    </div>

    <div class="pointer-events-auto flex items-center gap-1.5">
      <!-- Informações da mídia -->
      <button
        type="button"
        on:click={() => (showVideoInfo = !showVideoInfo)}
        class="p-2 text-white/80 hover:text-white rounded-lg hover:bg-white/10 transition-colors"
        aria-expanded={showVideoInfo}
        title="Informações do vídeo"
      >
        <Info size={18} />
      </button>

      {#if $playerStore.sourceKind !== 'iptv'}
      <!-- Fila e ações de biblioteca pertencem aos clientes YouTube. -->
      <button
        type="button"
        on:click={() => (showQueueModal = true)}
        class="relative p-2 text-white/80 hover:text-white rounded-lg hover:bg-white/10 transition-colors"
        title="Fila de Reprodução"
      >
        <ListMusic size={18} />
        {#if $playerStore.queue.length > 0}
          <span class="absolute top-1 right-1 w-2 h-2 rounded-full bg-primary animate-pulse"></span>
        {/if}
      </button>

      <!-- Notes & Bookmarks -->
      <button
        type="button"
        on:click={() => (showNotesModal = true)}
        class="p-2 text-white/80 hover:text-white rounded-lg hover:bg-white/10 transition-colors"
        title="Notas e Marcadores"
      >
        <StickyNote size={18} />
      </button>

      <!-- Add to Playlist -->
      <button
        type="button"
        on:click={() => (showPlaylistModal = true)}
        class="p-2 text-white/80 hover:text-white rounded-lg hover:bg-white/10 transition-colors"
        title="Salvar na Playlist"
      >
        <ListPlus size={18} />
      </button>

      <!-- Favorite Toggle -->
      <button
        type="button"
        on:click={toggleFavorite}
        class="p-2 text-white/80 hover:text-red-400 rounded-lg hover:bg-white/10 transition-colors"
        title={isFavorite ? 'Remover dos favoritos' : 'Favoritar vídeo'}
      >
        <Heart size={18} fill={isFavorite ? 'currentColor' : 'none'} class={isFavorite ? 'text-red-500' : ''} />
      </button>
      {/if}

      <!-- Minimize to Miniplayer -->
      <button
        type="button"
        on:click={() => playerStore.setMiniplayer(true)}
        class="p-2 text-white/80 hover:text-white rounded-lg hover:bg-white/10 transition-colors"
        title="Minimizar para Miniplayer"
      >
        <Minimize2 size={18} />
      </button>
    </div>
  </div>

  <!-- Bottom Controls -->
  <div class="pointer-events-auto flex flex-col gap-2 opacity-100" data-testid="player-bottom-controls">
    <!-- Seek Bar -->
    <div class="flex items-center gap-3">
      <span class="text-xs text-white/80 font-mono">{formatTime($playerStore.currentTime)}</span>
      <input
        type="range"
        min="0"
        max={$playerStore.duration || 100}
        value={$playerStore.currentTime}
        on:input={handleSeek}
        on:keydown={handleSeekBarKey}
        data-component="player-seek-bar"
        aria-label="Linha do tempo do vídeo"
        class="flex-1 h-1.5 bg-white/20 rounded-lg appearance-none cursor-pointer accent-primary"
      />
      <span class="text-xs text-white/80 font-mono">{formatTime($playerStore.duration)}</span>
    </div>

    <!-- Buttons Bar -->
    <div class="flex items-center justify-between">
      <div class="flex items-center gap-1.5">
        {#if $playerStore.sourceKind !== 'iptv'}
        <!-- Previous Track -->
        <button
          type="button"
          on:click={playerStore.playPrevious}
          disabled={$playerStore.historyQueue.length === 0}
          class="p-2 rounded-lg text-white/80 hover:text-white hover:bg-white/10 disabled:opacity-30 disabled:cursor-not-allowed transition-colors"
          title="Vídeo Anterior"
        >
          <SkipBack size={18} />
        </button>
        {/if}

        <!-- Play / Pause -->
        <button
          type="button"
          on:click={playerStore.togglePlay}
          data-component="player-control-button"
          class="p-2 rounded-lg text-white hover:bg-white/10 transition-colors"
        >
          {#if $playerStore.isPaused}
            <Play size={20} fill="currentColor" />
          {:else}
            <Pause size={20} fill="currentColor" />
          {/if}
        </button>

        {#if $playerStore.sourceKind !== 'iptv'}
        <!-- Next Track -->
        <button
          type="button"
          on:click={playerStore.playNext}
          disabled={$playerStore.queue.length === 0}
          class="p-2 rounded-lg text-white/80 hover:text-white hover:bg-white/10 disabled:opacity-30 disabled:cursor-not-allowed transition-colors"
          title="Próximo Vídeo da Fila"
        >
          <SkipForward size={18} />
        </button>
        {/if}

        <!-- Rewind 10s -->
        <button
          type="button"
          on:click={() => seekRelative(-10)}
          class="p-2 rounded-lg text-white/80 hover:text-white hover:bg-white/10 transition-colors"
          title="Voltar 10s (J)"
        >
          <RotateCcw size={17} />
        </button>

        <!-- Forward 10s -->
        <button
          type="button"
          on:click={() => seekRelative(10)}
          class="p-2 rounded-lg text-white/80 hover:text-white hover:bg-white/10 transition-colors"
          title="Avançar 10s (L)"
        >
          <RotateCw size={17} />
        </button>

        <div class="flex items-center gap-2 group/vol ml-1">
          <button type="button" on:click={playerStore.toggleMute} class="p-2 text-white hover:bg-white/10 rounded-lg">
            {#if $playerStore.isMuted || $playerStore.volume === 0}
              <VolumeX size={19} />
            {:else}
              <Volume2 size={19} />
            {/if}
          </button>
          <input
            type="range"
            min="0"
            max="1"
            step="0.05"
            value={$playerStore.volume}
            on:input={(e) => playerStore.setVolume(parseFloat(e.currentTarget.value))}
            on:keydown={handleVolumeBarKey}
            data-component="player-volume-bar"
            aria-label="Volume do player"
            class="w-16 h-1 bg-white/20 rounded-lg appearance-none cursor-pointer accent-primary"
          />
        </div>

        <!-- Audio Track -->
        {#if audioTracks.length > 0}
        <div class="relative shrink-0">
          <button
            bind:this={audioButtonElement}
            type="button"
            on:click={toggleAudioMenu}
            class="flex items-center gap-1.5 rounded-lg p-2 text-white/80 transition-colors hover:bg-white/10 hover:text-white focus:outline-none focus:ring-2 focus:ring-white/70"
            aria-label={audioButtonLabel}
            aria-haspopup="menu"
            aria-expanded={showAudioMenu}
            aria-controls="player-audio-menu"
            title="Faixa de áudio"
          >
            <Music size={18} />
            <span class="hidden text-[11px] font-bold sm:inline">{audioButtonLabel}</span>
          </button>

          {#if showAudioMenu}
            <div
              bind:this={audioMenuElement}
              id="player-audio-menu"
              role="menu"
              tabindex="-1"
              aria-label="Selecionar faixa de áudio"
              on:keydown={navigateAudioMenu}
              class="absolute bottom-12 right-0 z-40 w-64 overflow-hidden rounded-xl border border-white/15 bg-black/95 p-2 text-white shadow-2xl backdrop-blur-md"
              data-testid="player-audio-menu"
            >
              <div class="border-b border-white/10 px-2 pb-2 pt-1">
                <strong class="block text-xs">Faixa de Áudio</strong>
                <span class="text-[10px] text-white/55">Escolha a faixa de áudio.</span>
              </div>
              <div class="mt-1 flex max-h-64 flex-col overflow-y-auto">
                {#each audioTracks as track (track.id)}
                  <button
                    type="button"
                    role="menuitemradio"
                    aria-checked={track.id === activeAudioTrackID}
                    on:click={() => onAudioTrackChange(track.id)}
                    class="flex w-full items-center gap-3 rounded-lg px-2.5 py-2 text-left text-xs transition-colors hover:bg-white/10 focus:bg-white/10 focus:outline-none"
                    data-audio-id={track.id}
                  >
                    <span class="flex h-5 w-5 shrink-0 items-center justify-center rounded-full border border-white/20 {track.id === activeAudioTrackID ? 'border-primary bg-primary text-white' : ''}">
                      {#if track.id === activeAudioTrackID}<Check size={13} strokeWidth={3} />{/if}
                    </span>
                    <span class="min-w-0 flex-1">
                      <span class="block font-semibold">{track.label}</span>
                      {#if track.language}<span class="block text-[10px] text-white/50">{track.language}</span>{/if}
                    </span>
                  </button>
                {/each}
              </div>
            </div>
          {/if}
        </div>
        {/if}

        <!-- Subtitle Track -->
        {#if subtitles.length > 0}
        <div class="relative shrink-0">
          <button
            bind:this={subtitleButtonElement}
            type="button"
            on:click={toggleSubtitleMenu}
            class="flex items-center gap-1.5 rounded-lg p-2 text-white/80 transition-colors hover:bg-white/10 hover:text-white focus:outline-none focus:ring-2 focus:ring-white/70"
            aria-label={subtitleButtonLabel}
            aria-haspopup="menu"
            aria-expanded={showSubtitleMenu}
            aria-controls="player-subtitle-menu"
            title="Legendas"
          >
            <FileText size={18} />
            <span class="hidden text-[11px] font-bold sm:inline">{subtitleButtonLabel}</span>
          </button>

          {#if showSubtitleMenu}
            <div
              bind:this={subtitleMenuElement}
              id="player-subtitle-menu"
              role="menu"
              tabindex="-1"
              aria-label="Selecionar legenda"
              on:keydown={navigateSubtitleMenu}
              class="absolute bottom-12 right-0 z-40 w-64 overflow-hidden rounded-xl border border-white/15 bg-black/95 p-2 text-white shadow-2xl backdrop-blur-md"
              data-testid="player-subtitle-menu"
            >
              <div class="border-b border-white/10 px-2 pb-2 pt-1">
                <strong class="block text-xs">Legendas</strong>
                <span class="text-[10px] text-white/55">Escolha a legenda ou desative.</span>
              </div>
              <div class="mt-1 flex max-h-64 flex-col overflow-y-auto">
                <button
                  type="button"
                  role="menuitemradio"
                  aria-checked={activeSubtitleURL === ''}
                  on:click={() => onSubtitleChange('')}
                  class="flex w-full items-center gap-3 rounded-lg px-2.5 py-2 text-left text-xs transition-colors hover:bg-white/10 focus:bg-white/10 focus:outline-none"
                >
                  <span class="flex h-5 w-5 shrink-0 items-center justify-center rounded-full border border-white/20 {activeSubtitleURL === '' ? 'border-primary bg-primary text-white' : ''}">
                    {#if activeSubtitleURL === ''}<Check size={13} strokeWidth={3} />{/if}
                  </span>
                  <span class="min-w-0 flex-1"><span class="block font-semibold">Desativado</span></span>
                </button>
                {#each subtitles as track (track.url)}
                  <button
                    type="button"
                    role="menuitemradio"
                    aria-checked={track.url === activeSubtitleURL}
                    on:click={() => onSubtitleChange(track.url)}
                    class="flex w-full items-center gap-3 rounded-lg px-2.5 py-2 text-left text-xs transition-colors hover:bg-white/10 focus:bg-white/10 focus:outline-none"
                  >
                    <span class="flex h-5 w-5 shrink-0 items-center justify-center rounded-full border border-white/20 {track.url === activeSubtitleURL ? 'border-primary bg-primary text-white' : ''}">
                      {#if track.url === activeSubtitleURL}<Check size={13} strokeWidth={3} />{/if}
                    </span>
                    <span class="min-w-0 flex-1">
                      <span class="block font-semibold">{track.label || track.language}</span>
                      {#if track.language}<span class="block text-[10px] text-white/50">{track.language}</span>{/if}
                    </span>
                  </button>
                {/each}
              </div>
            </div>
          {/if}
        </div>
        {/if}
      </div>

      <div class="flex items-center gap-1.5">
        <!-- Speed -->
        <button
          type="button"
          on:click={toggleSpeed}
          class="px-2.5 py-1 text-xs font-bold text-white bg-white/10 hover:bg-white/20 rounded-lg transition-colors"
        >
          {$playerStore.speed}x
        </button>

        <div class="relative shrink-0">
            <button
              bind:this={qualityButtonElement}
              type="button"
              on:click={toggleQualityMenu}
              disabled={!isQualityAvailable}
              class="flex items-center gap-1.5 rounded-lg p-2 text-white/80 transition-colors hover:bg-white/10 hover:text-white focus:outline-none focus:ring-2 focus:ring-white/70 disabled:cursor-wait disabled:opacity-55"
              aria-label={`Qualidade do vídeo: ${qualityButtonLabel}`}
              aria-haspopup="menu"
              aria-expanded={showQualityMenu}
              aria-controls="player-quality-menu"
              title="Qualidade do vídeo"
              data-testid="player-quality-button"
              data-state={isQualityAvailable ? 'ready' : 'unavailable'}
            >
              <MonitorCog size={18} />
              <span class="hidden text-[11px] font-bold sm:inline">{qualityButtonLabel}</span>
            </button>

            {#if showQualityMenu && isQualityAvailable}
              <div
                bind:this={qualityMenuElement}
                id="player-quality-menu"
                role="menu"
                tabindex="-1"
                aria-label="Selecionar qualidade do vídeo"
                on:keydown={navigateQualityMenu}
                class="absolute bottom-12 right-0 z-40 w-60 overflow-hidden rounded-xl border border-white/15 bg-black/95 p-2 text-white shadow-2xl backdrop-blur-md"
                data-testid="player-quality-menu"
              >
                <div class="border-b border-white/10 px-2 pb-2 pt-1">
                  <strong class="block text-xs">Qualidade</strong>
                  <span class="text-[10px] text-white/55">Escolha a resolução ou deixe o player decidir.</span>
                </div>
                <div class="mt-1 flex max-h-64 flex-col overflow-y-auto">
                  {#each qualityOptions as option (option.id)}
                    <button
                      type="button"
                      role="menuitemradio"
                      aria-checked={option.id === selectedQualityID}
                      on:click={() => selectQuality(option.id)}
                      class="flex w-full items-center gap-3 rounded-lg px-2.5 py-2 text-left text-xs transition-colors hover:bg-white/10 focus:bg-white/10 focus:outline-none"
                      data-quality-id={option.id}
                    >
                      <span class="flex h-5 w-5 shrink-0 items-center justify-center rounded-full border border-white/20 {option.id === selectedQualityID ? 'border-primary bg-primary text-white' : ''}">
                        {#if option.id === selectedQualityID}<Check size={13} strokeWidth={3} />{/if}
                      </span>
                      <span class="min-w-0 flex-1">
                        <span class="block font-semibold">{option.label}</span>
                        {#if option.requiresSeparateAudio}<span class="block text-[10px] text-white/50">Vídeo e áudio adaptativos</span>{/if}
                      </span>
                    </button>
                  {/each}
                </div>
                {#if videoQualityCount <= 1}
                  <p class="mt-1 border-t border-white/10 px-2 pt-2 text-[10px] leading-relaxed text-amber-200/80">
                    O provedor expôs poucas resoluções reproduzíveis para esta mídia.
                  </p>
                {/if}
              </div>
            {/if}
        </div>

        <!-- DSP Gain Modal -->
        <button
          type="button"
          on:click={() => (showDSPModal = true)}
          class="p-2 text-white/80 hover:text-white hover:bg-white/10 rounded-lg transition-colors"
          title="DSP de Áudio (Ganho / Boost)"
          aria-label="DSP de áudio e ganho digital"
        >
          <Sliders size={18} />
        </button>

        <!-- Sleep Timer -->
        <button
          type="button"
          on:click={() => (showSleepModal = true)}
          class="p-2 text-white/80 hover:text-white hover:bg-white/10 rounded-lg transition-colors"
          title="Temporizador de Sono"
        >
          <Moon size={18} />
        </button>

        <!-- Fullscreen -->
        <button
          type="button"
          on:click={onToggleFullscreen}
          class="p-2 text-white/80 hover:text-white hover:bg-white/10 rounded-lg transition-colors"
          aria-label={isFullscreen ? 'Sair da tela cheia' : 'Entrar em tela cheia'}
          title={isFullscreen ? 'Sair da tela cheia' : 'Entrar em tela cheia'}
        >
          {#if isFullscreen}
            <Minimize size={18} />
          {:else}
            <Maximize size={18} />
          {/if}
        </button>
      </div>
    </div>
  </div>
</div>

{#if showVideoInfo && $playerStore.currentVideo}
  <aside class="pointer-events-auto absolute bottom-24 left-4 z-30 max-h-[calc(100%-8rem)] w-[min(38rem,calc(100%-2rem))] overflow-hidden rounded-2xl border border-white/15 bg-black/92 p-5 text-white shadow-2xl backdrop-blur-md" aria-label="Informações do vídeo e fila">
    <div class="flex items-start justify-between gap-3">
      <div class="min-w-0">
        <h3 class="line-clamp-2 text-sm font-bold">{$playerStore.currentVideo.title}</h3>
        <p class="mt-1 text-xs text-white/65">{$playerStore.currentVideo.channel_title || 'Canal'}</p>
      </div>
      <button type="button" class="rounded-lg px-2 py-1 text-xs text-white/70 hover:bg-white/10 hover:text-white" on:click={() => (showVideoInfo = false)}>Fechar</button>
    </div>
    <div class="mt-4 flex rounded-lg border border-white/10 bg-white/5 p-1" role="tablist" aria-label="Detalhes do player">
      <button type="button" role="tab" aria-selected={infoTab === 'about'} on:click={() => (infoTab = 'about')} class="flex-1 rounded-md px-3 py-2 text-xs font-semibold {infoTab === 'about' ? 'bg-white/15 text-white' : 'text-white/60 hover:text-white'}">Sobre</button>
      {#if $playerStore.sourceKind !== 'iptv'}
        <button type="button" role="tab" aria-selected={infoTab === 'queue'} on:click={() => (infoTab = 'queue')} class="flex-1 rounded-md px-3 py-2 text-xs font-semibold {infoTab === 'queue' ? 'bg-white/15 text-white' : 'text-white/60 hover:text-white'}">Fila ({$playerStore.queue.length})</button>
      {/if}
    </div>
    {#if infoTab === 'about'}
      <p class="mt-4 max-h-[min(34vh,18rem)] overflow-y-auto whitespace-pre-wrap text-sm leading-relaxed text-white/80">
        {$playerStore.currentVideo.description || $playerStore.currentVideo.description_excerpt || 'Este provedor não forneceu uma descrição para o vídeo.'}
      </p>
      {#if $playerStore.sourceKind === 'youtube'}
        <div class="mt-4 flex flex-wrap gap-2 border-t border-white/10 pt-3">
          <button type="button" class="inline-flex items-center gap-1.5 rounded-lg bg-white/10 px-3 py-2 text-xs font-semibold hover:bg-white/20" on:click={() => openChannelContent('video')}><ListVideo size={14} /> Vídeos do canal</button>
          <button type="button" class="inline-flex items-center gap-1.5 rounded-lg bg-white/10 px-3 py-2 text-xs font-semibold hover:bg-white/20" on:click={() => openChannelContent('playlist')}><ListMusic size={14} /> Playlists do canal</button>
        </div>
      {/if}
    {:else}
      <div class="mt-3 max-h-[min(44vh,24rem)] overflow-y-auto">
        {#if $playerStore.queue.length === 0}
          <p class="rounded-lg border border-dashed border-white/15 p-4 text-center text-xs text-white/55">Adicione vídeos à fila para reprodução contínua.</p>
        {:else}
          <ol class="flex flex-col gap-2" aria-label="Próximos vídeos">
            {#each $playerStore.queue.slice(0, 8) as item, index (item.queue_item_id)}
              <li class="flex items-center gap-2 rounded-lg border border-white/10 bg-white/5 p-2 {item.queue_state === 'played' ? 'opacity-55' : ''}">
                <span class="w-5 text-center text-[11px] font-bold text-white/45">{index + 1}</span>
                <img src={item.thumbnail_url} alt="" class="h-10 w-16 shrink-0 rounded object-cover" />
                <button type="button" class="min-w-0 flex-1 text-left" on:click={() => playerStore.playQueueItem(index)}><span class="block truncate text-xs font-semibold text-white">{item.title}</span><span class="block truncate text-[10px] text-white/55">{item.channel_title || 'Canal'}</span></button>
                <button type="button" class="rounded-md p-1.5 text-white/50 hover:bg-white/10 hover:text-white" title="Remover da fila" on:click={() => playerStore.removeFromQueue(index)}>×</button>
              </li>
            {/each}
          </ol>
          {#if $playerStore.queue.length > 8}<p class="mt-2 text-center text-[11px] text-white/50">+ {$playerStore.queue.length - 8} vídeos — abra a fila completa para organizar.</p>{/if}
        {/if}
      </div>
      <button type="button" class="mt-3 w-full rounded-lg bg-primary/80 px-3 py-2 text-xs font-bold text-white hover:bg-primary" on:click={() => (showQueueModal = true)}>Abrir fila completa</button>
    {/if}
  </aside>
{/if}

<AudioDSPModal
  bind:open={showDSPModal}
  isAvailable={isDSPAvailable}
  unavailableReason={dspUnavailableReason}
/>
<SleepTimerModal bind:open={showSleepModal} />
{#if $playerStore.sourceKind !== 'iptv'}
  <VideoNotesModal bind:open={showNotesModal} {videoElement} />
  <AddToPlaylistModal bind:open={showPlaylistModal} video={$playerStore.currentVideo} />
  <QueueModal bind:open={showQueueModal} />
{/if}
