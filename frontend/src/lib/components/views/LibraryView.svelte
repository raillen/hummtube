<script lang="ts">
  import { onMount } from 'svelte';
  import { LibraryService, SettingsService } from '../../wailsjs/services';
  import type { Video, LocalStats } from '../../types';
  import type { ContentViewMode } from '../../stores/uiStores';
  import { toast } from '../../stores/uiStores';
  import VideoCard from '../video/VideoCard.svelte';
  import ViewModeToggle from '../ui/ViewModeToggle.svelte';
  import Chip from '../ui/Chip.svelte';
  import Button from '../ui/Button.svelte';
  import ConfirmDialog from '../ui/ConfirmDialog.svelte';
  import History from 'lucide-svelte/icons/history';
  import Heart from 'lucide-svelte/icons/heart';
  import Trash2 from 'lucide-svelte/icons/trash-2';
  import BarChart3 from 'lucide-svelte/icons/chart-column';
  import Search from 'lucide-svelte/icons/search';

  let activeSection: 'history' | 'favorites' | 'stats' = 'history';
  let historyVideos: Video[] = [];
  let favoriteVideos: Video[] = [];
  let stats: LocalStats | null = null;
  let query = '';
  let selectedIDs = new Set<string>();
  let viewMode: ContentViewMode = 'grid';
  let pendingConfirm: { kind: 'selected' | 'all'; trigger: HTMLElement | null } | null = null;

  $: sourceVideos = activeSection === 'history' ? historyVideos : favoriteVideos;
  $: normalizedQuery = query.trim().toLocaleLowerCase();
  $: visibleVideos = sourceVideos.filter((video) => !normalizedQuery || `${video.title} ${video.channel_title || ''}`.toLocaleLowerCase().includes(normalizedQuery));

  async function loadLibrary(): Promise<void> {
    try {
      const [history, favorites, localStats] = await Promise.all([
        LibraryService.getHistory(), LibraryService.getFavorites(), LibraryService.getLocalStats(),
      ]);
      // Go serializes nil slices as null. Keep the component state normalized so
      // empty libraries remain safe for filtering, counters and bulk actions.
      historyVideos = history ?? [];
      favoriteVideos = favorites ?? [];
      stats = localStats ?? null;
    } catch (error: unknown) {
      toast.add(error instanceof Error ? error.message : 'Erro ao carregar biblioteca.', 'error');
    }
  }

  function toggleSelection(videoID: string): void {
    const next = new Set(selectedIDs);
    if (next.has(videoID)) next.delete(videoID); else next.add(videoID);
    selectedIDs = next;
  }

  function selectVisible(): void {
    const next = new Set(selectedIDs);
    const allSelected = visibleVideos.every((video) => next.has(video.id));
    for (const video of visibleVideos) allSelected ? next.delete(video.id) : next.add(video.id);
    selectedIDs = next;
  }

  async function deleteSelected(): Promise<void> {
    if (selectedIDs.size === 0) return;
    pendingConfirm = { kind: 'selected', trigger: document.activeElement instanceof HTMLElement ? document.activeElement : null };
  }

  async function applyDeleteSelected(): Promise<void> {
    const ids = [...selectedIDs];
    try {
      if (activeSection === 'history') await Promise.all(ids.map(LibraryService.removeHistoryItem));
      else await Promise.all(ids.map(LibraryService.removeFavorite));
      selectedIDs = new Set();
      await loadLibrary();
      toast.add('Itens removidos da biblioteca.', 'success');
    } catch (error: unknown) {
      toast.add(error instanceof Error ? error.message : 'Não foi possível remover os itens.', 'error');
    }
  }

  async function handleClearHistory(): Promise<void> {
    pendingConfirm = { kind: 'all', trigger: document.activeElement instanceof HTMLElement ? document.activeElement : null };
  }

  async function applyClearHistory(): Promise<void> {
    await LibraryService.clearHistory();
    historyVideos = [];
    selectedIDs = new Set();
  }

  function changeSection(section: typeof activeSection): void {
    activeSection = section;
    selectedIDs = new Set();
    query = '';
  }

  function changeViewMode(mode: ContentViewMode): void {
    viewMode = mode;
    void SettingsService.saveSetting('library_view_mode', mode).catch(() => undefined);
  }

  onMount(() => {
    void SettingsService.getSettings().then((settings) => {
      if (['grid', 'list', 'compact'].includes(settings.library_view_mode)) viewMode = settings.library_view_mode as ContentViewMode;
    }).finally(loadLibrary);
  });
</script>

<section class="mx-auto flex w-full max-w-7xl flex-col gap-5 p-6" aria-labelledby="library-heading">
  <header class="flex flex-wrap items-center justify-between gap-3"><div><h1 id="library-heading" class="text-xl font-bold text-foreground">Biblioteca Local</h1><p class="mt-0.5 text-xs text-muted">Pesquise, selecione e organize histórico e favoritos.</p></div>{#if activeSection !== 'stats'}<ViewModeToggle value={viewMode} onChange={changeViewMode} />{/if}</header>
  <nav class="flex flex-wrap items-center gap-2" aria-label="Seções da biblioteca">
    <Chip active={activeSection === 'history'} on:click={() => changeSection('history')}><History size={14} class="mr-1.5 inline" /> Histórico ({historyVideos.length})</Chip>
    <Chip active={activeSection === 'favorites'} on:click={() => changeSection('favorites')}><Heart size={14} class="mr-1.5 inline" /> Favoritos ({favoriteVideos.length})</Chip>
    <Chip active={activeSection === 'stats'} on:click={() => changeSection('stats')}><BarChart3 size={14} class="mr-1.5 inline" /> Estatísticas Pessoais</Chip>
  </nav>

  {#if activeSection !== 'stats'}
    <div class="flex flex-wrap items-center gap-2 rounded-xl border border-border bg-surface p-3">
      <label class="relative min-w-52 flex-1"><span class="sr-only">Pesquisar na seção</span><Search class="absolute left-3 top-2.5 text-muted" size={15} /><input type="search" bind:value={query} placeholder={`Pesquisar em ${activeSection === 'history' ? 'histórico' : 'favoritos'}`} class="w-full rounded-lg border border-border bg-background py-2 pl-9 pr-3 text-xs text-foreground tv-focusable" /></label>
      <Button size="sm" variant="secondary" on:click={selectVisible}>{visibleVideos.every((video) => selectedIDs.has(video.id)) && visibleVideos.length ? 'Limpar visíveis' : 'Selecionar visíveis'}</Button>
      <Button size="sm" variant="danger" on:click={deleteSelected} disabled={!selectedIDs.size}><Trash2 size={14} /> Excluir selecionados ({selectedIDs.size})</Button>
      {#if activeSection === 'history'}<Button size="sm" variant="danger" on:click={handleClearHistory} disabled={!historyVideos.length}>Limpar tudo</Button>{/if}
    </div>
    {#if visibleVideos.length}
      <div class={viewMode === 'list' ? 'grid grid-cols-1 gap-2' : viewMode === 'compact' ? 'grid grid-cols-2 gap-2 md:grid-cols-4 xl:grid-cols-6' : 'grid grid-cols-1 gap-4 sm:grid-cols-2 md:grid-cols-3 lg:grid-cols-4'}>
        {#each visibleVideos as video (video.id)}
          <article class="relative rounded-xl border p-1 {selectedIDs.has(video.id) ? 'border-primary bg-primary/5' : 'border-transparent'}">
            <label class="absolute left-2 top-2 z-10 flex cursor-pointer rounded-md bg-black/70 p-1.5 text-white"><input type="checkbox" checked={selectedIDs.has(video.id)} on:change={() => toggleSelection(video.id)} aria-label={`Selecionar ${video.title}`} class="accent-primary" /></label>
            <VideoCard {video} {viewMode} />
          </article>
        {/each}
      </div>
    {:else}<div class="rounded-xl border border-dashed border-border py-16 text-center text-sm text-muted">Nenhum vídeo corresponde à pesquisa.</div>{/if}
  {:else if stats}
    <div class="grid grid-cols-2 gap-4 sm:grid-cols-3"><div class="rounded-xl border border-border bg-surface p-4"><span class="text-xs font-bold uppercase text-muted">Vídeos assistidos</span><p class="mt-1 text-2xl font-bold text-foreground">{stats.videos_watched || 0}</p></div><div class="rounded-xl border border-border bg-surface p-4"><span class="text-xs font-bold uppercase text-muted">Inscrições</span><p class="mt-1 text-2xl font-bold text-foreground">{stats.subscriptions_count || 0}</p></div><div class="rounded-xl border border-border bg-surface p-4"><span class="text-xs font-bold uppercase text-muted">Playlists</span><p class="mt-1 text-2xl font-bold text-foreground">{stats.playlists_count || 0}</p></div></div>
  {/if}

  <ConfirmDialog
    open={pendingConfirm !== null}
    trigger={pendingConfirm?.trigger ?? null}
    danger
    title={pendingConfirm?.kind === 'all' ? 'Limpar histórico' : 'Remover selecionados'}
    description={pendingConfirm?.kind === 'all'
      ? 'Esta ação apaga todo o histórico de reprodução deste perfil.'
      : `Remover ${selectedIDs.size} vídeos de ${activeSection === 'history' ? 'histórico' : 'favoritos'}?`}
    confirmLabel={pendingConfirm?.kind === 'all' ? 'Limpar tudo' : 'Remover'}
    cancelLabel="Cancelar"
    on:confirm={async () => {
      const kind = pendingConfirm?.kind;
      pendingConfirm = null;
      if (kind === 'selected') await applyDeleteSelected();
      else if (kind === 'all') await applyClearHistory();
    }}
    on:cancel={() => (pendingConfirm = null)}
  />
</section>
