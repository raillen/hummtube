<script lang="ts">
  import { onMount } from 'svelte';
  import { CatalogService } from '../../wailsjs/services';
  import type { Channel, ChannelFolder, HomeModel, Video } from '../../types';
  import Folder from 'lucide-svelte/icons/folder';
  import FolderPlus from 'lucide-svelte/icons/folder-plus';
  import Star from 'lucide-svelte/icons/star';
  import Trash2 from 'lucide-svelte/icons/trash-2';
  import UserPlus from 'lucide-svelte/icons/user-plus';
  import RefreshCw from 'lucide-svelte/icons/refresh-cw';
  import Search from 'lucide-svelte/icons/search';
  import Users from 'lucide-svelte/icons/users';
  import VideoIcon from 'lucide-svelte/icons/video';
  import AlertTriangle from 'lucide-svelte/icons/triangle-alert';
  import Button from '../ui/Button.svelte';
  import Modal from '../ui/Modal.svelte';
  import Input from '../ui/Input.svelte';
  import Chip from '../ui/Chip.svelte';
  import VideoGrid from '../video/VideoGrid.svelte';
  import { toast } from '../../stores/uiStores';

  type SubscriptionSection = 'videos' | 'channels';

  let channels: Channel[] = [];
  let subscriptionVideos: Video[] = [];
  let folders: ChannelFolder[] = [];
  let favoriteChannelIds: Record<string, boolean> = {};
  let folderMembership: Record<string, string[]> = {};

  let activeSection: SubscriptionSection = 'videos';
  let selectedFolderId: string | null = null;
  let filterFavorites = false;
  let searchQuery = '';

  let showNewFolderModal = false;
  let showAddChannelModal = false;
  let newFolderName = '';
  let newChannelId = '';
  let newChannelTitle = '';
  let isLoading = true;
  let isRefreshing = false;
  let loadError: string | null = null;

  function errorMessage(error: unknown, fallback: string): string {
    return error instanceof Error && error.message ? error.message : fallback;
  }

  function addPersistedChannelTitles(videos: Video[], subscribedChannels: Channel[]): Video[] {
    const channelTitles = new Map(subscribedChannels.map((channel) => [channel.id, channel.title]));
    return videos.map((video) => ({
      ...video,
      channel_title: video.channel_title || channelTitles.get(video.channel_id) || 'Canal inscrito'
    }));
  }

  async function loadData() {
    isLoading = true;
    loadError = null;
    try {
      const [home, loadedChannels, loadedFolders, loadedFavorites, loadedMembership] = await Promise.all([
        CatalogService.getHome(),
        CatalogService.getChannels(),
        CatalogService.getChannelFolders(),
        CatalogService.getFavoriteChannelIDs(),
        CatalogService.getFolderMembership()
      ]);
      const homeModel: HomeModel = home;
      channels = loadedChannels || [];
      subscriptionVideos = addPersistedChannelTitles(homeModel.recent_subscriptions || [], channels);
      folders = loadedFolders || [];
      favoriteChannelIds = loadedFavorites || {};
      folderMembership = loadedMembership || {};
    } catch (error: unknown) {
      loadError = errorMessage(error, 'Falha ao carregar suas inscrições.');
      console.error('Erro ao carregar dados de inscrições:', error);
    } finally {
      isLoading = false;
    }
  }

  async function handleRefresh() {
    isRefreshing = true;
    try {
      await CatalogService.refreshSubscriptions();
      await loadData();
      toast.add('Inscrições atualizadas com sucesso!', 'success');
    } catch (error: unknown) {
      toast.add(errorMessage(error, 'Erro ao atualizar inscrições'), 'error');
    } finally {
      isRefreshing = false;
    }
  }

  async function handleCreateFolder() {
    if (!newFolderName.trim()) return;
    try {
      await CatalogService.createChannelFolder(newFolderName.trim());
      toast.add(`Pasta "${newFolderName.trim()}" criada`, 'success');
      newFolderName = '';
      showNewFolderModal = false;
      await loadData();
    } catch (error: unknown) {
      toast.add(errorMessage(error, 'Erro ao criar pasta'), 'error');
    }
  }

  async function handleDeleteFolder(id: string) {
    try {
      await CatalogService.deleteChannelFolder(id);
      if (selectedFolderId === id) selectedFolderId = null;
      toast.add('Pasta removida', 'info');
      await loadData();
    } catch (error: unknown) {
      toast.add(errorMessage(error, 'Erro ao remover pasta'), 'error');
    }
  }

  async function handleAddChannel() {
    if (!newChannelId.trim()) return;
    try {
      await CatalogService.subscribeChannel(newChannelId.trim(), newChannelTitle.trim());
      toast.add('Inscrição adicionada!', 'success');
      newChannelId = '';
      newChannelTitle = '';
      showAddChannelModal = false;
      await loadData();
    } catch (error: unknown) {
      toast.add(errorMessage(error, 'Erro ao adicionar canal'), 'error');
    }
  }

  async function handleToggleFavorite(channelId: string, e: MouseEvent) {
    e.stopPropagation();
    const isFav = !!favoriteChannelIds[channelId];
    try {
      if (isFav) {
        await CatalogService.removeChannelFavorite(channelId);
        favoriteChannelIds[channelId] = false;
        toast.add('Canal removido dos favoritos', 'info');
      } else {
        await CatalogService.addChannelFavorite(channelId);
        favoriteChannelIds[channelId] = true;
        toast.add('Canal marcado como favorito!', 'success');
      }
      favoriteChannelIds = { ...favoriteChannelIds };
    } catch (error: unknown) {
      toast.add(errorMessage(error, 'Erro ao atualizar favorito'), 'error');
    }
  }

  $: filteredChannels = (channels || []).filter(ch => {
    if (filterFavorites && !favoriteChannelIds[ch.id]) return false;
    if (selectedFolderId) {
      const memberIds = folderMembership[selectedFolderId] || [];
      if (!memberIds.includes(ch.id)) return false;
    }
    if (searchQuery.trim()) {
      const q = searchQuery.toLowerCase().trim();
      return ch.title.toLowerCase().includes(q) || ch.id.toLowerCase().includes(q);
    }
    return true;
  });

  onMount(() => {
    void loadData();
  });
</script>

<section class="flex flex-col gap-6 p-6 max-w-7xl mx-auto w-full" aria-labelledby="subscriptions-heading">
  <header class="flex flex-wrap items-center justify-between gap-4">
    <div class="flex items-center gap-3">
      <div class="w-10 h-10 rounded-xl bg-primary/10 text-primary flex items-center justify-center">
        <Users size={22} />
      </div>
      <div>
        <h1 id="subscriptions-heading" class="text-xl font-bold text-foreground">Canais &amp; Inscrições</h1>
        <p class="text-xs text-muted mt-0.5">Assista aos vídeos sincronizados e organize seus canais locais</p>
      </div>
    </div>

    <div class="flex items-center gap-2">
      <Button variant="secondary" size="sm" on:click={handleRefresh} disabled={isRefreshing}>
        <RefreshCw size={14} class={isRefreshing ? 'animate-spin' : ''} />
        Atualizar Feeds
      </Button>

      <Button variant="secondary" size="sm" on:click={() => (showNewFolderModal = true)}>
        <FolderPlus size={14} /> Nova Pasta
      </Button>

      <Button variant="primary" size="sm" on:click={() => (showAddChannelModal = true)}>
        <UserPlus size={14} /> Adicionar Canal
      </Button>
    </div>
  </header>

  <nav class="flex items-center gap-2 border-b border-border" aria-label="Conteúdo das inscrições">
    <button
      type="button"
      class="px-3 py-2 text-xs font-semibold border-b-2 transition-colors tv-focusable {activeSection === 'videos' ? 'border-primary text-foreground' : 'border-transparent text-muted hover:text-foreground'}"
      aria-current={activeSection === 'videos' ? 'page' : undefined}
      on:click={() => (activeSection = 'videos')}
    >
      <VideoIcon size={14} class="mr-1.5 inline" />
      Vídeos recentes ({subscriptionVideos.length})
    </button>
    <button
      type="button"
      class="px-3 py-2 text-xs font-semibold border-b-2 transition-colors tv-focusable {activeSection === 'channels' ? 'border-primary text-foreground' : 'border-transparent text-muted hover:text-foreground'}"
      aria-current={activeSection === 'channels' ? 'page' : undefined}
      on:click={() => (activeSection = 'channels')}
    >
      <Users size={14} class="mr-1.5 inline" />
      Gerenciar canais ({channels.length})
    </button>
  </nav>

  {#if loadError}
    <div role="alert" class="p-4 bg-red-500/10 border border-red-500/30 rounded-2xl flex flex-wrap items-center justify-between gap-4 text-xs text-red-300">
      <div class="flex items-center gap-3">
        <AlertTriangle size={18} class="text-red-400 shrink-0" />
        <div>
          <strong class="text-foreground block">Não foi possível carregar suas inscrições</strong>
          <span>{loadError}</span>
        </div>
      </div>
      <Button variant="secondary" size="sm" on:click={loadData}>Tentar novamente</Button>
    </div>
  {/if}

  {#if isLoading}
    <div role="status" aria-live="polite" class="flex flex-col gap-4">
      <span class="text-xs text-muted">Carregando vídeos das suas inscrições…</span>
      <div class="grid grid-cols-1 sm:grid-cols-2 md:grid-cols-3 lg:grid-cols-4 xl:grid-cols-5 gap-4 animate-pulse" aria-hidden="true">
        {#each Array(10) as _}
          <div class="flex flex-col gap-2">
            <div class="w-full aspect-video rounded-xl bg-surfaceHover"></div>
            <div class="h-4 bg-surfaceHover rounded w-3/4"></div>
            <div class="h-3 bg-surfaceHover rounded w-1/2"></div>
          </div>
        {/each}
      </div>
    </div>
  {:else if !loadError && activeSection === 'videos'}
    <section class="flex flex-col gap-3" aria-labelledby="subscription-videos-heading">
      <div>
        <h2 id="subscription-videos-heading" class="text-base font-bold text-foreground">Vídeos dos canais inscritos</h2>
        <p class="text-xs text-muted mt-1">Conteúdo salvo no catálogo local durante a última sincronização.</p>
      </div>

      {#if subscriptionVideos.length === 0}
        <div class="text-center py-16 px-6 bg-surface/30 rounded-2xl border border-dashed border-border flex flex-col items-center justify-center gap-3">
          <div class="w-12 h-12 rounded-xl bg-surfaceHover text-muted flex items-center justify-center">
            <VideoIcon size={24} />
          </div>
          <div>
            <h3 class="text-sm font-bold text-foreground">
              {channels.length === 0 ? 'Você ainda não tem inscrições' : 'Nenhum vídeo sincronizado'}
            </h3>
            <p class="text-xs text-muted max-w-md mt-1 leading-relaxed">
              {#if channels.length === 0}
                Adicione ou importe canais para montar seu feed de inscrições.
              {:else}
                Seus canais já estão salvos. Atualize os feeds para buscar os vídeos mais recentes.
              {/if}
            </p>
          </div>
          <Button
            variant="primary"
            size="sm"
            on:click={channels.length === 0 ? () => (showAddChannelModal = true) : handleRefresh}
            disabled={isRefreshing}
          >
            {#if channels.length === 0}
              <UserPlus size={14} /> Adicionar canal
            {:else}
              <RefreshCw size={14} class={isRefreshing ? 'animate-spin' : ''} /> Atualizar feeds
            {/if}
          </Button>
        </div>
      {:else}
        <VideoGrid videos={subscriptionVideos} />
      {/if}
    </section>
  {:else if !loadError}
    <section class="flex flex-col gap-6" aria-labelledby="channel-management-heading">
      <h2 id="channel-management-heading" class="sr-only">Gerenciar canais inscritos</h2>

      <div class="flex flex-col sm:flex-row items-center gap-3">
        <label class="relative flex-1 w-full">
          <span class="sr-only">Filtrar canais inscritos</span>
          <Search class="absolute left-3 top-2.5 text-muted pointer-events-none" size={16} />
          <input
            type="search"
            bind:value={searchQuery}
            placeholder="Filtrar canais inscritos..."
            class="w-full bg-surface border border-border rounded-xl pl-9 pr-4 py-2 text-xs text-foreground placeholder:text-muted focus:outline-none focus:border-primary tv-focusable"
          />
        </label>

        <div class="flex items-center gap-2 shrink-0 overflow-x-auto w-full sm:w-auto" aria-label="Filtros de canais">
          <Chip active={!filterFavorites && selectedFolderId === null} on:click={() => { filterFavorites = false; selectedFolderId = null; }}>
            Todos ({channels.length})
          </Chip>
          <Chip active={filterFavorites} on:click={() => { filterFavorites = !filterFavorites; selectedFolderId = null; }}>
            <Star size={12} class="mr-1 inline text-amber-400 fill-amber-400" /> Favoritos
          </Chip>
        </div>
      </div>

      {#if folders.length > 0}
        <section class="flex flex-col gap-2" aria-labelledby="local-folders-heading">
          <h3 id="local-folders-heading" class="text-xs font-bold uppercase tracking-wider text-muted px-1">Pastas locais</h3>
          <div class="grid grid-cols-1 sm:grid-cols-2 md:grid-cols-3 gap-3">
            {#each folders as folder (folder.id)}
              <article class="flex items-center bg-surface border rounded-xl transition-colors {selectedFolderId === folder.id ? 'border-primary bg-primary/5' : 'border-border hover:border-primary/40'}">
                <button
                  type="button"
                  class="flex flex-1 items-center gap-2.5 min-w-0 p-3 text-left tv-focusable"
                  aria-pressed={selectedFolderId === folder.id}
                  on:click={() => { selectedFolderId = selectedFolderId === folder.id ? null : folder.id; filterFavorites = false; }}
                >
                  <Folder size={16} class={selectedFolderId === folder.id ? 'text-primary' : 'text-muted'} />
                  <span class="text-xs font-semibold text-foreground truncate">{folder.name}</span>
                  <span class="text-[10px] text-muted px-1.5 py-0.5 rounded bg-surfaceHover">
                    {(folderMembership[folder.id] || []).length}
                  </span>
                </button>
                <button
                  type="button"
                  on:click={() => handleDeleteFolder(folder.id)}
                  class="p-2 mr-1 text-muted hover:text-red-400 rounded-lg hover:bg-surfaceHover tv-focusable"
                  aria-label={`Excluir pasta ${folder.name}`}
                >
                  <Trash2 size={13} />
                </button>
              </article>
            {/each}
          </div>
        </section>
      {/if}

      <section class="flex flex-col gap-2" aria-labelledby="channel-list-heading">
        <h3 id="channel-list-heading" class="text-xs font-bold uppercase tracking-wider text-muted px-1">
          {#if selectedFolderId}
            Canais na pasta ({filteredChannels.length})
          {:else if filterFavorites}
            Canais favoritos ({filteredChannels.length})
          {:else}
            Todos os canais ({filteredChannels.length})
          {/if}
        </h3>

        {#if filteredChannels.length === 0}
          <div class="text-center py-16 px-6 bg-surface/30 rounded-2xl border border-dashed border-border flex flex-col items-center justify-center gap-3">
            <div class="w-12 h-12 rounded-xl bg-surfaceHover text-muted flex items-center justify-center">
              <Users size={24} />
            </div>
            <div>
              <h4 class="text-sm font-bold text-foreground">Nenhum canal encontrado</h4>
              <p class="text-xs text-muted max-w-md mt-1 leading-relaxed">
                {#if searchQuery}
                  Nenhum canal corresponde ao termo de busca "{searchQuery}".
                {:else}
                  Adicione canais manualmente ou importe Google Takeout CSV, OPML ou NewPipe JSON.
                {/if}
              </p>
            </div>
            {#if !searchQuery}
              <Button variant="primary" size="sm" on:click={() => (showAddChannelModal = true)}>
                <UserPlus size={14} /> Adicionar canal
              </Button>
            {/if}
          </div>
        {:else}
          <div class="grid grid-cols-1 sm:grid-cols-2 md:grid-cols-3 gap-3">
            {#each filteredChannels as channel (channel.id)}
              <article class="flex items-center justify-between p-3 bg-surface border border-border rounded-xl hover:border-primary/40 transition-colors group">
                <div class="flex items-center gap-3 min-w-0">
                  <div class="w-9 h-9 rounded-full bg-primary/10 text-primary flex items-center justify-center font-bold text-xs shrink-0 border border-primary/20" aria-hidden="true">
                    {channel.title ? channel.title.charAt(0).toUpperCase() : 'C'}
                  </div>
                  <div class="flex flex-col min-w-0">
                    <span class="text-xs font-semibold text-foreground truncate group-hover:text-primary transition-colors">
                      {channel.title || channel.id}
                    </span>
                    <span class="text-[10px] text-muted truncate">{channel.id}</span>
                  </div>
                </div>

                <button
                  type="button"
                  on:click={(event) => handleToggleFavorite(channel.id, event)}
                  class="p-1.5 rounded-lg text-muted hover:text-amber-400 hover:bg-surfaceHover transition-colors shrink-0 tv-focusable"
                  aria-label={favoriteChannelIds[channel.id] ? `Remover ${channel.title || channel.id} dos favoritos` : `Favoritar ${channel.title || channel.id}`}
                  aria-pressed={!!favoriteChannelIds[channel.id]}
                >
                  <Star
                    size={16}
                    fill={favoriteChannelIds[channel.id] ? 'currentColor' : 'none'}
                    class={favoriteChannelIds[channel.id] ? 'text-amber-400' : ''}
                  />
                </button>
              </article>
            {/each}
          </div>
        {/if}
      </section>
    </section>
  {/if}
</section>

<!-- Modal: Create Folder -->
<Modal title="Criar Pasta de Canais" bind:open={showNewFolderModal} on:close={() => (showNewFolderModal = false)}>
  <form on:submit|preventDefault={handleCreateFolder} class="flex flex-col gap-4">
    <Input bind:value={newFolderName} placeholder="Nome da pasta (ex: Podcasts, Tech, Música)..." label="Nome da Pasta" />
    <div class="flex justify-end gap-2">
      <Button variant="ghost" size="sm" on:click={() => (showNewFolderModal = false)}>Cancelar</Button>
      <Button variant="primary" size="sm" type="submit">Salvar Pasta</Button>
    </div>
  </form>
</Modal>

<!-- Modal: Add Channel -->
<Modal title="Adicionar Canal" bind:open={showAddChannelModal} on:close={() => (showAddChannelModal = false)}>
  <form on:submit|preventDefault={handleAddChannel} class="flex flex-col gap-4">
    <Input bind:value={newChannelId} placeholder="ID ou handle do canal (ex: UC..., @Canal)..." label="Identificador do Canal" />
    <Input bind:value={newChannelTitle} placeholder="Título opcional..." label="Título Amigável (Opcional)" />
    <div class="flex justify-end gap-2">
      <Button variant="ghost" size="sm" on:click={() => (showAddChannelModal = false)}>Cancelar</Button>
      <Button variant="primary" size="sm" type="submit">Inscrever</Button>
    </div>
  </form>
</Modal>
