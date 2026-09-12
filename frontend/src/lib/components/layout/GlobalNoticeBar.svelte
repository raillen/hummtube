<script lang="ts">
  import { onMount, onDestroy } from 'svelte';
  import { SettingsService } from '../../wailsjs/services';
  import { activeTab } from '../../stores/uiStores';
  import AlertTriangle from 'lucide-svelte/icons/triangle-alert';
  import AlertCircle from 'lucide-svelte/icons/circle-alert';
  import WifiOff from 'lucide-svelte/icons/wifi-off';
  import X from 'lucide-svelte/icons/x';
  import ArrowRight from 'lucide-svelte/icons/arrow-right';
  import RefreshCw from 'lucide-svelte/icons/refresh-cw';
  import CheckCircle2 from 'lucide-svelte/icons/circle-check';

  interface Notice {
    id: string;
    type: 'error' | 'warning' | 'info';
    title: string;
    message: string;
    actionLabel?: string;
    actionTab?: typeof $activeTab;
    onAction?: () => void;
  }

  let notices: Notice[] = [];
  let isOffline = typeof navigator !== 'undefined' && !navigator.onLine;

  function handleOnline() {
    isOffline = false;
    notices = notices.filter(n => n.id !== 'offline');
  }

  function handleOffline() {
    isOffline = true;
    if (!notices.some(n => n.id === 'offline')) {
      notices = [
        {
          id: 'offline',
          type: 'warning',
          title: 'Modo Offline',
          message: 'Sem conexão com a internet. Apenas o catálogo local, histórico e favoritos em cache estão disponíveis.',
        },
        ...notices,
      ];
    }
  }

  async function checkSystemHealth() {
    try {
      // 1. Diagnósticos
      const diag = await SettingsService.getDiagnostics();
      if (diag.errors && diag.errors.length > 0) {
        for (const err of diag.errors) {
          if (!notices.some(n => n.message === err)) {
            notices.push({
              id: 'diag-err-' + Math.random().toString(36).slice(2, 6),
              type: 'error',
              title: 'Erro no Ambiente do Sistema',
              message: err,
              actionLabel: 'Ver Ajustes',
              actionTab: 'settings',
            });
          }
        }
      }
      if (diag.warnings && diag.warnings.length > 0) {
        for (const warn of diag.warnings) {
          if (!notices.some(n => n.message === warn)) {
            notices.push({
              id: 'diag-warn-' + Math.random().toString(36).slice(2, 6),
              type: 'warning',
              title: 'Alerta do Sistema',
              message: warn,
              actionLabel: 'Ver Diagnósticos',
              actionTab: 'settings',
            });
          }
        }
      }

      notices = [...notices];
    } catch (e) {
      console.warn('Falha ao verificar integridade do sistema:', e);
    }
  }

  function dismissNotice(id: string) {
    notices = notices.filter(n => n.id !== id);
  }

  function handleNoticeAction(notice: Notice) {
    if (notice.onAction) {
      notice.onAction();
    } else if (notice.actionTab) {
      $activeTab = notice.actionTab;
    }
    dismissNotice(notice.id);
  }

  onMount(() => {
    if (typeof window !== 'undefined') {
      window.addEventListener('online', handleOnline);
      window.addEventListener('offline', handleOffline);
      if (!navigator.onLine) handleOffline();
    }
    checkSystemHealth();
  });

  onDestroy(() => {
    if (typeof window !== 'undefined') {
      window.removeEventListener('online', handleOnline);
      window.removeEventListener('offline', handleOffline);
    }
  });
</script>

{#if notices.length > 0}
  <section 
    aria-label="Avisos do Sistema e Notificações" 
    class="flex flex-col gap-1 w-full shrink-0 z-20 px-3 pt-2 select-none"
  >
    {#each notices as notice (notice.id)}
      <div 
        role="alert" 
        aria-live="polite"
        class="flex items-center justify-between gap-3 px-4 py-2.5 rounded-xl border text-xs shadow-sm transition-all animate-in fade-in slide-in-from-top-2 duration-150
          {notice.type === 'error' ? 'bg-red-500/10 border-red-500/30 text-red-300' : ''}
          {notice.type === 'warning' ? 'bg-amber-500/10 border-amber-500/30 text-amber-200' : ''}
          {notice.type === 'info' ? 'bg-blue-500/10 border-blue-500/30 text-blue-200' : ''}"
      >
        <div class="flex items-center gap-2.5 min-w-0">
          {#if notice.type === 'error'}
            <AlertCircle size={16} class="text-red-400 shrink-0" />
          {:else if notice.id === 'offline'}
            <WifiOff size={16} class="text-amber-400 shrink-0" />
          {:else if notice.type === 'warning'}
            <AlertTriangle size={16} class="text-amber-400 shrink-0" />
          {:else}
            <CheckCircle2 size={16} class="text-blue-400 shrink-0" />
          {/if}

          <div class="flex flex-wrap items-center gap-1.5 min-w-0">
            <strong class="font-bold text-foreground">{notice.title}:</strong>
            <span class="opacity-90 truncate max-w-xl">{notice.message}</span>
          </div>
        </div>

        <div class="flex items-center gap-2 shrink-0">
          {#if notice.actionLabel}
            <button
              type="button"
              on:click={() => handleNoticeAction(notice)}
              class="px-2.5 py-1 rounded-lg font-bold transition-colors tv-focusable flex items-center gap-1 text-[11px] cursor-pointer
                {notice.type === 'error' ? 'bg-red-500/20 hover:bg-red-500/30 text-red-200' : ''}
                {notice.type === 'warning' ? 'bg-amber-500/20 hover:bg-amber-500/30 text-amber-200' : ''}
                {notice.type === 'info' ? 'bg-blue-500/20 hover:bg-blue-500/30 text-blue-200' : ''}"
            >
              <span>{notice.actionLabel}</span>
              <ArrowRight size={12} />
            </button>
          {/if}

          <button
            type="button"
            on:click={() => dismissNotice(notice.id)}
            aria-label="Dispensar aviso"
            class="p-1 rounded-lg opacity-70 hover:opacity-100 hover:bg-white/10 transition-opacity tv-focusable cursor-pointer"
          >
            <X size={14} />
          </button>
        </div>
      </div>
    {/each}
  </section>
{/if}
