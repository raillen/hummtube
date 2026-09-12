<script lang="ts">
  import { onMount } from 'svelte';
  import { CatalogService, SettingsService } from '../../wailsjs/services';
  import type { HomeModel } from '../../types';
  import VideoGrid from '../video/VideoGrid.svelte';
  import Chip from '../ui/Chip.svelte';
  import Button from '../ui/Button.svelte';
  import { activeTab, toast } from '../../stores/uiStores';
  import { submitGlobalSearch } from '../../stores/searchQueryStore';
  import { displayPreferences } from '../../stores/displayPreferences';
  import Sparkles from 'lucide-svelte/icons/sparkles';
  import Compass from 'lucide-svelte/icons/compass';
  import Clock from 'lucide-svelte/icons/clock';
  import Flame from 'lucide-svelte/icons/flame';
  import RefreshCw from 'lucide-svelte/icons/refresh-cw';
  import Search from 'lucide-svelte/icons/search';
  import Users from 'lucide-svelte/icons/users';
  import Info from 'lucide-svelte/icons/info';
  import HelpCircle from 'lucide-svelte/icons/circle-help';
  import ArrowRight from 'lucide-svelte/icons/arrow-right';
  import AlertTriangle from 'lucide-svelte/icons/triangle-alert';

  let homeData: HomeModel | null = null;
  let activeFilter: 'all' | 'unwatched' | 'favorites' | 'long' = 'all';
  let isLoading = true;
  let isRefreshing = false;
  let loadError: string | null = null;
  let showAlgoInfo = false;
  let visibleCount = 24;
  let appliedHomePageSize = 0;
  let hasExpandedHome = false;
  let visibleTopicCount = 4;
  let appliedTopicPageSize = 0;
  type UploadPeriod = 'today' | 'week' | 'month' | 'year';
  let uploadPeriods: UploadPeriod[] = [];
  const uploadPeriodOptions: Array<{ id: UploadPeriod; label: string }> = [
    { id: 'today', label: 'Hoje' },
    { id: 'week', label: '7 dias' },
    { id: 'month', label: '30 dias' },
    { id: 'year', label: '1 ano' },
  ];
  let preferencesReady = false;
  let savedFilterSignature = '';

  $: filteredRecommendations = (homeData?.for_you || []).filter((video) => {
    if (uploadPeriods.length === 0 || !video.published_at) return true;
    const age = Date.now() - new Date(video.published_at).getTime();
    const limits = { today: 86400000, week: 7 * 86400000, month: 30 * 86400000, year: 365 * 86400000 };
    return uploadPeriods.some((period) => age <= limits[period]);
  });

  $: if (!hasExpandedHome && $displayPreferences.homePageSize !== appliedHomePageSize) {
    appliedHomePageSize = $displayPreferences.homePageSize;
    visibleCount = $displayPreferences.homePageSize;
  }

  $: if (appliedTopicPageSize !== $displayPreferences.homePageSize) {
    appliedTopicPageSize = $displayPreferences.homePageSize;
    visibleTopicCount = Math.min(4, Math.max(1, Math.floor($displayPreferences.homePageSize / 6)));
  }

  $: visibleTopicSections = (homeData?.topic_sections || []).slice(0, visibleTopicCount);
  $: hasMoreTopicSections = (homeData?.topic_sections || []).length > visibleTopicCount;

  const quickTopics = [
    'Tecnologia',
    'Música & Lo-Fi',
    'Ciência & Espaço',
    'Podcasts',
    'Educação',
    'Jogos & Gameplay'
  ];

  let homeLoadGeneration = 0;

  async function loadHome() {
    const generation = ++homeLoadGeneration;
    isLoading = homeData === null;
    loadError = null;
    try {
      const result = await CatalogService.getHome();
      if (generation !== homeLoadGeneration) return;
      homeData = result;
    } catch (e: unknown) {
      if (generation !== homeLoadGeneration) return;
      loadError = e instanceof Error ? e.message : 'Falha ao carregar o feed inicial.';
    } finally {
      if (generation === homeLoadGeneration) isLoading = false;
    }
  }

  async function handleRefresh() {
    isRefreshing = true;
    try {
      await CatalogService.refreshSubscriptions();
      toast.add('Catálogo atualizado!', 'success');
    } catch (e: any) {
      toast.add('Erro ao atualizar catálogo: ' + (e?.message || 'rede indisponível'), 'error');
    } finally {
      isRefreshing = false;
    }
  }

  function handleTopicClick(topic: string) {
    submitGlobalSearch(topic, { resource_types: ['video'] });
    $activeTab = 'search';
  }

  function loadMoreHome(): void {
    hasExpandedHome = true;
    visibleCount += $displayPreferences.homePageSize;
  }

  function loadMoreTopicSections(): void {
    visibleTopicCount += Math.max(1, Math.floor($displayPreferences.homePageSize / 6));
  }

  function toggleUploadPeriod(period: UploadPeriod): void {
    uploadPeriods = uploadPeriods.includes(period) ? uploadPeriods.filter((value) => value !== period) : [...uploadPeriods, period];
  }

  function homeFilterSignature(): string {
    return JSON.stringify({ active_filter: activeFilter, upload_periods: uploadPeriods });
  }

  onMount(() => {
    let mounted = true;
    window.addEventListener('nanotube-catalog-refreshed', loadHome);
    window.addEventListener('nanotube-profile-changed', loadHome);
    void loadHome();
    void SettingsService.getSettings().then((settings) => {
      if (!mounted) return;
      void CatalogService.refreshOnStartup(settings).catch((error: unknown) => {
        if (mounted) toast.add(error instanceof Error ? error.message : 'Falha ao atualizar catálogo na inicialização.', 'error');
      });
      if (!settings.home_filters) return;
      try {
        const saved = JSON.parse(settings.home_filters) as { active_filter?: typeof activeFilter; upload_periods?: UploadPeriod[] };
        if (saved.active_filter && ['all', 'unwatched', 'favorites', 'long'].includes(saved.active_filter)) activeFilter = saved.active_filter;
        uploadPeriods = (saved.upload_periods || []).filter((value) => uploadPeriodOptions.some((option) => option.id === value));
      } catch { /* preferência anterior incompatível */ }
    }).catch(() => undefined).finally(() => {
      if (!mounted) return;
      preferencesReady = true;
      savedFilterSignature = homeFilterSignature();
    });
    return () => {
      mounted = false;
      homeLoadGeneration++;
      window.removeEventListener('nanotube-catalog-refreshed', loadHome);
      window.removeEventListener('nanotube-profile-changed', loadHome);
    };
  });

  $: currentHomeFilterSignature = homeFilterSignature();
  $: if (preferencesReady && currentHomeFilterSignature !== savedFilterSignature) {
    savedFilterSignature = currentHomeFilterSignature;
    void SettingsService.saveSetting('home_filters', currentHomeFilterSignature).catch(() => undefined);
  }
</script>

<div class="flex flex-col gap-6 p-6 max-w-7xl mx-auto w-full" role="region" aria-label="Feed de Recomendações e Inscrições">
  <!-- Top Filter Bar -->
  <div class="flex flex-wrap items-center justify-between gap-4">
    <div class="flex items-center gap-2 overflow-x-auto pb-1 shrink-0">
      <Chip active={activeFilter === 'all'} on:click={() => (activeFilter = 'all')}>
        <Sparkles size={14} class="mr-1.5 inline text-amber-400" /> Para Você
      </Chip>
      <Chip active={activeFilter === 'unwatched'} on:click={() => (activeFilter = 'unwatched')}>
        <Clock size={14} class="mr-1.5 inline" /> Não Assistidos
      </Chip>
      <Chip active={activeFilter === 'favorites'} on:click={() => (activeFilter = 'favorites')}>
        <Flame size={14} class="mr-1.5 inline text-red-400" /> Favoritos
      </Chip>
      <Chip active={activeFilter === 'long'} on:click={() => (activeFilter = 'long')}>
        <Compass size={14} class="mr-1.5 inline" /> Vídeos Longos
      </Chip>
    </div>

    <div class="flex items-center gap-2">
      <button
        type="button"
        on:click={() => (showAlgoInfo = !showAlgoInfo)}
        class="flex items-center gap-1.5 px-3 py-1.5 text-xs text-muted hover:text-foreground bg-surface border border-border rounded-lg transition-colors tv-focusable"
        title="Entenda como funcionam as recomendações locais"
        aria-expanded={showAlgoInfo}
      >
        <Info size={13} class="text-primary" />
        <span class="hidden sm:inline">Como Funciona o Feed</span>
      </button>

      <Button variant="secondary" size="sm" on:click={handleRefresh} disabled={isRefreshing}>
        <RefreshCw size={13} class={isRefreshing ? 'animate-spin' : ''} />
        Atualizar
      </Button>
    </div>
  </div>
  <fieldset class="flex flex-wrap items-center gap-2 rounded-xl border border-border bg-surface/50 p-3">
    <legend class="px-1 text-xs font-semibold text-foreground">Período de upload das recomendações</legend>
    {#each uploadPeriodOptions as period (period.id)}<button type="button" aria-pressed={uploadPeriods.includes(period.id)} on:click={() => toggleUploadPeriod(period.id)} class="rounded-full border px-3 py-1.5 text-xs tv-focusable {uploadPeriods.includes(period.id) ? 'border-primary bg-primary/10 text-primary' : 'border-border text-muted hover:text-foreground'}">{period.label}</button>{/each}
    {#if uploadPeriods.length === 0}<span class="text-[10px] text-muted">Todos os períodos</span>{/if}
  </fieldset>

  <!-- Algorithmic Transparency Box (Local Explainability) -->
  {#if showAlgoInfo}
    <div class="p-4 bg-surface/80 border border-primary/30 rounded-2xl flex flex-col gap-2.5 text-xs text-muted leading-relaxed animate-in fade-in slide-in-from-top-2 duration-150 shadow-lg">
      <div class="flex items-center justify-between text-foreground">
        <div class="flex items-center gap-2">
          <Sparkles size={16} class="text-primary" />
          <strong class="font-bold">Algoritmo de Recomendação Local (MMR + Afinidade)</strong>
        </div>
        <button type="button" on:click={() => (showAlgoInfo = false)} class="text-xs text-muted hover:text-foreground">
          Fechar
        </button>
      </div>
      <p>
        O HummTube prioriza vídeos usando uma fórmula matemática 100% executada no seu hardware:
        <code class="bg-surfaceHover px-1.5 py-0.5 rounded font-mono text-primary text-[11px]">Score = (Afinidade_Canal × 0.45) + (Decaimento_Temporal × 0.35) - (Penalidade_Redundância × 0.20)</code>.
      </p>
      <ul class="list-disc list-inside space-y-1 pl-1">
        <li><strong>Sem Telemetria:</strong> Seus hábitos de reprodução nunca saem do seu SQLite local.</li>
        <li><strong>Diversidade Garantida (MMR):</strong> Evita repetição excessiva do mesmo canal no topo do feed.</li>
        <li><strong>Filtros Transparentes:</strong> Vídeos marcados como não assistidos recebem bônus de relevância.</li>
      </ul>
    </div>
  {/if}

  <!-- Error Banner -->
  {#if loadError}
    <div role="alert" class="p-4 bg-red-500/10 border border-red-500/30 rounded-2xl flex items-center justify-between gap-4 text-xs text-red-300">
      <div class="flex items-center gap-3">
        <AlertTriangle size={18} class="text-red-400 shrink-0" />
        <div>
          <strong class="text-foreground block">Falha ao carregar conteúdo inicial</strong>
          <span>{loadError}</span>
        </div>
      </div>
      <Button variant="secondary" size="sm" on:click={loadHome}>
        Tentar Novamente
      </Button>
    </div>
  {/if}

  {#if isLoading}
    <!-- Skeleton Loaders -->
    <div class="grid grid-cols-1 sm:grid-cols-2 md:grid-cols-3 lg:grid-cols-4 gap-4 animate-pulse">
      {#each Array(8) as _}
        <div class="flex flex-col gap-2 p-3 bg-surface border border-border rounded-xl">
          <div class="w-full aspect-video rounded-lg bg-surfaceHover"></div>
          <div class="h-4 bg-surfaceHover rounded w-3/4"></div>
          <div class="h-3 bg-surfaceHover rounded w-1/2"></div>
        </div>
      {/each}
    </div>
  {:else if homeData}
    {#if activeFilter === 'all'}
      {#if (!homeData.for_you || homeData.for_you.length === 0) && (!homeData.topic_sections || homeData.topic_sections.length === 0)}
        <!-- Welcome / Empty State with Suggestions & Recommendations -->
        <div class="p-8 sm:p-12 border border-border bg-surface/40 rounded-2xl flex flex-col items-center justify-center text-center gap-6 my-4">
          <div class="w-16 h-16 rounded-2xl bg-primary/10 text-primary flex items-center justify-center font-bold text-xl shadow-lg shadow-primary/10">
            <Sparkles size={32} />
          </div>
          
          <div class="max-w-lg">
            <h3 class="text-lg font-bold text-foreground">Seu feed local está vazio</h3>
            <p class="text-xs text-muted mt-2 leading-relaxed">
              O algoritmo de recomendação do HummTube opera 100% no seu dispositivo com base nas suas inscrições locais e hábitos de reprodução (sem telemetria externa ou LLMs).
            </p>
          </div>

          <!-- Quick Action Buttons -->
          <div class="flex flex-wrap items-center justify-center gap-2.5">
            <Button variant="primary" size="sm" on:click={() => ($activeTab = 'channels')}>
              <Users size={14} /> Adicionar Inscrições
            </Button>
            <Button variant="secondary" size="sm" on:click={() => ($activeTab = 'search')}>
              <Search size={14} /> Pesquisar Vídeos
            </Button>
          </div>

          <!-- Suggested Topics Exploration -->
          <div class="flex flex-col items-center gap-2.5 pt-4 border-t border-border/80 w-full max-w-xl">
            <span class="text-xs font-semibold text-muted uppercase tracking-wider">💡 Temas Sugeridos para Explorar</span>
            <div class="flex flex-wrap items-center justify-center gap-2">
              {#each quickTopics as topic}
                <button
                  type="button"
                  on:click={() => handleTopicClick(topic)}
                  class="px-3 py-1.5 rounded-full text-xs font-medium bg-surface hover:bg-surfaceHover border border-border text-foreground hover:border-primary/40 transition-all tv-focusable cursor-pointer"
                >
                  {topic}
                </button>
              {/each}
            </div>
          </div>
        </div>
      {:else}
        {#if homeData.for_you && homeData.for_you.length > 0}
          <VideoGrid videos={filteredRecommendations.slice(0, visibleCount)} title="Para Você" />
          {#if visibleCount < filteredRecommendations.length}
            <div class="flex justify-center pt-2">
              <Button variant="secondary" on:click={loadMoreHome}>Carregar mais</Button>
            </div>
          {/if}
        {/if}

        {#if homeData.topic_sections && homeData.topic_sections.length > 0}
          {#each visibleTopicSections as section (section.id)}
            <div class="pt-4">
              <VideoGrid videos={section.videos} title={section.title} />
            </div>
          {/each}
          {#if hasMoreTopicSections}
            <div class="flex justify-center pt-2">
              <Button variant="secondary" on:click={loadMoreTopicSections}>Carregar mais tópicos</Button>
            </div>
          {/if}
        {/if}
      {/if}
    {:else if activeFilter === 'unwatched'}
      <VideoGrid videos={homeData.unwatched || []} title="Não Assistidos" />
    {:else if activeFilter === 'favorites'}
      <VideoGrid videos={homeData.favorites || []} title="Canais Favoritos" />
    {:else if activeFilter === 'long'}
      <VideoGrid videos={homeData.long_videos || []} title="Vídeos Longos (> 20 min)" />
    {/if}
  {/if}
</div>
