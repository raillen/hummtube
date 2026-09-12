<script lang="ts">
  import { createEventDispatcher } from 'svelte';
  export let active = false;

  const dispatch = createEventDispatcher();
</script>

<button
  class="inline-flex items-center px-3 py-1.5 rounded-full text-xs font-medium transition-colors border tv-focusable
    {active 
      ? 'bg-foreground text-background border-transparent' 
      : 'bg-surface hover:bg-surfaceHover text-muted hover:text-foreground border-border'}"
  on:click
>
  <slot />
</button>
