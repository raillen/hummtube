<script lang="ts">
  import type { Video } from '../../types';
  import VideoCard from './VideoCard.svelte';
  import { displayPreferences } from '../../stores/displayPreferences';
  import type { ContentViewMode } from '../../stores/uiStores';

  export let videos: Video[] = [];
  export let title: string = '';
  export let viewMode: ContentViewMode = 'grid';

  function isVisible(video: Video): boolean {
    const title = `${video.title} ${video.description_excerpt || ''}`.toLocaleLowerCase();
    const isShort = video.duration > 0 && video.duration <= 180 * 1e9;
    const isLive = ['live', 'is_live'].includes(video.live_status || '');
    const isUpcoming = ['upcoming', 'is_upcoming'].includes(video.live_status || '');
    if ($displayPreferences.hideShorts && isShort) return false;
    if ($displayPreferences.hideLives && isLive) return false;
    if ($displayPreferences.hideUpcoming && isUpcoming) return false;
    if ($displayPreferences.hideMixes && /\bmix(?:es)?\b|mixagem/.test(title)) return false;
    if ($displayPreferences.hideMembers && /membros|members[ -]?only|só para membros/.test(title)) return false;
    return true;
  }

  $: visibleVideos = videos.filter(isVisible);
</script>

<div class="flex flex-col gap-3">
  {#if title}
    <h2 class="text-base font-bold text-foreground px-1">{title}</h2>
  {/if}

  <div data-component="video-grid-cards" class={viewMode === 'list'
    ? 'grid grid-cols-1 gap-2'
    : viewMode === 'compact'
      ? 'grid grid-cols-2 gap-2 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-6 xl:grid-cols-8'
      : $displayPreferences.thumbnailSize === 'compact'
    ? 'grid grid-cols-1 gap-3 sm:grid-cols-2 md:grid-cols-3 lg:grid-cols-5 xl:grid-cols-6'
    : $displayPreferences.thumbnailSize === 'large'
      ? 'grid grid-cols-1 gap-5 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4'
      : 'grid grid-cols-1 gap-4 sm:grid-cols-2 md:grid-cols-3 lg:grid-cols-4 xl:grid-cols-5'}>
    {#each visibleVideos as video (video.id)}
      <VideoCard {video} {viewMode} />
    {/each}
  </div>
</div>
