<script lang="ts">
  import { onMount } from 'svelte';
  import ArrowDown from 'lucide-svelte/icons/arrow-down';
  import ArrowUp from 'lucide-svelte/icons/arrow-up';
  import CheckCircle2 from 'lucide-svelte/icons/circle-check';
  import GripVertical from 'lucide-svelte/icons/grip-vertical';
  import ListMusic from 'lucide-svelte/icons/list-music';
  import MoreVertical from 'lucide-svelte/icons/ellipsis-vertical';
  import Play from 'lucide-svelte/icons/play';
  import RotateCcw from 'lucide-svelte/icons/rotate-ccw';
  import Save from 'lucide-svelte/icons/save';
  import Trash2 from 'lucide-svelte/icons/trash-2';
  import ContextMenu from '../ui/ContextMenu.svelte';
  import { playerStore } from '../../stores/playerStore';
  import { QueueService } from '../../wailsjs/services';
  import { toast } from '../../stores/uiStores';
  import Button from '../ui/Button.svelte';
  import Modal from '../ui/Modal.svelte';

  let draggedIndex: number | null = null;
  let showSaveModal = false;
  let playlistName = '';
  let isSaving = false;
  let contextMenuIndex: number | null = null;
  let contextMenuX = 0;
  let contextMenuY = 0;
  let contextMenuTrigger: HTMLElement | null = null;

  $: contextActions = contextMenuIndex === null ? [] : [
    { id: 'play', label: 'Tocar agora' },
    { id: 'up', label: 'Mover para cima', disabled: contextMenuIndex === 0 },
    { id: 'down', label: 'Mover para baixo', disabled: contextMenuIndex === $playerStore.queue.length - 1 },
    { id: 'reset', label: 'Marcar como não tocado', disabled: $playerStore.queue[contextMenuIndex]?.queue_state !== 'played' },
    { id: 'remove', label: 'Remover da fila', danger: true },
  ];

  async function saveAsPlaylist(): Promise<void> {
    if (!playlistName.trim()) return;
    isSaving = true;
    try {
      await playerStore.saveQueueAsPlaylist(playlistName.trim());
      toast.add('Fila salva como playlist.', 'success');
      playlistName = '';
      showSaveModal = false;
    } catch (error: unknown) {
      toast.add(error instanceof Error ? error.message : 'Não foi possível salvar a playlist.', 'error');
    } finally {
      isSaving = false;
    }
  }

  function dropAt(index: number): void {
    if (draggedIndex !== null) void playerStore.reorderQueue(draggedIndex, index);
    draggedIndex = null;
  }

  function handleRemovePlayedChange(event: Event): void {
    void playerStore.setRemovePlayed((event.currentTarget as HTMLInputElement).checked);
  }

  function openContextMenu(event: MouseEvent | KeyboardEvent, index: number): void {
    event.preventDefault();
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
    contextMenuIndex = index;
  }

  function registerQueueContextMenu(node: HTMLElement, index: number): { destroy: () => void } {
    const handleContextMenu = (event: MouseEvent) => openContextMenu(event, index);
    const handleKeyDown = (event: KeyboardEvent) => {
      if (event.key === 'ContextMenu' || event.key === 'Apps' || (event.key === 'F10' && event.shiftKey)) openContextMenu(event, index);
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

  async function runContextAction(event: CustomEvent<string>): Promise<void> {
    const index = contextMenuIndex;
    if (index === null) return;
    if (event.detail === 'play') playerStore.playQueueItem(index);
    if (event.detail === 'up') await playerStore.reorderQueue(index, index - 1);
    if (event.detail === 'down') await playerStore.reorderQueue(index, index + 1);
    if (event.detail === 'remove') await playerStore.removeFromQueue(index);
    if (event.detail === 'reset') {
      const item = $playerStore.queue[index];
      if (item) {
        await QueueService.markQueueItemPlayed(item.queue_item_id, false);
        await playerStore.refreshQueue();
      }
    }
    contextMenuIndex = null;
  }

  onMount(() => { void playerStore.refreshQueue(); });
</script>

<section class="mx-auto flex w-full max-w-5xl flex-col gap-5 p-6" aria-labelledby="queue-page-heading">
  <header class="flex flex-wrap items-center justify-between gap-4">
    <div class="flex items-center gap-3"><div class="flex h-10 w-10 items-center justify-center rounded-xl bg-primary/10 text-primary"><ListMusic size={21} /></div><div><h1 id="queue-page-heading" class="text-xl font-bold text-foreground">Fila de reprodução</h1><p class="text-xs text-muted">Arraste ou use os botões para definir a sequência.</p></div></div>
    <div class="flex flex-wrap gap-2"><Button size="sm" variant="secondary" on:click={playerStore.resetPlayedItems} disabled={!$playerStore.queue.some((video) => video.queue_state === 'played')}><RotateCcw size={13} /> Reabrir tocados</Button><Button size="sm" variant="secondary" on:click={() => playerStore.clearQueue('played')} disabled={!$playerStore.queue.some((video) => video.queue_state === 'played')}><CheckCircle2 size={13} /> Limpar tocados</Button><Button size="sm" on:click={() => (showSaveModal = true)} disabled={!$playerStore.queue.length}><Save size={13} /> Salvar playlist</Button><Button size="sm" variant="danger" on:click={() => playerStore.clearQueue('all')} disabled={!$playerStore.queue.length}><Trash2 size={13} /> Limpar fila</Button></div>
  </header>

  <div class="grid gap-3 rounded-2xl border border-border bg-surface p-4 text-sm sm:grid-cols-3">
    <label class="flex items-center gap-2 rounded-xl bg-background/60 px-3 py-3"><input type="checkbox" checked={$playerStore.autoplay} on:change={playerStore.toggleAutoplay} class="accent-primary tv-focusable" /> Reprodução automática</label>
    <label class="flex items-center gap-2 rounded-xl bg-background/60 px-3 py-3"><input type="checkbox" checked={$playerStore.removePlayed} on:change={handleRemovePlayedChange} class="accent-primary tv-focusable" /> Manter após tocar</label>
    <span class="flex items-center justify-end rounded-xl bg-primary/10 px-3 py-3 font-semibold text-primary">{$playerStore.queue.length} itens na fila</span>
  </div>

  {#if $playerStore.queue.length === 0}
    <div class="rounded-2xl border border-dashed border-border py-20 text-center text-sm text-muted">A fila está vazia. Use o botão de fila nos vídeos ou o menu contextual.</div>
  {:else}
    <ol class="flex flex-col gap-2" aria-label="Ordem da fila">
      {#each $playerStore.queue as video, index (video.queue_item_id)}
        <li
          draggable="true"
          on:dragstart={() => (draggedIndex = index)}
          on:dragover|preventDefault
          on:drop={() => dropAt(index)}
          use:registerQueueContextMenu={index}
          class="group flex items-center gap-3 rounded-2xl border p-4 outline-none transition-colors focus-within:border-primary focus:border-primary {video.queue_state === 'played' ? 'border-border bg-surface/40 opacity-65' : 'border-border bg-surface'}"
        >
          <GripVertical size={20} class="cursor-grab text-muted" aria-label="Arraste para reordenar" />
          <span class="w-7 text-center text-sm font-bold text-muted">{index + 1}</span>
          <img src={video.thumbnail_url} alt="" class="aspect-video w-28 rounded-xl bg-black object-cover sm:w-36" />
          <button type="button" class="min-w-0 flex-1 text-left tv-focusable" on:click={() => playerStore.playQueueItem(index)}><span class="line-clamp-2 text-sm font-semibold text-foreground">{video.title}</span><span class="block truncate text-xs text-muted">{video.channel_title || 'Canal'}{video.queue_state === 'played' ? ' · Tocado' : ''}</span></button>
          <div class="flex gap-1 opacity-70 transition-opacity group-hover:opacity-100 group-focus-within:opacity-100">
            <button type="button" aria-label="Tocar agora" title="Tocar agora" class="rounded-lg p-2 text-muted hover:bg-surfaceHover hover:text-foreground tv-focusable" on:click={() => playerStore.playQueueItem(index)}><Play size={17} /></button>
            <button type="button" aria-label="Mover para cima" title="Mover para cima" disabled={index === 0} class="rounded-lg p-2 text-muted hover:bg-surfaceHover disabled:opacity-30 tv-focusable" on:click={() => playerStore.reorderQueue(index, index - 1)}><ArrowUp size={17} /></button>
            <button type="button" aria-label="Mover para baixo" title="Mover para baixo" disabled={index === $playerStore.queue.length - 1} class="rounded-lg p-2 text-muted hover:bg-surfaceHover disabled:opacity-30 tv-focusable" on:click={() => playerStore.reorderQueue(index, index + 1)}><ArrowDown size={17} /></button>
            <button type="button" aria-label="Mais ações" title="Mais ações" aria-haspopup="menu" class="rounded-lg p-2 text-muted hover:bg-surfaceHover hover:text-foreground tv-focusable" on:click={(event) => openContextMenu(event, index)}><MoreVertical size={17} /></button>
            <button type="button" aria-label="Remover da fila" title="Remover da fila" class="rounded-lg p-2 text-muted hover:bg-red-500/10 hover:text-red-400 tv-focusable" on:click={() => playerStore.removeFromQueue(index)}><Trash2 size={17} /></button>
          </div>
        </li>
      {/each}
    </ol>
  {/if}
</section>

<ContextMenu open={contextMenuIndex !== null} x={contextMenuX} y={contextMenuY} actions={contextActions} on:select={runContextAction} on:close={() => { contextMenuIndex = null; contextMenuTrigger?.focus(); }} />

<Modal title="Salvar fila como playlist" bind:open={showSaveModal}>
  <form class="flex flex-col gap-4" on:submit|preventDefault={saveAsPlaylist}><label class="flex flex-col gap-2 text-xs"><span>Nome da playlist</span><input bind:value={playlistName} maxlength="120" required class="rounded-lg border border-border bg-background px-3 py-2 text-foreground" /></label><div class="flex justify-end gap-2"><Button variant="ghost" on:click={() => (showSaveModal = false)}>Cancelar</Button><Button type="submit" disabled={isSaving || !playlistName.trim()}>{isSaving ? 'Salvando…' : 'Salvar'}</Button></div></form>
</Modal>
