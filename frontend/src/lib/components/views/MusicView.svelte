<script lang="ts">
  import Headphones from 'lucide-svelte/icons/headphones';
  import Loader2 from 'lucide-svelte/icons/loader-circle';
  import Mic2 from 'lucide-svelte/icons/mic-vocal';
  import Music2 from 'lucide-svelte/icons/music-2';
  import Search from 'lucide-svelte/icons/search';
  import type { SearchPage, SearchRequest, Video } from '../../types';
  import { SearchService } from '../../wailsjs/services';
  import Button from '../ui/Button.svelte';
  import VideoGrid from '../video/VideoGrid.svelte';

  type MusicMode = 'music' | 'podcast';

  let mode: MusicMode = 'music';
  let query = '';
  let page: SearchPage | null = null;
  let videos: Video[] = [];
  let loading = false;
  let error = '';

  function request(pageToken = ''): SearchRequest {
    const modeTerm = mode === 'podcast' ? 'podcast' : 'music';
    return {
      query: query.trim() || modeTerm,
      include_terms: mode === 'podcast' ? 'podcast' : undefined,
      resource_types: ['video'],
      category_id: mode === 'music' ? '10' : undefined,
      order: 'relevance',
      max_results: 24,
      page_token: pageToken || undefined,
      hide_rejected: true,
    };
  }

  async function search(reset = true): Promise<void> {
    loading = true;
    error = '';
    try {
      const nextPage = await SearchService.search(request(reset ? '' : page?.next_page_token || ''));
      page = nextPage;
      videos = reset ? nextPage.items : [...videos, ...nextPage.items];
    } catch (cause: unknown) {
      error = cause instanceof Error ? cause.message : 'Não foi possível carregar resultados de música.';
    } finally {
      loading = false;
    }
  }

  function selectMode(nextMode: MusicMode): void {
    if (nextMode === mode) return;
    mode = nextMode;
    void search(true);
  }
</script>

<section class="mx-auto flex w-full max-w-7xl flex-col gap-6 p-6" aria-labelledby="music-title">
  <header class="flex flex-col gap-3 rounded-2xl border border-border bg-gradient-to-br from-primary/15 to-surface p-5">
    <div class="flex items-start gap-3">
      <div class="rounded-xl bg-primary/15 p-2.5 text-primary"><Headphones size={22} /></div>
      <div>
        <h1 id="music-title" class="text-xl font-bold text-foreground">Modo Música</h1>
        <p class="mt-1 text-xs text-muted">Pesquisa focada em música e podcasts. No player, escolha “Somente áudio” no seletor de qualidade.</p>
      </div>
    </div>

    <div class="flex flex-wrap gap-2" role="group" aria-label="Tipo de conteúdo musical">
      <button type="button" aria-pressed={mode === 'music'} on:click={() => selectMode('music')}
        class="tv-focusable inline-flex items-center gap-2 rounded-full border px-3 py-1.5 text-xs font-semibold {mode === 'music' ? 'border-primary bg-primary/15 text-primary' : 'border-border text-muted hover:text-foreground'}">
        <Music2 size={14} /> Música
      </button>
      <button type="button" aria-pressed={mode === 'podcast'} on:click={() => selectMode('podcast')}
        class="tv-focusable inline-flex items-center gap-2 rounded-full border px-3 py-1.5 text-xs font-semibold {mode === 'podcast' ? 'border-primary bg-primary/15 text-primary' : 'border-border text-muted hover:text-foreground'}">
        <Mic2 size={14} /> Podcasts
      </button>
    </div>

    <form class="flex gap-2" role="search" aria-label="Pesquisar música e podcasts" on:submit|preventDefault={() => search(true)}>
      <label class="sr-only" for="music-search">Pesquisar</label>
      <input id="music-search" type="search" bind:value={query} placeholder={mode === 'music' ? 'Artista, álbum ou faixa…' : 'Podcast, episódio ou assunto…'}
        class="tv-focusable min-w-0 flex-1 rounded-xl border border-border bg-surface px-4 py-2.5 text-sm text-foreground placeholder:text-muted focus:border-primary focus:outline-none" />
      <Button type="submit" variant="primary" disabled={loading}>
        {#if loading}<Loader2 size={16} class="animate-spin" />{:else}<Search size={16} />{/if} Buscar
      </Button>
    </form>
  </header>

  {#if error}
    <p role="alert" class="rounded-xl border border-red-500/30 bg-red-500/10 p-4 text-xs text-red-300">{error}</p>
  {/if}

  {#if videos.length > 0}
    <VideoGrid {videos} title={mode === 'music' ? 'Músicas' : 'Podcasts'} />
    {#if page?.next_page_token}
      <div class="flex justify-center"><Button variant="secondary" on:click={() => search(false)} disabled={loading}>Carregar mais</Button></div>
    {/if}
  {:else if !loading && !error}
    <div class="rounded-2xl border border-dashed border-border p-10 text-center text-sm text-muted">
      Pesquise para montar sua sessão de {mode === 'music' ? 'música' : 'podcasts'}.
    </div>
  {/if}
</section>
