<script lang="ts">
  import { onMount } from 'svelte';
  import { PlaylistService, SettingsService } from '../../wailsjs/services';
  import type { Playlist } from '../../types';
  import { activePlaylistId, toast } from '../../stores/uiStores';
  import { selectRemotePlaylistReference } from '../../stores/searchQueryStore';
  import ListMusic from 'lucide-svelte/icons/list-music';
  import Plus from 'lucide-svelte/icons/plus';
  import Sparkles from 'lucide-svelte/icons/sparkles';
  import ListPlus from 'lucide-svelte/icons/list-plus';
  import ExternalLink from 'lucide-svelte/icons/external-link';
import Button from '../ui/Button.svelte';
  import Modal from '../ui/Modal.svelte';
  import ConfirmDialog from '../ui/ConfirmDialog.svelte';
  import Input from '../ui/Input.svelte';
  import ContextMenu from '../ui/ContextMenu.svelte';
  import ViewModeToggle from '../ui/ViewModeToggle.svelte';
  import Pencil from 'lucide-svelte/icons/pencil';
  import Trash2 from 'lucide-svelte/icons/trash-2';
  import CopyPlus from 'lucide-svelte/icons/copy-plus';
  import type { ContentViewMode } from '../../stores/uiStores';

  let playlists: Playlist[] = [];
  let showCreateModal = false;
  let newName = '';
  let newDesc = '';
  let remotePlaylistInput = '';
  let contextPlaylist: Playlist | null = null;
  let contextMenuX = 0;
  let contextMenuY = 0;
  let contextMenuTrigger: HTMLElement | null = null;
  let viewMode: ContentViewMode = 'grid';
  let editingPlaylist: Playlist | null = null;
  let editName = '';
  let editDescription = '';
  let mergingPlaylist: Playlist | null = null;
  let mergeTargetID = '';
  let pendingConfirm: { kind: 'delete'; playlist: Playlist; trigger: HTMLElement | null } | null = null;

  $: contextActions = contextPlaylist ? [
    { id: 'open', label: 'Abrir playlist' },
    { id: 'edit', label: 'Editar nome e descrição' },
    { id: 'merge', label: 'Adicionar conteúdo à playlist existente' },
    { id: 'delete', label: 'Excluir playlist', danger: true },
  ] : [];

  function errorMessage(error: unknown): string {
    return error instanceof Error && error.message ? error.message : 'falha desconhecida';
  }

  async function loadPlaylists() {
    try {
      playlists = (await PlaylistService.listPlaylists()) || [];
    } catch (e) {
      console.error('Erro ao listar playlists:', e);
      playlists = [];
    }
  }

  async function handleCreate() {
    if (!newName.trim()) return;
    try {
      await PlaylistService.createPlaylist(newName.trim(), newDesc.trim());
      toast.add(`Playlist "${newName.trim()}" criada!`, 'success');
      newName = '';
      newDesc = '';
      showCreateModal = false;
      await loadPlaylists();
    } catch (error: unknown) {
      toast.add('Erro ao criar playlist: ' + errorMessage(error), 'error');
    }
  }

  function changeViewMode(mode: ContentViewMode): void {
    viewMode = mode;
    void SettingsService.saveSetting('playlist_view_mode', mode).catch(() => undefined);
  }

  function openEdit(playlist: Playlist): void {
    editingPlaylist = playlist;
    editName = playlist.name;
    editDescription = playlist.description || '';
  }

  async function saveEdit(): Promise<void> {
    if (!editingPlaylist || !editName.trim()) return;
    await PlaylistService.updatePlaylist(editingPlaylist.id, editName.trim(), editDescription.trim(), editingPlaylist.color || '');
    toast.add('Playlist atualizada.', 'success');
    editingPlaylist = null;
    await loadPlaylists();
  }

  async function mergeIntoPlaylist(): Promise<void> {
    if (!mergingPlaylist || !mergeTargetID) return;
    await PlaylistService.mergePlaylist(mergingPlaylist.id, mergeTargetID);
    toast.add('Conteúdo adicionado sem duplicar vídeos.', 'success');
    mergingPlaylist = null;
    mergeTargetID = '';
    await loadPlaylists();
  }

  function remotePlaylistID(value: string): string | null {
    const normalized = value.trim();
    if (!normalized) return null;
    try {
      const parsed = new URL(normalized);
      const hostname = parsed.hostname.toLowerCase();
      if (hostname !== 'youtube.com' && !hostname.endsWith('.youtube.com') && hostname !== 'youtu.be') return null;
      return parsed.searchParams.get('list')?.trim() || null;
    } catch {
      return /^[A-Za-z0-9_-]{1,128}$/.test(normalized) ? normalized : null;
    }
  }

  function openRemotePlaylist(): void {
    const playlistID = remotePlaylistID(remotePlaylistInput);
    if (!playlistID) {
      toast.add('Informe um ID ou URL de playlist do YouTube válida.', 'error');
      return;
    }
    const routeID = selectRemotePlaylistReference({ id: playlistID, title: 'Playlist do YouTube' });
    if (routeID) $activePlaylistId = routeID;
  }

  function openPlaylistContext(event: MouseEvent | KeyboardEvent, playlist: Playlist): void {
    event.preventDefault();
    contextPlaylist = playlist;
    contextMenuTrigger = event.currentTarget as HTMLElement;
    const bounds = contextMenuTrigger.getBoundingClientRect();
    contextMenuX = event instanceof MouseEvent ? event.clientX : bounds.left + 24;
    contextMenuY = event instanceof MouseEvent ? event.clientY : bounds.top + 24;
  }

  async function runPlaylistContextAction(event: CustomEvent<string>): Promise<void> {
    const playlist = contextPlaylist;
    if (!playlist) return;
    if (event.detail === 'open') $activePlaylistId = playlist.id;
    if (event.detail === 'edit') openEdit(playlist);
    if (event.detail === 'merge') { mergingPlaylist = playlist; mergeTargetID = ''; }
    if (event.detail === 'delete') {
      pendingConfirm = { kind: 'delete', playlist, trigger: contextMenuTrigger };
      contextPlaylist = null;
    }
  }

  async function applyDeletePlaylist(): Promise<void> {
    const playlist = pendingConfirm?.playlist;
    if (!playlist) return;
    try {
      await PlaylistService.deletePlaylist(playlist.id);
      toast.add('Playlist excluída.', 'success');
      await loadPlaylists();
    } catch (error: unknown) {
      toast.add('Não foi possível excluir a playlist: ' + errorMessage(error), 'error');
    }
  }

  onMount(() => {
    void SettingsService.getSettings().then((settings) => {
      if (['grid', 'list', 'compact'].includes(settings.playlist_view_mode)) viewMode = settings.playlist_view_mode as ContentViewMode;
    }).finally(loadPlaylists);
  });
</script>

<div class="flex flex-col gap-6 p-6 max-w-6xl mx-auto w-full" role="region" aria-label="Gerenciador de Playlists">
  <div class="flex items-center justify-between">
    <div>
      <h1 class="text-xl font-bold text-foreground">Playlists Locais</h1>
      <p class="text-xs text-muted mt-0.5">Organize coleções no SQLite ou abra uma playlist pública do YouTube sem importá-la</p>
    </div>

    <div class="flex items-center gap-2"><ViewModeToggle value={viewMode} onChange={changeViewMode} /><Button variant="primary" size="sm" on:click={() => (showCreateModal = true)}><Plus size={16} /> Nova Playlist</Button></div>
  </div>

  <form on:submit|preventDefault={openRemotePlaylist} class="flex flex-col gap-2 rounded-xl border border-border bg-surface/50 p-4 sm:flex-row sm:items-end" aria-label="Abrir playlist remota">
    <label for="remote-playlist-reference" class="flex flex-1 flex-col gap-1 text-xs text-muted">
      ID ou URL de playlist do YouTube
      <input id="remote-playlist-reference" bind:value={remotePlaylistInput} placeholder="https://www.youtube.com/playlist?list=..." autocomplete="off" spellcheck="false" class="rounded-lg border border-border bg-surface px-3 py-2 text-sm text-foreground placeholder:text-muted focus:border-primary focus:outline-none tv-focusable" />
    </label>
    <Button type="submit" variant="secondary" size="sm"><ExternalLink size={14} /> Abrir remota</Button>
  </form>

  {#if (playlists?.length || 0) === 0}
    <div class="text-center py-16 px-6 text-muted text-xs bg-surface/30 rounded-2xl border border-dashed border-border flex flex-col items-center justify-center gap-3">
      <div class="w-12 h-12 rounded-xl bg-surfaceHover text-muted flex items-center justify-center">
        <ListMusic size={24} />
      </div>
      <div>
        <h4 class="text-sm font-bold text-foreground">Nenhuma playlist criada ainda</h4>
        <p class="text-xs text-muted max-w-md mt-1 leading-relaxed">
          Organize seus vídeos favoritos em coleções personalizadas ou playlists com regras automatizadas.
        </p>
      </div>
      <div class="flex items-center gap-2 mt-2">
        <Button variant="primary" size="sm" on:click={() => (showCreateModal = true)}>
          <ListPlus size={14} /> Criar Primeira Playlist
        </Button>
      </div>
    </div>
  {:else}
    <div class={viewMode === 'list' ? 'grid grid-cols-1 gap-2' : viewMode === 'compact' ? 'grid grid-cols-2 gap-2 md:grid-cols-4 xl:grid-cols-5' : 'grid grid-cols-1 gap-4 sm:grid-cols-2 md:grid-cols-3'}>
      {#each playlists as pl (pl.id)}
        <article
          class="relative flex justify-between gap-3 p-4 bg-surface border border-border rounded-xl hover:border-primary/50 transition-all group text-left w-full {viewMode === 'grid' ? 'flex-col' : 'items-center'}"
        >
          <button type="button" aria-label={`Abrir playlist ${pl.name}`} aria-haspopup="menu" on:click={() => ($activePlaylistId = pl.id)} on:contextmenu={(event) => openPlaylistContext(event, pl)} on:keydown={(event) => { if (event.key === 'ContextMenu' || event.key === 'Apps' || (event.shiftKey && event.key === 'F10')) openPlaylistContext(event, pl); }} class="absolute inset-0 rounded-xl tv-focusable" />
          <div class="pointer-events-none flex items-start justify-between w-full">
            <div class="w-10 h-10 rounded-lg bg-primary/10 text-primary flex items-center justify-center">
              {#if pl.is_smart}
                <Sparkles size={20} />
              {:else}
                <ListMusic size={20} />
              {/if}
            </div>
            <span class="text-xs font-semibold px-2 py-0.5 rounded bg-surfaceHover text-muted">
              {pl.item_count} {pl.item_count === 1 ? 'vídeo' : 'vídeos'}
            </span>
          </div>

          <div class="pointer-events-none {viewMode === 'grid' ? 'mt-4' : ''} min-w-0 w-full">
            <h3 class="text-sm font-bold text-foreground group-hover:text-primary transition-colors truncate">{pl.name}</h3>
            {#if pl.description}
              <p class="text-xs text-muted truncate mt-0.5">{pl.description}</p>
            {/if}
          </div>
          <div class="relative z-[2] ml-auto flex shrink-0 gap-1 opacity-0 transition-opacity group-hover:opacity-100 focus-within:opacity-100">
            <button type="button" on:click={() => openEdit(pl)} aria-label={`Editar ${pl.name}`} title="Editar" class="rounded-lg p-2 text-muted hover:bg-surfaceHover hover:text-primary tv-focusable"><Pencil size={14} /></button>
            <button type="button" on:click={() => { contextPlaylist = pl; void runPlaylistContextAction(new CustomEvent('select', { detail: 'delete' })); }} aria-label={`Excluir ${pl.name}`} title="Excluir" class="rounded-lg p-2 text-muted hover:bg-red-500/10 hover:text-red-400 tv-focusable"><Trash2 size={14} /></button>
          </div>
        </article>
      {/each}
    </div>
  {/if}
</div>

<ContextMenu open={contextPlaylist !== null} x={contextMenuX} y={contextMenuY} actions={contextActions} on:select={runPlaylistContextAction} on:close={() => { contextPlaylist = null; contextMenuTrigger?.focus(); }} />

<Modal title="Criar Nova Playlist" bind:open={showCreateModal} on:close={() => (showCreateModal = false)}>
  <form on:submit|preventDefault={handleCreate} class="flex flex-col gap-4">
    <Input bind:value={newName} placeholder="Nome da playlist..." label="Nome da Playlist" />
    <Input bind:value={newDesc} placeholder="Descrição opcional..." label="Descrição" />
    <div class="flex justify-end gap-2">
      <Button variant="ghost" size="sm" on:click={() => (showCreateModal = false)}>Cancelar</Button>
      <Button variant="primary" size="sm" type="submit">Criar</Button>
    </div>
  </form>
</Modal>

<Modal title="Editar playlist" open={editingPlaylist !== null} on:close={() => (editingPlaylist = null)}>
  <form on:submit|preventDefault={saveEdit} class="flex flex-col gap-4"><Input bind:value={editName} label="Nome" /><Input bind:value={editDescription} label="Descrição" /><div class="flex justify-end gap-2"><Button type="button" variant="ghost" on:click={() => (editingPlaylist = null)}>Cancelar</Button><Button type="submit" disabled={!editName.trim()}><Pencil size={14} /> Salvar</Button></div></form>
</Modal>

<Modal title="Adicionar conteúdo à playlist" open={mergingPlaylist !== null} on:close={() => (mergingPlaylist = null)}>
  <form on:submit|preventDefault={mergeIntoPlaylist} class="flex flex-col gap-4 text-xs"><p class="text-muted">Todos os vídeos de <strong class="text-foreground">{mergingPlaylist?.name}</strong> serão adicionados ao destino. Duplicados serão descartados.</p><label class="flex flex-col gap-2">Playlist de destino<select bind:value={mergeTargetID} class="rounded-lg border border-border bg-surface px-3 py-2 text-foreground"><option value="">Selecione…</option>{#each playlists.filter((playlist) => playlist.id !== mergingPlaylist?.id) as playlist}<option value={playlist.id}>{playlist.name}</option>{/each}</select></label><div class="flex justify-end gap-2"><Button type="button" variant="ghost" on:click={() => (mergingPlaylist = null)}>Cancelar</Button><Button type="submit" disabled={!mergeTargetID}><CopyPlus size={14} /> Adicionar sem duplicar</Button></div></form>
</Modal>

<ConfirmDialog
  open={pendingConfirm !== null}
  trigger={pendingConfirm?.trigger ?? null}
  danger
  title="Excluir playlist"
  description={`Excluir a playlist “${pendingConfirm?.playlist?.name || ''}”? Os vídeos não serão removidos da biblioteca.`}
  confirmLabel="Excluir"
  cancelLabel="Cancelar"
  on:confirm={async () => {
    pendingConfirm = null;
    await applyDeletePlaylist();
  }}
  on:close={() => (pendingConfirm = null)}
/>
