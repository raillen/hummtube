<script lang="ts">
  import { createEventDispatcher, tick } from 'svelte';
  import X from 'lucide-svelte/icons/x';
  export let title = '';
  export let open = false;

  const dispatch = createEventDispatcher();
  const titleId = 'modal-title-' + Math.random().toString(36).substring(2, 7);
  let dialogElement: HTMLElement | null = null;
  let previouslyFocused: HTMLElement | null = null;
  let wasOpen = false;

  $: if (open && !wasOpen) {
    wasOpen = true;
    previouslyFocused = document.activeElement instanceof HTMLElement ? document.activeElement : null;
    void tick().then(() => focusDialog());
  } else if (!open && wasOpen) {
    wasOpen = false;
    const restoreTarget = previouslyFocused;
    previouslyFocused = null;
    void tick().then(() => {
      if (restoreTarget?.isConnected) restoreTarget.focus({ preventScroll: true });
    });
  }

  function focusDialog() {
    const target = dialogElement?.querySelector<HTMLElement>(
      'button:not([disabled]), [href], input:not([disabled]), select:not([disabled]), textarea:not([disabled]), [tabindex]:not([tabindex="-1"])'
    );
    (target ?? dialogElement)?.focus({ preventScroll: true });
  }

  function close() {
    open = false;
    dispatch('close');
  }

  function handleKeydown(e: KeyboardEvent) {
    if (!open) return;
    if (e.key === 'Escape' && !e.defaultPrevented && !(e.target instanceof HTMLInputElement && e.target.type === 'file')) {
      e.stopPropagation();
      close();
      return;
    }
    if (e.key === 'Tab' && dialogElement) {
      const focusables = Array.from(dialogElement.querySelectorAll<HTMLElement>(
        'button:not([disabled]), [href], input:not([disabled]), select:not([disabled]), textarea:not([disabled]), [tabindex]:not([tabindex="-1"])'
      )).filter((element) => element.getBoundingClientRect().width > 0);
      if (focusables.length === 0) return;
      const first = focusables[0];
      const last = focusables[focusables.length - 1];
      if (e.shiftKey && document.activeElement === first) {
        e.preventDefault();
        last.focus();
      } else if (!e.shiftKey && document.activeElement === last) {
        e.preventDefault();
        first.focus();
      }
    }
  }
</script>

<svelte:window on:keydown={handleKeydown} />

{#if open}
  <div 
    class="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/60 backdrop-blur-sm"
    role="presentation"
    on:click|self={close}
  >
    <div 
      bind:this={dialogElement}
      role="dialog"
      tabindex="-1"
      aria-modal="true"
      aria-labelledby={titleId}
      class="relative w-full max-w-lg bg-surface border border-border rounded-2xl shadow-2xl overflow-hidden animate-in fade-in zoom-in-95 duration-150"
    >
      <div class="flex items-center justify-between px-5 py-4 border-b border-border">
        <h3 id={titleId} class="text-base font-semibold text-foreground">{title}</h3>
        <button 
          type="button" 
          on:click={close} 
          aria-label="Fechar modal"
          data-action="close"
          class="p-1.5 text-muted hover:text-foreground rounded-lg hover:bg-surfaceHover transition-colors tv-focusable cursor-pointer"
        >
          <X size={18} />
        </button>
      </div>
      <div class="p-5 max-h-[75vh] overflow-y-auto">
        <slot />
      </div>
    </div>
  </div>
{/if}
