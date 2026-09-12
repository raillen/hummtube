import { writable, get } from 'svelte/store';
import { SettingsService } from '../wailsjs/services';

export interface ShortcutItem {
  id: string;
  label: string;
  description: string;
  category: 'player' | 'navigation' | 'system';
  defaultKey: string;
  currentKey: string;
}

export const DEFAULT_SHORTCUTS: ShortcutItem[] = [
  { id: 'play_pause', label: 'Reproduzir / Pausar', description: 'Alterna entre reprodução e pausa do vídeo ativo', category: 'player', defaultKey: 'Space', currentKey: 'Space' },
  { id: 'seek_forward', label: 'Avançar 10s', description: 'Avança 10 segundos na linha do tempo', category: 'player', defaultKey: 'ArrowRight', currentKey: 'ArrowRight' },
  { id: 'seek_backward', label: 'Retroceder 10s', description: 'Retrocede 10 segundos na linha do tempo', category: 'player', defaultKey: 'ArrowLeft', currentKey: 'ArrowLeft' },
  { id: 'toggle_fullscreen', label: 'Tela Cheia', description: 'Entra ou sai do modo tela cheia', category: 'player', defaultKey: 'f', currentKey: 'f' },
  { id: 'toggle_mute', label: 'Mudo / Desmudo', description: 'Silencia ou restaura o áudio do player', category: 'player', defaultKey: 'm', currentKey: 'm' },
  { id: 'volume_up', label: 'Aumentar Volume', description: 'Aumenta o volume do player em 5%', category: 'player', defaultKey: 'ArrowUp', currentKey: 'ArrowUp' },
  { id: 'volume_down', label: 'Diminuir Volume', description: 'Diminui o volume do player em 5%', category: 'player', defaultKey: 'ArrowDown', currentKey: 'ArrowDown' },
  { id: 'focus_search', label: 'Focar Busca', description: 'Direciona o foco para a barra de pesquisa rápida', category: 'navigation', defaultKey: 'Control+k', currentKey: 'Control+k' },
  { id: 'toggle_tv_mode', label: 'Alternar Modo TV', description: 'Ativa ou desativa a interface 10-Foot UI para TV', category: 'navigation', defaultKey: 'F10', currentKey: 'F10' },
  { id: 'refresh_subscriptions', label: 'Atualizar Inscrições', description: 'Sincroniza novos vídeos de canais inscritos', category: 'system', defaultKey: 'Control+r', currentKey: 'Control+r' },
  { id: 'open_devtools', label: 'Abrir DevTools', description: 'Abre o console e ferramentas de desenvolvimento', category: 'system', defaultKey: 'F12', currentKey: 'F12' },
];

function createShortcutsStore() {
  const { subscribe, set, update } = writable<ShortcutItem[]>(DEFAULT_SHORTCUTS);

  return {
    subscribe,
    set,
    init: () => {
      if (typeof window === 'undefined') return;
      try {
        const saved = localStorage.getItem('nanotube_shortcuts');
        if (saved) {
          const parsed = JSON.parse(saved) as Record<string, string>;
          update((list) =>
            list.map((item) => ({
              ...item,
              // Control+F era o atalho inicial. Migre apenas esse valor
              // padrão para não perpetuar a configuração antiga.
              currentKey: item.id === 'focus_search' && parsed[item.id]?.toLowerCase() === 'control+f'
                ? item.defaultKey
                : parsed[item.id] || item.defaultKey,
            }))
          );
        }
      } catch (e) {
        console.warn('Falha ao restaurar atalhos salvos:', e);
      }
    },
    updateShortcut: (id: string, newKey: string) => {
      update((list) => {
        const updated = list.map((item) => (item.id === id ? { ...item, currentKey: newKey } : item));
        if (typeof window !== 'undefined') {
          const map: Record<string, string> = {};
          updated.forEach((i) => (map[i.id] = i.currentKey));
          try {
            localStorage.setItem('nanotube_shortcuts', JSON.stringify(map));
            SettingsService.saveSetting('custom_shortcuts', JSON.stringify(map)).catch(() => {});
          } catch {}
        }
        return updated;
      });
    },
    resetToDefaults: () => {
      set(DEFAULT_SHORTCUTS);
      if (typeof window !== 'undefined') {
        localStorage.removeItem('nanotube_shortcuts');
        SettingsService.saveSetting('custom_shortcuts', '').catch(() => {});
      }
    },
  };
}

export const shortcutsStore = createShortcutsStore();

/**
 * Normaliza um evento de teclado para a string de representação do atalho.
 * Exemplos: "Control+f", "Shift+ArrowRight", "Space", "F10", "k", "Alt+KeyP"
 */
export function normalizeKeyboardEvent(e: KeyboardEvent): string {
  const parts: string[] = [];
  if (e.ctrlKey || e.metaKey) parts.push('Control');
  if (e.altKey) parts.push('Alt');
  if (e.shiftKey && e.key !== 'Shift') parts.push('Shift');

  let key = e.key;
  if (key === ' ') key = 'Space';
  if (key === 'Control' || key === 'Alt' || key === 'Shift' || key === 'Meta') {
    return parts.join('+');
  }

  // Chaves de função ou teclas de seta
  if (key.startsWith('Arrow') || key.startsWith('F') || key === 'Space' || key === 'Escape' || key === 'Enter') {
    parts.push(key);
  } else {
    parts.push(key.toLowerCase());
  }

  return parts.join('+');
}

/**
 * Verifica se um evento de teclado corresponde a um atalho configurado.
 */
export function matchesShortcut(e: KeyboardEvent, shortcutId: string): boolean {
  const list = get(shortcutsStore);
  const item = list.find((i) => i.id === shortcutId);
  if (!item) return false;

  const currentFormatted = item.currentKey.trim();
  const eventFormatted = normalizeKeyboardEvent(e);

  // Comparações case-insensitive para comodidade
  return currentFormatted.toLowerCase() === eventFormatted.toLowerCase();
}
