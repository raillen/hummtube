<script lang="ts">
  import Modal from '../ui/Modal.svelte';
  import { playerStore } from '../../stores/playerStore';
  import ListMusic from 'lucide-svelte/icons/list-music';
  import MoreVertical from 'lucide-svelte/icons/ellipsis-vertical';
  import Play from 'lucide-svelte/icons/play';
  import Trash2 from 'lucide-svelte/icons/trash-2';
  import ArrowDown from 'lucide-svelte/icons/arrow-down';
  import ArrowUp from 'lucide-svelte/icons/arrow-up';
  import ExternalLink from 'lucide-svelte/icons/external-link';
  import { activeTab } from '../../stores/uiStores';
  import ContextMenu from '../ui/ContextMenu.svelte';

  export let open = false;
  let draggedIndex: number | null = null;
  let contextMenuIndex: number | null = null;
  let contextMenuX = 0;
  let contextMenuY = 0;
  let contextMenuTrigger: HTMLElement | null = null;

  $: contextActions = contextMenuIndex === null ? [] : [
    { id: 'play', label: 'Tocar agora' },
    { id: 'up', label: 'Mover para cima', disabled: contextMenuIndex === 0 },
    { id: 'down', label: 'Mover para baixo', disabled: contextMenuIndex === $playerStore.queue.length - 1 },
    { id: 'remove', label: 'Remover da fila', danger: true },
  ];

  function handlePlayQueueItem(index: number) {
    playerStore.playQueueItem(index);
  }

  function handleRemovePlayedChange(event: Event): void {
    void playerStore.setRemovePlayed((event.currentTarget as HTMLInputElement).checked);
  }

  function dropAt(index: number): void {
    if (draggedIndex !== null) void playerStore.reorderQueue(draggedIndex, index);
    draggedIndex = null;
  }

  function openQueueManager(): void {
    open = false;
    $activeTab = 'queue';
  }

  function openContextMenu(event: MouseEvent | KeyboardEvent, index: number): void {
    event.preventDefault();
    contextMenuTrigger = event instanceof KeyboardEvent
      ? document.activeElement as HTMLElement
      : event.currentTarget as HTMLElement;
    if (event instanceof MouseEvent && event.type === 'contextmenu') {
      contextMenuX = event.clientX;
      contextMenuY = event.clientY;
    } else {
      const bounds = contextMenuTrigger.getBoundingClientRect();
      contextMenuX = bounds.left + 24;
      contextMenuY = bounds.top + 24;
    }
    contextMenuIndex = index;
  }

  function registerContextMenu(node: HTMLElement, index: number): { destroy: () => void } {
    const handler = (event: MouseEvent) => openContextMenu(event, index);
    const keyHandler = (event: KeyboardEvent) => {
      if (event.key === 'ContextMenu' || event.key === 'Apps' || (event.key === 'F10' && event.shiftKey)) openContextMenu(event, index);
    };
    node.addEventListener('contextmenu', handler);
    node.addEventListener('keydown', keyHandler);
    return {
      destroy: () => {
        node.removeEventListener('contextmenu', handler);
        node.removeEventListener('keydown', keyHandler);
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
    contextMenuIndex = null;
  }
</script>

<Modal title="Fila de Reprodução (A Seguir)" bind:open on:close={() => (open = false)}>
  <div class="flex flex-col gap-4 text-xs">
    <!-- Top Bar: Autoplay Toggle & Count -->
    <div class="flex items-center justify-between p-3 bg-surfaceHover/50 rounded-xl border border-border">
      <div class="flex items-center gap-2">
        <ListMusic size={16} class="text-primary" />
        <span class="font-semibold text-foreground">
          {$playerStore.queue.length} {$playerStore.queue.length === 1 ? 'vídeo na fila' : 'vídeos na fila'}
        </span>
      </div>

      <div class="flex items-center gap-3">
        <label class="flex items-center gap-2 cursor-pointer select-none text-muted hover:text-foreground">
          <input
            type="checkbox"
            checked={$playerStore.autoplay}
            on:change={playerStore.toggleAutoplay}
            class="rounded border-border text-primary focus:ring-primary h-3.5 w-3.5 accent-primary"
          />
          <span>Reprodução Automática</span>
        </label>

        <label class="flex items-center gap-2 cursor-pointer select-none text-muted hover:text-foreground">
          <input
            type="checkbox"
            checked={$playerStore.removePlayed}
            on:change={handleRemovePlayedChange}
            class="rounded border-border text-primary focus:ring-primary h-3.5 w-3.5 accent-primary"
          />
          <span>Remover após tocar</span>
        </label>

        {#if $playerStore.queue.length > 0}
          <button
            type="button"
            on:click={() => playerStore.clearQueue('all')}
            class="text-[11px] text-red-400 hover:underline cursor-pointer"
          >
            Limpar Fila
          </button>
        {/if}
        <button type="button" class="inline-flex items-center gap-1 text-[11px] font-semibold text-primary hover:underline" on:click={openQueueManager}><ExternalLink size={13} /> Gerenciar fila</button>
      </div>
    </div>

    <!-- Currently Playing Info -->
    {#if $playerStore.currentVideo}
      <div class="flex flex-col gap-1.5">
        <span class="text-[10px] font-bold uppercase tracking-wider text-muted px-1">Tocando Agora</span>
        <div class="flex items-center gap-3 p-2.5 bg-primary/10 border border-primary/20 rounded-xl">
          <div class="relative w-16 aspect-video rounded-lg overflow-hidden bg-black shrink-0">
            <img
              src={$playerStore.currentVideo.thumbnail_url}
              alt={$playerStore.currentVideo.title}
              class="w-full h-full object-cover"
            />
          </div>
          <div class="flex-1 min-w-0">
            <h4 class="text-xs font-semibold text-foreground truncate">{$playerStore.currentVideo.title}</h4>
            <p class="text-[10px] text-muted truncate">{$playerStore.currentVideo.channel_title}</p>
          </div>
        </div>
      </div>
    {/if}

    <!-- Queue List -->
    <div class="flex flex-col gap-1.5">
      <span class="text-[10px] font-bold uppercase tracking-wider text-muted px-1">A Seguir</span>
      {#if $playerStore.queue.length === 0}
        <div class="text-center py-8 text-muted bg-surfaceHover/30 rounded-xl border border-dashed border-border">
          A fila está vazia. Adicione vídeos para reprodução contínua.
        </div>
      {:else}
        <div class="flex flex-col gap-2 max-h-60 overflow-y-auto">
          {#each $playerStore.queue as item, index (item.queue_item_id)}
            <div use:registerContextMenu={index} role="listitem" draggable="true" on:dragstart={() => (draggedIndex = index)} on:dragover|preventDefault on:drop={() => dropAt(index)} class="flex items-center justify-between p-3 border border-border rounded-xl group hover:border-primary/40 transition-colors {item.queue_state === 'played' ? 'bg-surface/40 opacity-65' : 'bg-surfaceHover/50'}">
              <span class="mr-1 text-muted" aria-label="Arraste para reordenar">⋮⋮</span>
              <button
                type="button"
                on:click={() => handlePlayQueueItem(index)}
                class="flex items-center gap-2.5 text-left min-w-0 flex-1 hover:text-primary transition-colors cursor-pointer"
              >
                <div class="relative w-14 aspect-video rounded-md overflow-hidden bg-black shrink-0">
                  <img src={item.thumbnail_url} alt={item.title} class="w-full h-full object-cover" />
                </div>
                <div class="flex-1 min-w-0">
                  <h4 class="text-xs font-semibold text-foreground truncate group-hover:text-primary">{item.title}</h4>
                  <p class="text-[10px] text-muted truncate">{item.channel_title}</p>
                </div>
              </button>

              <div class="flex items-center gap-0.5">
                <button type="button" aria-label="Mover para cima" disabled={index === 0} on:click={() => playerStore.reorderQueue(index, index - 1)} class="rounded-lg p-1.5 text-muted hover:bg-surface hover:text-foreground disabled:opacity-30"><ArrowUp size={13} /></button>
                <button type="button" aria-label="Mover para baixo" disabled={index === $playerStore.queue.length - 1} on:click={() => playerStore.reorderQueue(index, index + 1)} class="rounded-lg p-1.5 text-muted hover:bg-surface hover:text-foreground disabled:opacity-30"><ArrowDown size={13} /></button>
                <button type="button" aria-label="Mais ações" title="Mais ações" aria-haspopup="menu" on:click={(event) => openContextMenu(event, index)} class="rounded-lg p-1.5 text-muted hover:bg-surface hover:text-foreground"><MoreVertical size={13} /></button>
                <button
                type="button"
                on:click={() => playerStore.removeFromQueue(index)}
                class="p-1.5 text-muted hover:text-red-400 rounded-lg hover:bg-surface transition-colors shrink-0"
                title="Remover da fila"
              >
                <Trash2 size={13} />
                </button>
              </div>
            </div>
          {/each}
        </div>
      {/if}
    </div>
  </div>
</Modal>

<ContextMenu open={contextMenuIndex !== null} x={contextMenuX} y={contextMenuY} actions={contextActions} on:select={runContextAction} on:close={() => { contextMenuIndex = null; contextMenuTrigger?.focus(); }} />
