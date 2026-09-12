<script lang="ts">
  import { onDestroy } from 'svelte';
  import Loader2 from 'lucide-svelte/icons/loader-circle';
  import X from 'lucide-svelte/icons/x';
  import Clock from 'lucide-svelte/icons/clock';
  import Button from '../ui/Button.svelte';
  import { playerStore } from '../../stores/playerStore';
  import { translate } from '../../i18n';

  export let isVisible = false;

  $: title = $playerStore.currentVideo?.title || '';
  $: channelTitle = $playerStore.currentVideo?.channel_title || '';
  $: posterUrl = $playerStore.posterUrl || $playerStore.currentVideo?.thumbnail_url || '';
  $: loadingPhase = $playerStore.loadingPhase;
  $: bufferingProgress = $playerStore.bufferingProgress;
  $: isLoading = $playerStore.isLoading;

  // P2: Tempo decorrido desde o início do carregamento
  let loadStartTime: number = 0;
  let currentTime: number = Date.now();
  let elapsedInterval: ReturnType<typeof setInterval> | null = null;

  function startTimer(): void {
    loadStartTime = Date.now();
    currentTime = Date.now();
    if (!elapsedInterval) {
      elapsedInterval = setInterval(() => {
        currentTime = Date.now();
      }, 500);
    }
  }

  function stopTimer(): void {
    if (elapsedInterval) {
      clearInterval(elapsedInterval);
      elapsedInterval = null;
    }
    loadStartTime = 0;
  }

  onDestroy(() => {
    stopTimer();
  });

  $: if (isVisible) {
    if (!elapsedInterval) startTimer();
  } else {
    stopTimer();
  }

  $: elapsedTime = loadStartTime > 0 ? Math.floor((currentTime - loadStartTime) / 1000) : 0;
  $: elapsedTimeFormatted = formatElapsedTime(elapsedTime);

  function formatElapsedTime(seconds: number): string {
    if (seconds < 60) return `${seconds}s`;
    const mins = Math.floor(seconds / 60);
    const secs = seconds % 60;
    return `${mins}:${secs.toString().padStart(2, '0')}`;
  }

  $: phaseText = loadingPhase === 'resolving'
    ? translate('player.loading.phase.resolving')
    : loadingPhase === 'buffering'
      ? translate('player.loading.phase.buffering')
      : loadingPhase === 'ready'
        ? translate('player.loading.phase.ready')
        : translate('player.loading.phase.initializing');

  $: phaseDetail = loadingPhase === 'resolving'
    ? translate('player.loading.resolving')
    : loadingPhase === 'buffering'
      ? (bufferingProgress > 0
          ? translate('player.loading.buffering_progress', { percent: Math.round(bufferingProgress * 100) })
          : translate('player.loading.buffering'))
      : loadingPhase === 'ready'
        ? translate('player.loading.ready')
        : translate('player.loading.initializing');

  $: showCancelButton = isLoading && (loadingPhase === 'resolving' || loadingPhase === 'buffering');

  function cancelLoading(): void {
    window.dispatchEvent(new CustomEvent('nanotube-player-cancel-load'));
  }
</script>

{#if isVisible}
  <div
    class="pointer-events-none absolute inset-0 z-10 flex flex-col items-center justify-center overflow-hidden bg-black/90 p-6 text-white backdrop-blur-sm select-none transition-opacity duration-200"
    aria-busy="true"
    aria-label={translate('player.loading.aria_label')}
    data-testid="player-loading-overlay"
  >
    <!-- Background poster borrado para sensação imediata de conteúdo -->
    {#if posterUrl}
      <img
        src={posterUrl}
        alt=""
        class="pointer-events-none absolute inset-0 h-full w-full object-cover opacity-25 filter blur-xl scale-110"
        aria-hidden="true"
      />
    {/if}

    <div class="relative z-10 flex flex-col items-center justify-center gap-4 text-center max-w-md">
      <div class="relative flex items-center justify-center">
        <Loader2 size={40} class="animate-spin text-primary" />
      </div>

      <!-- P2: Skeleton com título/canal -->
      <div class="flex flex-col gap-1 min-w-0 max-w-md">
        <h3 class="line-clamp-2 text-sm font-semibold text-foreground/95 drop-shadow">
          {title}
        </h3>
        {#if channelTitle}
          <p class="truncate text-xs text-muted">
            {channelTitle}
          </p>
        {/if}
      </div>

      <!-- P2: Tempo decorrido -->
      <span class="flex items-center gap-1 text-[10px] font-mono text-muted">
        <Clock size={10} />
        <span>{elapsedTimeFormatted}</span>
      </span>

      <!-- Barra de progresso de buffer se disponível -->
      {#if loadingPhase === 'buffering' && bufferingProgress > 0}
        <div class="w-48 h-1 bg-white/20 rounded-full overflow-hidden">
          <div
            class="h-full bg-primary transition-all duration-150"
            style={`width: ${Math.round(bufferingProgress * 100)}%`}
          />
        </div>
      {/if}

      <!-- P2: Três estados de loading -->
      <div class="flex flex-col gap-1 text-center">
        <span class="text-xs font-medium tracking-wide uppercase text-primary/90">
          {phaseText}
        </span>
        <span class="text-[11px] text-muted/80">
          {phaseDetail}
        </span>
      </div>

      <!-- P2: Botão cancelar durante loading -->
      {#if showCancelButton}
        <div class="pointer-events-auto mt-2 flex items-center gap-2">
          <Button
            variant="ghost"
            size="sm"
            on:click={cancelLoading}
            class="flex items-center gap-1.5 text-xs text-muted hover:text-foreground"
            data-testid="player-loading-cancel"
          >
            <X size={12} />
            <span>{translate('player.loading.cancel')}</span>
          </Button>
        </div>
      {/if}
    </div>
  </div>
{/if}