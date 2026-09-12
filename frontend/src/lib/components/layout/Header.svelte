<script lang="ts">
  import { onMount } from 'svelte';
  import Rows3 from 'lucide-svelte/icons/rows-3';
  import Search from 'lucide-svelte/icons/search';
  import RefreshCw from 'lucide-svelte/icons/refresh-cw';
  import Tv from 'lucide-svelte/icons/tv';
  import Moon from 'lucide-svelte/icons/moon';
  import Sun from 'lucide-svelte/icons/sun';
  import User from 'lucide-svelte/icons/user';
  import LogIn from 'lucide-svelte/icons/log-in';
  import ArrowLeft from 'lucide-svelte/icons/arrow-left';
  import ArrowRight from 'lucide-svelte/icons/arrow-right';
  import { get } from 'svelte/store';
  import { activeTab, activePlaylistId, isTvMode, setTvMode, theme, toast } from '../../stores/uiStores';
  import { activeChannelSelection } from '../../stores/channelRouteStore';
  import { CatalogService, AccountService, SettingsService } from '../../wailsjs/services';
  import type { AccountInfo, Profile } from '../../types';
  import LoginModal from '../auth/LoginModal.svelte';
  import QueueModal from '../player/QueueModal.svelte';
  import { playerStore, playerQueue } from '../../stores/playerStore';
  import { submitGlobalSearch } from '../../stores/searchQueryStore';
  import { SpatialNavigation } from '../../navigation/SpatialNav';
  import { canNavigateBack, canNavigateForward, navigateBack, navigateForward } from '../../stores/navigationHistory';
  import ChevronDown from 'lucide-svelte/icons/chevron-down';
  import Check from 'lucide-svelte/icons/check';

  let searchQuery = '';
  let isRefreshing = false;
  let showLoginModal = false;
  let account: AccountInfo | null = null;
  let showQueueModal = false;
  let profiles: Profile[] = [];
  let activeProfileID = '';
  let showProfileMenu = false;
  let profileBusy = false;

  async function loadAccount() {
    try {
      const [loadedAccount, loadedProfiles, loadedActiveProfile] = await Promise.all([
        AccountService.getAccount(), AccountService.listProfiles(), AccountService.getActiveProfile(),
      ]);
      account = loadedAccount;
      profiles = loadedProfiles || [];
      activeProfileID = loadedActiveProfile?.id || '';
    } catch (e) {
      console.error('Erro ao carregar conta no header:', e);
    }
  }

  async function switchProfile(profileID: string): Promise<void> {
    if (!profileID || profileID === activeProfileID || profileBusy) return;
    profileBusy = true;
    try {
      await AccountService.setActiveProfile(profileID);
      activeProfileID = profileID;
      await loadAccount();
      showProfileMenu = false;
      window.dispatchEvent(new Event('nanotube-profile-changed'));
      toast.add('Perfil ativo atualizado.', 'success');
    } catch (error: unknown) {
      toast.add(error instanceof Error ? error.message : 'Falha ao trocar de perfil.', 'error');
    } finally {
      profileBusy = false;
    }
  }

  function openAccountControl(): void {
    if (!account?.email) {
      showLoginModal = true;
      return;
    }
    showProfileMenu = !showProfileMenu;
  }

  async function handleRefresh() {
    isRefreshing = true;
    try {
      await CatalogService.refreshSubscriptions();
      toast.add('Catálogo atualizado com sucesso!', 'success');
    } catch (err: any) {
      toast.add('Falha na atualização do catálogo', 'error');
    } finally {
      isRefreshing = false;
    }
  }

  function handleSearchSubmit() {
    if (searchQuery.trim()) {
      submitGlobalSearch(searchQuery);
      $activePlaylistId = null;
      $activeChannelSelection = null;
      $activeTab = 'search';
    }
  }

  function toggleTheme() {
    $theme = $theme === 'dark' ? 'light' : 'dark';
    document.documentElement.classList.toggle('light', $theme === 'light');
    void SettingsService.saveSetting('theme', $theme).catch(() => undefined);
  }

  function toggleTvMode() {
    const nextTvMode = !get(isTvMode);
    setTvMode(nextTvMode);
    if (nextTvMode) requestAnimationFrame(() => SpatialNavigation.focusFirst());
  }

  onMount(() => {
    void loadAccount();
    const handleProfileChanged = () => { void loadAccount(); };
    window.addEventListener('nanotube-profile-changed', handleProfileChanged);
    return () => window.removeEventListener('nanotube-profile-changed', handleProfileChanged);
  });
</script>

<header class="h-14 border-b border-border bg-surface/50 backdrop-blur-md px-4 flex items-center justify-between gap-4 z-30 shrink-0 select-none">
  <!-- Logo Button -->
  <button 
    type="button"
    class="flex items-center cursor-pointer focus:outline-none focus:ring-2 focus:ring-primary/40 rounded-lg p-0.5 transition-transform hover:opacity-90" 
    on:click={() => { $activePlaylistId = null; $activeChannelSelection = null; $activeTab = 'home'; }}
    title="HummTube"
    aria-label="Ir para a página inicial do HummTube"
  >
    <img
      src="/branding/logo-full.png"
      alt="HummTube"
      class="h-8 md:h-9 w-auto object-contain select-none"
    />
  </button>

  <!-- Search Bar -->
  <div class="flex items-center gap-1" aria-label="Histórico de navegação">
    <button type="button" on:click={navigateBack} disabled={!$canNavigateBack} aria-label="Voltar" title="Voltar" class="rounded-lg p-2 text-muted hover:bg-surfaceHover hover:text-foreground disabled:opacity-30 tv-focusable"><ArrowLeft size={17} /></button>
    <button type="button" on:click={navigateForward} disabled={!$canNavigateForward} aria-label="Avançar" title="Avançar" class="rounded-lg p-2 text-muted hover:bg-surfaceHover hover:text-foreground disabled:opacity-30 tv-focusable"><ArrowRight size={17} /></button>
  </div>

  <!-- Search Bar -->
  <div class="flex-1 max-w-xl">
    <form on:submit|preventDefault={handleSearchSubmit} class="relative flex items-center">
      <Search class="absolute left-3 text-muted pointer-events-none" size={16} />
      <input
        type="search"
        id="global-search-input"
        bind:value={searchQuery}
        placeholder="Buscar no catálogo ou YouTube (Ctrl+K)..."
        class="w-full bg-surface border border-border rounded-full pl-9 pr-4 py-1.5 text-sm text-foreground placeholder:text-muted focus:outline-none focus:border-primary transition-all tv-focusable"
      />
    </form>
  </div>

  <!-- Actions -->
  <div class="flex items-center gap-2">
    <button
      type="button"
      on:click={handleRefresh}
      disabled={isRefreshing}
      title="Atualizar inscrições (Ctrl+R)"
      class="p-2 rounded-lg text-muted hover:text-foreground hover:bg-surfaceHover transition-colors tv-focusable"
    >
      <RefreshCw size={18} class={isRefreshing ? 'animate-spin text-primary' : ''} />
    </button>

    <button
      type="button"
      on:click={toggleTheme}
      title="Alternar tema"
      class="p-2 rounded-lg text-muted hover:text-foreground hover:bg-surfaceHover transition-colors tv-focusable"
    >
      {#if $theme === 'dark'}
        <Sun size={18} />
      {:else}
        <Moon size={18} />
      {/if}
    </button>

    <button
      type="button"
      on:click={toggleTvMode}
      title="Modo TV (F10)"
      class="p-2 rounded-lg {$isTvMode ? 'text-primary bg-primary/10' : 'text-muted hover:text-foreground hover:bg-surfaceHover'} transition-colors tv-focusable"
    >
      <Tv size={18} />
    </button>

    <button
      type="button"
      on:click={() => (showQueueModal = true)}
      title="Abrir fila de reprodução"
      aria-label={`Abrir fila de reprodução (${ $playerQueue.length } vídeos)`}
      class="relative p-2 rounded-lg text-muted hover:text-foreground hover:bg-surfaceHover transition-colors tv-focusable"
    >
      <Rows3 size={18} />
      {#if $playerQueue.length > 0}
        <span class="absolute -right-0.5 -top-0.5 min-w-4 h-4 rounded-full bg-primary px-1 text-[9px] font-bold leading-4 text-white">
          {$playerQueue.length}
        </span>
      {/if}
    </button>

    <!-- Botão de Login / Conta posicionado após o Modo TV -->
    {#if account?.email}
      <button
        type="button"
        on:click={openAccountControl}
        title={`Conectado como ${account.email}`}
        class="flex items-center gap-2 pl-1 pr-2.5 py-1 rounded-full bg-surface hover:bg-surfaceHover border border-border text-foreground transition-colors tv-focusable"
      >
        <div class="w-7 h-7 rounded-full bg-primary/20 text-primary flex items-center justify-center font-bold text-xs">
          {account.email.charAt(0).toUpperCase()}
        </div>
        <span class="text-xs font-semibold truncate max-w-[100px] hidden md:inline">{account.email.split('@')[0]}</span>
        <ChevronDown size={13} class="hidden sm:block text-muted" />
      </button>
    {:else}
      <button
        type="button"
        on:click={openAccountControl}
        title="Entrar / Conectar Conta"
        class="flex items-center gap-1.5 px-3 py-1.5 rounded-full bg-primary text-white hover:bg-primary/90 font-semibold text-xs transition-colors tv-focusable shadow-sm shadow-primary/20"
      >
        <LogIn size={15} />
        <span class="hidden sm:inline">Entrar</span>
      </button>
      {#if profiles.length > 0}
        <button
          type="button"
          on:click={() => (showProfileMenu = !showProfileMenu)}
          title="Gerenciar perfis e contas"
          aria-label="Gerenciar perfis e contas"
          aria-expanded={showProfileMenu}
          class="rounded-lg p-2 text-muted hover:bg-surfaceHover hover:text-foreground transition-colors tv-focusable"
        >
          <User size={17} />
        </button>
      {/if}
    {/if}
  </div>

  {#if showProfileMenu}
    <button
      type="button"
      class="fixed inset-0 z-40 cursor-default bg-transparent"
      aria-label="Fechar menu de perfis"
      on:click={() => (showProfileMenu = false)}
      tabindex="-1"
    />
    <div class="absolute right-4 top-12 z-50 w-64 rounded-xl border border-border bg-surface p-2 shadow-2xl" role="menu" aria-label="Perfis e contas">
      <div class="px-2 py-1.5 text-[10px] font-semibold uppercase tracking-wide text-muted">Perfil ativo</div>
      {#each profiles as profile (profile.id)}
        <button type="button" role="menuitemradio" aria-checked={profile.id === activeProfileID} disabled={profileBusy} on:click={() => switchProfile(profile.id)} class="flex w-full items-center gap-2 rounded-lg px-2.5 py-2 text-left text-xs text-foreground hover:bg-surfaceHover disabled:opacity-50 tv-focusable">
          <span class="flex h-6 w-6 items-center justify-center rounded-full bg-primary/15 text-[10px] font-bold text-primary">{profile.name.slice(0, 1).toUpperCase()}</span>
          <span class="min-w-0 flex-1 truncate">{profile.name}{profile.kind === 'guest' ? ' (visitante)' : ''}</span>
          {#if profile.id === activeProfileID}<Check size={14} class="text-primary" />{/if}
        </button>
      {/each}
      <div class="my-1 border-t border-border"></div>
      <button type="button" role="menuitem" on:click={async () => { showProfileMenu = false; try { await AccountService.createGuestProfile('Convidado'); await loadAccount(); window.dispatchEvent(new Event('nanotube-profile-changed')); toast.add('Sessão de convidado ativada.', 'info'); } catch { toast.add('Erro ao criar sessão de convidado.', 'error'); } }} class="flex w-full items-center rounded-lg px-2.5 py-2 text-left text-xs text-muted hover:bg-surfaceHover hover:text-foreground tv-focusable">Entrar como convidado</button>
      <button type="button" role="menuitem" on:click={() => { showProfileMenu = false; $activeTab = 'settings'; }} class="flex w-full items-center rounded-lg px-2.5 py-2 text-left text-xs text-muted hover:bg-surfaceHover hover:text-foreground tv-focusable">Gerenciar contas e perfis</button>
      <button type="button" role="menuitem" on:click={() => { showProfileMenu = false; showLoginModal = true; }} class="flex w-full items-center rounded-lg px-2.5 py-2 text-left text-xs text-muted hover:bg-surfaceHover hover:text-foreground tv-focusable">Adicionar conta</button>
    </div>
  {/if}
</header>

<LoginModal bind:open={showLoginModal} bind:account />
<QueueModal bind:open={showQueueModal} />
