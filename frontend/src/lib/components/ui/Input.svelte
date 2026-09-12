<script lang="ts">
  export let value = '';
  export let placeholder = '';
  export let tvFocusable = true;
  export let label = '';
  export let id = `input-${Math.random().toString(36).slice(2, 9)}`;
</script>

<label class="flex w-full flex-col gap-1.5">
  {#if label}<span class="text-xs font-semibold text-foreground">{label}</span>{/if}
  <input
    {id}
    type="text"
    bind:value
    {placeholder}
    class="w-full px-3.5 py-2 text-sm bg-surface border border-border rounded-lg text-foreground placeholder:text-muted focus:outline-none focus:border-primary transition-colors {tvFocusable ? 'tv-focusable' : ''}"
    {...$$restProps}
    on:input
    on:keydown
  />
</label>
