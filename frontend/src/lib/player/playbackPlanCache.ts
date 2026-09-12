/**
 * Cache LRU com TTL para PlaybackPlan
 * Reduz o tempo de resolução ao clicar ao pré-carregar em hover/focus
 */
import type { PlaybackPlan } from '../types';

interface CacheEntry {
  plan: PlaybackPlan;
  timestamp: number;
  expiresAt: number;
}

const MAX_ENTRIES = 50;
const TTL_MS = 5 * 60 * 1000; // 5 minutos

const cache = new Map<string, CacheEntry>();

function isExpired(entry: CacheEntry): boolean {
  return Date.now() > entry.expiresAt;
}

function evictExpired(): void {
  const now = Date.now();
  for (const [key, entry] of cache.entries()) {
    if (entry.expiresAt <= now) {
      cache.delete(key);
    }
  }
}

function evictLRU(): void {
  if (cache.size <= MAX_ENTRIES) return;
  // Remove a entrada mais antiga (primeira inserida)
  const firstKey = cache.keys().next().value;
  if (firstKey) cache.delete(firstKey);
}

export function getCachedPlan(videoId: string): PlaybackPlan | null {
  evictExpired();
  const entry = cache.get(videoId);
  if (!entry) return null;
  if (isExpired(entry)) {
    cache.delete(entry.timestamp.toString()); // Limpar entrada expirada
    return null;
  }
  // Move para o final (recente)
  cache.delete(videoId);
  cache.set(videoId, entry);
  return entry.plan;
}

export function setCachedPlan(videoId: string, plan: PlaybackPlan): void {
  evictExpired();
  evictLRU();
  const now = Date.now();
  cache.set(videoId, {
    plan,
    timestamp: now,
    expiresAt: now + TTL_MS,
  });
}

export function clearCache(): void {
  cache.clear();
}

export function getCacheStats(): { size: number; maxEntries: number; ttlMs: number } {
  evictExpired();
  return {
    size: cache.size,
    maxEntries: MAX_ENTRIES,
    ttlMs: TTL_MS,
  };
}