<script lang="ts">
  import { onMount } from 'svelte';
  import Home from 'lucide-svelte/icons/house';
  import Tv2 from 'lucide-svelte/icons/tv-minimal';
  import Search from 'lucide-svelte/icons/search';
  import ListMusic from 'lucide-svelte/icons/list-music';
  import Rows3 from 'lucide-svelte/icons/rows-3';
  import PanelLeftClose from 'lucide-svelte/icons/panel-left-close';
  import PanelLeftOpen from 'lucide-svelte/icons/panel-left-open';
  import Bookmark from 'lucide-svelte/icons/bookmark';
  import Settings from 'lucide-svelte/icons/settings';
  import Users from 'lucide-svelte/icons/users';
  import Cpu from 'lucide-svelte/icons/cpu';
  import MemoryStick from 'lucide-svelte/icons/memory-stick';
  import ArrowDownUp from 'lucide-svelte/icons/arrow-down-up';
  import { activeTab, activePlaylistId, sidebarCollapsed, type AppTab } from '../../stores/uiStores';
  import { SettingsService } from '../../wailsjs/services';
  import { activeChannelSelection } from '../../stores/channelRouteStore';
  import type { ResourceUsage } from '../../types';

  interface NavItem {
    id: AppTab;
    label: string;
    icon: any;
  }

  const items: NavItem[] = [
    { id: 'home', label: 'Início', icon: Home },
    { id: 'subscriptions', label: 'Inscrições', icon: Tv2 },
    { id: 'channels', label: 'Gerenciar canais', icon: Users },
    { id: 'search', label: 'Pesquisa', icon: Search },
    { id: 'playlists', label: 'Playlists', icon: ListMusic },
    { id: 'queue', label: 'Fila', icon: Rows3 },
    { id: 'library', label: 'Biblioteca', icon: Bookmark },
    { id: 'settings', label: 'Ajustes', icon: Settings },
  ];

  let usage: ResourceUsage | null = null;


  function formatBytes(bytes: number): string {
    if (bytes < 1024) return bytes + ' B';
    if (bytes < 1048576) return (bytes / 1024).toFixed(0) + ' KiB';
    if (bytes < 1073741824) return (bytes / 1048576).toFixed(1) + ' MiB';
    return (bytes / 1073741824).toFixed(2) + ' GiB';
  }

  function formatRate(bytesPerSec: number): string {
    if (bytesPerSec < 1024) return bytesPerSec.toFixed(0) + ' B/s';
    if (bytesPerSec < 1048576) return (bytesPerSec / 1024).toFixed(1) + ' KiB/s';
    return (bytesPerSec / 1048576).toFixed(1) + ' MiB/s';
  }

  function readWebViewMemory(): number | null {
    if (typeof performance === 'undefined') return null;
    const memory = (performance as Performance & { memory?: { usedJSHeapSize?: number } }).memory;
    return memory?.usedJSHeapSize ?? null;
  }

  let webViewHeapBytes: number | null = null;

  onMount(() => {
    let disposed = false;
    let timer: ReturnType<typeof setTimeout>;
    let webViewTimer: ReturnType<typeof setInterval>;
    const desktop = window.matchMedia('(min-width: 768px)');
    async function pollResources(): Promise<void> {
      if (!document.hidden && desktop.matches && !$sidebarCollapsed) {
        try {
          const result = await SettingsService.getResourceUsage();
          if (!disposed) usage = result;
        } catch {
          if (!disposed) usage = null;
        }
      } else {
        usage = null;
      }
      if (!disposed) timer = setTimeout(pollResources, 5000);
    }
    void pollResources();
    webViewTimer = setInterval(() => {
      if (disposed) return;
      webViewHeapBytes = readWebViewMemory();
    }, 10000);
    return () => {
      disposed = true;
      clearTimeout(timer);
      clearInterval(webViewTimer);
    };
  });

  function toggleSidebar(): void {
    $sidebarCollapsed = !$sidebarCollapsed;
    void SettingsService.saveSetting('sidebar_collapsed', $sidebarCollapsed ? '1' : '0').catch(() => undefined);
  }
</script>

<aside 
  data-component="app-sidebar"
  class="border-r border-border bg-surface/30 flex flex-col justify-between p-2 shrink-0 select-none transition-[width] {$sidebarCollapsed ? 'w-16' : 'w-16 md:w-56'}"
  aria-label="Navegação Lateral"
>
  <nav class="flex flex-col gap-1" aria-label="Menu Principal">
    {#each items as item}
      <button
        type="button"
        on:click={() => {
          $activePlaylistId = null;
          $activeChannelSelection = null;
          $activeTab = item.id;
        }}
        aria-current={$activeTab === item.id ? 'page' : undefined}
        aria-label={item.label}
        class="flex items-center gap-3 px-3 py-2.5 rounded-xl text-sm font-medium transition-colors tv-focusable cursor-pointer
          {$activeTab === item.id 
            ? 'bg-primary/10 text-primary font-semibold' 
            : 'text-muted hover:text-foreground hover:bg-surfaceHover'}"
      >
        <svelte:component this={item.icon} size={20} class="shrink-0" />
        <span data-component="sidebar-label" class={$sidebarCollapsed ? 'hidden' : 'hidden md:inline'}>{item.label}</span>
      </button>
    {/each}
  </nav>

  <div class="border-t border-border p-1">
    <button type="button" on:click={toggleSidebar} class="mb-1 flex w-full items-center gap-3 rounded-lg p-2 text-muted hover:bg-surfaceHover hover:text-foreground tv-focusable" aria-label={$sidebarCollapsed ? 'Expandir menu lateral' : 'Recolher menu lateral'} aria-expanded={!$sidebarCollapsed} title={$sidebarCollapsed ? 'Expandir menu lateral' : 'Recolher menu lateral'}>
      {#if $sidebarCollapsed}<PanelLeftOpen size={18} />{:else}<PanelLeftClose size={18} /><span class="hidden text-xs md:inline">Recolher menu</span>{/if}
    </button>
    <section class="rounded-lg bg-surfaceHover/40 p-2 text-[11px] leading-tight text-muted {$sidebarCollapsed ? 'hidden' : 'hidden md:block'}" data-component="resource-usage-card" aria-label="Uso de recursos">
      {#if usage === null || (usage.rss_bytes === null && usage.cpu_percent === null && usage.receive_bytes_per_second === null && usage.transmit_bytes_per_second === null)}
        <p>Recursos indisponíveis</p>
      {:else}
        <p class="mb-1 font-medium text-foreground">Uso de recursos</p>
        <p class="mb-1">RAM: processo Go (sem WebView). CPU e rede: sistema.</p>
        {#if webViewHeapBytes !== null}
          <li class="flex items-center gap-1.5">
            <span class="font-mono text-[10px] uppercase text-muted">JS:</span>
            <span>WebView heap: {formatBytes(webViewHeapBytes)}</span>
          </li>
        {/if}
        <ul class="flex flex-col gap-1">
          <li class="flex items-center gap-1.5">
            <MemoryStick size={14} class="shrink-0" aria-hidden="true" />
            <span>RAM: {usage.rss_bytes !== null ? formatBytes(usage.rss_bytes) : 'indisponível'}</span>
          </li>
          <li class="flex items-center gap-1.5">
            <Cpu size={14} class="shrink-0" aria-hidden="true" />
            <span>CPU: {usage.cpu_percent !== null ? usage.cpu_percent.toFixed(1) + '%' : 'indisponível'}</span>
          </li>
          <li class="flex items-center gap-1.5">
            <ArrowDownUp size={14} class="shrink-0" aria-hidden="true" />
            <span>
              {#if usage.receive_bytes_per_second !== null && usage.transmit_bytes_per_second !== null}
                Recebido: {formatRate(usage.receive_bytes_per_second)}<br />
                Enviado: {formatRate(usage.transmit_bytes_per_second)}
              {:else}Rede indisponível{/if}
            </span>
          </li>
        </ul>
      {/if}
    </section>
  </div>
</aside>
