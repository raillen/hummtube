<script lang="ts">
  import { onMount } from 'svelte';
  import Header from './lib/components/layout/Header.svelte';
  import Sidebar from './lib/components/layout/Sidebar.svelte';
  import Toast from './lib/components/layout/Toast.svelte';
  import OnboardingWizard from './lib/components/onboarding/OnboardingWizard.svelte';
  import GlobalNoticeBar from './lib/components/layout/GlobalNoticeBar.svelte';
  import VideoPlayer from './lib/components/player/VideoPlayer.svelte';
  import type { ComponentType } from 'svelte';
  import { get } from 'svelte/store';
  import { activeTab, activePlaylistId, applyAccentColor, applyTvMode, isTvMode, resolveInitialTvMode, setTvMode, sidebarCollapsed, theme, type AppTab } from './lib/stores/uiStores';
  import { activeChannelSelection } from './lib/stores/channelRouteStore';
  import { playerStore, isMiniplayer, miniplayerPosition, hasActiveVideo } from './lib/stores/playerStore';
  import { shortcutsStore, matchesShortcut } from './lib/stores/shortcutsStore';
  import { SpatialNavigation } from './lib/navigation/SpatialNav';
  import { executePlayerCommand, isPlayerCommandDetail } from './lib/player/commands';
  import { applyDisplaySettings } from './lib/stores/displayPreferences';
  import { initializeNavigationHistory } from './lib/stores/navigationHistory';

  import { SettingsService } from './lib/wailsjs/services';

  const miniplayerPositionClasses: Record<string, string> = {
    'bottom-right': 'bottom-4 right-4',
    'bottom-left': 'bottom-4 left-4',
    'top-left': 'left-4 top-4',
    'top-right': 'right-4 top-4'
  };

  type NanoTubeTab = Exclude<AppTab, 'music' | 'iptv'>;
  type ViewKey = NanoTubeTab | 'playlist-detail' | 'channel-detail';
  type ViewModule = { default: ComponentType };
  const viewLoaders: Record<ViewKey, () => Promise<ViewModule>> = {
    home: () => import('./lib/components/views/HomeView.svelte'),
    subscriptions: () => import('./lib/components/views/SubscriptionsView.svelte'),
    channels: () => import('./lib/components/views/ChannelManagementView.svelte'),
    search: () => import('./lib/components/views/SearchView.svelte'),
    playlists: () => import('./lib/components/views/PlaylistsView.svelte'),
    queue: () => import('./lib/components/views/QueueView.svelte'),
    library: () => import('./lib/components/views/LibraryView.svelte'),
    settings: () => import('./lib/components/views/SettingsView.svelte'),
    'playlist-detail': () => import('./lib/components/views/PlaylistDetailView.svelte'),
    'channel-detail': () => import('./lib/components/views/ChannelDetailView.svelte'),
  };
  const loadedViews = new Map<ViewKey, ComponentType>();
  let activeView: ComponentType | null = null;
  let loadedViewKey = '';
  let viewLoadGeneration = 0;

  $: requestedViewKey = ($activePlaylistId
    ? 'playlist-detail'
    : $activeChannelSelection
      ? 'channel-detail'
      : ($activeTab === 'music' || $activeTab === 'iptv' ? 'home' : $activeTab)) as ViewKey;
  $: if (requestedViewKey !== loadedViewKey) void loadView(requestedViewKey);

  async function loadView(key: ViewKey): Promise<void> {
    const generation = ++viewLoadGeneration;
    loadedViewKey = key;
    const cached = loadedViews.get(key);
    if (cached) {
      activeView = cached;
      return;
    }
    activeView = null;
    const module = await viewLoaders[key]();
    if (generation !== viewLoadGeneration) return;
    loadedViews.set(key, module.default);
    activeView = module.default;
  }

  onMount(() => {
    shortcutsStore.init();
    const destroySpatialNavigation = SpatialNavigation.init();
    const destroyNavigationHistory = initializeNavigationHistory();
    void playerStore.initQueue();
    
    // Carrega configurações do backend
    void SettingsService.getSettings().then((settings) => {
      applyDisplaySettings(settings || {});
      $theme = settings?.theme === 'light' ? 'light' : 'dark';
      document.documentElement.classList.toggle('light', $theme === 'light');
      $sidebarCollapsed = settings?.sidebar_collapsed === '1';
      const initialTvMode = resolveInitialTvMode(settings);
      applyTvMode(initialTvMode);
      if (initialTvMode) requestAnimationFrame(() => SpatialNavigation.focusFirst());
      if (settings?.accent_color) {
        applyAccentColor(settings.accent_color);
      } else {
        const savedAccent = localStorage.getItem('nanotube_accent_color');
        if (savedAccent) applyAccentColor(savedAccent);
      }
    }).catch(() => {
      const savedAccent = localStorage.getItem('nanotube_accent_color');
      if (savedAccent) applyAccentColor(savedAccent);
      applyTvMode(resolveInitialTvMode(null));
    });
    const handleNativePlayerCommand = (event: Event) => {
      const detail = (event as CustomEvent<unknown>).detail;
      if (!isPlayerCommandDetail(detail)) return;
      if (executePlayerCommand(detail) === 'open_queue') $activeTab = 'queue';
    };
    window.addEventListener('nanotube-player-command', handleNativePlayerCommand);
    return () => {
      window.removeEventListener('nanotube-player-command', handleNativePlayerCommand);
      destroySpatialNavigation();
      destroyNavigationHistory();
    };
  });

  function handleGlobalKeyDown(e: KeyboardEvent) {
    const target = e.target as HTMLElement;
    const isInput = target && (target.tagName === 'INPUT' || target.tagName === 'TEXTAREA' || target.tagName === 'SELECT' || target.isContentEditable);
    const dialogOpen = Array.from(document.querySelectorAll('[role="dialog"]')).some(
      (dialog) => dialog instanceof HTMLElement && dialog.getBoundingClientRect().width > 0
    );
    const tvMode = document.documentElement.classList.contains('tv-mode');

    // Cadeia de retorno consistente: menu aberto -> modal -> miniplayer -> histórico de navegação
    if ((e.key === 'Escape' || (tvMode && e.key === 'BrowserBack')) && !isInput) {
      const openMenuDismiss = document.querySelector<HTMLElement>('button[aria-label^="Fechar menu"]');
      if (openMenuDismiss) {
        e.preventDefault();
        openMenuDismiss.click();
        return;
      }
      if (dialogOpen) {
        // Modal gerencia o próprio Escape com foco restaurado; apenas ignore
        return;
      }
      if ($playerStore.currentVideo && !$playerStore.isMiniplayer) {
        e.preventDefault();
        playerStore.setMiniplayer(true);
        return;
      }
      if (tvMode) {
        e.preventDefault();
        SpatialNavigation.back();
        return;
      }
    }

    if (matchesShortcut(e, 'focus_search')) {
      e.preventDefault();
      const searchInput = document.querySelector<HTMLInputElement>('#global-search-input');
      if (searchInput) {
        searchInput.focus();
        searchInput.select();
      }
      return;
    }

    if (isInput) return;

    if (matchesShortcut(e, 'toggle_tv_mode')) {
      e.preventDefault();
      const nextTvMode = !get(isTvMode);
      setTvMode(nextTvMode);
      if (nextTvMode) requestAnimationFrame(() => SpatialNavigation.focusFirst());
      return;
    }

    if (matchesShortcut(e, 'play_pause')) {
      if ($playerStore.currentVideo) {
        e.preventDefault();
        executePlayerCommand({ command: 'play_pause' });
      }
      return;
    }

    if (matchesShortcut(e, 'seek_forward')) {
      if ($playerStore.currentVideo) {
        e.preventDefault();
        executePlayerCommand({ command: 'seek_relative', value: 10 });
      }
      return;
    }

    if (matchesShortcut(e, 'seek_backward')) {
      if ($playerStore.currentVideo) {
        e.preventDefault();
        executePlayerCommand({ command: 'seek_relative', value: -10 });
      }
      return;
    }

    if (matchesShortcut(e, 'toggle_mute')) {
      if ($playerStore.currentVideo) {
        e.preventDefault();
        executePlayerCommand({ command: 'toggle_mute' });
      }
      return;
    }

    if (matchesShortcut(e, 'volume_up')) {
      if ($playerStore.currentVideo) {
        e.preventDefault();
        executePlayerCommand({ command: 'volume_up' });
      }
      return;
    }

    if (matchesShortcut(e, 'volume_down')) {
      if ($playerStore.currentVideo) {
        e.preventDefault();
        executePlayerCommand({ command: 'volume_down' });
      }
      return;
    }

    if (matchesShortcut(e, 'toggle_fullscreen')) {
      if ($playerStore.currentVideo) {
        e.preventDefault();
        if ($playerStore.isMiniplayer) {
          playerStore.setMiniplayer(false);
        } else {
          executePlayerCommand({ command: 'toggle_fullscreen' });
        }
      }
      return;
    }
  }
</script>

<svelte:window on:keydown={handleGlobalKeyDown} />

<div class="flex flex-col h-screen w-screen bg-background text-foreground overflow-hidden">
  <!-- Header -->
  <Header />

  <!-- Global Warning / Health Notice Bar -->
  <GlobalNoticeBar />

  <a
    href="#main-content"
    class="sr-only focus:not-sr-only focus:fixed focus:left-2 focus:top-2 focus:z-[200] focus:rounded-md focus:bg-primary focus:px-3 focus:py-1.5 focus:text-xs focus:font-semibold focus:text-white"
  >
    Pular para o conteúdo principal
  </a>

  <!-- Main Body -->
  <div class="flex flex-1 min-h-0 relative">
    <!-- Sidebar -->
    <Sidebar />

    <!-- Content Views Area -->
    <main id="main-content" class="flex-1 overflow-y-auto relative" aria-label="Conteúdo Principal">
      {#if activeView}
        <svelte:component this={activeView} />
      {:else}
        <div class="flex h-full items-center justify-center text-xs text-muted" role="status" aria-live="polite">Carregando módulo…</div>
      {/if}
    </main>

    <!-- Fullscreen / In-App Player View Overlay -->
    {#if $hasActiveVideo}
      <div
        class={$isMiniplayer
          ? `fixed z-50 aspect-video w-[min(24rem,calc(100vw-2rem))] overflow-hidden rounded-xl border border-white/15 bg-black shadow-2xl isolate transition-all duration-200 ${miniplayerPositionClasses[$miniplayerPosition] || miniplayerPositionClasses['bottom-right']}`
          : 'absolute inset-0 z-40 bg-black flex flex-col'}
        role="region"
        aria-label="Player de Vídeo"
        data-player-mode={$isMiniplayer ? 'mini' : 'expanded'}
        data-miniplayer-position={$isMiniplayer ? $miniplayerPosition : undefined}
      >
        <VideoPlayer />
      </div>
    {/if}
  </div>

  <OnboardingWizard />

  <!-- Toast Notification Overlay -->
  <Toast />
</div>
