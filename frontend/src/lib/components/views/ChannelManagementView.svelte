<script lang="ts">
  import { onMount } from 'svelte';
  import ExternalLink from 'lucide-svelte/icons/external-link';
  import FolderInput from 'lucide-svelte/icons/folder-input';
  import FolderPlus from 'lucide-svelte/icons/folder-plus';
  import MoreVertical from 'lucide-svelte/icons/ellipsis-vertical';
  import Search from 'lucide-svelte/icons/search';
  import Star from 'lucide-svelte/icons/star';
  import Tags from 'lucide-svelte/icons/tags';
  import UserMinus from 'lucide-svelte/icons/user-minus';
  import UserPlus from 'lucide-svelte/icons/user-plus';
  import Users from 'lucide-svelte/icons/users';
  import type { ChannelFolder, ManagedChannel } from '../../types';
  import { CatalogService, SettingsService } from '../../wailsjs/services';
  import { toast, type ContentViewMode } from '../../stores/uiStores';
  import { openChannel } from '../../stores/channelRouteStore';
  import Button from '../ui/Button.svelte';
  import Modal from '../ui/Modal.svelte';
  import ContextMenu from '../ui/ContextMenu.svelte';
  import ViewModeToggle from '../ui/ViewModeToggle.svelte';

  type ChannelSort = 'name' | 'subscribed' | 'last_watched';
  type DateRange = 'today' | 'yesterday' | 'week' | 'month' | 'year' | 'never';
  const dateRangeOptions: Array<{ id: DateRange; label: string }> = [
    { id: 'today', label: 'Hoje' }, { id: 'yesterday', label: 'Ontem' },
    { id: 'week', label: '7 dias' }, { id: 'month', label: '30 dias' },
    { id: 'year', label: '1 ano' }, { id: 'never', label: 'Nunca' },
  ];
  let channels: ManagedChannel[] = [];
  let favoriteChannelIDs: Record<string, boolean> = {};
  let folders: ChannelFolder[] = [];
  let selectedIDs = new Set<string>();
  let query = '';
  let category = '';
  let tag = '';
  let sort: ChannelSort = 'name';
  let subscribedRanges: DateRange[] = [];
  let lastWatchedRanges: DateRange[] = [];
  let isLoading = true;
  let errorMessage = '';
  let isApplying = false;
  let showUnsubscribeConfirmation = false;
  let showTagEditor = false;
  let tagsText = '';
  let selectedFolderID = '';
  let showCreateFolder = false;
  let showAddChannel = false;
  let newFolderName = '';
  let newChannelID = '';
  let newChannelTitle = '';
  let contextChannel: ManagedChannel | null = null;
  let contextMenuX = 0;
  let contextMenuY = 0;
  let contextMenuTrigger: HTMLElement | null = null;
  let viewMode: ContentViewMode = 'grid';
  let preferencesReady = false;
  let persistedFilterSignature = '';

  $: contextActions = contextChannel ? [
    { id: 'open', label: 'Abrir canal' },
    { id: 'select', label: selectedIDs.has(contextChannel.channel.id) ? 'Desmarcar seleção' : 'Selecionar canal' },
    { id: 'favorite', label: favoriteChannelIDs[contextChannel.channel.id] ? 'Remover dos favoritos' : 'Adicionar aos favoritos' },
    { id: 'unsubscribe', label: 'Desinscrever canal', danger: true },
  ] : [];

  function dateValue(value?: string): number {
    return value ? new Date(value).getTime() || 0 : 0;
  }

  function matchesSingleDateRange(value: string | undefined, range: DateRange): boolean {
    const timestamp = dateValue(value);
    if (range === 'never') return timestamp === 0;
    if (!timestamp) return false;
    if (range === 'today' || range === 'yesterday') {
      const nowDate = new Date();
      const startOfToday = new Date(nowDate.getFullYear(), nowDate.getMonth(), nowDate.getDate()).getTime();
      return range === 'today' ? timestamp >= startOfToday : timestamp >= startOfToday - 24 * 60 * 60 * 1000 && timestamp < startOfToday;
    }
    const now = Date.now();
    const days = range === 'week' ? 7 : range === 'month' ? 30 : 365;
    return timestamp >= now - days * 24 * 60 * 60 * 1000;
  }

  function matchesDateRanges(value: string | undefined, ranges: DateRange[]): boolean {
    return ranges.length === 0 || ranges.some((range) => matchesSingleDateRange(value, range));
  }

  $: categories = [...new Set(channels.map((entry) => entry.category).filter((value): value is string => Boolean(value)))].sort((left, right) => left.localeCompare(right));
  $: tags = [...new Set(channels.flatMap((entry) => entry.tags))].sort((left, right) => left.localeCompare(right));
  $: filteredChannels = channels.filter((entry) => {
    const normalizedQuery = query.trim().toLocaleLowerCase();
    return (!normalizedQuery || entry.channel.title.toLocaleLowerCase().includes(normalizedQuery) || entry.channel.id.toLocaleLowerCase().includes(normalizedQuery))
      && (!category || entry.category === category)
      && (!tag || entry.tags.includes(tag))
      && matchesDateRanges(entry.subscribed_at, subscribedRanges)
      && matchesDateRanges(entry.last_watched_at, lastWatchedRanges);
  }).sort((left, right) => {
    if (sort === 'subscribed') return dateValue(right.subscribed_at) - dateValue(left.subscribed_at);
    if (sort === 'last_watched') return dateValue(right.last_watched_at) - dateValue(left.last_watched_at);
    return left.channel.title.localeCompare(right.channel.title);
  });

  async function loadChannels(): Promise<void> {
    isLoading = true;
    errorMessage = '';
    try {
      const [loadedChannels, loadedFolders, loadedFavorites] = await Promise.all([
        CatalogService.listManagedChannels(), CatalogService.getChannelFolders(), CatalogService.getFavoriteChannelIDs(),
      ]);
      channels = loadedChannels || [];
      folders = loadedFolders || [];
      favoriteChannelIDs = loadedFavorites || {};
      selectedIDs = new Set([...selectedIDs].filter((id) => channels.some((entry) => entry.channel.id === id)));
    } catch (error: unknown) {
      errorMessage = error instanceof Error ? error.message : 'Não foi possível carregar os canais.';
    } finally {
      isLoading = false;
    }
  }

  function toggleSelection(channelID: string): void {
    const next = new Set(selectedIDs);
    if (next.has(channelID)) next.delete(channelID); else next.add(channelID);
    selectedIDs = next;
  }

  function openChannelContext(event: MouseEvent | KeyboardEvent, entry: ManagedChannel): void {
    event.preventDefault();
    contextChannel = entry;
    contextMenuTrigger = event instanceof KeyboardEvent
      ? document.activeElement as HTMLElement
      : event.currentTarget as HTMLElement;
    const bounds = contextMenuTrigger.getBoundingClientRect();
    if (event instanceof MouseEvent && event.type === 'contextmenu') {
      contextMenuX = event.clientX;
      contextMenuY = event.clientY;
    } else {
      contextMenuX = bounds.left + 24;
      contextMenuY = bounds.top + 24;
    }
  }

  function registerChannelContextMenu(node: HTMLElement, entry: ManagedChannel): { destroy: () => void } {
    const handleContextMenu = (event: MouseEvent) => openChannelContext(event, entry);
    const handleKeyDown = (event: KeyboardEvent) => {
      if (event.key === 'ContextMenu' || event.key === 'Apps' || (event.key === 'F10' && event.shiftKey)) openChannelContext(event, entry);
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

  async function runChannelContextAction(event: CustomEvent<string>): Promise<void> {
    const entry = contextChannel;
    if (!entry) return;
    if (event.detail === 'open') openChannel({ id: entry.channel.id, title: entry.channel.title, thumbnailUrl: entry.channel.thumbnail_url });
    if (event.detail === 'select') toggleSelection(entry.channel.id);
    if (event.detail === 'favorite') {
      const isFavorite = Boolean(favoriteChannelIDs[entry.channel.id]);
      isApplying = true;
      try {
        await (isFavorite ? CatalogService.removeChannelFavorite(entry.channel.id) : CatalogService.addChannelFavorite(entry.channel.id));
        toast.add(isFavorite ? 'Canal removido dos favoritos.' : 'Canal adicionado aos favoritos.', 'success');
        await loadChannels();
      } catch (error: unknown) {
        toast.add(error instanceof Error ? error.message : 'Não foi possível atualizar o favorito.', 'error');
      } finally {
        isApplying = false;
      }
    }
    if (event.detail === 'unsubscribe') {
      selectedIDs = new Set([entry.channel.id]);
      showUnsubscribeConfirmation = true;
    }
    contextChannel = null;
  }

  function toggleVisibleSelection(): void {
    const next = new Set(selectedIDs);
    const allVisibleSelected = filteredChannels.every((entry) => next.has(entry.channel.id));
    for (const entry of filteredChannels) {
      if (allVisibleSelected) next.delete(entry.channel.id); else next.add(entry.channel.id);
    }
    selectedIDs = next;
  }

  async function runBulkAction(action: () => Promise<void>, successMessage: string): Promise<void> {
    if (selectedIDs.size === 0) return;
    isApplying = true;
    try {
      await action();
      toast.add(successMessage, 'success');
      await loadChannels();
    } catch (error: unknown) {
      toast.add(error instanceof Error ? error.message : 'Não foi possível concluir a ação.', 'error');
    } finally {
      isApplying = false;
    }
  }

  function formatDate(value?: string): string {
    if (!value) return 'Desconhecido';
    const date = new Date(value);
    return Number.isNaN(date.getTime()) ? 'Desconhecido' : new Intl.DateTimeFormat('pt-BR', { dateStyle: 'medium' }).format(date);
  }

  async function applyTags(): Promise<void> {
    const parsedTags = tagsText.split(',').map((value) => value.trim()).filter(Boolean);
    await runBulkAction(
      () => Promise.all([...selectedIDs].map((channelID) => CatalogService.setChannelTags(channelID, parsedTags))).then(() => undefined),
      'Tags atualizadas.'
    );
    showTagEditor = false;
  }

  async function createFolder(): Promise<void> {
    const name = newFolderName.trim();
    if (!name) return;
    isApplying = true;
    try {
      await CatalogService.createChannelFolder(name);
      newFolderName = '';
      showCreateFolder = false;
      await loadChannels();
      toast.add('Pasta criada.', 'success');
    } catch (error: unknown) {
      toast.add(error instanceof Error ? error.message : 'Não foi possível criar a pasta.', 'error');
    } finally {
      isApplying = false;
    }
  }

  async function addChannel(): Promise<void> {
    const channelID = newChannelID.trim();
    const title = newChannelTitle.trim();
    if (!channelID || !title) return;
    isApplying = true;
    try {
      await CatalogService.subscribeChannel(channelID, title);
      newChannelID = '';
      newChannelTitle = '';
      showAddChannel = false;
      await loadChannels();
      toast.add('Canal adicionado às inscrições.', 'success');
    } catch (error: unknown) {
      toast.add(error instanceof Error ? error.message : 'Não foi possível adicionar o canal.', 'error');
    } finally {
      isApplying = false;
    }
  }

  function changeViewMode(mode: ContentViewMode): void {
    viewMode = mode;
    void SettingsService.saveSetting('channel_view_mode', mode).catch(() => undefined);
  }

  function toggleDateRange(target: 'subscribed' | 'watched', range: DateRange): void {
    const current = target === 'subscribed' ? subscribedRanges : lastWatchedRanges;
    const next = current.includes(range) ? current.filter((value) => value !== range) : [...current, range];
    if (target === 'subscribed') subscribedRanges = next; else lastWatchedRanges = next;
  }

  function filterSignature(): string {
    return JSON.stringify({ query, category, tag, sort, subscribed_ranges: subscribedRanges, last_watched_ranges: lastWatchedRanges });
  }

  onMount(() => { void SettingsService.getSettings().then((settings) => {
    if (['grid', 'list', 'compact'].includes(settings.channel_view_mode)) viewMode = settings.channel_view_mode as ContentViewMode;
    if (settings.channel_filters) {
      try {
        const saved = JSON.parse(settings.channel_filters) as { query?: string; category?: string; tag?: string; sort?: ChannelSort; subscribed_ranges?: DateRange[]; last_watched_ranges?: DateRange[] };
        query = saved.query || '';
        category = saved.category || '';
        tag = saved.tag || '';
        if (saved.sort === 'subscribed' || saved.sort === 'last_watched') sort = saved.sort;
        subscribedRanges = (saved.subscribed_ranges || []).filter((value) => dateRangeOptions.some((option) => option.id === value));
        lastWatchedRanges = (saved.last_watched_ranges || []).filter((value) => dateRangeOptions.some((option) => option.id === value));
      } catch { /* preferência anterior incompatível */ }
    }
  }).finally(() => { preferencesReady = true; persistedFilterSignature = filterSignature(); void loadChannels(); }); });

  $: currentFilterSignature = filterSignature();
  $: if (preferencesReady && currentFilterSignature !== persistedFilterSignature) {
    persistedFilterSignature = currentFilterSignature;
    void SettingsService.saveSetting('channel_filters', currentFilterSignature).catch(() => undefined);
  }
</script>

<section class="mx-auto flex w-full max-w-7xl flex-col gap-5 p-6" aria-labelledby="channel-manager-heading">
  <header class="flex flex-wrap items-center gap-3">
    <div class="flex h-10 w-10 items-center justify-center rounded-xl bg-primary/10 text-primary"><Users size={21} /></div>
    <div class="mr-auto"><h1 id="channel-manager-heading" class="text-xl font-bold text-foreground">Gerenciar canais</h1><p class="text-xs text-muted">Organize, classifique ou remova várias inscrições de uma vez.</p></div>
    <ViewModeToggle value={viewMode} onChange={changeViewMode} />
    <Button size="sm" variant="secondary" on:click={() => (showCreateFolder = true)}><FolderPlus size={14} /> Nova pasta</Button>
    <Button size="sm" on:click={() => (showAddChannel = true)}><UserPlus size={14} /> Adicionar canal</Button>
  </header>

  <div class="grid gap-3 rounded-xl border border-border bg-surface p-4 md:grid-cols-4">
    <label class="relative md:col-span-1"><span class="sr-only">Pesquisar canais</span><Search class="absolute left-3 top-2.5 text-muted" size={15} /><input type="search" bind:value={query} placeholder="Pesquisar canais" class="w-full rounded-lg border border-border bg-background py-2 pl-9 pr-3 text-xs text-foreground tv-focusable" /></label>
    <label><span class="sr-only">Filtrar categoria</span><select bind:value={category} class="w-full rounded-lg border border-border bg-background px-3 py-2 text-xs text-foreground tv-focusable"><option value="">Todas as categorias</option>{#each categories as value}<option value={value}>{value}</option>{/each}</select></label>
    <label><span class="sr-only">Filtrar tag</span><select bind:value={tag} class="w-full rounded-lg border border-border bg-background px-3 py-2 text-xs text-foreground tv-focusable"><option value="">Todas as tags</option>{#each tags as value}<option value={value}>{value}</option>{/each}</select></label>
    <label><span class="sr-only">Ordenar canais</span><select bind:value={sort} class="w-full rounded-lg border border-border bg-background px-3 py-2 text-xs text-foreground tv-focusable"><option value="name">Nome</option><option value="subscribed">Inscrições mais recentes</option><option value="last_watched">Último vídeo assistido</option></select></label>
    <fieldset class="flex flex-col gap-1.5 md:col-span-2"><legend class="text-[10px] font-semibold uppercase tracking-wide text-muted">Período de inscrição (combine)</legend><div class="flex flex-wrap gap-1">{#each dateRangeOptions.filter((option) => option.id !== 'never') as option (option.id)}<button type="button" aria-pressed={subscribedRanges.includes(option.id)} on:click={() => toggleDateRange('subscribed', option.id)} class="rounded-full border px-2 py-1 text-[10px] tv-focusable {subscribedRanges.includes(option.id) ? 'border-primary bg-primary/10 text-primary' : 'border-border text-muted'}">{option.label}</button>{/each}{#if subscribedRanges.length === 0}<span class="self-center text-[10px] text-muted">Todas</span>{/if}</div></fieldset>
    <fieldset class="flex flex-col gap-1.5 md:col-span-2"><legend class="text-[10px] font-semibold uppercase tracking-wide text-muted">Último vídeo visto (combine)</legend><div class="flex flex-wrap gap-1">{#each dateRangeOptions as option (option.id)}<button type="button" aria-pressed={lastWatchedRanges.includes(option.id)} on:click={() => toggleDateRange('watched', option.id)} class="rounded-full border px-2 py-1 text-[10px] tv-focusable {lastWatchedRanges.includes(option.id) ? 'border-primary bg-primary/10 text-primary' : 'border-border text-muted'}">{option.label}</button>{/each}{#if lastWatchedRanges.length === 0}<span class="self-center text-[10px] text-muted">Todos</span>{/if}</div></fieldset>
  </div>

  <div class="flex flex-wrap items-center gap-2 rounded-xl border border-border bg-surface/60 p-3">
    <Button size="sm" variant="secondary" on:click={toggleVisibleSelection}>{filteredChannels.every((entry) => selectedIDs.has(entry.channel.id)) ? 'Limpar visíveis' : 'Selecionar visíveis'}</Button>
    <span class="mr-auto text-xs text-muted">{selectedIDs.size} selecionados</span>
    <Button size="sm" on:click={() => runBulkAction(() => CatalogService.bulkFavoriteChannels([...selectedIDs]), 'Canais adicionados aos favoritos.')} disabled={!selectedIDs.size || isApplying}><Star size={13} /> Favoritar</Button>
    <select bind:value={selectedFolderID} aria-label="Pasta de destino" class="rounded-lg border border-border bg-background px-2.5 py-1.5 text-xs text-foreground"><option value="">Escolha uma pasta</option>{#each folders as folder}<option value={folder.id}>{folder.name}</option>{/each}</select>
    <Button size="sm" on:click={() => runBulkAction(() => CatalogService.bulkAddChannelsToFolder(selectedFolderID, [...selectedIDs]), 'Canais adicionados à pasta.')} disabled={!selectedIDs.size || !selectedFolderID || isApplying}><FolderInput size={13} /> Adicionar</Button>
    <Button size="sm" on:click={() => { tagsText = ''; showTagEditor = true; }} disabled={!selectedIDs.size || isApplying}><Tags size={13} /> Tags</Button>
    <Button size="sm" variant="danger" on:click={() => (showUnsubscribeConfirmation = true)} disabled={!selectedIDs.size || isApplying}><UserMinus size={13} /> Desinscrever</Button>
  </div>

  {#if errorMessage}<div role="alert" class="rounded-xl border border-red-500/30 bg-red-500/10 p-4 text-xs text-red-300">{errorMessage}</div>
  {:else if isLoading}<p role="status" class="py-12 text-sm text-muted">Carregando canais…</p>
  {:else if filteredChannels.length === 0}<div class="rounded-xl border border-dashed border-border py-16 text-center text-sm text-muted">Nenhum canal corresponde aos filtros.</div>
  {:else}
    <div class={viewMode === 'list' ? 'grid grid-cols-1 gap-2' : viewMode === 'compact' ? 'grid grid-cols-2 gap-2 lg:grid-cols-4' : 'grid gap-3 sm:grid-cols-2 xl:grid-cols-3'}>
      {#each filteredChannels as entry (entry.channel.id)}
        <article use:registerChannelContextMenu={entry} class="flex gap-3 rounded-xl border p-4 {selectedIDs.has(entry.channel.id) ? 'border-primary bg-primary/5' : 'border-border bg-surface'}">
          <input type="checkbox" checked={selectedIDs.has(entry.channel.id)} on:change={() => toggleSelection(entry.channel.id)} aria-label={`Selecionar ${entry.channel.title}`} class="mt-1 accent-primary tv-focusable" />
          <div class="flex h-10 w-10 shrink-0 items-center justify-center overflow-hidden rounded-full bg-primary/15 text-xs font-bold text-primary" aria-hidden="true">{#if entry.channel.thumbnail_url}<img src={entry.channel.thumbnail_url} alt="" loading="lazy" class="h-full w-full object-cover" />{:else}{entry.channel.title.slice(0, 2).toLocaleUpperCase()}{/if}</div>
          <button type="button" class="min-w-0 flex-1 text-left tv-focusable" on:click={() => toggleSelection(entry.channel.id)}>
            <h2 class="truncate text-sm font-semibold text-foreground">{entry.channel.title}</h2>
            <p class="truncate text-[11px] text-muted">{entry.category || 'Sem categoria automática'}</p>
            <p class="mt-2 text-[10px] text-muted">Inscrito: {formatDate(entry.subscribed_at)} · Último visto: {formatDate(entry.last_watched_at)}</p>
            {#if entry.tags.length}<div class="mt-2 flex flex-wrap gap-1">{#each entry.tags as channelTag}<span class="rounded-full bg-surfaceHover px-2 py-0.5 text-[10px] text-muted">{channelTag}</span>{/each}</div>{/if}
          </button>
          <button type="button" aria-label={`Mais ações para ${entry.channel.title}`} aria-haspopup="menu" title="Mais ações" on:click={(event) => openChannelContext(event, entry)} class="self-start rounded-lg p-2 text-muted hover:bg-surfaceHover hover:text-foreground tv-focusable"><MoreVertical size={15} /></button>
          <button type="button" on:click={() => openChannel({ id: entry.channel.id, title: entry.channel.title, thumbnailUrl: entry.channel.thumbnail_url })} aria-label={`Abrir canal ${entry.channel.title}`} title="Ver canal" class="self-start rounded-lg p-2 text-muted hover:bg-surfaceHover hover:text-primary tv-focusable"><ExternalLink size={15} /></button>
        </article>
      {/each}
    </div>
  {/if}
</section>

<ContextMenu open={contextChannel !== null} x={contextMenuX} y={contextMenuY} actions={contextActions} on:select={runChannelContextAction} on:close={() => { contextChannel = null; contextMenuTrigger?.focus(); }} />

<Modal title="Editar tags dos canais" bind:open={showTagEditor}>
  <div class="flex flex-col gap-4 text-xs"><label class="flex flex-col gap-2"><span>Tags separadas por vírgula</span><input bind:value={tagsText} maxlength="500" class="rounded-lg border border-border bg-background px-3 py-2 text-foreground" placeholder="Música, Tecnologia, Podcasts" /></label><div class="flex justify-end gap-2"><Button variant="ghost" on:click={() => (showTagEditor = false)}>Cancelar</Button><Button on:click={applyTags} disabled={isApplying}>Salvar tags</Button></div></div>
</Modal>

<Modal title="Confirmar desinscrição" bind:open={showUnsubscribeConfirmation}>
  <div class="flex flex-col gap-4 text-xs"><p>Você será desinscrito de {selectedIDs.size} canais. O catálogo já salvo não será apagado.</p><div class="flex justify-end gap-2"><Button variant="ghost" on:click={() => (showUnsubscribeConfirmation = false)}>Cancelar</Button><Button variant="danger" on:click={async () => { await runBulkAction(() => CatalogService.bulkUnsubscribeChannels([...selectedIDs]), 'Canais removidos das inscrições.'); showUnsubscribeConfirmation = false; selectedIDs = new Set(); }}>Desinscrever</Button></div></div>
</Modal>

<Modal title="Criar pasta de canais" bind:open={showCreateFolder}>
  <form class="flex flex-col gap-4 text-xs" on:submit|preventDefault={createFolder}>
    <label class="flex flex-col gap-2">Nome da pasta<input bind:value={newFolderName} maxlength="80" placeholder="Nome da pasta" class="rounded-lg border border-border bg-background px-3 py-2 text-foreground" /></label>
    <div class="flex justify-end gap-2"><Button type="button" variant="ghost" on:click={() => (showCreateFolder = false)}>Cancelar</Button><Button type="submit" disabled={isApplying || !newFolderName.trim()}>Criar pasta</Button></div>
  </form>
</Modal>

<Modal title="Adicionar canal" bind:open={showAddChannel}>
  <form class="flex flex-col gap-4 text-xs" on:submit|preventDefault={addChannel}>
    <label class="flex flex-col gap-2">ID do canal<input bind:value={newChannelID} maxlength="128" placeholder="UC…" class="rounded-lg border border-border bg-background px-3 py-2 text-foreground" /></label>
    <label class="flex flex-col gap-2">Nome do canal<input bind:value={newChannelTitle} maxlength="120" placeholder="Nome exibido" class="rounded-lg border border-border bg-background px-3 py-2 text-foreground" /></label>
    <div class="flex justify-end gap-2"><Button type="button" variant="ghost" on:click={() => (showAddChannel = false)}>Cancelar</Button><Button type="submit" disabled={isApplying || !newChannelID.trim() || !newChannelTitle.trim()}>Adicionar canal</Button></div>
  </form>
</Modal>
