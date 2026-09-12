import { writable, get, derived } from 'svelte/store';
import type { Video, PlaybackPlan, QueueSnapshot, QueuePreferences, Playlist } from '../types';
import { CatalogService, PlayerService, QueueService } from '../wailsjs/services';
import { getCachedPlan, setCachedPlan } from '../player/playbackPlanCache';

export type PlaybackSourceKind = 'youtube' | 'iptv' | 'direct';

export type LoadingPhase = 'idle' | 'resolving' | 'buffering' | 'ready';

export interface QueuedVideo extends Video {
  queue_item_id: string;
  queue_state: 'pending' | 'played';
}

export type MiniplayerPosition = 'bottom-right' | 'bottom-left' | 'top-left' | 'top-right';

export interface PlayerState {
  currentVideo: Video | null;
  playbackPlan: PlaybackPlan | null;
  sourceKind: PlaybackSourceKind | null;
  resumeAt: number;
  queue: QueuedVideo[];
  currentQueueItemId: string | null;
  historyQueue: Video[];
  autoplay: boolean;
  removePlayed: boolean;
  isPlaying: boolean;
  isPaused: boolean;
  currentTime: number;
  duration: number;
  volume: number; // 0 to 1
  gain: number;   // -12 to 12 dB
  speed: number;  // 0.25 to 2.0
  isMuted: boolean;
  isLoading: boolean;
  isFullscreen: boolean;
  isMiniplayer: boolean;
  miniplayerPosition: MiniplayerPosition;
  loadingPhase: LoadingPhase;
  bufferingProgress: number; // 0 to 1
  posterUrl: string;
  error: string | null;
}

const QUEUE_STORAGE_KEY = 'nanotube_player_queue';
const MAX_QUEUE_ITEMS = 100;
const MINIPLAYER_POSITION_STORAGE_KEY = 'nanotube_miniplayer_position';
const MINIPLAYER_POSITIONS: MiniplayerPosition[] = ['bottom-right', 'bottom-left', 'top-left', 'top-right'];

function readStoredMiniplayerPosition(): MiniplayerPosition {
  if (typeof window === 'undefined') return 'bottom-right';
  try {
    const stored = window.localStorage.getItem(MINIPLAYER_POSITION_STORAGE_KEY);
    return MINIPLAYER_POSITIONS.includes(stored as MiniplayerPosition)
      ? (stored as MiniplayerPosition)
      : 'bottom-right';
  } catch {
    return 'bottom-right';
  }
}

const initialState: PlayerState = {
  currentVideo: null,
  playbackPlan: null,
  sourceKind: null,
  resumeAt: 0,
  queue: [],
  currentQueueItemId: null,
  historyQueue: [],
  autoplay: true,
  removePlayed: false,
  isPlaying: false,
  isPaused: false,
  currentTime: 0,
  duration: 0,
  volume: 1,
  gain: 0,
  speed: 1,
  isMuted: false,
  isLoading: false,
  isFullscreen: false,
  isMiniplayer: false,
  miniplayerPosition: readStoredMiniplayerPosition(),
  loadingPhase: 'idle' as LoadingPhase,
  bufferingProgress: 0,
  posterUrl: '',
  error: null,
};

function readStoredQueue(): Video[] {
  if (typeof window === 'undefined') return [];
  try {
    const parsed: unknown = JSON.parse(window.localStorage.getItem(QUEUE_STORAGE_KEY) || '[]');
    if (!Array.isArray(parsed)) return [];
    return parsed
      .filter((item): item is Video => Boolean(item && typeof item === 'object' && typeof item.id === 'string' && typeof item.title === 'string'))
      .slice(0, MAX_QUEUE_ITEMS);
  } catch {
    return [];
  }
}

function projectQueue(snapshot: QueueSnapshot): QueuedVideo[] {
  return snapshot.items.map((item) => ({
    ...item.video,
    queue_item_id: item.id,
    queue_state: item.state,
  }));
}

function createPlayerStore() {
  const store = writable<PlayerState>({ ...initialState });
  const { subscribe, set, update } = store;
  let loadGeneration = 0;

  const actions = {
    subscribe,
    initQueue: async () => {
      try {
        let snapshot = await QueueService.getQueue();
        const legacyQueue = readStoredQueue();
        if (snapshot.items.length === 0 && legacyQueue.length > 0) {
          for (const video of legacyQueue.slice(0, MAX_QUEUE_ITEMS)) {
            await QueueService.enqueueVideo(video);
          }
          snapshot = await QueueService.getQueue();
        }
        window.localStorage.removeItem(QUEUE_STORAGE_KEY);
        update((state) => ({
          ...state,
          queue: projectQueue(snapshot),
          autoplay: snapshot.preferences.autoplay,
          removePlayed: snapshot.preferences.remove_played,
        }));
      } catch (error: unknown) {
        update((state) => ({ ...state, error: error instanceof Error ? error.message : 'Não foi possível carregar a fila.' }));
      }
    },
    refreshQueue: async () => {
      const snapshot = await QueueService.getQueue();
      update((state) => ({
        ...state,
        queue: projectQueue(snapshot),
        autoplay: snapshot.preferences.autoplay,
        removePlayed: snapshot.preferences.remove_played,
      }));
      return snapshot;
    },
    loadVideo: async (video: Video, queueItemId: string | null = null) => {
      const generation = ++loadGeneration;
      const state = get(store);
      const newHistory = state.currentVideo ? [...state.historyQueue, state.currentVideo] : state.historyQueue;
      // P0-1: pausa o player anterior imediatamente, antes do resolveMedia
      try { window.dispatchEvent(new CustomEvent('nanotube-player-abort')); } catch {}
      update((s) => ({
        ...s,
        currentVideo: video,
        currentQueueItemId: queueItemId,
        playbackPlan: null,
        sourceKind: 'youtube',
        resumeAt: 0,
        historyQueue: newHistory,
        isLoading: true,
        loadingPhase: 'resolving' as LoadingPhase,
        bufferingProgress: 0,
        // P0-2: poster imediato do novo vídeo, antes de tocar
        posterUrl: video.thumbnail_url || '',
        error: null,
        isPlaying: true,
        isPaused: false
      }));
      // P0-3: evento para a UI saber que começou a resolver
      try { window.dispatchEvent(new CustomEvent('nanotube-player-loading', { detail: { videoId: video.id, title: video.title, channelTitle: video.channel_title, posterUrl: video.thumbnail_url } })); } catch {}
      try {
        // P1-3: tenta cache antes de chamar o backend
        const cached = getCachedPlan(video.id);
        let plan: PlaybackPlan;
        if (cached) {
          plan = cached;
          void CatalogService.rememberVideo(video).catch(() => undefined);
        } else {
          const [resolved] = await Promise.all([
            PlayerService.resolveMedia(video.id, video.external_url),
            CatalogService.rememberVideo(video).catch(() => undefined),
          ]);
          plan = resolved;
          setCachedPlan(video.id, plan);
        }
        if (generation !== loadGeneration) return;
        update((s) => ({ ...s, playbackPlan: plan, isLoading: false, loadingPhase: 'buffering' as LoadingPhase }));
      } catch (error: unknown) {
        if (generation !== loadGeneration) return;
        const message = error instanceof Error ? error.message : 'Falha ao resolver vídeo';
        update((s) => ({ ...s, isLoading: false, isPlaying: false, loadingPhase: 'idle' as LoadingPhase, error: message }));
      }
    },
    loadResolvedVideo: async (
      video: Video,
      sourceKind: Exclude<PlaybackSourceKind, 'youtube'>,
      resolvePlan: () => Promise<PlaybackPlan>,
      resumeAt = 0,
    ) => {
      const generation = ++loadGeneration;
      const state = get(store);
      const newHistory = state.currentVideo ? [...state.historyQueue, state.currentVideo] : state.historyQueue;
      try { window.dispatchEvent(new CustomEvent('nanotube-player-abort')); } catch {}
      update((s) => ({
        ...s,
        currentVideo: video,
        currentQueueItemId: null,
        playbackPlan: null,
        sourceKind,
        resumeAt: Math.max(0, resumeAt),
        historyQueue: newHistory,
        isLoading: true,
        loadingPhase: 'resolving' as LoadingPhase,
        bufferingProgress: 0,
        posterUrl: video.thumbnail_url || '',
        error: null,
        isPlaying: true,
        isPaused: false
      }));
      try { window.dispatchEvent(new CustomEvent('nanotube-player-loading', { detail: { videoId: video.id, title: video.title, channelTitle: video.channel_title, posterUrl: video.thumbnail_url } })); } catch {}
      try {
        const plan = await resolvePlan();
        if (generation !== loadGeneration) return;
        update((s) => ({ ...s, playbackPlan: plan, isLoading: false, loadingPhase: 'buffering' as LoadingPhase }));
      } catch (error: unknown) {
        if (generation !== loadGeneration) return;
        const message = error instanceof Error ? error.message : 'Falha ao resolver a mídia';
        update((s) => ({ ...s, isLoading: false, isPlaying: false, loadingPhase: 'idle' as LoadingPhase, error: message }));
      }
    },
    loadDirectStream: (title: string, streamUrl: string, logoUrl = '') => {
      loadGeneration++;
      const syntheticVideo: Video = {
        id: 'direct_' + Date.now(),
        channel_id: 'direct',
        channel_title: 'Stream Direto',
        title: title,
        published_at: new Date().toISOString(),
        duration: 0,
        thumbnail_url: logoUrl,
        external_url: streamUrl,
      };
      const plan: PlaybackPlan = {
        mode: 'resolved-media',
        primary: { url: streamUrl },
        metadata: { title, video_id: syntheticVideo.id }
      };
      update((s) => ({
        ...s,
        currentVideo: syntheticVideo,
        currentQueueItemId: null,
        playbackPlan: plan,
        sourceKind: 'direct',
        resumeAt: 0,
        isLoading: false,
        error: null,
        isPlaying: true,
        isPaused: false
      }));
    },
    addToQueue: async (video: Video) => {
      if (!video?.id || !video.title) return;
      const state = get(store);
      if (state.queue.some((queued) => queued.id === video.id) || state.queue.length >= MAX_QUEUE_ITEMS) return;
      try {
        await QueueService.enqueueVideo(video);
        await actions.refreshQueue();
      } catch (error: unknown) {
        actions.setError(error instanceof Error ? error.message : 'Não foi possível adicionar à fila.');
      }
    },
    removeFromQueue: async (index: number) => {
      const queuedVideo = get(store).queue[index];
      if (!queuedVideo) return;
      await QueueService.removeQueueItem(queuedVideo.queue_item_id);
      await actions.refreshQueue();
    },
    clearQueue: async (scope: 'all' | 'played' = 'all') => {
      await QueueService.clearQueue(scope);
      await actions.refreshQueue();
    },
    playQueueItem: (index: number) => {
      const queuedVideo = get(store).queue[index];
      if (queuedVideo) void actions.loadVideo(queuedVideo, queuedVideo.queue_item_id);
    },
    playNext: async () => {
      const before = get(store);
      const previousIndex = before.currentQueueItemId
        ? before.queue.findIndex((item) => item.queue_item_id === before.currentQueueItemId)
        : -1;
      if (before.currentQueueItemId) {
        await QueueService.markQueueItemPlayed(before.currentQueueItemId, true);
      }
      const snapshot = await actions.refreshQueue();
      const retainedIndex = before.currentQueueItemId
        ? snapshot.items.findIndex((item) => item.id === before.currentQueueItemId)
        : -1;
      const startIndex = retainedIndex >= 0 ? retainedIndex + 1 : Math.max(0, previousIndex);
      const nextItem = snapshot.items.slice(startIndex).find((item) => item.state === 'pending')
        ?? snapshot.items.slice(0, startIndex).find((item) => item.state === 'pending');
      if (!nextItem) {
        actions.pause();
        return;
      }
      await actions.loadVideo(nextItem.video, nextItem.id);
    },
    playPrevious: () => {
      const state = get(store);
      if (state.historyQueue.length === 0) return;
      const prevVideo = state.historyQueue[state.historyQueue.length - 1];
      const newHistory = state.historyQueue.slice(0, -1);
      update((s) => ({ ...s, historyQueue: newHistory }));
      actions.loadVideo(prevVideo);
    },
    reorderQueue: async (fromIndex: number, toIndex: number) => {
      const queue = [...get(store).queue];
      if (fromIndex < 0 || toIndex < 0 || fromIndex >= queue.length || toIndex >= queue.length || fromIndex === toIndex) return;
      const [moved] = queue.splice(fromIndex, 1);
      queue.splice(toIndex, 0, moved);
      update((state) => ({ ...state, queue }));
      try {
        await QueueService.reorderQueue(queue.map((video) => video.queue_item_id));
      } catch (error: unknown) {
        await actions.refreshQueue();
        actions.setError(error instanceof Error ? error.message : 'Não foi possível reordenar a fila.');
      }
    },
    toggleAutoplay: async () => {
      const state = get(store);
      const preferences: QueuePreferences = { autoplay: !state.autoplay, remove_played: state.removePlayed };
      await QueueService.savePreferences(preferences);
      update((current) => ({ ...current, autoplay: preferences.autoplay }));
    },
    setRemovePlayed: async (removePlayed: boolean) => {
      const state = get(store);
      await QueueService.savePreferences({ autoplay: state.autoplay, remove_played: removePlayed });
      update((current) => ({ ...current, removePlayed }));
    },
    saveQueueAsPlaylist: async (name: string): Promise<Playlist> => QueueService.saveAsPlaylist(name),
    resetPlayedItems: async () => {
      const playedItems = get(store).queue.filter((video) => video.queue_state === 'played');
      await Promise.all(playedItems.map((video) => QueueService.markQueueItemPlayed(video.queue_item_id, false)));
      await actions.refreshQueue();
    },
    play: () => update((s) => ({ ...s, isPlaying: true, isPaused: false })),
    pause: () => update((s) => ({ ...s, isPaused: true })),
    togglePlay: () => update((s) => ({ ...s, isPaused: !s.isPaused })),
    setTime: (time: number) => update((s) => (Math.abs(s.currentTime - time) < 0.05 ? s : { ...s, currentTime: time })),
    setDuration: (dur: number) => update((s) => (s.duration === dur ? s : { ...s, duration: dur })),
    setVolume: (vol: number) => update((s) => ({ ...s, volume: Math.max(0, Math.min(1, vol)) })),
    setGain: (gain: number) => update((s) => ({ ...s, gain: Math.max(-12, Math.min(12, gain)) })),
    setSpeed: (speed: number) => update((s) => ({ ...s, speed: Math.max(0.25, Math.min(2, speed)) })),
    toggleMute: () => update((s) => ({ ...s, isMuted: !s.isMuted })),
    toggleFullscreen: () => update((s) => ({ ...s, isFullscreen: !s.isFullscreen })),
    setMiniplayer: (mini: boolean) => update((s) => ({ ...s, isMiniplayer: mini })),
    setMiniplayerPosition: (position: MiniplayerPosition) => {
      if (typeof window !== 'undefined') {
        try {
          window.localStorage.setItem(MINIPLAYER_POSITION_STORAGE_KEY, position);
        } catch {
          // Persistência é conveniência; posição em memória continua válida.
        }
      }
      update((s) => ({ ...s, miniplayerPosition: position }));
    },
    moveMiniplayer: (direction: 'previous' | 'next') => {
      const current = get(store).miniplayerPosition;
      const index = MINIPLAYER_POSITIONS.indexOf(current);
      const next = MINIPLAYER_POSITIONS[(index + (direction === 'next' ? 1 : MINIPLAYER_POSITIONS.length - 1)) % MINIPLAYER_POSITIONS.length];
      actions.setMiniplayerPosition(next);
    },
    setError: (message: string | null) => update((s) => ({ ...s, error: message, isLoading: false, loadingPhase: message ? 'idle' : s.loadingPhase })),
    setLoadingPhase: (phase: LoadingPhase) => update((s) => ({ ...s, loadingPhase: phase })),
    setBufferingProgress: (progress: number) => update((s) => ({ ...s, bufferingProgress: Math.max(0, Math.min(1, progress)) })),
    markReady: () => update((s) => ({ ...s, loadingPhase: 'ready' as LoadingPhase, bufferingProgress: 1, isLoading: false })),
    closePlayer: () => {
      loadGeneration++;
      // Fechar o player não deve destruir uma fila montada pelo usuário.
      const state = get(store);
      try { window.dispatchEvent(new CustomEvent('nanotube-player-abort')); } catch {}
      set({ ...initialState, queue: state.queue, autoplay: state.autoplay, removePlayed: state.removePlayed, miniplayerPosition: state.miniplayerPosition });
      // Libera imediatamente os tokens de mídia e downloads em curso.
      PlayerService.revokePlayerSession().catch(() => undefined);
    },
    prefetchVideo: (video: Video, externalUrl?: string) => {
      const videoId = video.id;
      if (getCachedPlan(videoId)) return;
      if (get(store).isLoading) return;
      void (async () => {
        try {
          const plan = await PlayerService.resolveMedia(videoId, externalUrl || video.external_url);
          setCachedPlan(videoId, plan);
        } catch {
          // Silencia falhas de prefetch - serão tratadas no loadVideo
        }
      })();
    },
  };

  return { ...actions, subscribe };
}

export const playerStore = createPlayerStore();
export const playerQueue = derived(playerStore, ($s) => $s.queue);
export const isMiniplayer = derived(playerStore, ($s) => $s.isMiniplayer);
export const miniplayerPosition = derived(playerStore, ($s) => $s.miniplayerPosition);
export const hasActiveVideo = derived(playerStore, ($s) => Boolean($s.currentVideo));