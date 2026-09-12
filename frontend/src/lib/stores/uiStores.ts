import { writable } from 'svelte/store';
import { SettingsService } from '../wailsjs/services';

export type AppTab = 'home' | 'subscriptions' | 'channels' | 'search' | 'playlists' | 'queue' | 'music' | 'library' | 'iptv' | 'settings';

export const activeTab = writable<AppTab>('home');
export const activePlaylistId = writable<string | null>(null);

export const theme = writable<'dark' | 'light'>('dark');
export const isTvMode = writable<boolean>(false);
export const sidebarCollapsed = writable<boolean>(false);

const TV_MODE_STORAGE_KEY = 'nanotube_tv_mode';

export function applyTvMode(enabled: boolean): void {
  isTvMode.set(enabled);
  if (typeof document !== 'undefined') {
    document.documentElement.classList.toggle('tv-mode', enabled);
    try {
      localStorage.setItem(TV_MODE_STORAGE_KEY, enabled ? '1' : '0');
    } catch {}
  }
}

export function setTvMode(enabled: boolean): void {
  applyTvMode(enabled);
  SettingsService.saveSetting('ui.tv_mode', enabled ? '1' : '0').catch(() => {});
}

export function resolveInitialTvMode(settings: Record<string, string> | null | undefined): boolean {
  if (typeof window !== 'undefined') {
    const param = new URLSearchParams(window.location.search).get('tv');
    if (param === '1') return true;
    if (param === '0') return false;
  }
  const stored = settings?.['ui.tv_mode'];
  if (stored === '1') return true;
  if (stored === '0') return false;
  if (typeof localStorage !== 'undefined') {
    try {
      return localStorage.getItem(TV_MODE_STORAGE_KEY) === '1';
    } catch {}
  }
  return false;
}

export type ContentViewMode = 'grid' | 'list' | 'compact';

export interface AccentOption {
  id: string;
  name: string;
  primary: string;
  primaryHover: string;
  rgb: string;
  twClass: string;
}

export const ACCENT_PRESETS: AccentOption[] = [
  { id: 'red', name: 'Vermelho YouTube', primary: '#ef4444', primaryHover: '#dc2626', rgb: '239, 68, 68', twClass: 'bg-red-500' },
  { id: 'sky', name: 'Ciano / Sky', primary: '#0ea5e9', primaryHover: '#0284c7', rgb: '14, 165, 233', twClass: 'bg-sky-500' },
  { id: 'emerald', name: 'Verde Esmeralda', primary: '#10b981', primaryHover: '#059669', rgb: '16, 185, 129', twClass: 'bg-emerald-500' },
  { id: 'purple', name: 'Roxo / Violeta', primary: '#8b5cf6', primaryHover: '#7c3aed', rgb: '139, 92, 246', twClass: 'bg-purple-500' },
  { id: 'amber', name: 'Laranja / Âmbar', primary: '#f59e0b', primaryHover: '#d97706', rgb: '245, 158, 11', twClass: 'bg-amber-500' },
  { id: 'rose', name: 'Rosa / Rose', primary: '#f43f5e', primaryHover: '#e11d48', rgb: '244, 63, 94', twClass: 'bg-rose-500' },
  { id: 'blue', name: 'Azul Clássico', primary: '#3b82f6', primaryHover: '#2563eb', rgb: '59, 130, 246', twClass: 'bg-blue-500' },
];

export const accentColor = writable<string>('red');

export function applyAccentColor(accentId: string) {
  const preset = ACCENT_PRESETS.find(p => p.id === accentId) || ACCENT_PRESETS[0];
  accentColor.set(preset.id);
  if (typeof document !== 'undefined') {
    document.documentElement.style.setProperty('--color-primary', preset.primary);
    document.documentElement.style.setProperty('--color-primary-hover', preset.primaryHover);
    document.documentElement.style.setProperty('--color-primary-rgb', preset.rgb);
    try {
      localStorage.setItem('nanotube_accent_color', preset.id);
      SettingsService.saveSetting('accent_color', preset.id).catch(() => {});
    } catch {}
  }
}

export interface ToastMessage {
  id: string;
  type: 'info' | 'success' | 'warning' | 'error';
  message: string;
}

function createToastStore() {
  const { subscribe, update } = writable<ToastMessage[]>([]);

  return {
    subscribe,
    add: (message: string, type: ToastMessage['type'] = 'info') => {
      const id = Math.random().toString(36).substring(2, 9);
      update((toasts) => [...toasts, { id, type, message }]);
      setTimeout(() => {
        update((toasts) => toasts.filter((t) => t.id !== id));
      }, 4000);
    },
    remove: (id: string) => update((toasts) => toasts.filter((t) => t.id !== id)),
  };
}

export const toast = createToastStore();
