<script lang="ts">
  import { createEventDispatcher } from 'svelte';
  import { PlaylistService } from '../../wailsjs/services';
  import type { Playlist, PlaylistDetail } from '../../types';
  import { toast } from '../../stores/uiStores';
  import Button from '../ui/Button.svelte';
  import Input from '../ui/Input.svelte';
  import Modal from '../ui/Modal.svelte';
  import Download from 'lucide-svelte/icons/download';
  import ListPlus from 'lucide-svelte/icons/list-plus';
  import Loader2 from 'lucide-svelte/icons/loader-circle';

  export let open = false;
  export let remoteID = '';
  export let suggestedName = '';
  export let suggestedDescription = '';

  let playlists: Playlist[] = [];
  let name = '';
  let description = '';
  let isBusy = false;
  let loadedForID = '';
  const dispatch = createEventDispatcher<{ saved: PlaylistDetail }>();

  $: if (open && remoteID && loadedForID !== remoteID) {
    loadedForID = remoteID;
    name = suggestedName || 'Playlist importada';
    description = suggestedDescription;
    void loadPlaylists();
  }
  $: if (!open) loadedForID = '';

  function errorMessage(error: unknown): string {
    return error instanceof Error && error.message ? error.message : 'Não foi possível salvar a playlist.';
  }

  async function loadPlaylists(): Promise<void> {
    try {
      playlists = (await PlaylistService.listPlaylists()) || [];
    } catch (error: unknown) {
      toast.add(errorMessage(error), 'error');
    }
  }

  async function saveAsNew(): Promise<void> {
    if (!remoteID || !name.trim() || isBusy) return;
    isBusy = true;
    try {
      const imported = await PlaylistService.importRemotePlaylist(remoteID, name.trim(), description.trim(), 500);
      toast.add(`Playlist salva com ${imported.playlist.item_count} vídeos.`, 'success');
      dispatch('saved', imported);
      open = false;
    } catch (error: unknown) {
      toast.add(errorMessage(error), 'error');
    } finally {
      isBusy = false;
    }
  }

  async function addToExisting(playlist: Playlist): Promise<void> {
    if (!remoteID || isBusy) return;
    isBusy = true;
    try {
      const added = await PlaylistService.addRemotePlaylistToPlaylist(remoteID, playlist.id, 500);
      toast.add(added > 0 ? `${added} vídeos adicionados a “${playlist.name}”.` : 'Nenhum vídeo novo: duplicados foram descartados.', added > 0 ? 'success' : 'info');
      open = false;
    } catch (error: unknown) {
      toast.add(errorMessage(error), 'error');
    } finally {
      isBusy = false;
    }
  }
</script>

<Modal title="Salvar playlist" bind:open on:close={() => (open = false)}>
  <div class="flex flex-col gap-5">
    <form class="flex flex-col gap-3 rounded-xl border border-border bg-surfaceHover/30 p-4" on:submit|preventDefault={saveAsNew}>
      <div>
        <h3 class="text-sm font-semibold text-foreground">Criar uma playlist local</h3>
        <p class="mt-0.5 text-xs text-muted">Nome e descrição ficam editáveis no HummTube.</p>
      </div>
      <Input bind:value={name} label="Nome" maxlength={120} />
      <Input bind:value={description} label="Descrição" maxlength={1000} />
      <Button type="submit" variant="primary" size="sm" disabled={!name.trim() || isBusy}>
        {#if isBusy}<Loader2 size={14} class="animate-spin" />{:else}<Download size={14} />{/if}
        Salvar como nova
      </Button>
    </form>

    <section class="flex flex-col gap-2" aria-labelledby="existing-playlists-heading">
      <div>
        <h3 id="existing-playlists-heading" class="text-sm font-semibold text-foreground">Adicionar a uma existente</h3>
        <p class="mt-0.5 text-xs text-muted">Os vídeos duplicados são ignorados automaticamente.</p>
      </div>
      {#if playlists.length === 0}
        <p class="rounded-xl border border-dashed border-border p-5 text-center text-xs text-muted">Nenhuma playlist local disponível.</p>
      {:else}
        <div class="max-h-56 space-y-2 overflow-y-auto pr-1">
          {#each playlists as playlist (playlist.id)}
            <button type="button" disabled={isBusy} on:click={() => addToExisting(playlist)} class="flex w-full items-center gap-3 rounded-xl border border-border bg-surface p-3 text-left hover:border-primary/50 hover:bg-surfaceHover disabled:opacity-50 tv-focusable">
              <ListPlus size={16} class="shrink-0 text-primary" />
              <span class="min-w-0 flex-1 truncate text-xs font-semibold text-foreground">{playlist.name}</span>
              <span class="text-[10px] text-muted">{playlist.item_count} vídeos</span>
            </button>
          {/each}
        </div>
      {/if}
    </section>
  </div>
</Modal>
