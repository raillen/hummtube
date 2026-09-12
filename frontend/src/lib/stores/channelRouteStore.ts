import { writable } from 'svelte/store';

export type ChannelSection = 'videos' | 'playlists';

export interface ChannelRouteSelection {
  id: string;
  title?: string;
  thumbnailUrl?: string;
  initialSection?: ChannelSection;
}

export const activeChannelSelection = writable<ChannelRouteSelection | null>(null);

export function openChannel(selection: ChannelRouteSelection): boolean {
  const id = selection.id.trim();
  if (!id) return false;
  activeChannelSelection.set({
    ...selection,
    id,
    title: selection.title?.trim() || undefined,
    thumbnailUrl: selection.thumbnailUrl?.trim() || undefined,
    initialSection: selection.initialSection || 'videos',
  });
  return true;
}

export function closeChannel(): void {
  activeChannelSelection.set(null);
}
