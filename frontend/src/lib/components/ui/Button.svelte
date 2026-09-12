<script lang="ts">
  import { createEventDispatcher } from 'svelte';
  export let variant: 'primary' | 'secondary' | 'ghost' | 'danger' = 'secondary';
  export let size: 'sm' | 'md' | 'lg' = 'md';
  export let disabled = false;
  export let tvFocusable = true;
  export let type: 'button' | 'submit' | 'reset' = 'button';

  const dispatch = createEventDispatcher();

  const baseStyles = "inline-flex items-center justify-center font-medium rounded-lg transition-all focus:outline-none disabled:opacity-50 disabled:cursor-not-allowed cursor-pointer";
  
  const variantStyles = {
    primary: "bg-primary text-white hover:bg-primary-hover shadow-sm",
    secondary: "bg-surface hover:bg-surfaceHover text-foreground border border-border",
    ghost: "text-muted hover:text-foreground hover:bg-surfaceHover",
    danger: "bg-red-600/10 text-red-500 hover:bg-red-600/20 border border-red-500/20",
  };

  const sizeStyles = {
    sm: "text-xs px-2.5 py-1.5 gap-1.5",
    md: "text-sm px-3.5 py-2 gap-2",
    lg: "text-base px-5 py-2.5 gap-2.5",
  };
</script>

<button
  {type}
  class="{baseStyles} {variantStyles[variant]} {sizeStyles[size]} {tvFocusable ? 'tv-focusable' : ''}"
  {disabled}
  on:click={(e) => dispatch('click', e)}
  {...$$restProps}
>
  <slot />
</button>
