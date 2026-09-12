<script lang="ts">
  import { onDestroy, onMount } from 'svelte';
  import { get } from 'svelte/store';
  import { SearchService, SettingsService } from '../../wailsjs/services';
  import type {
    SearchCaption,
    SearchDefinition,
    SearchDimension,
    SearchDuration,
    SearchEventType,
    SearchHit,
    SearchLicense,
    SearchOrder,
    SearchPage,
    SearchRequest,
    SearchResourceType,
    SearchSafe,
    SearchSavedState,
    SearchTriState,
    SearchVideoType,
    SearchWatchState,
    Video,
  } from '../../types';
  import { activePlaylistId, activeTab } from '../../stores/uiStores';
  import { consumeGlobalSearch, searchRouteRequest, searchViewSession, selectRemotePlaylist } from '../../stores/searchQueryStore';
  import { openChannel } from '../../stores/channelRouteStore';
  import VideoGrid from '../video/VideoGrid.svelte';
  import Button from '../ui/Button.svelte';
  import AlertCircle from 'lucide-svelte/icons/circle-alert';
  import CalendarDays from 'lucide-svelte/icons/calendar-days';
  import ChevronLeft from 'lucide-svelte/icons/chevron-left';
  import ChevronRight from 'lucide-svelte/icons/chevron-right';
  import ListVideo from 'lucide-svelte/icons/list-video';
  import Loader2 from 'lucide-svelte/icons/loader-circle';
  import MoreVertical from 'lucide-svelte/icons/ellipsis-vertical';
  import Search from 'lucide-svelte/icons/search';
  import SlidersHorizontal from 'lucide-svelte/icons/sliders-horizontal';
  import Sparkles from 'lucide-svelte/icons/sparkles';
  import UserRound from 'lucide-svelte/icons/user-round';
  import ContextMenu from '../ui/ContextMenu.svelte';
  import RemotePlaylistSaveModal from '../playlist/RemotePlaylistSaveModal.svelte';

  type PublishedWindow = 'hour' | 'day' | 'week' | 'month' | 'year';
  type SavedSearchFilters = Partial<SearchRequest> & {
    published_windows?: PublishedWindow[];
    selected_resource_types?: SearchResourceType[];
  };

  let query = '';
  let exactPhrase = '';
  let includeTerms = '';
  let excludeTerms = '';
  let resourceTypes: SearchResourceType[] = [];
  let duration: SearchDuration = 'any';
  let minDuration = '';
  let maxDuration = '';
  let shortsOnly = false;
  let regularOnly = false;
  let eventType: SearchEventType = '';
  let publishedWindows: PublishedWindow[] = [];
  let publishedBefore = '';
  let order: SearchOrder = 'relevance';
  let onlySubscribed = false;
  let hideRejected = true;
  let watchState: SearchWatchState = 'any';
  let savedState: SearchSavedState = 'any';
  let channelID = '';
  let caption: SearchCaption = 'any';
  let definition: SearchDefinition = 'any';
  let dimension: SearchDimension = 'any';
  let license: SearchLicense = 'any';
  let embeddable: SearchTriState = 'any';
  let syndicated: SearchTriState = 'any';
  let paidPromotion: SearchTriState = 'any';
  let videoType: SearchVideoType = 'any';
  let categoryID = '';
  let topicID = '';
  let relevanceLanguage = '';
  let regionCode = '';
  let safeSearch: SearchSafe = 'any';
  let location = '';
  let locationRadius = '';
  let page: SearchPage | null = null;
  let isSearching = false;
  let hasSearched = false;
  let searchError: string | null = null;
  let localHits: SearchHit[] = [];
  let localSearchTimer: ReturnType<typeof setTimeout> | null = null;
  let pageTokens: string[] = [''];
  let currentPageIndex = 0;
  let consumedRouteRequestId = 0;
  let searchGeneration = 0;
  let localSearchGeneration = 0;
  let contextResult: Video | null = null;
  let contextMenuX = 0;
  let contextMenuY = 0;
  let contextMenuTrigger: HTMLElement | null = null;
  let playlistToSave: Video | null = null;
  let showPlaylistSave = false;
  let realtimeFilterTimer: ReturnType<typeof setTimeout> | null = null;
  let filtersReady = false;
  let appliedFilterSignature = '';

  $: contextActions = contextResult
    ? [
        { id: 'open', label: contextResult.resource_type === 'playlist' ? 'Abrir playlist' : 'Abrir canal' },
        ...(contextResult.resource_type === 'playlist' ? [{ id: 'save', label: 'Salvar ou adicionar à playlist existente' }] : []),
      ]
    : [];

  const resourceTypeOptions: Array<{ id: SearchResourceType; label: string }> = [
    { id: 'video', label: 'Vídeos' },
    { id: 'channel', label: 'Canais' },
    { id: 'playlist', label: 'Playlists' },
  ];
  const publishedWindowOptions: Array<{ id: PublishedWindow; label: string }> = [
    { id: 'hour', label: 'Última hora' },
    { id: 'day', label: 'Hoje' },
    { id: 'week', label: 'Esta semana' },
    { id: 'month', label: 'Este mês' },
    { id: 'year', label: 'Este ano' },
  ];

  $: results = page?.items || [];
  $: videoResults = results.filter((result) => (result.resource_type || 'video') === 'video');
  $: catalogResults = results.filter((result) => result.resource_type === 'channel' || result.resource_type === 'playlist');
  $: onlyVideos = resourceTypes.length === 1 && resourceTypes[0] === 'video';
  $: if (!onlyVideos && order !== 'relevance' && order !== 'date' && order !== 'title') order = 'relevance';
  $: if (filtersReady && $searchRouteRequest.pending && $searchRouteRequest.requestId > consumedRouteRequestId) {
    consumedRouteRequestId = $searchRouteRequest.requestId;
    consumeGlobalSearch($searchRouteRequest.requestId);
    applyRouteRequest($searchRouteRequest.query, $searchRouteRequest.filters);
  }

  function errorMessage(error: unknown): string {
    return error instanceof Error && error.message ? error.message : 'Falha ao processar pesquisa.';
  }

  function applyRouteRequest(routeQuery: string, filters: Partial<SearchRequest>): void {
    query = routeQuery;
    if (Object.keys(filters).length > 0) applyFilters(filters as SavedSearchFilters);
    pageTokens = [''];
    currentPageIndex = 0;
    void executeSearch('');
  }

  function applyFilters(savedFilters: SavedSearchFilters): void {
    const filters = savedFilters;
    exactPhrase = filters.exact_phrase || '';
    includeTerms = filters.include_terms || '';
    excludeTerms = filters.exclude_terms || '';
    resourceTypes = Array.isArray(savedFilters.selected_resource_types)
      ? savedFilters.selected_resource_types.filter((value) => resourceTypeOptions.some((option) => option.id === value))
      : (filters.resource_types?.length ? [...filters.resource_types] : []);
    duration = filters.duration || 'any';
    minDuration = filters.min_duration ? String(filters.min_duration) : '';
    maxDuration = filters.max_duration ? String(filters.max_duration) : '';
    shortsOnly = Boolean(filters.shorts_only);
    regularOnly = Boolean(filters.regular_only);
    eventType = filters.event_type || '';
    publishedWindows = (savedFilters.published_windows || []).filter((value) => publishedWindowOptions.some((option) => option.id === value));
    order = filters.order || 'relevance';
    publishedBefore = filters.published_before ? filters.published_before.slice(0, 10) : '';
    onlySubscribed = Boolean(filters.only_subscribed);
    hideRejected = filters.hide_rejected !== false;
    watchState = filters.watch_state || 'any';
    savedState = filters.saved_state || 'any';
    channelID = filters.channel_id || '';
    caption = filters.caption || 'any';
    definition = filters.definition || 'any';
    dimension = filters.dimension || 'any';
    license = filters.license || 'any';
    embeddable = filters.embeddable || 'any';
    syndicated = filters.syndicated || 'any';
    paidPromotion = filters.paid_promotion || 'any';
    videoType = filters.video_type || 'any';
    categoryID = filters.category_id || '';
    topicID = filters.topic_id || '';
    relevanceLanguage = filters.relevance_language || '';
    regionCode = filters.region_code || '';
    safeSearch = filters.safe_search || 'any';
    location = filters.location || '';
    locationRadius = filters.location_radius || '';
  }

  function toggleResourceType(resourceType: SearchResourceType): void {
    channelID = '';
    if (resourceTypes.includes(resourceType)) {
      resourceTypes = resourceTypes.filter((current) => current !== resourceType);
      return;
    }
    resourceTypes = [...resourceTypes, resourceType];
    if (resourceType !== 'video' || resourceTypes.length > 1) {
      duration = 'any';
      watchState = 'any';
      savedState = 'any';
      onlySubscribed = false;
    }
  }

  function publishedAfterISO(): string | undefined {
    const milliseconds: Record<PublishedWindow, number> = {
      hour: 60 * 60 * 1000, day: 24 * 60 * 60 * 1000,
      week: 7 * 24 * 60 * 60 * 1000, month: 30 * 24 * 60 * 60 * 1000,
      year: 365 * 24 * 60 * 60 * 1000,
    };
    if (publishedWindows.length === 0) return undefined;
    const widestWindow = Math.max(...publishedWindows.map((window) => milliseconds[window]));
    return new Date(Date.now() - widestWindow).toISOString();
  }

  function togglePublishedWindow(window: PublishedWindow): void {
    publishedWindows = publishedWindows.includes(window)
      ? publishedWindows.filter((value) => value !== window)
      : [...publishedWindows, window];
  }

  function buildSearchRequest(pageToken = ''): SearchRequest {
    return {
      query: query.trim(), exact_phrase: exactPhrase.trim() || undefined,
      include_terms: includeTerms.trim() || undefined, exclude_terms: excludeTerms.trim() || undefined,
      max_results: 24, page_token: pageToken || undefined,
      resource_types: resourceTypes.length ? resourceTypes : ['video', 'channel', 'playlist'],
      order, published_after: publishedAfterISO(), published_before: publishedBefore ? new Date(`${publishedBefore}T23:59:59.999Z`).toISOString() : undefined,
      duration: onlyVideos ? duration : 'any', min_duration: onlyVideos && minDuration ? Number(minDuration) : undefined,
      max_duration: onlyVideos && maxDuration ? Number(maxDuration) : undefined,
      shorts_only: onlyVideos && shortsOnly, regular_only: onlyVideos && regularOnly,
      event_type: onlyVideos ? eventType : '',
      only_subscribed: onlyVideos && onlySubscribed, watch_state: onlyVideos ? watchState : 'any',
      saved_state: onlyVideos ? savedState : 'any', hide_rejected: hideRejected,
      channel_id: channelID || undefined, caption: onlyVideos ? caption : 'any', definition: onlyVideos ? definition : 'any',
      dimension: onlyVideos ? dimension : 'any', license: onlyVideos ? license : 'any', embeddable: onlyVideos ? embeddable : 'any',
      syndicated: onlyVideos ? syndicated : 'any', paid_promotion: onlyVideos ? paidPromotion : 'any', video_type: onlyVideos ? videoType : 'any',
      category_id: onlyVideos ? categoryID.trim() || undefined : undefined, topic_id: onlyVideos ? topicID.trim() || undefined : undefined,
      relevance_language: relevanceLanguage.trim() || undefined, region_code: regionCode.trim().toUpperCase() || undefined,
      safe_search: safeSearch, location: location.trim() || undefined, location_radius: locationRadius.trim() || undefined,
    };
  }

  async function executeSearch(pageToken = ''): Promise<void> {
    if (!query.trim() && !exactPhrase.trim() && !includeTerms.trim()) return;
    const generation = ++searchGeneration;
    isSearching = true;
    hasSearched = true;
    searchError = null;
    localHits = [];
    try {
      const nextPage = await SearchService.search(buildSearchRequest(pageToken));
      if (generation !== searchGeneration) return;
      page = nextPage;
    } catch (error: unknown) {
      if (generation !== searchGeneration) return;
      console.error('Erro na busca:', error);
      searchError = errorMessage(error);
      page = null;
    } finally {
      if (generation === searchGeneration) isSearching = false;
    }
  }

  function submitNewSearch(): void {
    pageTokens = [''];
    currentPageIndex = 0;
    void executeSearch('');
  }

  function persistAndRefreshFilters(): void {
    if (!filtersReady) return;
    const { query: _query, page_token: _pageToken, published_after: _publishedAfter, ...savedFilters } = buildSearchRequest('');
    void SettingsService.saveSetting('search_filters', JSON.stringify({
      ...savedFilters,
      selected_resource_types: resourceTypes,
      published_windows: publishedWindows,
    })).catch(() => undefined);
    if (!hasSearched) return;
    if (realtimeFilterTimer) clearTimeout(realtimeFilterTimer);
    realtimeFilterTimer = setTimeout(submitNewSearch, 280);
  }

  onMount(() => {
    if (consumedRouteRequestId === 0) {
      const session = get(searchViewSession);
      query = session.query;
      page = session.page;
      pageTokens = session.pageTokens.length ? [...session.pageTokens] : [''];
      currentPageIndex = session.currentPageIndex;
      hasSearched = session.hasSearched;
      searchError = session.searchError;
    }
    void SettingsService.getSettings().then((settings) => {
      const saved = settings.search_filters;
      if (saved) {
        try { applyFilters(JSON.parse(saved) as SavedSearchFilters); } catch { /* ignora preferência antiga inválida */ }
      }
    }).finally(() => {
      filtersReady = true;
      appliedFilterSignature = JSON.stringify(buildSearchRequest(''));
    });
  });

  $: filterSignature = JSON.stringify(buildSearchRequest(''));
  $: if (filtersReady && filterSignature !== appliedFilterSignature) {
    appliedFilterSignature = filterSignature;
    persistAndRefreshFilters();
  }

  function loadNextPage(): void {
    const nextPageToken = page?.next_page_token;
    if (!nextPageToken) return;
    pageTokens = [...pageTokens.slice(0, currentPageIndex + 1), nextPageToken];
    currentPageIndex += 1;
    void executeSearch(nextPageToken);
  }

  function loadPreviousPage(): void {
    if (currentPageIndex <= 0) return;
    currentPageIndex -= 1;
    void executeSearch(pageTokens[currentPageIndex] || '');
  }

  function scheduleLocalSearch(): void {
    const generation = ++localSearchGeneration;
    if (localSearchTimer) clearTimeout(localSearchTimer);
    if (realtimeFilterTimer) clearTimeout(realtimeFilterTimer);
    const localQuery = query.trim();
    channelID = '';
    if (localQuery.length < 2) {
      localHits = [];
      return;
    }
    localSearchTimer = setTimeout(async () => {
      try {
        const hits = (await SearchService.searchLocalFTS(localQuery, 6)) || [];
        if (generation === localSearchGeneration) localHits = hits;
      } catch {
        if (generation === localSearchGeneration) localHits = [];
      }
    }, 180);
  }

  function chooseLocalHit(hit: SearchHit): void {
    query = hit.title;
    localHits = [];
    submitNewSearch();
  }

  function openCatalogResult(result: Video): void {
    if (result.resource_type === 'playlist') {
      const routeId = selectRemotePlaylist(result);
      if (!routeId) return;
      $activePlaylistId = routeId;
      $activeTab = 'playlists';
      return;
    }
    if (result.resource_type === 'channel') {
      openChannel({ id: result.id, title: result.title, thumbnailUrl: result.thumbnail_url, initialSection: 'videos' });
    }
  }

  function openResultContext(event: MouseEvent | KeyboardEvent, result: Video): void {
    event.preventDefault();
    contextResult = result;
    contextMenuTrigger = event instanceof KeyboardEvent
      ? document.activeElement as HTMLElement
      : event.currentTarget as HTMLElement;
    if (event instanceof MouseEvent && event.type === 'contextmenu') {
      contextMenuX = event.clientX;
      contextMenuY = event.clientY;
    } else {
      const bounds = contextMenuTrigger.getBoundingClientRect();
      contextMenuX = bounds.left + 24;
      contextMenuY = bounds.top + 24;
    }
  }

  function registerResultContextMenu(node: HTMLElement, result: Video): { destroy: () => void } {
    const handleContextMenu = (event: MouseEvent) => openResultContext(event, result);
    const handleKeyDown = (event: KeyboardEvent) => {
      if (event.key === 'ContextMenu' || event.key === 'Apps' || (event.key === 'F10' && event.shiftKey)) openResultContext(event, result);
    };
    node.addEventListener('contextmenu', handleContextMenu);
    node.addEventListener('keydown', handleKeyDown);
    return {
      destroy: () => {
        node.removeEventListener('contextmenu', handleContextMenu);
        node.removeEventListener('keydown', handleKeyDown);
      },
    };
  }

  function handleContextAction(event: CustomEvent<string>): void {
    const result = contextResult;
    contextResult = null;
    if (!result) return;
    if (event.detail === 'save' && result.resource_type === 'playlist') {
      playlistToSave = result;
      showPlaylistSave = true;
      return;
    }
    openCatalogResult(result);
  }

  onDestroy(() => {
    if (localSearchTimer) clearTimeout(localSearchTimer);
    if (realtimeFilterTimer) clearTimeout(realtimeFilterTimer);
    searchViewSession.set({ query, page, pageTokens: [...pageTokens], currentPageIndex, hasSearched, searchError });
  });
</script>

<section class="flex flex-col gap-6 p-6 max-w-6xl mx-auto w-full" aria-labelledby="search-view-title">
  <header class="flex flex-col gap-1">
    <h1 id="search-view-title" class="text-xl font-bold text-foreground">Pesquisa avançada</h1>
    <p class="text-xs text-muted">Sugestões locais via FTS5; busca remota somente ao enviar. A relevância padrão usa NanoRank local.</p>
  </header>

  <form on:submit|preventDefault={submitNewSearch} class="flex flex-col gap-3" role="search" aria-label="Pesquisa no catálogo e YouTube">
    <div class="relative flex gap-2">
      <div class="relative flex-1">
        <Search class="absolute left-3.5 top-3 text-muted pointer-events-none" size={18} />
        <input id="advanced-search-query" type="search" bind:value={query} on:input={scheduleLocalSearch}
          placeholder="Vídeos, canais ou playlists..." aria-label="Termo de pesquisa" autocomplete="off"
          class="w-full bg-surface border border-border rounded-xl pl-11 pr-4 py-2.5 text-sm text-foreground placeholder:text-muted focus:outline-none focus:border-primary tv-focusable" />
        {#if localHits.length > 0 && !isSearching}
          <section aria-label="Resultados instantâneos do catálogo local" class="absolute z-20 left-0 right-0 top-full mt-1 rounded-xl border border-border bg-surface p-1 shadow-xl">
            {#each localHits as hit (hit.video_id)}
              <button type="button" on:click={() => chooseLocalHit(hit)} class="flex w-full flex-col rounded-lg px-3 py-2 text-left hover:bg-surfaceHover tv-focusable">
                <span class="text-xs font-semibold text-foreground">{hit.title}</span>
                {#if hit.channel_name}<span class="text-[10px] text-muted">{hit.channel_name}</span>{/if}
              </button>
            {/each}
          </section>
        {/if}
      </div>
      <Button type="submit" variant="primary" disabled={isSearching || (!query.trim() && !exactPhrase.trim() && !includeTerms.trim())}>
        {#if isSearching}<Loader2 size={16} class="animate-spin" />{:else}<Search size={16} />{/if} Buscar
      </Button>
    </div>

    <fieldset class="flex flex-wrap items-center gap-2">
      <legend class="sr-only">Tipos de resultado</legend>
      {#each resourceTypeOptions as option (option.id)}
        <button type="button" aria-pressed={resourceTypes.includes(option.id)} on:click={() => toggleResourceType(option.id)}
          class="rounded-full border px-3 py-1.5 text-xs font-medium tv-focusable {resourceTypes.includes(option.id) ? 'border-primary bg-primary/10 text-primary' : 'border-border bg-surface text-muted hover:text-foreground'}">{option.label}</button>
      {/each}
      {#if resourceTypes.length === 0}<span class="text-[10px] text-muted">Nenhum tipo selecionado: buscar tudo.</span>{/if}
    </fieldset>

    <details class="rounded-xl border border-border bg-surface/50 p-3">
      <summary class="flex cursor-pointer items-center gap-2 text-xs font-semibold text-foreground tv-focusable"><SlidersHorizontal size={15} class="text-primary" /> Filtros combináveis</summary>
      <div class="mt-4 grid grid-cols-1 gap-3 md:grid-cols-3">
        <label class="flex flex-col gap-1 text-xs text-muted">Frase exata<input bind:value={exactPhrase} class="rounded-lg border border-border bg-surface px-3 py-2 text-foreground focus:border-primary focus:outline-none" /></label>
        <label class="flex flex-col gap-1 text-xs text-muted">Incluir termos<input bind:value={includeTerms} class="rounded-lg border border-border bg-surface px-3 py-2 text-foreground focus:border-primary focus:outline-none" /></label>
        <label class="flex flex-col gap-1 text-xs text-muted">Excluir termos<input bind:value={excludeTerms} class="rounded-lg border border-border bg-surface px-3 py-2 text-foreground focus:border-primary focus:outline-none" /></label>
        <fieldset class="flex flex-col gap-1 text-xs text-muted">
          <legend>Publicação (combine)</legend>
          <div class="flex flex-wrap gap-1.5">
            {#each publishedWindowOptions as window (window.id)}
              <button type="button" aria-pressed={publishedWindows.includes(window.id)} on:click={() => togglePublishedWindow(window.id)} class="rounded-full border px-2.5 py-1.5 tv-focusable {publishedWindows.includes(window.id) ? 'border-primary bg-primary/10 text-primary' : 'border-border bg-surface text-muted hover:text-foreground'}">{window.label}</button>
            {/each}
          </div>
          {#if publishedWindows.length === 0}<span class="text-[10px]">Todas as datas</span>{/if}
        </fieldset>
        <label class="flex flex-col gap-1 text-xs text-muted">Ordenação<select bind:value={order} class="rounded-lg border border-border bg-surface px-3 py-2 text-foreground focus:border-primary focus:outline-none">
            <option value="relevance">NanoRank / relevância</option><option value="date">Mais recentes</option><option value="viewCount" disabled={!onlyVideos}>Mais vistos</option><option value="rating" disabled={!onlyVideos}>Avaliação</option><option value="title">Título</option>
        </select></label>
        <label class="flex flex-col gap-1 text-xs text-muted">Publicado antes de<input type="date" bind:value={publishedBefore} class="rounded-lg border border-border bg-surface px-3 py-2 text-foreground focus:border-primary focus:outline-none" /></label>
        {#if onlyVideos}
          <label class="flex flex-col gap-1 text-xs text-muted">Duração<select bind:value={duration} class="rounded-lg border border-border bg-surface px-3 py-2 text-foreground focus:border-primary focus:outline-none">
            <option value="any">Qualquer</option><option value="short">Curto (&lt; 4 min)</option><option value="medium">Médio (4–20 min)</option><option value="long">Longo (&gt; 20 min)</option>
          </select></label>
          <label class="flex flex-col gap-1 text-xs text-muted">Duração mínima (s)<input type="number" min="0" max="86400" bind:value={minDuration} placeholder="0" class="rounded-lg border border-border bg-surface px-3 py-2 text-foreground focus:border-primary focus:outline-none" /></label>
          <label class="flex flex-col gap-1 text-xs text-muted">Duração máxima (s)<input type="number" min="0" max="86400" bind:value={maxDuration} placeholder="sem limite" class="rounded-lg border border-border bg-surface px-3 py-2 text-foreground focus:border-primary focus:outline-none" /></label>
          <label class="flex flex-col gap-1 text-xs text-muted">Transmissão<select bind:value={eventType} class="rounded-lg border border-border bg-surface px-3 py-2 text-foreground focus:border-primary focus:outline-none"><option value="">Qualquer</option><option value="live">Ao vivo</option><option value="upcoming">Agendada</option><option value="completed">Encerrada</option></select></label>
          <label class="flex flex-col gap-1 text-xs text-muted">Legendas<select bind:value={caption} class="rounded-lg border border-border bg-surface px-3 py-2 text-foreground focus:border-primary focus:outline-none"><option value="any">Qualquer</option><option value="closedCaption">Com legendas</option><option value="none">Sem legendas</option></select></label>
          <label class="flex flex-col gap-1 text-xs text-muted">Definição<select bind:value={definition} class="rounded-lg border border-border bg-surface px-3 py-2 text-foreground focus:border-primary focus:outline-none"><option value="any">Qualquer</option><option value="high">Alta</option><option value="standard">Padrão</option></select></label>
          <label class="flex flex-col gap-1 text-xs text-muted">Dimensão<select bind:value={dimension} class="rounded-lg border border-border bg-surface px-3 py-2 text-foreground focus:border-primary focus:outline-none"><option value="any">Qualquer</option><option value="2d">2D</option><option value="3d">3D</option></select></label>
          <label class="flex flex-col gap-1 text-xs text-muted">Licença<select bind:value={license} class="rounded-lg border border-border bg-surface px-3 py-2 text-foreground focus:border-primary focus:outline-none"><option value="any">Qualquer</option><option value="youtube">YouTube</option><option value="creativeCommon">Creative Commons</option></select></label>
          <label class="flex flex-col gap-1 text-xs text-muted">Tipo de vídeo<select bind:value={videoType} class="rounded-lg border border-border bg-surface px-3 py-2 text-foreground focus:border-primary focus:outline-none"><option value="any">Qualquer</option><option value="movie">Filme</option><option value="episode">Episódio</option></select></label>
          <label class="flex flex-col gap-1 text-xs text-muted">Categoria (ID)<input bind:value={categoryID} placeholder="ex.: 10 música" class="rounded-lg border border-border bg-surface px-3 py-2 text-foreground focus:border-primary focus:outline-none" /></label>
          <label class="flex flex-col gap-1 text-xs text-muted">Tópico (ID)<input bind:value={topicID} placeholder="ID Freebase" class="rounded-lg border border-border bg-surface px-3 py-2 text-foreground focus:border-primary focus:outline-none" /></label>
          <label class="flex flex-col gap-1 text-xs text-muted">Idioma de relevância<input bind:value={relevanceLanguage} maxlength="10" placeholder="pt-BR" class="rounded-lg border border-border bg-surface px-3 py-2 text-foreground focus:border-primary focus:outline-none" /></label>
          <label class="flex flex-col gap-1 text-xs text-muted">Região<input bind:value={regionCode} maxlength="2" placeholder="BR" class="rounded-lg border border-border bg-surface px-3 py-2 text-foreground uppercase focus:border-primary focus:outline-none" /></label>
          <label class="flex flex-col gap-1 text-xs text-muted">Pesquisa segura<select bind:value={safeSearch} class="rounded-lg border border-border bg-surface px-3 py-2 text-foreground focus:border-primary focus:outline-none"><option value="any">Sem filtro adicional</option><option value="moderate">Moderada</option><option value="none">Desativada</option><option value="strict">Restrita</option></select></label>
          <label class="flex flex-col gap-1 text-xs text-muted">Localização<input bind:value={location} placeholder="lat,long" class="rounded-lg border border-border bg-surface px-3 py-2 text-foreground focus:border-primary focus:outline-none" /></label>
          <label class="flex flex-col gap-1 text-xs text-muted">Raio<input bind:value={locationRadius} placeholder="10km" class="rounded-lg border border-border bg-surface px-3 py-2 text-foreground focus:border-primary focus:outline-none" /></label>
          <label class="flex flex-col gap-1 text-xs text-muted">Assistido<select bind:value={watchState} class="rounded-lg border border-border bg-surface px-3 py-2 text-foreground focus:border-primary focus:outline-none">
            <option value="any">Qualquer</option><option value="unwatched">Não assistidos</option><option value="continue">Continuar assistindo</option><option value="watched">Assistidos</option>
          </select></label>
          <label class="flex flex-col gap-1 text-xs text-muted">Salvo localmente<select bind:value={savedState} class="rounded-lg border border-border bg-surface px-3 py-2 text-foreground focus:border-primary focus:outline-none">
            <option value="any">Qualquer</option><option value="favorites">Favoritos</option><option value="queue">Na fila</option><option value="not-saved">Não salvos</option>
          </select></label>
          <label class="flex items-center gap-2 text-xs text-muted"><input type="checkbox" bind:checked={onlySubscribed} class="accent-primary" /> Somente inscrições locais</label>
          <label class="flex items-center gap-2 text-xs text-muted"><input type="checkbox" bind:checked={shortsOnly} on:change={() => { if (shortsOnly) regularOnly = false; }} class="accent-primary" /> Somente Shorts</label>
          <label class="flex items-center gap-2 text-xs text-muted"><input type="checkbox" bind:checked={regularOnly} on:change={() => { if (regularOnly) shortsOnly = false; }} class="accent-primary" /> Excluir Shorts</label>
          <label class="flex flex-col gap-1 text-xs text-muted">Incorporável<select bind:value={embeddable} class="rounded-lg border border-border bg-surface px-3 py-2 text-foreground focus:border-primary focus:outline-none"><option value="any">Qualquer</option><option value="yes">Sim</option><option value="no">Não</option></select></label>
          <label class="flex flex-col gap-1 text-xs text-muted">Distribuição<select bind:value={syndicated} class="rounded-lg border border-border bg-surface px-3 py-2 text-foreground focus:border-primary focus:outline-none"><option value="any">Qualquer</option><option value="yes">Permitida</option><option value="no">Bloqueada</option></select></label>
          <label class="flex flex-col gap-1 text-xs text-muted">Promoção paga<select bind:value={paidPromotion} class="rounded-lg border border-border bg-surface px-3 py-2 text-foreground focus:border-primary focus:outline-none"><option value="any">Qualquer</option><option value="yes">Com promoção</option><option value="no">Sem promoção</option></select></label>
        {/if}
        <label class="flex items-center gap-2 text-xs text-muted"><input type="checkbox" bind:checked={hideRejected} class="accent-primary" /> Ocultar rejeitados localmente</label>
      </div>
    </details>
  </form>

  {#if searchError}
    <section role="alert" class="flex items-center gap-3 rounded-xl border border-red-500/30 bg-red-500/10 p-4 text-xs text-red-300"><AlertCircle size={18} class="shrink-0" /><div><strong class="block text-foreground">Erro na pesquisa</strong>{searchError}</div></section>
  {/if}

  {#if isSearching}
    <div class="flex flex-col items-center justify-center gap-3 py-20 text-muted" aria-live="polite"><Loader2 size={32} class="animate-spin text-primary" /><span class="text-xs">Consultando o provedor remoto…</span></div>
  {:else if hasSearched && results.length === 0 && !searchError}
    <div class="flex flex-col items-center justify-center gap-3 rounded-2xl border border-dashed border-border bg-surface/30 px-6 py-16 text-center"><Search size={24} class="text-muted" /><h2 class="text-sm font-bold text-foreground">Nenhum resultado para “{query}”</h2><p class="max-w-md text-xs text-muted">Remova um filtro ou tente termos mais amplos.</p></div>
  {:else if results.length > 0}
    <section class="flex flex-col gap-5" aria-label="Resultados da pesquisa">
      <div class="flex flex-wrap items-center justify-between gap-2 text-[11px] text-muted"><span class="flex items-center gap-1.5"><Sparkles size={13} class="text-primary" /> {page?.source === 'youtube-api' ? 'YouTube Data API + NanoRank local' : 'yt-dlp público + NanoRank local'}</span><span>Página {currentPageIndex + 1} · {results.length} resultados</span></div>
      {#if page?.notices?.length}<ul class="rounded-xl border border-amber-500/20 bg-amber-500/5 p-3 text-[11px] text-amber-200">{#each page.notices as notice}<li>{notice}</li>{/each}</ul>{/if}
      {#if catalogResults.length > 0}
        <div class="grid grid-cols-1 gap-3 md:grid-cols-2">
          {#each catalogResults as result (result.resource_type + ':' + result.id)}
            <article use:registerResultContextMenu={result} class="flex items-center gap-3 rounded-xl border border-border bg-surface p-3">
              <div class="flex h-11 w-11 shrink-0 items-center justify-center rounded-lg bg-primary/10 text-primary">{#if result.resource_type === 'playlist'}<ListVideo size={21} />{:else}<UserRound size={21} />{/if}</div>
              <div class="min-w-0 flex-1"><h2 class="truncate text-sm font-semibold text-foreground">{result.title}</h2><p class="text-[10px] uppercase tracking-wide text-muted">{result.resource_type === 'playlist' ? `Playlist remota${result.resource_item_count ? ` · ${result.resource_item_count} vídeos` : ''}` : 'Canal'}</p></div>
              <button type="button" aria-label={`Mais ações para ${result.title}`} aria-haspopup="menu" title="Mais ações" on:click={(event) => openResultContext(event, result)} class="rounded-lg p-2 text-muted hover:bg-surfaceHover hover:text-foreground tv-focusable"><MoreVertical size={16} /></button>
              <Button size="sm" variant="secondary" on:click={() => openCatalogResult(result)}>{result.resource_type === 'playlist' ? 'Abrir' : 'Ver vídeos'}</Button>
            </article>
          {/each}
        </div>
      {/if}
      {#if videoResults.length > 0}<VideoGrid videos={videoResults} />{/if}
      <nav aria-label="Paginação da pesquisa" class="sticky bottom-3 z-10 mx-auto flex w-fit items-center gap-3 rounded-xl border border-border bg-surface/95 p-2 shadow-xl backdrop-blur"><Button variant="secondary" disabled={currentPageIndex === 0} on:click={loadPreviousPage}><ChevronLeft size={16} /> Anterior</Button><strong class="min-w-20 text-center text-xs text-foreground">Página {currentPageIndex + 1}</strong><Button variant="secondary" disabled={!page?.next_page_token} on:click={loadNextPage}>Próxima <ChevronRight size={16} /></Button></nav>
    </section>
  {:else if !hasSearched}
    <div class="flex items-start gap-3 rounded-2xl border border-border bg-surface/50 p-5 text-xs text-muted"><CalendarDays size={18} class="shrink-0 text-primary" /><p>A digitação consulta apenas o índice local. Pressione <strong class="text-foreground">Buscar</strong> para usar rede e quota, quando aplicável.</p></div>
  {/if}
</section>

<ContextMenu open={contextResult !== null} x={contextMenuX} y={contextMenuY} actions={contextActions} on:select={handleContextAction} on:close={() => { contextResult = null; contextMenuTrigger?.focus(); }} />
<RemotePlaylistSaveModal bind:open={showPlaylistSave} remoteID={playlistToSave?.id || ''} suggestedName={playlistToSave?.title || ''} suggestedDescription={playlistToSave?.description || playlistToSave?.description_excerpt || ''} />
