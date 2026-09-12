import type { Video } from '../../lib/types';
import type { PlaybackSourceKind } from '../../lib/stores/playerStore';
import { LastFMService, PlayerService } from '../../lib/wailsjs/services';

export function savePlaybackProgress(videoID: string, _sourceKind: PlaybackSourceKind | null, positionMs: number, durationMs: number, completed: boolean): Promise<void> {
  return PlayerService.saveProgress(videoID, positionMs, durationMs, completed);
}

export function scrobbleTrack(video: Video, sourceKind: PlaybackSourceKind | null, playedMs: number, durationMs: number): Promise<void> {
  if (sourceKind !== 'youtube') return Promise.resolve();
  return LastFMService.scrobble(video.channel_title || 'YouTube', video.title, playedMs, durationMs);
}
