import { get, writable, type Unsubscriber } from 'svelte/store';
import { activePlaylistId, activeTab, type AppTab } from './uiStores';
import { activeChannelSelection, type ChannelRouteSelection } from './channelRouteStore';

interface NavigationState {
  nanotube: true;
  index: number;
  tab: AppTab;
  playlistId: string | null;
  channel: ChannelRouteSelection | null;
}

export const canNavigateBack = writable(false);
export const canNavigateForward = writable(false);

let currentIndex = 0;
let highestIndex = 0;
let isRestoring = false;
let scheduled = false;

function snapshot(index: number): NavigationState {
  return {
    nanotube: true,
    index,
    tab: get(activeTab),
    playlistId: get(activePlaylistId),
    channel: get(activeChannelSelection),
  };
}

function updateCapabilities(): void {
  canNavigateBack.set(currentIndex > 0);
  canNavigateForward.set(currentIndex < highestIndex);
}

function scheduleHistoryEntry(): void {
  if (isRestoring || scheduled) return;
  scheduled = true;
  queueMicrotask(() => {
    scheduled = false;
    if (isRestoring) return;
    const previous = window.history.state as NavigationState | null;
    const candidate = snapshot(currentIndex);
    if (previous?.nanotube && previous.tab === candidate.tab && previous.playlistId === candidate.playlistId && JSON.stringify(previous.channel) === JSON.stringify(candidate.channel)) return;
    currentIndex += 1;
    highestIndex = currentIndex;
    window.history.pushState(snapshot(currentIndex), '', window.location.href);
    updateCapabilities();
  });
}

export function initializeNavigationHistory(): () => void {
  currentIndex = 0;
  highestIndex = 0;
  window.history.replaceState(snapshot(0), '', window.location.href);
  const subscriptions: Unsubscriber[] = [
    activeTab.subscribe(scheduleHistoryEntry),
    activePlaylistId.subscribe(scheduleHistoryEntry),
    activeChannelSelection.subscribe(scheduleHistoryEntry),
  ];
  const handlePopState = (event: PopStateEvent): void => {
    const state = event.state as NavigationState | null;
    if (!state?.nanotube) return;
    isRestoring = true;
    currentIndex = state.index;
    activeTab.set(state.tab);
    activePlaylistId.set(state.playlistId);
    activeChannelSelection.set(state.channel);
    queueMicrotask(() => { isRestoring = false; updateCapabilities(); });
  };
  window.addEventListener('popstate', handlePopState);
  updateCapabilities();
  return () => {
    subscriptions.forEach((unsubscribe) => unsubscribe());
    window.removeEventListener('popstate', handlePopState);
  };
}

export function navigateBack(): void { window.history.back(); }
export function navigateForward(): void { window.history.forward(); }
