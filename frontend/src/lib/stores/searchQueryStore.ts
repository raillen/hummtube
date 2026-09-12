import { writable } from 'svelte/store';
import type { SearchPage, SearchRequest, Video } from '../types';

export interface SearchRouteRequest {
  requestId: number;
  query: string;
  filters: Partial<SearchRequest>;
  pending: boolean;
}

export interface RemotePlaylistSelection {
  id: string;
  title: string;
  description?: string;
  thumbnailUrl?: string;
}

export interface SearchViewSession {
  query: string;
  page: SearchPage | null;
  pageTokens: string[];
  currentPageIndex: number;
  hasSearched: boolean;
  searchError: string | null;
}

let nextSearchRequestId = 1;

// Header e outras rotas publicam aqui. SearchView observa requestId, copia os
// filtros e executa uma busca explícita; mudar apenas de aba não gera rede.
export const searchRouteRequest = writable<SearchRouteRequest>({
  requestId: 0,
  query: '',
  filters: {},
  pending: false,
});

export const remotePlaylistSelection = writable<RemotePlaylistSelection | null>(null);
export const searchViewSession = writable<SearchViewSession>({
  query: '', page: null, pageTokens: [''], currentPageIndex: 0, hasSearched: false, searchError: null,
});

export function submitGlobalSearch(query: string, filters: Partial<SearchRequest> = {}): void {
  const normalizedQuery = query.trim();
  if (!normalizedQuery) return;
  searchRouteRequest.set({ requestId: nextSearchRequestId++, query: normalizedQuery, filters, pending: true });
}

export function consumeGlobalSearch(requestId: number): void {
  searchRouteRequest.update((request) => request.requestId === requestId ? { ...request, pending: false } : request);
}

export function remotePlaylistRouteId(playlistId: string): string {
  return `remote:${encodeURIComponent(playlistId.trim())}`;
}

export function playlistIdFromRemoteRoute(routeId: string): string | null {
  if (!routeId.startsWith('remote:')) return null;
  try {
    const playlistId = decodeURIComponent(routeId.slice('remote:'.length)).trim();
    return playlistId || null;
  } catch {
    return null;
  }
}

export function selectRemotePlaylist(result: Video): string | null {
  if (result.resource_type !== 'playlist' || !result.id.trim()) return null;
  remotePlaylistSelection.set({
    id: result.id,
    title: result.title || 'Playlist do YouTube',
    description: result.description_excerpt,
    thumbnailUrl: result.thumbnail_url,
  });
  return remotePlaylistRouteId(result.id);
}

export function selectRemotePlaylistReference(selection: RemotePlaylistSelection): string | null {
  const playlistId = selection.id.trim();
  if (!playlistId) return null;
  remotePlaylistSelection.set({ ...selection, id: playlistId, title: selection.title.trim() || 'Playlist do YouTube' });
  return remotePlaylistRouteId(playlistId);
}
