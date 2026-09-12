<script lang="ts">
  import Modal from '../ui/Modal.svelte';
  import { playerStore } from '../../stores/playerStore';

  export let open = false;
  export let isAvailable = true;
  export let unavailableReason = '';
</script>

<Modal title="DSP de Áudio & Ganho Digital" bind:open on:close={() => (open = false)}>
  <div class="flex flex-col gap-6">
    <div class="flex flex-col gap-2">
      <div class="flex items-center justify-between text-sm">
        <span class="font-medium text-foreground">Ganho Digital (Boost)</span>
        <span class="font-mono text-primary font-bold">{$playerStore.gain > 0 ? `+${$playerStore.gain}` : $playerStore.gain} dB</span>
      </div>
      <input
        type="range"
        aria-label="Ganho digital"
        aria-describedby="audio-dsp-status"
        min="-12"
        max="12"
        step="0.5"
        value={$playerStore.gain}
        disabled={!isAvailable}
        on:input={(e) => playerStore.setGain(parseFloat(e.currentTarget.value))}
        class="h-2 w-full appearance-none rounded-lg bg-surfaceHover accent-primary disabled:cursor-not-allowed disabled:opacity-45"
      />
      <div class="flex justify-between text-[11px] text-muted font-mono">
        <span>-12 dB (Atenuado)</span>
        <span>0 dB (Normal)</span>
        <span>+12 dB (Boost)</span>
      </div>
    </div>

    <div
      id="audio-dsp-status"
      class="rounded-xl border p-3 text-xs leading-relaxed {isAvailable
        ? 'border-border bg-surfaceHover/50 text-muted'
        : 'border-amber-500/30 bg-amber-500/10 text-amber-700 dark:text-amber-200'}"
      role={isAvailable ? undefined : 'status'}
    >
      {#if isAvailable}
        O ganho digital usa Web Audio e pode amplificar ou atenuar o sinal desta mídia.
      {:else}
        {unavailableReason || 'O ganho digital não está disponível para esta mídia. O áudio nativo foi preservado.'}
      {/if}
    </div>
  </div>
</Modal>
