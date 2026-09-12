<script lang="ts">
  import { onMount } from 'svelte';
  import { IPTVService } from '../../wailsjs/services';
  import type { 
    IPTVSourceState, IPTVEndpointDiagnostic, IPTVItem, IPTVItemFilter, IPTVPageResult, 
    IPTVGuideEntry, IPTVResumeEntry 
  } from '../../types';
  import { playIPTVItem } from '../../../apps/nanoiptv/iptvPlayer';
  import { toast } from '../../stores/uiStores';
  import Button from '../ui/Button.svelte';
  import Modal from '../ui/Modal.svelte';
  import Chip from '../ui/Chip.svelte';
  import Tv from 'lucide-svelte/icons/tv';
  import Film from 'lucide-svelte/icons/film';
  import Clapperboard from 'lucide-svelte/icons/clapperboard';
  import Radio from 'lucide-svelte/icons/radio';
  import Heart from 'lucide-svelte/icons/heart';
  import Calendar from 'lucide-svelte/icons/calendar';
  import History from 'lucide-svelte/icons/history';
  import Plus from 'lucide-svelte/icons/plus';
  import RefreshCw from 'lucide-svelte/icons/refresh-cw';
  import Trash2 from 'lucide-svelte/icons/trash-2';
  import Search from 'lucide-svelte/icons/search';
  import Play from 'lucide-svelte/icons/play';
  import Check from 'lucide-svelte/icons/check';
  import AlertCircle from 'lucide-svelte/icons/circle-alert';
  import AlertTriangle from 'lucide-svelte/icons/triangle-alert';
  import Layers from 'lucide-svelte/icons/layers';
  import Star from 'lucide-svelte/icons/star';
  import StarOff from 'lucide-svelte/icons/star-off';
  import Clock from 'lucide-svelte/icons/clock';
  import Pencil from 'lucide-svelte/icons/pencil';
  import Network from 'lucide-svelte/icons/network';

  let sources: IPTVSourceState[] = [];
  let activeTab: 'tv' | 'movie' | 'series' | 'unknown' | 'saved' | 'guide' | 'resume' = 'tv';
  let searchQuery = '';
  let selectedGroup = '';
  let groups: string[] = [];
  let pageResult: IPTVPageResult = { items: [], total_count: 0, page: 1, page_size: 36, total_pages: 1 };
  let guideEntries: IPTVGuideEntry[] = [];
  let resumeEntries: IPTVResumeEntry[] = [];
  let savedItemIds: Set<string> = new Set();
  
  let isLoading = false;
  let isSyncingSource: Record<string, boolean> = {};
  let showSourcesModal = false;
  let isSavingSource = false;
  let isDiagnosingEndpoint = false;
  let endpointDiagnostic: IPTVEndpointDiagnostic | null = null;

  // Formulário único para criação e edição de uma fonte.
  let editingSourceId: string | null = null;
  let sourceName = '';
  let sourceUrl = '';
  let sourceGuideUrl = '';
  let sourceEnabled = true;
  let sourceUsername = '';
  let sourcePassword = '';
  let clearSavedCredentials = false;
  let sourceOutputMode: 'preserve' | 'hls' = 'hls';
  let hasSavedCredentials = false;

  async function loadSources() {
    try {
      sources = (await IPTVService.listSources()) || [];
    } catch (e) {
      console.error('Erro ao carregar fontes IPTV:', e);
      sources = [];
    }
  }

  async function loadCatalog(page = 1) {
    if (!sources || sources.length === 0) {
      pageResult = { items: [], total_count: 0, page: 1, page_size: 36, total_pages: 1 };
      return;
    }
    isLoading = true;
    try {
      if (activeTab === 'guide') {
        guideEntries = (await IPTVService.listGuide('', 100)) || [];
      } else if (activeTab === 'resume') {
        resumeEntries = (await IPTVService.listResume(30)) || [];
      } else {
        const filter: IPTVItemFilter = {
          page,
          page_size: 36,
          query: searchQuery.trim() || undefined,
          group: selectedGroup || undefined,
        };

        if (activeTab === 'saved') {
          filter.saved_only = true;
        } else {
          filter.kind = activeTab;
        }

        const res = await IPTVService.listItemsPaginated(filter);
        pageResult = {
          items: res?.items || [],
          total_count: res?.total_count || 0,
          page: res?.page || 1,
          page_size: res?.page_size || 36,
          total_pages: res?.total_pages || 1,
        };
      }
    } catch (e) {
      console.error('Erro ao carregar catálogo IPTV:', e);
      pageResult = { items: [], total_count: 0, page: 1, page_size: 36, total_pages: 1 };
    } finally {
      isLoading = false;
    }
  }

  async function loadGroups() {
    if (activeTab === 'guide' || activeTab === 'resume' || activeTab === 'saved') {
      groups = [];
      return;
    }
    try {
      groups = (await IPTVService.listGroups(activeTab)) || [];
    } catch (e) {
      console.error('Erro ao carregar grupos:', e);
      groups = [];
    }
  }

  async function loadSavedItems() {
    try {
      const savedItems = await IPTVService.listSavedItems(1000);
      savedItemIds = new Set((savedItems || []).map((item) => item.id));
    } catch (error: unknown) {
      console.error('Erro ao carregar favoritos IPTV:', error);
      savedItemIds = new Set();
    }
  }

  function errorMessage(error: unknown, fallback: string): string {
    return error instanceof Error && error.message ? error.message : fallback;
  }

  function resetSourceForm() {
    editingSourceId = null;
    sourceName = '';
    sourceUrl = '';
    sourceGuideUrl = '';
    sourceEnabled = true;
    sourceUsername = '';
    sourcePassword = '';
    clearSavedCredentials = false;
    sourceOutputMode = 'hls';
    hasSavedCredentials = false;
    endpointDiagnostic = null;
  }

  function beginEditingSource(source: IPTVSourceState) {
    editingSourceId = source.config.id;
    sourceName = source.config.name;
    sourceUrl = source.config.playlist_url || '';
    sourceGuideUrl = source.config.guide_url || '';
    sourceEnabled = source.config.enabled;
    sourceUsername = '';
    sourcePassword = '';
    clearSavedCredentials = false;
    hasSavedCredentials = Boolean(source.config.credential_ref);
    sourceOutputMode = playlistUsesHLSOutput(sourceUrl) ? 'hls' : 'preserve';
    endpointDiagnostic = null;
  }

  function playlistUsesHLSOutput(value: string): boolean {
    try {
      const output = new URL(value).searchParams.get('output')?.toLowerCase();
      return output === 'hls' || output === 'm3u8';
    } catch {
      return false;
    }
  }

  async function diagnoseSourceEndpoint(): Promise<IPTVEndpointDiagnostic | null> {
    if (!sourceUrl.trim()) {
      toast.add('Informe a URL antes de executar o diagnóstico', 'error');
      return null;
    }
    isDiagnosingEndpoint = true;
    try {
      endpointDiagnostic = await IPTVService.diagnoseEndpoint(sourceUrl.trim());
      if (endpointDiagnostic.tcp_reachable) {
        toast.add('DNS público e conexão TCP verificados', 'success');
      } else {
        toast.add(endpointDiagnostic.error || 'O endpoint não aceitou conexão TCP', 'error');
      }
      return endpointDiagnostic;
    } catch (error: unknown) {
      endpointDiagnostic = null;
      toast.add(errorMessage(error, 'Falha ao diagnosticar a URL'), 'error');
      return null;
    } finally {
      isDiagnosingEndpoint = false;
    }
  }

  async function saveSourceConfiguration() {
    if (!sourceName.trim() || !sourceUrl.trim()) {
      toast.add('Informe o nome e a URL da playlist M3U', 'error');
      return;
    }
    if ((sourceUsername.trim() === '') !== (sourcePassword.trim() === '')) {
      toast.add('Informe usuário e senha juntos', 'error');
      return;
    }
    const diagnostic = await diagnoseSourceEndpoint();
    if (!diagnostic?.url_valid || !diagnostic.dns_resolved || !diagnostic.public_target) {
      return;
    }

    const sourceId = editingSourceId || 'src_' + Date.now().toString(36);
    const shouldSyncAfterSave = sourceEnabled;
    isSavingSource = true;
    try {
      await IPTVService.saveSourceWithCredentials(
        sourceId,
        sourceName.trim(),
        sourceUrl.trim(),
        sourceGuideUrl.trim(),
        sourceEnabled,
        sourceUsername.trim(),
        sourcePassword.trim(),
        clearSavedCredentials,
        sourceOutputMode,
      );
      toast.add(editingSourceId ? 'Fonte IPTV atualizada' : 'Fonte IPTV adicionada', 'success');
      resetSourceForm();
      await loadSources();
      if (shouldSyncAfterSave) {
        await handleSyncSource(sourceId);
      }
    } catch (error: unknown) {
      toast.add(errorMessage(error, 'Falha ao salvar fonte IPTV'), 'error');
    } finally {
      isSavingSource = false;
    }
  }

  async function handleDeleteSource(id: string) {
    try {
      await IPTVService.deleteSource(id);
      toast.add('Fonte IPTV removida', 'success');
      if (editingSourceId === id) resetSourceForm();
      await loadSources();
      await loadCatalog(1);
    } catch (error: unknown) {
      toast.add(errorMessage(error, 'Falha ao remover fonte'), 'error');
    }
  }

  async function handleSyncSource(id: string) {
    isSyncingSource = { ...isSyncingSource, [id]: true };
    try {
      const count = await IPTVService.syncSource(id);
      toast.add(`Sincronização concluída (${count} canais/itens importados)`, 'success');
      await loadSources();
      await loadGroups();
      await loadCatalog(1);
    } catch (error: unknown) {
      toast.add(errorMessage(error, 'Falha na sincronização da lista'), 'error');
    } finally {
      isSyncingSource = { ...isSyncingSource, [id]: false };
    }
  }

  async function handleToggleSave(item: IPTVItem, e: MouseEvent) {
    e.stopPropagation();
    const isSaved = savedItemIds.has(item.id);
    try {
      await IPTVService.setItemSaved(item.id, !isSaved);
      if (isSaved) {
        savedItemIds.delete(item.id);
        toast.add(`"${item.title}" removido dos favoritos`, 'info');
      } else {
        savedItemIds.add(item.id);
        toast.add(`"${item.title}" adicionado aos favoritos`, 'success');
      }
      savedItemIds = new Set(savedItemIds);
      if (activeTab === 'saved') {
        loadCatalog(pageResult.page);
      }
    } catch (err: any) {
      toast.add('Falha ao salvar favorito', 'error');
    }
  }

  function handlePlay(item: IPTVItem, resumePosition = 0) {
    playIPTVItem(item, resumePosition / 1e9);
  }

  function handleTabChange(tab: typeof activeTab) {
    activeTab = tab;
    selectedGroup = '';
    searchQuery = '';
    loadGroups();
    loadCatalog(1);
  }

  function handleSearchSubmit() {
    loadCatalog(1);
  }

  function formatTime(isoStr: string) {
    if (!isoStr) return '';
    try {
      const d = new Date(isoStr);
      return d.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' });
    } catch {
      return '';
    }
  }

  function openSourcesModal() {
    resetSourceForm();
    showSourcesModal = true;
  }

  function handleImageError(e: Event) {
    const target = e.currentTarget as HTMLElement;
    if (target) target.style.display = 'none';
  }

  onMount(async () => {
    await loadSources();
    await loadSavedItems();
    await loadGroups();
    await loadCatalog(1);
  });
</script>

<div class="flex flex-col gap-6 p-6 max-w-7xl mx-auto w-full">
  <!-- Top Header Section -->
  <div class="flex flex-wrap items-center justify-between gap-4">
    <div class="flex items-center gap-3">
      <div class="w-10 h-10 rounded-xl bg-primary/10 text-primary flex items-center justify-center">
        <Radio size={22} />
      </div>
      <div>
        <h1 class="text-xl font-bold text-foreground">HummIPTV</h1>
        <p class="text-xs text-muted mt-0.5">Transmissões ao vivo, catálogo VOD e guia de programação XMLTV</p>
      </div>
    </div>

    <div class="flex items-center gap-2">
      {#if (sources?.length || 0) > 0}
        <Button 
          variant="secondary" 
          size="sm" 
          on:click={() => sources.forEach(s => handleSyncSource(s.config.id))}
        >
          <RefreshCw size={14} class={Object.values(isSyncingSource).some(Boolean) ? 'animate-spin' : ''} />
          Sincronizar
        </Button>
      {/if}

      <button
        type="button"
        on:click={openSourcesModal}
        class="px-3 py-1.5 bg-primary hover:bg-primary/90 text-white text-xs font-semibold rounded-lg flex items-center gap-1.5 tv-focusable cursor-pointer shadow-sm shadow-primary/20"
      >
        <Layers size={14} />
        <span>Listas IPTV ({sources?.length || 0})</span>
      </button>
    </div>
  </div>

  <!-- Sync Warning Alert if any source has error -->
  {#if sources.some(s => s.last_error)}
    <div role="alert" class="p-4 bg-amber-500/10 border border-amber-500/30 rounded-2xl flex flex-wrap items-center justify-between gap-4 text-xs text-amber-200">
      <div class="flex items-center gap-3">
        <AlertTriangle size={18} class="text-amber-400 shrink-0" />
        <div>
          <strong class="text-foreground block">Alerta de Sincronização de Lista</strong>
          {#each sources.filter(s => s.last_error) as src}
            <span class="block mt-0.5"><span class="font-semibold">{src.config.name}:</span> {src.last_error}</span>
          {/each}
        </div>
      </div>
      <Button variant="secondary" size="sm" on:click={() => sources.forEach(s => handleSyncSource(s.config.id))}>
        <RefreshCw size={12} /> Tentar Novamente
      </Button>
    </div>
  {/if}

  <!-- Tabs Navigation -->
  <div class="flex items-center gap-2 overflow-x-auto pb-1 shrink-0 border-b border-border">
    <Chip active={activeTab === 'tv'} on:click={() => handleTabChange('tv')}>
      <Tv size={14} class="mr-1.5 inline" /> TV Ao Vivo
    </Chip>
    <Chip active={activeTab === 'movie'} on:click={() => handleTabChange('movie')}>
      <Film size={14} class="mr-1.5 inline" /> Filmes VOD
    </Chip>
    <Chip active={activeTab === 'series'} on:click={() => handleTabChange('series')}>
      <Clapperboard size={14} class="mr-1.5 inline" /> Séries VOD
    </Chip>
    <Chip active={activeTab === 'unknown'} on:click={() => handleTabChange('unknown')}>
      <AlertCircle size={14} class="mr-1.5 inline" /> Outros
    </Chip>
    <Chip active={activeTab === 'saved'} on:click={() => handleTabChange('saved')}>
      <Heart size={14} class="mr-1.5 inline" /> Favoritos
    </Chip>
    <Chip active={activeTab === 'guide'} on:click={() => handleTabChange('guide')}>
      <Calendar size={14} class="mr-1.5 inline" /> Guia EPG
    </Chip>
    <Chip active={activeTab === 'resume'} on:click={() => handleTabChange('resume')}>
      <History size={14} class="mr-1.5 inline" /> Continuar Assistindo
    </Chip>
  </div>

  <!-- Search & Category Filters (for TV, Movies, Series, Saved) -->
  {#if activeTab !== 'guide' && activeTab !== 'resume'}
    <div class="flex flex-col sm:flex-row items-center gap-3">
      <!-- Search Input -->
      <form on:submit|preventDefault={handleSearchSubmit} class="relative flex-1 w-full">
        <Search class="absolute left-3 top-2.5 text-muted pointer-events-none" size={16} />
        <input
          type="text"
          bind:value={searchQuery}
          placeholder="Buscar canal ou título IPTV..."
          class="w-full bg-surface border border-border rounded-xl pl-9 pr-4 py-2 text-xs text-foreground placeholder:text-muted focus:outline-none focus:border-primary tv-focusable"
        />
      </form>

      <!-- Category Filter Dropdown -->
      {#if (groups?.length || 0) > 0}
        <div class="flex items-center gap-2 w-full sm:w-auto">
          <select
            bind:value={selectedGroup}
            on:change={() => loadCatalog(1)}
            class="bg-surface border border-border rounded-xl px-3 py-2 text-xs text-foreground focus:outline-none focus:border-primary tv-focusable cursor-pointer w-full sm:w-48"
          >
            <option value="">Todos os Grupos ({groups?.length || 0})</option>
            {#each (groups || []) as grp}
              <option value={grp}>{grp}</option>
            {/each}
          </select>
        </div>
      {/if}
    </div>
  {/if}

  <!-- Main Content Grid / Empty States -->
  {#if (sources?.length || 0) === 0}
    <!-- No Sources Empty State -->
    <div class="p-12 border border-border bg-surface/40 rounded-2xl flex flex-col items-center justify-center text-center gap-4 my-8">
      <div class="w-16 h-16 rounded-2xl bg-primary/10 text-primary flex items-center justify-center">
        <Radio size={32} />
      </div>
      <div class="max-w-md">
        <h3 class="text-base font-bold text-foreground">Nenhuma lista IPTV adicionada</h3>
        <p class="text-xs text-muted mt-1 leading-relaxed">
          Adicione uma playlist M3U pública (ex: iptv-org) ou uma lista personalizada para assistir transmissões de TV e conteúdos VOD diretamente no player leve integrado.
        </p>
      </div>
      <Button variant="primary" size="md" on:click={openSourcesModal}>
        <Plus size={16} /> Adicionar Lista M3U
      </Button>
    </div>
  {:else if isLoading}
    <!-- Loading Skeletons -->
    <div class="grid grid-cols-2 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-6 gap-3 animate-pulse">
      {#each Array(12) as _}
        <div class="flex flex-col gap-2 p-3 bg-surface border border-border rounded-xl">
          <div class="w-full aspect-video rounded-lg bg-surfaceHover"></div>
          <div class="h-3 bg-surfaceHover rounded w-3/4"></div>
          <div class="h-2.5 bg-surfaceHover rounded w-1/2"></div>
        </div>
      {/each}
    </div>
  {:else if activeTab === 'guide'}
    <!-- EPG Guide View -->
    {#if (guideEntries?.length || 0) === 0}
      <div class="text-center py-20 text-muted text-xs bg-surface/30 rounded-xl border border-dashed border-border">
        Nenhum programa no guia EPG para o horário atual. Adicione uma URL de XMLTV à sua fonte IPTV e sincronize.
      </div>
    {:else}
      <div class="flex flex-col gap-3">
        {#each (guideEntries || []) as entry (entry.channel_id + entry.program.title)}
          <div class="flex items-start justify-between p-4 bg-surface border border-border rounded-xl hover:border-primary/40 transition-colors">
            <div class="flex items-start gap-3.5 min-w-0">
              <div class="w-10 h-10 rounded-lg bg-surfaceHover flex items-center justify-center text-primary font-bold text-sm shrink-0 overflow-hidden">
                {#if entry.logo_url}
                  <img 
                    src={entry.logo_url} 
                    alt="" 
                    class="w-full h-full object-contain" 
                    loading="lazy"
                    on:error={handleImageError}
                  />
                {:else}
                  <Tv size={18} />
                {/if}
              </div>
              <div class="min-w-0">
                <span class="text-[11px] font-semibold text-primary uppercase tracking-wider">{entry.channel_name}</span>
                <h4 class="text-sm font-bold text-foreground truncate mt-0.5">{entry.program.title}</h4>
                {#if entry.program.description}
                  <p class="text-xs text-muted line-clamp-2 mt-1">{entry.program.description}</p>
                {/if}
              </div>
            </div>
            <div class="flex flex-col items-end gap-1 shrink-0 ml-4">
              <span class="text-xs font-mono text-muted flex items-center gap-1">
                <Clock size={12} />
                {formatTime(entry.program.start)} - {formatTime(entry.program.end)}
              </span>
              <span class="text-[10px] px-2 py-0.5 rounded bg-green-500/10 text-green-400 font-semibold">
                No Ar
              </span>
            </div>
          </div>
        {/each}
      </div>
    {/if}
  {:else if activeTab === 'resume'}
    <!-- Resume Watching VOD -->
    {#if (resumeEntries?.length || 0) === 0}
      <div class="text-center py-20 text-muted text-xs bg-surface/30 rounded-xl border border-dashed border-border">
        Nenhum conteúdo em andamento. Inicie um filme ou episódio VOD para continuar assistindo de onde parou.
      </div>
    {:else}
      <div class="grid grid-cols-1 sm:grid-cols-2 md:grid-cols-3 lg:grid-cols-4 gap-4">
        {#each (resumeEntries || []) as entry (entry.item.id)}
          <button
            type="button"
            class="flex flex-col p-3 bg-surface border border-border rounded-xl hover:border-primary/50 cursor-pointer transition-all tv-focusable group text-left"
            on:click={() => handlePlay(entry.item, entry.position)}
          >
            <div class="relative w-full aspect-video rounded-lg bg-surfaceHover overflow-hidden flex items-center justify-center">
              {#if entry.item.logo_url}
                <img 
                  src={entry.item.logo_url} 
                  alt="" 
                  class="w-full h-full object-cover group-hover:scale-105 transition-transform" 
                  loading="lazy"
                  on:error={handleImageError}
                />
              {:else}
                <Film size={28} class="text-muted group-hover:text-primary transition-colors" />
              {/if}
              <div class="absolute inset-0 bg-black/40 opacity-0 group-hover:opacity-100 flex items-center justify-center transition-opacity">
                <Play size={28} fill="white" class="text-white drop-shadow" />
              </div>
              <!-- Progress bar -->
              <div class="absolute bottom-0 inset-x-0 h-1 bg-black/50">
                <div 
                  class="h-full bg-primary" 
                  style="width: {entry.duration > 0 ? (entry.position / entry.duration) * 100 : 0}%"
                ></div>
              </div>
            </div>
            <div class="mt-2.5 flex flex-col">
              <h4 class="text-xs font-semibold text-foreground truncate group-hover:text-primary transition-colors">
                {entry.item.title}
              </h4>
              <span class="text-[10px] text-muted truncate">{entry.item.group || entry.item.source_name || 'VOD'}</span>
            </div>
          </button>
        {/each}
      </div>
    {/if}
  {:else}
    <!-- Channels & Media Grid -->
    {#if (!pageResult?.items || pageResult.items.length === 0)}
      <div class="text-center py-20 text-muted text-xs bg-surface/30 rounded-xl border border-dashed border-border">
        {#if searchQuery}
          Nenhum resultado encontrado para "{searchQuery}".
        {:else}
          Nenhum item encontrado nesta categoria.
        {/if}
      </div>
    {:else}
      <div class="grid grid-cols-2 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-6 gap-3">
        {#each pageResult.items as item (item.id)}
          <div
            class="group relative flex flex-col p-3 bg-surface border border-border rounded-xl hover:border-primary/50 transition-all tv-focusable cursor-pointer select-none"
            role="button"
            tabindex="0"
            on:click={() => handlePlay(item)}
            on:keydown={(e) => e.key === 'Enter' && handlePlay(item)}
          >
            <!-- Thumbnail / Logo Box -->
            <div class="relative w-full aspect-video rounded-lg bg-surfaceHover overflow-hidden flex items-center justify-center">
              {#if item.logo_url}
                <img 
                  src={item.logo_url} 
                  alt="" 
                  class="w-full h-full object-contain p-2 group-hover:scale-105 transition-transform" 
                  loading="lazy" 
                  on:error={handleImageError}
                />
              {:else if item.kind === 'movie'}
                <Film size={24} class="text-muted group-hover:text-primary transition-colors" />
              {:else if item.kind === 'series'}
                <Clapperboard size={24} class="text-muted group-hover:text-primary transition-colors" />
              {:else}
                <Tv size={24} class="text-muted group-hover:text-primary transition-colors" />
              {/if}

              <!-- Channel Number Badge -->
              {#if item.channel_number}
                <span class="absolute top-1.5 left-1.5 text-[9px] font-bold px-1.5 py-0.5 rounded bg-black/70 text-white backdrop-blur-xs font-mono">
                  {item.channel_number}
                </span>
              {/if}

              <!-- Star / Favorite Toggle -->
              <button
                type="button"
                on:click={(e) => handleToggleSave(item, e)}
                class="absolute top-1.5 right-1.5 p-1 rounded-md bg-black/60 hover:bg-black/80 text-white/80 hover:text-amber-400 opacity-0 group-hover:opacity-100 transition-opacity z-10"
                title="Favoritar canal"
              >
                <Star size={12} fill={savedItemIds.has(item.id) ? 'currentColor' : 'none'} class={savedItemIds.has(item.id) ? 'text-amber-400' : ''} />
              </button>

              <!-- Play Overlay on Hover -->
              <div class="absolute inset-0 bg-primary/20 opacity-0 group-hover:opacity-100 flex items-center justify-center transition-opacity">
                <div class="w-8 h-8 rounded-full bg-primary text-white flex items-center justify-center shadow-lg transform scale-90 group-hover:scale-100 transition-transform">
                  <Play size={14} fill="currentColor" class="ml-0.5" />
                </div>
              </div>
            </div>

            <!-- Metadata -->
            <div class="mt-2.5 flex flex-col">
              <h4 class="text-xs font-semibold text-foreground truncate group-hover:text-primary transition-colors" title={item.title}>
                {item.title}
              </h4>
              {#if item.group}
                <span class="text-[10px] text-muted truncate mt-0.5">{item.group}</span>
              {/if}
            </div>
          </div>
        {/each}
      </div>

      <!-- Pagination Footer -->
      {#if pageResult.total_pages > 1}
        <div class="flex items-center justify-between pt-4 border-t border-border mt-2">
          <span class="text-xs text-muted font-medium">
            Mostrando página {pageResult.page} de {pageResult.total_pages} ({pageResult.total_count} itens)
          </span>
          <div class="flex items-center gap-2">
            <Button
              variant="secondary"
              size="sm"
              disabled={pageResult.page <= 1}
              on:click={() => loadCatalog(pageResult.page - 1)}
            >
              Anterior
            </Button>
            <Button
              variant="secondary"
              size="sm"
              disabled={pageResult.page >= pageResult.total_pages}
              on:click={() => loadCatalog(pageResult.page + 1)}
            >
              Próxima
            </Button>
          </div>
        </div>
      {/if}
    {/if}
  {/if}
</div>

<!-- Manage Sources Modal -->
<Modal title="Gerenciar Fontes IPTV" bind:open={showSourcesModal} on:close={() => (showSourcesModal = false)}>
  <div class="flex flex-col gap-6">
    <!-- Existing Sources List -->
    {#if (sources?.length || 0) > 0}
      <div class="flex flex-col gap-2">
        <h4 class="text-xs font-bold uppercase tracking-wider text-muted">Fontes Ativas ({sources?.length || 0})</h4>
        <div class="flex flex-col gap-2">
          {#each (sources || []) as src (src.config.id)}
            <div class="flex items-center justify-between p-3 bg-surfaceHover/50 border border-border rounded-xl">
              <div class="flex flex-col min-w-0 pr-2">
                <span class="text-xs font-bold text-foreground truncate">{src.config.name}</span>
                <span class="text-[10px] text-muted truncate mt-0.5">{src.config.playlist_url || 'URL segura'}</span>
                {#if src.last_error}
                  <span class="text-[10px] text-red-400 mt-0.5 truncate">{src.last_error}</span>
                {/if}
              </div>
              <div class="flex items-center gap-1 shrink-0">
                <button
                  type="button"
                  on:click={() => beginEditingSource(src)}
                  class="p-1.5 text-muted hover:text-foreground rounded-lg hover:bg-surface"
                  title="Editar fonte"
                  aria-label={`Editar fonte ${src.config.name}`}
                >
                  <Pencil size={14} />
                </button>
                <button
                  type="button"
                  on:click={() => handleSyncSource(src.config.id)}
                  disabled={isSyncingSource[src.config.id]}
                  class="p-1.5 text-muted hover:text-foreground rounded-lg hover:bg-surface disabled:opacity-50"
                  title="Sincronizar lista agora"
                >
                  <RefreshCw size={14} class={isSyncingSource[src.config.id] ? 'animate-spin text-primary' : ''} />
                </button>
                <button
                  type="button"
                  on:click={() => handleDeleteSource(src.config.id)}
                  class="p-1.5 text-muted hover:text-red-400 rounded-lg hover:bg-surface"
                  title="Excluir fonte"
                >
                  <Trash2 size={14} />
                </button>
              </div>
            </div>
          {/each}
        </div>
      </div>
    {/if}

    <!-- Create / Edit Source Form -->
    <form on:submit|preventDefault={saveSourceConfiguration} class="flex flex-col gap-4 pt-2 border-t border-border">
      <div class="flex items-center justify-between gap-3">
        <h4 class="text-xs font-bold uppercase tracking-wider text-muted">
          {editingSourceId ? 'Editar Lista M3U / M3U+' : 'Adicionar Nova Lista M3U / M3U+'}
        </h4>
        {#if editingSourceId}
          <Button variant="ghost" size="sm" on:click={resetSourceForm}>Cancelar edição</Button>
        {/if}
      </div>

      <div class="flex flex-col gap-1.5">
        <label for="iptv-source-name" class="text-xs font-medium text-foreground">Nome da lista</label>
        <input
          id="iptv-source-name"
          type="text"
          bind:value={sourceName}
          maxlength="120"
          autocomplete="off"
          placeholder="Ex.: Canais Brasil e Filmes"
          class="w-full px-3.5 py-2 text-sm bg-surface border border-border rounded-lg text-foreground placeholder:text-muted focus:outline-none focus:border-primary tv-focusable"
        />
      </div>

      <div class="flex flex-col gap-1.5">
        <label for="iptv-source-url" class="text-xs font-medium text-foreground">URL da lista M3U</label>
        <div class="flex flex-col sm:flex-row gap-2">
          <input
            id="iptv-source-url"
            type="url"
            bind:value={sourceUrl}
            on:input={() => (endpointDiagnostic = null)}
            autocomplete="url"
            spellcheck="false"
            placeholder="https://provedor.example/get.php?..."
            class="flex-1 min-w-0 px-3.5 py-2 text-sm bg-surface border border-border rounded-lg text-foreground placeholder:text-muted focus:outline-none focus:border-primary tv-focusable"
          />
          <Button variant="secondary" size="sm" disabled={isDiagnosingEndpoint} on:click={diagnoseSourceEndpoint}>
            <Network size={14} class={isDiagnosingEndpoint ? 'animate-pulse' : ''} />
            {isDiagnosingEndpoint ? 'Verificando' : 'Diagnosticar'}
          </Button>
        </div>
        <p class="text-[10px] text-muted">Use a URL completa informada pelo provedor. O aplicativo não troca o domínio por aliases.</p>
      </div>

      {#if endpointDiagnostic}
        <section
          aria-label="Resultado do diagnóstico da fonte IPTV"
          class="rounded-xl border p-3 text-[11px] {endpointDiagnostic.tcp_reachable ? 'border-green-500/30 bg-green-500/5' : 'border-amber-500/30 bg-amber-500/5'}"
        >
          <div class="flex items-center gap-2 font-semibold text-foreground">
            {#if endpointDiagnostic.tcp_reachable}<Check size={14} class="text-green-400" />{:else}<AlertTriangle size={14} class="text-amber-400" />{/if}
            {endpointDiagnostic.tcp_reachable ? 'Destino público acessível' : 'Diagnóstico incompleto ou com falha'}
          </div>
          {#if endpointDiagnostic.base_url}<p class="mt-1 text-muted">Base: {endpointDiagnostic.base_url}</p>{/if}
          {#if endpointDiagnostic.resolved_addresses.length > 0}
            <p class="mt-1 text-muted">DNS: {endpointDiagnostic.resolved_addresses.join(', ')}</p>
          {/if}
          {#if endpointDiagnostic.credential_hint}
            <p class="mt-1 text-amber-300">A URL contém credenciais; elas serão removidas da URL e enviadas ao keyring.</p>
          {/if}
          {#if endpointDiagnostic.error}<p class="mt-1 text-amber-300">{endpointDiagnostic.error}</p>{/if}
          <p class="mt-1 text-muted">Este teste não autentica nem valida o conteúdo M3U; isso ocorre ao sincronizar.</p>
        </section>
      {/if}

      <div class="flex flex-col gap-1.5">
        <label for="iptv-guide-url" class="text-xs font-medium text-foreground">URL do guia EPG (opcional)</label>
        <input
          id="iptv-guide-url"
          type="url"
          bind:value={sourceGuideUrl}
          autocomplete="url"
          spellcheck="false"
          placeholder="https://provedor.example/epg.xml"
          class="w-full px-3.5 py-2 text-sm bg-surface border border-border rounded-lg text-foreground placeholder:text-muted focus:outline-none focus:border-primary tv-focusable"
        />
      </div>

      <fieldset class="grid grid-cols-1 sm:grid-cols-2 gap-3">
        <legend class="sr-only">Credenciais da lista IPTV</legend>
        <div class="flex flex-col gap-1.5">
          <label for="iptv-source-username" class="text-xs font-medium text-foreground">Usuário do provedor</label>
          <input
            id="iptv-source-username"
            type="text"
            bind:value={sourceUsername}
            disabled={clearSavedCredentials}
            autocomplete="username"
            placeholder={hasSavedCredentials ? 'Deixe vazio para manter' : 'Opcional'}
            class="w-full px-3.5 py-2 text-sm bg-surface border border-border rounded-lg text-foreground placeholder:text-muted focus:outline-none focus:border-primary disabled:opacity-50 tv-focusable"
          />
        </div>
        <div class="flex flex-col gap-1.5">
          <label for="iptv-source-password" class="text-xs font-medium text-foreground">Senha do provedor</label>
          <input
            id="iptv-source-password"
            type="password"
            bind:value={sourcePassword}
            disabled={clearSavedCredentials}
            autocomplete="current-password"
            placeholder={hasSavedCredentials ? 'Deixe vazio para manter' : 'Opcional'}
            class="w-full px-3.5 py-2 text-sm bg-surface border border-border rounded-lg text-foreground placeholder:text-muted focus:outline-none focus:border-primary disabled:opacity-50 tv-focusable"
          />
        </div>
      </fieldset>
      <p class="-mt-2 text-[10px] text-muted">Usuário e senha são armazenados no keyring do sistema, nunca no SQLite.</p>

      {#if editingSourceId && hasSavedCredentials}
        <label class="flex items-center gap-2 text-xs text-muted cursor-pointer">
          <input type="checkbox" bind:checked={clearSavedCredentials} class="accent-primary tv-focusable" />
          Remover as credenciais salvas ao atualizar
        </label>
      {/if}

      <div class="flex flex-col gap-1.5">
        <label for="iptv-output-mode" class="text-xs font-medium text-foreground">Compatibilidade do player web</label>
        <select
          id="iptv-output-mode"
          bind:value={sourceOutputMode}
          class="w-full px-3.5 py-2 text-sm bg-surface border border-border rounded-lg text-foreground focus:outline-none focus:border-primary tv-focusable"
        >
          <option value="hls">Preferir HLS/M3U8 (recomendado)</option>
          <option value="preserve">Preservar parâmetro output da URL</option>
        </select>
        <p class="text-[10px] text-muted">HLS troca apenas o parâmetro <code>output</code> para <code>m3u8</code>. URLs MPEG-TS podem não tocar no WebView.</p>
      </div>

      <label class="flex items-center gap-2 text-xs text-muted cursor-pointer">
        <input type="checkbox" bind:checked={sourceEnabled} class="accent-primary tv-focusable" />
        Fonte ativa
      </label>

      <div class="flex justify-end gap-2 mt-1">
        <Button variant="ghost" size="sm" on:click={() => { resetSourceForm(); showSourcesModal = false; }}>Fechar</Button>
        <Button variant="primary" size="sm" type="submit" disabled={isSavingSource || isDiagnosingEndpoint}>
          {#if editingSourceId}<Pencil size={14} />{:else}<Plus size={14} />{/if}
          {isSavingSource ? 'Salvando...' : editingSourceId ? 'Atualizar & Sincronizar' : 'Salvar & Sincronizar'}
        </Button>
      </div>
    </form>
  </div>
</Modal>
