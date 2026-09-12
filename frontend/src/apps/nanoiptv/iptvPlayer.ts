import type { IPTVItem, Video } from '../../lib/types';
import { playerStore } from '../../lib/stores/playerStore';
import { IPTVService } from '../../lib/wailsjs/services';

export function playIPTVItem(item: IPTVItem, resumeAtSeconds = 0): void {
  const video: Video = {
    id: item.id,
    channel_id: item.source_id,
    channel_title: item.group || item.source_name || 'NanoIPTV',
    title: item.title,
    description_excerpt: item.classification ? `Classificação: ${item.classification}` : undefined,
    published_at: new Date().toISOString(),
    duration: 0,
    thumbnail_url: item.logo_url || '',
    external_url: item.stream_url || '',
  };
  void playerStore.loadResolvedVideo(
    video,
    'iptv',
    () => IPTVService.resolveIPTVStream(item.source_id, item.id),
    resumeAtSeconds,
  );
}
