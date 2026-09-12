/// <reference types="vite/client" />

declare module '$product-app' {
  import type { ComponentType } from 'svelte';
  const component: ComponentType;
  export default component;
}

declare module '$product-player-hooks' {
  import type { Video } from './lib/types';
  import type { PlaybackSourceKind } from './lib/stores/playerStore';
  export function savePlaybackProgress(videoID: string, sourceKind: PlaybackSourceKind | null, positionMs: number, durationMs: number, completed: boolean): Promise<void>;
  export function scrobbleTrack(video: Video, sourceKind: PlaybackSourceKind | null, playedMs: number, durationMs: number): Promise<void>;
}
