<script lang="ts">
  import { onMount } from 'svelte';
  import Headphones from 'lucide-svelte/icons/headphones';
  import Library from 'lucide-svelte/icons/library';
  import ListMusic from 'lucide-svelte/icons/list-music';
  import Settings from 'lucide-svelte/icons/settings';
  import Rows3 from 'lucide-svelte/icons/rows-3';
  import MusicView from '../../lib/components/views/MusicView.svelte';
  import QueueView from '../../lib/components/views/QueueView.svelte';
  import PlaylistsView from '../../lib/components/views/PlaylistsView.svelte';
  import LibraryView from '../../lib/components/views/LibraryView.svelte';
  import MusicSettingsView from './MusicSettingsView.svelte';
  import PlayerSurface from '../../lib/components/player/PlayerSurface.svelte';
  import Toast from '../../lib/components/layout/Toast.svelte';
  import { activeTab, applyAccentColor, isTvMode, theme, type AppTab } from '../../lib/stores/uiStores';
  import { playerStore } from '../../lib/stores/playerStore';
  import { SettingsService } from '../../lib/wailsjs/services';
  import { applyDisplaySettings } from '../../lib/stores/displayPreferences';
  import { SpatialNavigation } from '../../lib/navigation/SpatialNav';
  import { executePlayerCommand, isPlayerCommandDetail } from '../../lib/player/commands';

  type NanoMusicTab = Extract<AppTab, 'music' | 'queue' | 'playlists' | 'library' | 'settings'>;
  const navigation: Array<{ id: NanoMusicTab; label: string; icon: typeof Headphones }> = [
    { id: 'music', label: 'Descobrir', icon: Headphones },
    { id: 'queue', label: 'Fila', icon: Rows3 },
    { id: 'playlists', label: 'Playlists', icon: ListMusic },
    { id: 'library', label: 'Biblioteca', icon: Library },
    { id: 'settings', label: 'Ajustes', icon: Settings },
  ];

  function selectTab(tab: NanoMusicTab): void {
    $activeTab = tab;
  }

  onMount(() => {
    $activeTab = 'music';
    const destroySpatialNavigation = SpatialNavigation.init();
    void playerStore.initQueue();
    void SettingsService.getSettings().then((settings) => {
      applyDisplaySettings(settings || {});
      applyAccentColor(settings?.accent_color || 'purple');
      $theme = settings?.theme === 'light' ? 'light' : 'dark';
      document.documentElement.classList.toggle('light', $theme === 'light');
    });
    const params = new URLSearchParams(window.location.search);
    if (params.get('tv') === '1') {
      $isTvMode = true;
      document.documentElement.classList.add('tv-mode');
    }
    const handleNativePlayerCommand = (event: Event): void => {
      const detail = (event as CustomEvent<unknown>).detail;
      if (!isPlayerCommandDetail(detail)) return;
      if (executePlayerCommand(detail) === 'open_queue') selectTab('queue');
    };
    window.addEventListener('nanotube-player-command', handleNativePlayerCommand);
    return () => {
      window.removeEventListener('nanotube-player-command', handleNativePlayerCommand);
      destroySpatialNavigation();
    };
  });
</script>

<div class="flex h-screen w-screen flex-col overflow-hidden bg-background text-foreground">
  <header class="flex min-h-14 shrink-0 flex-wrap items-center justify-between gap-3 border-b border-border bg-surface/70 px-4 py-2" aria-label="Cabeçalho do HummMusic">
    <div class="flex items-center gap-2.5">
      <span class="flex h-8 w-8 items-center justify-center rounded-lg bg-primary text-white"><Headphones size={18} /></span>
      <div><strong class="block text-sm">HummMusic</strong><span class="block text-[10px] text-muted">Música e podcasts sem distrações</span></div>
    </div>
    <nav class="flex max-w-full gap-1 overflow-x-auto" aria-label="Navegação principal do HummMusic">
      {#each navigation as item}
        <button type="button" class="tv-focusable inline-flex items-center gap-1.5 rounded-lg px-3 py-2 text-xs font-semibold {$activeTab === item.id ? 'bg-primary/15 text-primary' : 'text-muted hover:bg-surfaceHover hover:text-foreground'}" aria-current={$activeTab === item.id ? 'page' : undefined} on:click={() => selectTab(item.id)}>
          <svelte:component this={item.icon} size={15} /> {item.label}
        </button>
      {/each}
    </nav>
  </header>
  <main class="relative min-h-0 flex-1 overflow-y-auto" aria-label="Conteúdo do HummMusic">
    {#if $activeTab === 'queue'}
      <QueueView />
    {:else if $activeTab === 'playlists'}
      <PlaylistsView />
    {:else if $activeTab === 'library'}
      <LibraryView />
    {:else if $activeTab === 'settings'}
      <MusicSettingsView />
    {:else}
      <MusicView />
    {/if}
    <PlayerSurface />
  </main>
  <Toast />
</div>
