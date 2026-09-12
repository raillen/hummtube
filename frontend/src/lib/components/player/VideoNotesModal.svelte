<script lang="ts">
  import { onMount } from 'svelte';
  import Modal from '../ui/Modal.svelte';
  import Button from '../ui/Button.svelte';
  import Input from '../ui/Input.svelte';
  import { LibraryService } from '../../wailsjs/services';
  import type { VideoBookmark } from '../../types';
  import { playerStore } from '../../stores/playerStore';
  import { toast } from '../../stores/uiStores';
  import Bookmark from 'lucide-svelte/icons/bookmark';
  import BookmarkPlus from 'lucide-svelte/icons/bookmark-plus';
  import StickyNote from 'lucide-svelte/icons/sticky-note';
  import Trash2 from 'lucide-svelte/icons/trash-2';
  import Clock from 'lucide-svelte/icons/clock';
  import Check from 'lucide-svelte/icons/check';
  import Save from 'lucide-svelte/icons/save';

  export let open = false;
  export let videoElement: HTMLVideoElement;

  let noteContent = '';
  let bookmarks: VideoBookmark[] = [];
  let newBookmarkLabel = '';
  let isLoading = false;
  let isSavingNote = false;
  let activeTab: 'notes' | 'bookmarks' = 'notes';

  async function loadData() {
    if (!$playerStore.currentVideo?.id) return;
    const vid = $playerStore.currentVideo.id;
    isLoading = true;
    try {
      noteContent = (await LibraryService.getNote(vid)) || '';
      bookmarks = (await LibraryService.getBookmarks(vid)) || [];
    } catch (e) {
      console.error('Erro ao carregar notas/marcadores:', e);
    } finally {
      isLoading = false;
    }
  }

  async function handleSaveNote() {
    if (!$playerStore.currentVideo?.id) return;
    isSavingNote = true;
    try {
      await LibraryService.saveNote($playerStore.currentVideo.id, noteContent);
      toast.add('Anotações salvas com sucesso!', 'success');
    } catch (e) {
      toast.add('Falha ao salvar anotações', 'error');
    } finally {
      isSavingNote = false;
    }
  }

  async function handleAddBookmark() {
    if (!$playerStore.currentVideo?.id || !videoElement) return;
    const posMs = Math.floor(videoElement.currentTime * 1000);
    const label = newBookmarkLabel.trim() || `Marcador em ${formatDuration(posMs * 1e6)}`;
    try {
      const bm = await LibraryService.addBookmark($playerStore.currentVideo.id, posMs, label);
      bookmarks = [...bookmarks, bm];
      newBookmarkLabel = '';
      toast.add('Marcador adicionado!', 'success');
    } catch (e) {
      toast.add('Falha ao adicionar marcador', 'error');
    }
  }

  async function handleDeleteBookmark(id: string) {
    try {
      await LibraryService.deleteBookmark(id);
      bookmarks = bookmarks.filter(b => b.id !== id);
      toast.add('Marcador removido', 'info');
    } catch (e) {
      toast.add('Falha ao remover marcador', 'error');
    }
  }

  function handleSeekTo(posNs: number) {
    const secs = posNs / 1e9;
    if (videoElement) {
      videoElement.currentTime = secs;
      playerStore.setTime(secs);
    }
  }

  function formatDuration(ns: number) {
    if (!ns) return '0:00';
    const secs = Math.floor(ns / 1e9);
    const m = Math.floor(secs / 60);
    const s = secs % 60;
    return `${m}:${s < 10 ? '0' : ''}${s}`;
  }

  $: if (open && $playerStore.currentVideo?.id) {
    loadData();
  }
</script>

<Modal title="Notas & Marcadores do Vídeo" bind:open on:close={() => (open = false)}>
  <div class="flex flex-col gap-4 text-xs">
    <!-- Tab Switcher -->
    <div class="flex items-center gap-2 border-b border-border pb-2">
      <button
        type="button"
        on:click={() => (activeTab = 'notes')}
        class="flex items-center gap-1.5 px-3 py-1.5 rounded-lg font-semibold transition-colors
          {activeTab === 'notes' ? 'bg-primary/10 text-primary' : 'text-muted hover:text-foreground hover:bg-surfaceHover'}"
      >
        <StickyNote size={14} /> Anotações Pessoais
      </button>
      <button
        type="button"
        on:click={() => (activeTab = 'bookmarks')}
        class="flex items-center gap-1.5 px-3 py-1.5 rounded-lg font-semibold transition-colors
          {activeTab === 'bookmarks' ? 'bg-primary/10 text-primary' : 'text-muted hover:text-foreground hover:bg-surfaceHover'}"
      >
        <Bookmark size={14} /> Marcadores ({bookmarks.length})
      </button>
    </div>

    {#if activeTab === 'notes'}
      <!-- Notes Tab -->
      <div class="flex flex-col gap-3">
        <p class="text-muted leading-relaxed">
          Escreva anotações, resumos ou tópicos deste vídeo. Os dados são salvos exclusivamente no seu banco de dados local.
        </p>

        <textarea
          bind:value={noteContent}
          rows="7"
          placeholder="Digite suas observações sobre este vídeo aqui..."
          class="w-full bg-surfaceHover/50 border border-border rounded-xl p-3 text-foreground placeholder:text-muted focus:outline-none focus:border-primary font-sans resize-none text-xs leading-relaxed"
        ></textarea>

        <div class="flex justify-end gap-2">
          <Button variant="primary" size="sm" on:click={handleSaveNote} disabled={isSavingNote}>
            <Save size={14} /> {isSavingNote ? 'Salvando...' : 'Salvar Notas'}
          </Button>
        </div>
      </div>
    {:else}
      <!-- Bookmarks Tab -->
      <div class="flex flex-col gap-4">
        <!-- Add Bookmark Bar -->
        <div class="flex gap-2">
          <Input
            bind:value={newBookmarkLabel}
            placeholder="Rótulo do marcador (opcional)..."
            class="flex-1"
          />
          <Button variant="primary" size="sm" on:click={handleAddBookmark}>
            <BookmarkPlus size={14} /> Marcar Ponto Atual
          </Button>
        </div>

        <!-- Bookmarks List -->
        {#if bookmarks.length === 0}
          <div class="text-center py-8 text-muted text-xs bg-surfaceHover/30 rounded-xl border border-dashed border-border">
            Nenhum marcador de tempo salvo neste vídeo ainda.
          </div>
        {:else}
          <div class="flex flex-col gap-2 max-h-56 overflow-y-auto">
            {#each bookmarks as bm (bm.id)}
              <div class="flex items-center justify-between p-2.5 bg-surfaceHover/50 border border-border rounded-xl group hover:border-primary/40 transition-colors">
                <button
                  type="button"
                  on:click={() => handleSeekTo(bm.position)}
                  class="flex items-center gap-2.5 text-left min-w-0 flex-1 hover:text-primary transition-colors cursor-pointer"
                >
                  <span class="px-2 py-0.5 rounded bg-primary/10 text-primary font-mono font-bold text-[11px] shrink-0">
                    {formatDuration(bm.position)}
                  </span>
                  <span class="text-xs font-medium text-foreground truncate">{bm.label}</span>
                </button>

                <button
                  type="button"
                  on:click={() => handleDeleteBookmark(bm.id)}
                  class="p-1 text-muted hover:text-red-400 rounded-lg hover:bg-surface transition-colors"
                  title="Excluir marcador"
                >
                  <Trash2 size={13} />
                </button>
              </div>
            {/each}
          </div>
        {/if}
      </div>
    {/if}
  </div>
</Modal>
