<script lang="ts">
  import Modal from '../ui/Modal.svelte';
  import Button from '../ui/Button.svelte';
  import Input from '../ui/Input.svelte';
  import { CatalogService, PlaylistService } from '../../wailsjs/services';
  import type { Playlist } from '../../types';
  import type { Video } from '../../types';
  import { toast } from '../../stores/uiStores';
  import ListPlus from 'lucide-svelte/icons/list-plus';
  import Plus from 'lucide-svelte/icons/plus';
  import Check from 'lucide-svelte/icons/check';
  import ListMusic from 'lucide-svelte/icons/list-music';

  export let open = false;
  export let video: Video | null = null;

  let playlists: Playlist[] = [];
  let showCreate = false;
  let newPlaylistName = '';
  let newPlaylistDescription = '';
  let isLoading = false;

  async function loadPlaylists() {
    isLoading = true;
    try {
      playlists = await PlaylistService.listPlaylists();
    } catch (e) {
      console.error('Erro ao carregar playlists:', e);
    } finally {
      isLoading = false;
    }
  }

  async function handleAddToPlaylist(playlistId: string, playlistName: string) {
    if (!video?.id) return;
    try {
      await CatalogService.rememberVideo(video);
      await PlaylistService.addVideoToPlaylist(playlistId, video.id);
      toast.add(`Vídeo adicionado a "${playlistName}"!`, 'success');
      open = false;
    } catch (e) {
      toast.add('Erro ao adicionar à playlist', 'error');
    }
  }

  async function handleCreateAndAdd() {
    if (!newPlaylistName.trim() || !video?.id) return;
    try {
      await CatalogService.rememberVideo(video);
      const pl = await PlaylistService.createPlaylist(newPlaylistName.trim(), newPlaylistDescription.trim());
      await PlaylistService.addVideoToPlaylist(pl.id, video.id);
      toast.add(`Playlist "${pl.name}" criada e vídeo adicionado!`, 'success');
      newPlaylistName = '';
      newPlaylistDescription = '';
      showCreate = false;
      open = false;
    } catch (e) {
      toast.add('Erro ao criar playlist', 'error');
    }
  }

  $: if (open) {
    loadPlaylists();
  }
</script>

<Modal title="Adicionar à Playlist" bind:open on:close={() => (open = false)}>
  <div class="flex flex-col gap-4 text-xs">
    {#if !showCreate}
      <div class="flex items-center justify-between">
        <span class="text-muted">Selecione uma playlist para salvar o vídeo:</span>
        <Button variant="secondary" size="sm" on:click={() => (showCreate = true)}>
          <Plus size={14} /> Nova Playlist
        </Button>
      </div>

      {#if playlists.length === 0}
        <div class="text-center py-8 text-muted bg-surfaceHover/30 rounded-xl border border-dashed border-border">
          Você ainda não possui nenhuma playlist local.
        </div>
      {:else}
        <div class="flex flex-col gap-2 max-h-60 overflow-y-auto">
          {#each playlists as pl (pl.id)}
            <button
              type="button"
              on:click={() => handleAddToPlaylist(pl.id, pl.name)}
              class="flex items-center justify-between p-3 bg-surfaceHover/50 border border-border rounded-xl hover:border-primary/50 text-left transition-colors tv-focusable group"
            >
              <div class="flex items-center gap-2.5 min-w-0">
                <ListMusic size={16} class="text-primary shrink-0" />
                <span class="text-xs font-semibold text-foreground truncate group-hover:text-primary transition-colors">
                  {pl.name}
                </span>
              </div>
              <span class="text-[10px] text-muted px-2 py-0.5 rounded bg-surface">
                {pl.item_count} vídeos
              </span>
            </button>
          {/each}
        </div>
      {/if}
    {:else}
      <!-- Create New Playlist Form -->
      <form on:submit|preventDefault={handleCreateAndAdd} class="flex flex-col gap-3">
        <h4 class="text-xs font-bold text-foreground">Criar Nova Playlist</h4>
        <Input
          bind:value={newPlaylistName}
          placeholder="Nome da playlist (ex: Assistir Mais Tarde, Favoritos)..."
          label="Nome da Playlist"
        />
        <Input bind:value={newPlaylistDescription} placeholder="Descrição opcional..." label="Descrição" />
        <div class="flex justify-end gap-2 mt-1">
          <Button variant="ghost" size="sm" on:click={() => (showCreate = false)}>Voltar</Button>
          <Button variant="primary" size="sm" type="submit">Criar e Salvar</Button>
        </div>
      </form>
    {/if}
  </div>
</Modal>
