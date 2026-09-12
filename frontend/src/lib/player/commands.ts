import { get } from 'svelte/store';
import { playerStore } from '../stores/playerStore';

export type PlayerCommand =
  | 'play_pause'
  | 'previous'
  | 'next'
  | 'seek_relative'
  | 'volume_up'
  | 'volume_down'
  | 'toggle_mute'
  | 'toggle_fullscreen'
  | 'open_queue';

export interface PlayerCommandDetail {
  command: PlayerCommand;
  value?: number;
}

function activeVideoElement(): HTMLVideoElement | null {
  return document.querySelector('[data-testid="video-player-container"] video');
}

export function executePlayerCommand(detail: PlayerCommandDetail): 'handled' | 'open_queue' | 'ignored' {
  const state = get(playerStore);

  if (detail.command === 'open_queue') return 'open_queue';
  if (!state.currentVideo) return 'ignored';

  switch (detail.command) {
    case 'play_pause':
      playerStore.togglePlay();
      break;
    case 'previous':
      playerStore.playPrevious();
      break;
    case 'next':
      void playerStore.playNext();
      break;
    case 'seek_relative': {
      const video = activeVideoElement();
      if (!video) return 'ignored';
      const delta = Number.isFinite(detail.value) ? Number(detail.value) : 0;
      const upperBound = Number.isFinite(video.duration) ? video.duration : Number.POSITIVE_INFINITY;
      video.currentTime = Math.max(0, Math.min(upperBound, video.currentTime + delta));
      break;
    }
    case 'volume_up':
      playerStore.setVolume(Math.min(1, state.volume + 0.05));
      break;
    case 'volume_down':
      playerStore.setVolume(Math.max(0, state.volume - 0.05));
      break;
    case 'toggle_mute':
      playerStore.toggleMute();
      break;
    case 'toggle_fullscreen':
      window.dispatchEvent(new CustomEvent('nanotube-toggle-player-fullscreen'));
      break;
  }

  return 'handled';
}

export function isPlayerCommandDetail(value: unknown): value is PlayerCommandDetail {
  if (!value || typeof value !== 'object') return false;
  const command = (value as { command?: unknown }).command;
  return typeof command === 'string' && [
    'play_pause', 'previous', 'next', 'seek_relative', 'volume_up',
    'volume_down', 'toggle_mute', 'toggle_fullscreen', 'open_queue',
  ].includes(command);
}
