<script lang="ts">
  import { createEventDispatcher, tick } from 'svelte';
  import type { ComponentType } from 'svelte';

  interface ContextMenuAction {
    id: string;
    label: string;
    disabled?: boolean;
    danger?: boolean;
    shortcut?: string;
    icon?: ComponentType;
    separator?: boolean;
  }

  export let open = false;
  export let x = 0;
  export let y = 0;
  export let actions: ContextMenuAction[] = [];

  const dispatch = createEventDispatcher<{ select: string; close: void }>();
  let menuElement: HTMLElement | null = null;
  let left = 0;
  let top = 0;

  $: if (open) void positionAndFocus();

  async function positionAndFocus(): Promise<void> {
    await tick();
    if (!menuElement) return;
    const bounds = menuElement.getBoundingClientRect();
    left = Math.max(8, Math.min(x, window.innerWidth - bounds.width - 8));
    top = Math.max(8, Math.min(y, window.innerHeight - bounds.height - 8));
    menuElement.querySelector<HTMLButtonElement>('button:not([disabled])')?.focus();
  }

  function closeMenu(): void {
    if (!open) return;
    open = false;
    dispatch('close');
  }

  function selectAction(action: ContextMenuAction): void {
    if (action.disabled) return;
    dispatch('select', action.id);
    closeMenu();
  }

  function handleKeyDown(event: KeyboardEvent): void {
    if (!menuElement) return;
    if (event.key === 'Escape') {
      event.preventDefault();
      event.stopPropagation();
      closeMenu();
      return;
    }
    if (event.key === 'Tab') {
      // O menu é um pequeno foco cíclico: Tab não deve jogar o usuário
      // para um controle atrás do overlay sem antes fechá-lo.
      event.preventDefault();
    } else if (event.key !== 'ArrowDown' && event.key !== 'ArrowUp' && event.key !== 'Home' && event.key !== 'End') {
      return;
    }
    event.preventDefault();
    const buttons = Array.from(menuElement.querySelectorAll<HTMLButtonElement>('button:not([disabled])'));
    if (buttons.length === 0) return;
    const currentIndex = buttons.indexOf(document.activeElement as HTMLButtonElement);
    if (event.key === 'Home') {
      buttons[0].focus();
      return;
    }
    if (event.key === 'End') {
      buttons[buttons.length - 1].focus();
      return;
    }
    const direction = event.key === 'ArrowDown' || event.key === 'Tab' ? 1 : -1;
    const startIndex = currentIndex < 0 ? (direction > 0 ? 0 : buttons.length - 1) : currentIndex;
    buttons[(startIndex + direction + buttons.length) % buttons.length]?.focus();
  }
</script>

<svelte:window on:resize={closeMenu} on:blur={closeMenu} on:scroll={closeMenu} />

{#if open}
  <button
    type="button"
    class="fixed inset-0 z-[79] cursor-default bg-transparent"
    aria-label="Fechar menu contextual"
    on:click={closeMenu}
    tabindex="-1"
  />
  <div
    bind:this={menuElement}
    role="menu"
    data-context-menu
    tabindex="-1"
    aria-label="Ações disponíveis"
    class="fixed z-[80] m-0 min-w-48 list-none rounded-xl border border-border bg-surface p-1.5 shadow-2xl"
    style={`left:${left}px;top:${top}px`}
    on:keydown={handleKeyDown}
    on:contextmenu|preventDefault
    on:click|stopPropagation
  >
    {#each actions as action, actionIndex (action.id + ':' + actionIndex)}
      {#if action.separator}
        <li role="separator" class="my-1 border-t border-border/70"></li>
      {:else}
      <li role="none">
        <button
          type="button"
          role="menuitem"
          disabled={action.disabled}
          aria-keyshortcuts={action.shortcut}
          class="flex w-full items-center gap-2 rounded-lg px-3 py-2 text-left text-xs transition-colors disabled:opacity-40 {action.danger ? 'text-red-400 hover:bg-red-500/10' : 'text-foreground hover:bg-surfaceHover'}"
          on:click={() => selectAction(action)}
        >
          {#if action.icon}<svelte:component this={action.icon} size={14} class="shrink-0" aria-hidden="true" />{/if}
          <span class="min-w-0 flex-1">{action.label}</span>
          {#if action.shortcut}<kbd class="ml-2 shrink-0 rounded bg-background/70 px-1.5 py-0.5 font-mono text-[10px] text-muted">{action.shortcut}</kbd>{/if}
        </button>
      </li>
      {/if}
    {/each}
  </div>
{/if}
