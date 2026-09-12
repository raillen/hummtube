<script lang="ts">
  import { onMount } from 'svelte';
  import Radio from 'lucide-svelte/icons/radio';
  import Moon from 'lucide-svelte/icons/moon';
  import Sun from 'lucide-svelte/icons/sun';
  import IPTVView from '../../lib/components/views/IPTVView.svelte';
  import PlayerSurface from '../../lib/components/player/PlayerSurface.svelte';
  import Toast from '../../lib/components/layout/Toast.svelte';
  import { SettingsService } from '../../lib/wailsjs/services';
  import { applyDisplaySettings } from '../../lib/stores/displayPreferences';
  import { applyAccentColor, isTvMode, theme } from '../../lib/stores/uiStores';
  import { SpatialNavigation } from '../../lib/navigation/SpatialNav';
  import { executePlayerCommand, isPlayerCommandDetail } from '../../lib/player/commands';

  function toggleTheme(): void {
    $theme = $theme === 'dark' ? 'light' : 'dark';
    document.documentElement.classList.toggle('light', $theme === 'light');
    void SettingsService.saveSetting('theme', $theme).catch(() => undefined);
  }

  onMount(() => {
    const destroySpatialNavigation = SpatialNavigation.init();
    const params = new URLSearchParams(window.location.search);
    if (params.get('tv') === '1') {
      $isTvMode = true;
      document.documentElement.classList.add('tv-mode');
    }
    void SettingsService.getSettings().then((settings) => {
      applyDisplaySettings(settings || {});
      $theme = settings?.theme === 'light' ? 'light' : 'dark';
      document.documentElement.classList.toggle('light', $theme === 'light');
      applyAccentColor(settings?.accent_color || 'sky');
    });
    const handleNativePlayerCommand = (event: Event): void => {
      const detail = (event as CustomEvent<unknown>).detail;
      if (isPlayerCommandDetail(detail)) executePlayerCommand(detail);
    };
    window.addEventListener('nanotube-player-command', handleNativePlayerCommand);
    return () => {
      window.removeEventListener('nanotube-player-command', handleNativePlayerCommand);
      destroySpatialNavigation();
    };
  });
</script>

<div class="flex h-screen w-screen flex-col overflow-hidden bg-background text-foreground">
  <header class="flex h-14 shrink-0 items-center justify-between border-b border-border bg-surface/70 px-4" aria-label="Cabeçalho do HummIPTV">
    <div class="flex items-center gap-2.5">
      <span class="flex h-8 w-8 items-center justify-center rounded-lg bg-primary text-white"><Radio size={18} /></span>
      <div><strong class="block text-sm">HummIPTV</strong><span class="block text-[10px] text-muted">TV, filmes, séries e EPG</span></div>
    </div>
    <button type="button" class="tv-focusable rounded-lg p-2 text-muted hover:bg-surfaceHover hover:text-foreground" on:click={toggleTheme} aria-label={$theme === 'dark' ? 'Usar tema claro' : 'Usar tema escuro'}>
      {#if $theme === 'dark'}<Sun size={18} />{:else}<Moon size={18} />{/if}
    </button>
  </header>
  <main class="relative min-h-0 flex-1 overflow-y-auto" aria-label="Catálogo IPTV">
    <IPTVView />
    <PlayerSurface />
  </main>
  <Toast />
</div>
