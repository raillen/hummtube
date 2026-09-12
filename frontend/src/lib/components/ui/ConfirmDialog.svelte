<script lang="ts">
  import { createEventDispatcher, onDestroy, onMount, tick } from 'svelte';
  import AlertTriangle from 'lucide-svelte/icons/circle-alert';

  export let open = false;
  export let title = '';
  export let description = '';
  export let confirmLabel = 'Confirmar';
  export let cancelLabel = 'Cancelar';
  export let danger = false;
  export let trigger: HTMLElement | null = null;

  const dispatch = createEventDispatcher<{ confirm: void; cancel: void }>();
  let dialogElement: HTMLDivElement | null = null;
  let confirmButton: HTMLButtonElement | null = null;
  let lastTrigger: HTMLElement | null = null;

  $: if (open) void focusConfirm();
  $: if (open) lastTrigger ??= trigger;

  async function focusConfirm(): Promise<void> {
    await tick();
    confirmButton?.focus();
  }

  function close(value: 'confirm' | 'cancel'): void {
    open = false;
    if (value === 'confirm') dispatch('confirm');
    else dispatch('cancel');
    queueMicrotask(() => {
      const restoreTarget = (trigger ?? lastTrigger) as HTMLElement | null;
      lastTrigger = null;
      restoreTarget?.focus?.();
    });
  }

  function onKeyDown(event: KeyboardEvent): void {
    if (!open) return;
    if (event.key === 'Escape') {
      event.preventDefault();
      event.stopPropagation();
      close('cancel');
      return;
    }
    if (event.key === 'Tab') {
      const focusables = [confirmButton, dialogElement?.querySelector<HTMLButtonElement>('button[data-action="close"]')].filter(
        (el): el is HTMLButtonElement => Boolean(el)
      );
      if (focusables.length === 0) return;
      const currentIndex = focusables.indexOf(document.activeElement as HTMLButtonElement);
      event.preventDefault();
      const direction = event.shiftKey ? -1 : 1;
      const nextIndex = currentIndex < 0 ? 0 : (currentIndex + direction + focusables.length) % focusables.length;
      focusables[nextIndex]?.focus();
    }
  }

  onMount(() => {
    document.addEventListener('keydown', onKeyDown, true);
  });

  onDestroy(() => {
    document.removeEventListener('keydown', onKeyDown, true);
  });
</script>

{#if open}
  <div
    bind:this={dialogElement}
    role="alertdialog"
    aria-modal="true"
    aria-labelledby="confirm-dialog-title"
    aria-describedby="confirm-dialog-description"
    class="fixed inset-0 z-[120] flex items-center justify-center bg-black/70 p-4 backdrop-blur-sm"
    data-testid="confirm-dialog"
  >
    <button
      type="button"
      class="absolute inset-0 cursor-default"
      aria-label="Fechar diálogo"
      data-action="close"
      on:click={() => close('cancel')}
      tabindex="-1"
    />
    <div class="relative z-[121] w-full max-w-sm rounded-2xl border border-border bg-surface p-5 shadow-2xl">
      <div class="flex items-start gap-3">
        {#if danger}
          <span class="rounded-full bg-red-500/15 p-2 text-red-400" aria-hidden="true">
            <AlertTriangle size={18} />
          </span>
        {/if}
        <div class="min-w-0 flex-1">
          <h2 id="confirm-dialog-title" class="text-base font-semibold text-foreground">{title}</h2>
          <p id="confirm-dialog-description" class="mt-1 text-sm text-muted">{description}</p>
        </div>
      </div>
      <div class="mt-5 flex flex-col-reverse gap-2 sm:flex-row sm:justify-end">
        <button
          type="button"
          class="rounded-lg border border-border px-4 py-2 text-sm font-medium text-foreground hover:bg-surfaceHover focus:outline-none focus:ring-2 focus:ring-primary tv-focusable"
          on:click={() => close('cancel')}
        >
          {cancelLabel}
        </button>
        <button
          bind:this={confirmButton}
          type="button"
          data-testid="confirm-dialog-confirm"
          class="rounded-lg px-4 py-2 text-sm font-semibold text-white focus:outline-none focus:ring-2 focus:ring-white/70 tv-focusable {danger ? 'bg-red-600 hover:bg-red-500' : 'bg-primary hover:bg-primary-hover'}"
          on:click={() => close('confirm')}
        >
          {confirmLabel}
        </button>
      </div>
    </div>
  </div>
{/if}
