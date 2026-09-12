<script lang="ts">
  import { toast } from '../../stores/uiStores';
  import Info from 'lucide-svelte/icons/info';
  import CheckCircle2 from 'lucide-svelte/icons/circle-check';
  import AlertTriangle from 'lucide-svelte/icons/triangle-alert';
  import AlertCircle from 'lucide-svelte/icons/circle-alert';
  import X from 'lucide-svelte/icons/x';

  const icons = {
    info: Info,
    success: CheckCircle2,
    warning: AlertTriangle,
    error: AlertCircle,
  };

  const bgColors = {
    info: 'bg-blue-950/90 border-blue-700/80 text-blue-100 shadow-blue-950/50',
    success: 'bg-emerald-950/90 border-emerald-700/80 text-emerald-100 shadow-emerald-950/50',
    warning: 'bg-amber-950/90 border-amber-700/80 text-amber-100 shadow-amber-950/50',
    error: 'bg-red-950/90 border-red-700/80 text-red-100 shadow-red-950/50',
  };
</script>

<div 
  class="fixed bottom-4 right-4 z-50 flex flex-col gap-2 pointer-events-none max-w-sm w-full"
  aria-live="polite"
  aria-atomic="false"
>
  {#each $toast as item (item.id)}
    <div 
      role={item.type === 'error' || item.type === 'warning' ? 'alert' : 'status'}
      class="pointer-events-auto flex items-center justify-between gap-3 px-4 py-3 rounded-xl border backdrop-blur-md shadow-2xl text-xs font-medium animate-in slide-in-from-right-5 duration-150 {bgColors[item.type]}"
    >
      <div class="flex items-center gap-2.5 min-w-0">
        <svelte:component this={icons[item.type]} size={16} class="shrink-0" />
        <span class="leading-relaxed truncate">{item.message}</span>
      </div>
      <button 
        type="button" 
        on:click={() => toast.remove(item.id)} 
        aria-label="Fechar notificação"
        class="p-1 rounded-lg opacity-70 hover:opacity-100 hover:bg-white/10 transition-opacity tv-focusable cursor-pointer shrink-0"
      >
        <X size={14} />
      </button>
    </div>
  {/each}
</div>
