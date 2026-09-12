import type { Video } from '../../lib/types';
import type { PlaybackSourceKind } from '../../lib/stores/playerStore';
import { PlayerService } from '../../lib/wailsjs/services';

export function savePlaybackProgress(videoID: string, _sourceKind: PlaybackSourceKind | null, positionMs: number, durationMs: number, completed: boolean): Promise<void> {
  return PlayerService.saveProgress(videoID, positionMs, durationMs, completed);
}

export function scrobbleTrack(_video: Video, _sourceKind: PlaybackSourceKind | null, _playedMs: number, _durationMs: number): Promise<void> {
  return Promise.resolve();
}
