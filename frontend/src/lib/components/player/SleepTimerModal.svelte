<script lang="ts">
  import Modal from '../ui/Modal.svelte';
  import Button from '../ui/Button.svelte';
  import { playerStore } from '../../stores/playerStore';
  import { toast } from '../../stores/uiStores';

  export let open = false;
  let timerId: any = null;
  let activeMinutes: number | null = null;

  function setTimer(minutes: number) {
    if (timerId) clearTimeout(timerId);
    activeMinutes = minutes;
    timerId = setTimeout(() => {
      playerStore.pause();
      toast.add('Temporizador de sono pausou a reprodução.', 'info');
      activeMinutes = null;
    }, minutes * 60 * 1000);
    toast.add(`Temporizador ativado para ${minutes} minutos.`, 'success');
    open = false;
  }

  function cancelTimer() {
    if (timerId) clearTimeout(timerId);
    timerId = null;
    activeMinutes = null;
    toast.add('Temporizador cancelado.', 'info');
    open = false;
  }
</script>

<Modal title="Temporizador de Sono" bind:open on:close={() => (open = false)}>
  <div class="flex flex-col gap-4">
    <p class="text-sm text-muted">
      Pausa automaticamente a reprodução após o tempo selecionado.
    </p>

    <div class="grid grid-cols-3 gap-2.5">
      {#each [15, 30, 45, 60, 90, 120] as mins}
        <button
          on:click={() => setTimer(mins)}
          class="py-3 px-4 rounded-xl border text-sm font-semibold transition-all tv-focusable
            {activeMinutes === mins 
              ? 'bg-primary text-white border-transparent' 
              : 'bg-surface hover:bg-surfaceHover text-foreground border-border'}"
        >
          {mins} min
        </button>
      {/each}
    </div>

    {#if activeMinutes}
      <div class="pt-2">
        <Button variant="danger" on:click={cancelTimer} class="w-full">
          Cancelar Temporizador ({activeMinutes} min ativos)
        </Button>
      </div>
    {/if}
  </div>
</Modal>
