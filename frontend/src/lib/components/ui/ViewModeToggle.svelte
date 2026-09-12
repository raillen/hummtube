<script lang="ts">
  import LayoutGrid from 'lucide-svelte/icons/layout-grid';
  import List from 'lucide-svelte/icons/list';
  import Rows3 from 'lucide-svelte/icons/rows-3';
  import type { ContentViewMode } from '../../stores/uiStores';

  export let value: ContentViewMode = 'grid';
  export let label = 'Modo de visualização';
  export let allowCompact = true;
  export let onChange: (mode: ContentViewMode) => void = () => undefined;

  const modes: Array<{ id: ContentViewMode; label: string; icon: typeof LayoutGrid }> = [
    { id: 'grid', label: 'Grade', icon: LayoutGrid },
    { id: 'list', label: 'Lista', icon: List },
    { id: 'compact', label: 'Compacto', icon: Rows3 },
  ];
</script>

<div class="inline-flex items-center gap-1 rounded-lg border border-border bg-surface p-1" role="group" aria-label={label}>
  {#each modes.filter((mode) => allowCompact || mode.id !== 'compact') as mode (mode.id)}
    <button type="button" on:click={() => onChange(mode.id)} aria-pressed={value === mode.id} aria-label={`${label}: ${mode.label}`} title={mode.label} class="rounded-md p-1.5 tv-focusable {value === mode.id ? 'bg-primary/15 text-primary' : 'text-muted hover:bg-surfaceHover hover:text-foreground'}"><svelte:component this={mode.icon} size={15} /></button>
  {/each}
</div>
