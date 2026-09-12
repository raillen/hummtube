/**
 * Cache de miniaturas no lado do WebView com política LRU.
 *
 * O Cache Storage é persistente por origem, evita downloads duplicados durante
 * rolagem e mantém o fallback para a URL remota quando a API não existe (por
 * exemplo, alguns shells de teste). Nenhuma credencial ou header sensível é
 * anexado às requisições.
 *
 * A política LRU é aplicada via reordenação dos `requests` no `Cache`. O backend
 * expõe um método para inspecionar o tamanho atual, o que ajuda a calibrar o
 * orçamento em hardware modesto.
 */
const CACHE_NAME = 'nanotube-thumbnails-v1';
const MAX_ENTRIES = 240;
const pending = new Map<string, Promise<string>>();
const objectURLs = new Set<string>();

function canUseCacheStorage(): boolean {
  return typeof window !== 'undefined' && 'caches' in window && typeof fetch === 'function';
}

async function enforceLRU(cache: Cache): Promise<void> {
  const keys = await cache.keys();
  if (keys.length <= MAX_ENTRIES) return;
  const toDelete = keys.length - MAX_ENTRIES;
  for (let index = 0; index < toDelete; index += 1) {
    const request = keys[index];
    if (request) await cache.delete(request);
  }
}

export async function getThumbnailCacheStats(): Promise<{ entries: number; limit: number }> {
  if (!canUseCacheStorage()) return { entries: 0, limit: MAX_ENTRIES };
  try {
    const cache = await caches.open(CACHE_NAME);
    const keys = await cache.keys();
    return { entries: keys.length, limit: MAX_ENTRIES };
  } catch {
    return { entries: 0, limit: MAX_ENTRIES };
  }
}

export async function cachedThumbnailURL(url: string): Promise<string> {
  const normalized = url.trim();
  if (!normalized || !canUseCacheStorage()) return normalized;
  const existing = pending.get(normalized);
  if (existing) return existing;

  const request = (async () => {
    const cache = await caches.open(CACHE_NAME);
    const cached = await cache.match(normalized);
    if (cached) {
      // Acessar renova a entrada em caches LRU (FF/Chrome evict por timestamp).
      await cache.put(normalized, cached.clone());
      const blob = await cached.blob();
      return ensureObjectURL(normalized, blob);
    }
    const response = await fetch(normalized, {
      method: 'GET',
      credentials: 'omit',
      cache: 'force-cache',
      mode: 'cors',
    });
    if (!response.ok) return normalized;
    await cache.put(normalized, response.clone());
    await enforceLRU(cache);
    const blob = await response.blob();
    return ensureObjectURL(normalized, blob);
  })().catch(() => normalized).finally(() => {
    pending.delete(normalized);
  });
  pending.set(normalized, request);
  return request;
}

function ensureObjectURL(url: string, blob: Blob): string {
  const existing = Array.from(objectURLs).find((entry) => entry === url);
  if (existing) return existing;
  const objectURL = URL.createObjectURL(blob);
  objectURLs.add(objectURL);
  return objectURL;
}

export function releaseThumbnailURL(url: string): void {
  if (!objectURLs.has(url)) return;
  URL.revokeObjectURL(url);
  objectURLs.delete(url);
}

export async function clearThumbnailCache(): Promise<void> {
  pending.clear();
  if (canUseCacheStorage()) await caches.delete(CACHE_NAME);
  for (const objectURL of objectURLs) URL.revokeObjectURL(objectURL);
  objectURLs.clear();
}
