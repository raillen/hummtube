<script lang="ts">
  import { isMiniplayer, miniplayerPosition, hasActiveVideo } from '../../stores/playerStore';
  import VideoPlayer from './VideoPlayer.svelte';

  const miniplayerPositionClasses: Record<string, string> = {
    'bottom-right': 'bottom-4 right-4',
    'bottom-left': 'bottom-4 left-4',
    'top-left': 'left-4 top-4',
    'top-right': 'right-4 top-4'
  };
</script>

{#if $hasActiveVideo}
  <section
    class={$isMiniplayer
      ? `fixed z-50 aspect-video w-[min(24rem,calc(100vw-2rem))] overflow-hidden rounded-xl border border-white/15 bg-black shadow-2xl isolate transition-all duration-200 ${miniplayerPositionClasses[$miniplayerPosition] || miniplayerPositionClasses['bottom-right']}`
      : 'absolute inset-0 z-40 flex flex-col bg-black'}
    aria-label="Player de mídia"
    data-player-mode={$isMiniplayer ? 'mini' : 'expanded'}
    data-miniplayer-position={$isMiniplayer ? $miniplayerPosition : undefined}
  >
    <VideoPlayer />
  </section>
{/if}
