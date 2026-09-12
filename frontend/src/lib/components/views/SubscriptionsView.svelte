<script lang="ts">
  import { onMount } from 'svelte';
  import CalendarDays from 'lucide-svelte/icons/calendar-days';
  import RefreshCw from 'lucide-svelte/icons/refresh-cw';
  import SlidersHorizontal from 'lucide-svelte/icons/sliders-horizontal';
  import Users from 'lucide-svelte/icons/users';
  import type { SubscriptionVideoPage, SubscriptionVideoQuery } from '../../types';
  import { CatalogService, SettingsService } from '../../wailsjs/services';
  import { toast } from '../../stores/uiStores';
  import Button from '../ui/Button.svelte';
  import VideoGrid from '../video/VideoGrid.svelte';

  type DateRange = 'all' | 'today' | 'yesterday' | 'week' | 'month';
  const dateRangeOptions: Array<{ id: DateRange; label: string }> = [
    { id: 'today', label: 'Hoje' },
    { id: 'yesterday', label: 'Ontem' },
    { id: 'week', label: '7 dias' },
    { id: 'month', label: '30 dias' },
  ];

  let page: SubscriptionVideoPage | null = null;
  let dateRanges: DateRange[] = [];
  let contentTypes: Array<'regular' | 'live'> = [];
  let watch: SubscriptionVideoQuery['watch'] = 'any';
  let category = '';
  let pageSize = 24;
  let isLoading = true;
  let isLoadingMore = false;
  let isRefreshing = false;
  let errorMessage = '';
  let requestGeneration = 0;
  let hasMounted = false;
  let appliedFilterKey = '';

  function dateBounds(ranges: DateRange[]): Pick<SubscriptionVideoQuery, 'from' | 'to'> {
    if (ranges.length === 0 || ranges.includes('all')) return {};
    const now = new Date();
    const startToday = new Date(now.getFullYear(), now.getMonth(), now.getDate());
    if (ranges.includes('month')) return { from: new Date(now.getTime() - 30 * 86400000).toISOString() };
    if (ranges.includes('week')) return { from: new Date(now.getTime() - 7 * 86400000).toISOString() };
    if (ranges.includes('today') && ranges.includes('yesterday')) return { from: new Date(startToday.getTime() - 86400000).toISOString() };
    if (ranges.includes('today')) return { from: startToday.toISOString() };
    if (ranges.includes('yesterday')) {
      const startYesterday = new Date(startToday);
      startYesterday.setDate(startYesterday.getDate() - 1);
      return { from: startYesterday.toISOString(), to: startToday.toISOString() };
    }
    return {};
  }

  function buildQuery(offset = 0): SubscriptionVideoQuery {
    return {
      offset,
      limit: pageSize,
      content: contentTypes.length === 1 ? contentTypes[0] : 'all',
      watch,
      category: category || undefined,
      ...dateBounds(dateRanges)
    };
  }

  async function loadVideos(append = false): Promise<void> {
    const generation = ++requestGeneration;
    if (append) isLoadingMore = true;
    else isLoading = true;
    errorMessage = '';
    try {
      const nextPage = await CatalogService.listSubscriptionVideos(buildQuery(append ? page?.videos.length ?? 0 : 0));
      if (generation !== requestGeneration) return;
      page = append && page
        ? { ...nextPage, videos: [...page.videos, ...nextPage.videos.filter((video) => !page?.videos.some((known) => known.id === video.id))] }
        : nextPage;
    } catch (error: unknown) {
      if (generation === requestGeneration) {
        errorMessage = error instanceof Error ? error.message : 'Não foi possível carregar as inscrições.';
      }
    } finally {
      if (generation === requestGeneration) {
        isLoading = false;
        isLoadingMore = false;
      }
    }
  }

  async function refreshSubscriptions(): Promise<void> {
    isRefreshing = true;
    try {
      await CatalogService.refreshSubscriptions();
      await loadVideos();
      toast.add('Inscrições atualizadas.', 'success');
    } catch (error: unknown) {
      toast.add(error instanceof Error ? error.message : 'Falha ao atualizar inscrições.', 'error');
    } finally {
      isRefreshing = false;
    }
  }

  onMount(() => {
    void SettingsService.getSettings().then((settings) => {
      const configured = Number.parseInt(settings.subscriptions_page_size ?? '', 10);
      if ([12, 24, 36, 48].includes(configured)) pageSize = configured;
      if (settings.subscriptions_filters) {
        try {
          const saved = JSON.parse(settings.subscriptions_filters) as { date_ranges?: DateRange[]; content_types?: Array<'regular' | 'live'>; watch?: SubscriptionVideoQuery['watch']; category?: string };
          dateRanges = (saved.date_ranges || []).filter((value) => ['today', 'yesterday', 'week', 'month'].includes(value));
          contentTypes = (saved.content_types || []).filter((value) => value === 'regular' || value === 'live');
          watch = saved.watch || 'any';
          category = saved.category || '';
        } catch { /* ignora preferência incompatível */ }
      }
    }).catch(() => undefined).finally(() => {
      hasMounted = true;
      appliedFilterKey = `${dateRanges.join(',')}:${contentTypes.join(',')}:${watch}:${category}:${pageSize}`;
      void loadVideos();
    });
  });

  $: filterKey = `${dateRanges.join(',')}:${contentTypes.join(',')}:${watch}:${category}:${pageSize}`;
  $: if (hasMounted && filterKey !== appliedFilterKey) {
    appliedFilterKey = filterKey;
    void SettingsService.saveSetting('subscriptions_filters', JSON.stringify({ date_ranges: dateRanges, content_types: contentTypes, watch, category })).catch(() => undefined);
    void loadVideos();
  }

  function toggleDateRange(range: DateRange): void {
    dateRanges = dateRanges.includes(range) ? dateRanges.filter((value) => value !== range) : [...dateRanges, range];
  }

  function toggleContentType(type: 'regular' | 'live'): void {
    contentTypes = contentTypes.includes(type) ? contentTypes.filter((value) => value !== type) : [...contentTypes, type];
  }
</script>

<section class="mx-auto flex w-full max-w-7xl flex-col gap-5 p-6" aria-labelledby="subscriptions-page-heading">
  <header class="flex flex-wrap items-center justify-between gap-4">
    <div class="flex items-center gap-3">
      <div class="flex h-10 w-10 items-center justify-center rounded-xl bg-primary/10 text-primary"><Users size={21} /></div>
      <div>
        <h1 id="subscriptions-page-heading" class="text-xl font-bold text-foreground">Vídeos das inscrições</h1>
        <p class="mt-0.5 text-xs text-muted">Filtre todo o catálogo sincronizado, não apenas os vídeos mais recentes.</p>
      </div>
    </div>
    <Button variant="secondary" size="sm" on:click={refreshSubscriptions} disabled={isRefreshing}>
      <RefreshCw size={14} class={isRefreshing ? 'animate-spin' : ''} /> Atualizar vídeos
    </Button>
  </header>

  <form class="grid gap-3 rounded-xl border border-border bg-surface p-4 sm:grid-cols-2 lg:grid-cols-4" on:submit|preventDefault={() => loadVideos()}>
    <fieldset class="flex flex-col gap-1 text-xs text-muted">
      <legend class="flex items-center gap-1.5 font-semibold text-foreground"><CalendarDays size={13} /> Datas (combine)</legend>
      <div class="flex flex-wrap gap-1.5">{#each dateRangeOptions as option (option.id)}<button type="button" aria-pressed={dateRanges.includes(option.id)} on:click={() => toggleDateRange(option.id)} class="rounded-full border px-2.5 py-1.5 tv-focusable {dateRanges.includes(option.id) ? 'border-primary bg-primary/10 text-primary' : 'border-border bg-background text-muted'}">{option.label}</button>{/each}</div>
      {#if dateRanges.length === 0}<span class="text-[10px]">Nenhuma seleção: todas as datas.</span>{/if}
    </fieldset>
    <fieldset class="flex flex-col gap-1 text-xs text-muted">
      <legend class="font-semibold text-foreground">Tipos (combine)</legend>
      <div class="flex flex-wrap gap-1.5"><button type="button" aria-pressed={contentTypes.includes('regular')} on:click={() => toggleContentType('regular')} class="rounded-full border px-2.5 py-1.5 tv-focusable {contentTypes.includes('regular') ? 'border-primary bg-primary/10 text-primary' : 'border-border bg-background text-muted'}">Vídeos</button><button type="button" aria-pressed={contentTypes.includes('live')} on:click={() => toggleContentType('live')} class="rounded-full border px-2.5 py-1.5 tv-focusable {contentTypes.includes('live') ? 'border-primary bg-primary/10 text-primary' : 'border-border bg-background text-muted'}">Transmissões</button></div>
      {#if contentTypes.length === 0}<span class="text-[10px]">Nenhuma seleção: todos os tipos.</span>{/if}
    </fieldset>
    <label class="flex flex-col gap-1 text-xs text-muted">
      <span class="font-semibold text-foreground">Histórico</span>
      <select bind:value={watch} class="rounded-lg border border-border bg-background px-3 py-2 text-foreground tv-focusable">
        <option value="any">Assistidos e não assistidos</option><option value="unwatched">Não assistidos</option><option value="watched">Assistidos</option>
      </select>
    </label>
    <label class="flex flex-col gap-1 text-xs text-muted">
      <span class="font-semibold text-foreground">Categoria</span>
      <select bind:value={category} class="rounded-lg border border-border bg-background px-3 py-2 text-foreground tv-focusable">
        <option value="">Todas as categorias</option>
        {#each page?.categories ?? [] as availableCategory}<option value={availableCategory}>{availableCategory}</option>{/each}
      </select>
    </label>
  </form>

  {#if errorMessage}
    <div role="alert" class="flex items-center justify-between gap-3 rounded-xl border border-red-500/30 bg-red-500/10 p-4 text-xs text-red-300">
      <span>{errorMessage}</span><Button size="sm" on:click={() => loadVideos()}>Tentar novamente</Button>
    </div>
  {:else if isLoading}
    <div role="status" class="flex items-center gap-2 py-12 text-sm text-muted"><SlidersHorizontal size={18} /> Carregando vídeos filtrados…</div>
  {:else if page && page.videos.length > 0}
    <VideoGrid videos={page.videos} title={`${page.total} vídeos encontrados`} />
    {#if page.has_more}
      <div class="flex justify-center"><Button on:click={() => loadVideos(true)} disabled={isLoadingMore}>{isLoadingMore ? 'Carregando…' : 'Carregar mais'}</Button></div>
    {/if}
  {:else}
    <div class="flex flex-col items-center gap-3 rounded-2xl border border-dashed border-border py-16 text-center text-sm text-muted">
      <h2 class="text-base font-semibold text-foreground">Nenhum vídeo sincronizado</h2>
      <p>Nenhum vídeo corresponde aos filtros atuais. Atualize os feeds ou amplie o intervalo de data.</p>
      <Button variant="secondary" size="sm" on:click={refreshSubscriptions} disabled={isRefreshing}><RefreshCw size={14} /> Atualizar feeds</Button>
    </div>
  {/if}
</section>
