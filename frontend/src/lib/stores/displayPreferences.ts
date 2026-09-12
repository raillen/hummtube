import { writable } from 'svelte/store';

export type ThumbnailSize = 'compact' | 'comfortable' | 'large';
export type ThumbnailQuality = 'data_saver' | 'balanced' | 'high';

export interface DisplayPreferences {
  thumbnailSize: ThumbnailSize;
  thumbnailQuality: ThumbnailQuality;
  homePageSize: number;
  subscriptionsPageSize: number;
  hideShorts: boolean;
  hideLives: boolean;
  hideUpcoming: boolean;
  hideMixes: boolean;
  hideMembers: boolean;
}

export const displayPreferences = writable<DisplayPreferences>({
  thumbnailSize: 'comfortable',
  thumbnailQuality: 'balanced',
  homePageSize: 24,
  subscriptionsPageSize: 24,
  hideShorts: false,
  hideLives: false,
  hideUpcoming: false,
  hideMixes: false,
  hideMembers: false,
});

function allowedNumber(raw: string | undefined, allowed: number[], fallback: number): number {
  const parsed = Number.parseInt(raw || '', 10);
  return allowed.includes(parsed) ? parsed : fallback;
}

export function applyDisplaySettings(settings: Record<string, string>): void {
  const thumbnailSize: ThumbnailSize = ['compact', 'comfortable', 'large'].includes(settings.thumbnail_size)
    ? settings.thumbnail_size as ThumbnailSize
    : 'comfortable';
  const thumbnailQuality: ThumbnailQuality = ['data_saver', 'balanced', 'high'].includes(settings.thumbnail_quality)
    ? settings.thumbnail_quality as ThumbnailQuality
    : 'balanced';
  displayPreferences.set({
    thumbnailSize,
    thumbnailQuality,
    homePageSize: allowedNumber(settings.home_page_size, [12, 24, 36, 48], 24),
    subscriptionsPageSize: allowedNumber(settings.subscriptions_page_size, [12, 24, 36, 48], 24),
    hideShorts: settings.hide_shorts === '1',
    hideLives: settings.hide_lives === '1',
    hideUpcoming: settings.hide_upcoming === '1',
    hideMixes: settings.hide_mixes === '1',
    hideMembers: settings.hide_members === '1',
  });
}
