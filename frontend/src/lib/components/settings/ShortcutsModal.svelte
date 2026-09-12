<script lang="ts">
  import { createEventDispatcher } from 'svelte';
  import { shortcutsStore, normalizeKeyboardEvent, type ShortcutItem, DEFAULT_SHORTCUTS } from '../../stores/shortcutsStore';
  import { toast } from '../../stores/uiStores';
  import Modal from '../ui/Modal.svelte';
  import Button from '../ui/Button.svelte';
  import Keyboard from 'lucide-svelte/icons/keyboard';
  import RotateCcw from 'lucide-svelte/icons/rotate-ccw';
  import Check from 'lucide-svelte/icons/check';
  import Sparkles from 'lucide-svelte/icons/sparkles';
  import AlertCircle from 'lucide-svelte/icons/circle-alert';

  export let open = false;

  const dispatch = createEventDispatcher();
  let recordingShortcutId: string | null = null;
  let recordedKey = '';

  function startRecording(item: ShortcutItem) {
    recordingShortcutId = item.id;
    recordedKey = '';
  }

  function handleKeyDown(e: KeyboardEvent) {
    if (!recordingShortcutId) return;

    e.preventDefault();
    e.stopPropagation();

    // Se pressionar Escape sozinho enquanto grava, cancela a gravação
    if (e.key === 'Escape' && !e.ctrlKey && !e.altKey && !e.shiftKey) {
      recordingShortcutId = null;
      recordedKey = '';
      return;
    }

    const keyStr = normalizeKeyboardEvent(e);
    if (!keyStr || keyStr === 'Control' || keyStr === 'Alt' || keyStr === 'Shift') {
      return;
    }

    // Salva o novo atalho
    shortcutsStore.updateShortcut(recordingShortcutId, keyStr);
    toast.add(`Atalho atualizado para [${keyStr}]`, 'success');
    recordingShortcutId = null;
    recordedKey = '';
  }

  function handleReset() {
    shortcutsStore.resetToDefaults();
    recordingShortcutId = null;
    toast.add('Atalhos restaurados para o padrão de fábrica', 'info');
  }

  const categoryNames: Record<string, string> = {
    player: 'Player de Vídeo & Áudio',
    navigation: 'Navegação Espacial & Busca',
    system: 'Sistema & Ferramentas',
  };
</script>

<svelte:window on:keydown={handleKeyDown} />

<Modal title="Gerenciador de Atalhos de Teclado" bind:open on:close={() => { recordingShortcutId = null; dispatch('close'); }}>
  <div class="flex flex-col gap-5 text-xs">
    <!-- Notice / Instructions -->
    <div class="p-3 bg-primary/10 border border-primary/20 rounded-xl flex items-start gap-2.5 text-foreground">
      <Sparkles size={16} class="text-primary shrink-0 mt-0.5" />
      <div class="leading-relaxed">
        <strong>Personalização de Atalhos:</strong> Clique em qualquer tecla para gravar um novo atalho. Pressione a combinação desejada no teclado (ex: <kbd class="px-1 py-0.5 bg-background rounded font-mono text-[10px]">Ctrl+K</kbd>, <kbd class="px-1 py-0.5 bg-background rounded font-mono text-[10px]">Espaço</kbd>, <kbd class="px-1 py-0.5 bg-background rounded font-mono text-[10px]">F</kbd>).
      </div>
    </div>

    <!-- Categories List -->
    {#each ['player', 'navigation', 'system'] as cat}
      {@const items = $shortcutsStore.filter(i => i.category === cat)}
      {#if items.length > 0}
        <div class="flex flex-col gap-2">
          <h4 class="font-bold text-muted uppercase tracking-wider text-[11px]">
            {categoryNames[cat]}
          </h4>
          <div class="flex flex-col gap-1.5">
            {#each items as item (item.id)}
              {@const isCustom = item.currentKey !== item.defaultKey}
              {@const isRecording = recordingShortcutId === item.id}
              <div class="p-2.5 bg-surfaceHover/50 hover:bg-surfaceHover border border-border rounded-xl flex items-center justify-between gap-3 transition-colors">
                <div class="flex flex-col min-w-0 pr-2">
                  <span class="font-semibold text-foreground truncate">{item.label}</span>
                  <span class="text-[11px] text-muted truncate">{item.description}</span>
                </div>
                <div class="flex items-center gap-2 shrink-0">
                  {#if isRecording}
                    <span class="px-2.5 py-1 bg-amber-500/20 text-amber-300 border border-amber-500/40 rounded-lg font-mono font-bold animate-pulse text-xs">
                      Pressione a tecla... (Esc p/ cancelar)
                    </span>
                  {:else}
                    <button
                      type="button"
                      on:click={() => startRecording(item)}
                      title="Clique para alterar este atalho"
                      class="px-2.5 py-1 rounded-lg border font-mono font-bold text-xs transition-all cursor-pointer tv-focusable {isCustom ? 'bg-primary/20 border-primary text-primary shadow-xs' : 'bg-background hover:bg-surface border-border text-foreground'}"
                    >
                      {item.currentKey}
                    </button>
                  {/if}
                </div>
              </div>
            {/each}
          </div>
        </div>
      {/if}
    {/each}

    <!-- Bottom Actions -->
    <div class="flex items-center justify-between pt-3 border-t border-border mt-2">
      <Button variant="ghost" size="sm" on:click={handleReset}>
        <RotateCcw size={14} /> Restaurar Padrões
      </Button>
      <Button variant="primary" size="sm" on:click={() => { recordingShortcutId = null; open = false; }}>
        Salvar & Fechar
      </Button>
    </div>
  </div>
</Modal>
