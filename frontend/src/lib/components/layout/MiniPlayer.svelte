<script lang="ts">
  import Play from 'lucide-svelte/icons/play';
  import Pause from 'lucide-svelte/icons/pause';
  import X from 'lucide-svelte/icons/x';
  import Maximize2 from 'lucide-svelte/icons/maximize-2';
  import Move from 'lucide-svelte/icons/move';
  import Volume2 from 'lucide-svelte/icons/volume-2';
  import VolumeX from 'lucide-svelte/icons/volume-x';
  import { playerStore, type MiniplayerPosition } from '../../stores/playerStore';

  const positionLabels: Record<MiniplayerPosition, string> = {
    'bottom-right': 'inferior direito',
    'bottom-left': 'inferior esquerdo',
    'top-left': 'superior esquerdo',
    'top-right': 'superior direito'
  };
  const positionOrder: MiniplayerPosition[] = ['bottom-right', 'bottom-left', 'top-left', 'top-right'];

  $: currentPosition = $playerStore.miniplayerPosition;
  $: nextPosition = positionOrder[(positionOrder.indexOf(currentPosition) + 1) % positionOrder.length];
</script>

{#if $playerStore.currentVideo && $playerStore.isMiniplayer}
  <div
    class="absolute inset-0 flex flex-col justify-between bg-gradient-to-t from-black/80 via-transparent to-black/70 p-2.5 opacity-0 transition-opacity group-hover:opacity-100 focus-within:opacity-100 pointer-events-none"
    data-testid="miniplayer-controls"
  >
    <div class="flex items-start justify-between gap-2">
      <div class="min-w-0 text-left text-white drop-shadow">
        <h4 class="truncate text-xs font-semibold">{$playerStore.currentVideo.title}</h4>
        <p class="truncate text-[10px] text-white/70">{$playerStore.currentVideo.channel_title || 'Canal'}</p>
      </div>
      <div class="flex shrink-0 items-center gap-1 pointer-events-auto">
        <button
          type="button"
          on:click={() => playerStore.moveMiniplayer('next')}
          class="rounded-lg bg-black/55 p-1.5 text-white hover:bg-black/75 focus:outline-none focus:ring-2 focus:ring-white/70 tv-focusable"
          aria-label={`Mover miniplayer para o canto ${positionLabels[nextPosition]} (atual: ${positionLabels[currentPosition]})`}
          title={`Mover miniplayer (${positionLabels[currentPosition]})`}
          data-testid="miniplayer-move"
        >
          <Move size={16} />
        </button>
        <button
          type="button"
          on:click={() => playerStore.setMiniplayer(false)}
          class="rounded-lg bg-black/55 p-1.5 text-white hover:bg-black/75 focus:outline-none focus:ring-2 focus:ring-white/70"
          aria-label="Expandir player"
        >
          <Maximize2 size={16} />
        </button>
        <button
          type="button"
          on:click={playerStore.closePlayer}
          class="rounded-lg bg-black/55 p-1.5 text-white hover:bg-red-600 focus:outline-none focus:ring-2 focus:ring-white/70"
          aria-label="Fechar player"
        >
          <X size={16} />
        </button>
      </div>
    </div>

    <div class="flex items-center justify-between pointer-events-auto">
      <button
        type="button"
        on:click={playerStore.togglePlay}
        class="rounded-full bg-black/60 p-2 text-white hover:bg-black/80 focus:outline-none focus:ring-2 focus:ring-white/70"
        aria-label={$playerStore.isPaused ? 'Reproduzir' : 'Pausar'}
      >
        {#if $playerStore.isPaused}
          <Play size={17} fill="currentColor" />
        {:else}
          <Pause size={17} fill="currentColor" />
        {/if}
      </button>

      <button
        type="button"
        on:click={playerStore.toggleMute}
        class="rounded-full bg-black/60 p-2 text-white hover:bg-black/80 focus:outline-none focus:ring-2 focus:ring-white/70"
        aria-label={$playerStore.isMuted ? 'Ativar áudio' : 'Silenciar'}
      >
        {#if $playerStore.isMuted}
          <VolumeX size={16} />
        {:else}
          <Volume2 size={16} />
        {/if}
      </button>
    </div>
  </div>
{/if}
