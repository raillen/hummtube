<script lang="ts">
  import Modal from '../ui/Modal.svelte';
  import Button from '../ui/Button.svelte';
  import { onMount } from 'svelte';
  import { toast } from '../../stores/uiStores';
  import { DiagnosticService, SettingsService } from '../../wailsjs/services';
  import type { DiagnosticsReport } from '../../types';
  import CheckCircle2 from 'lucide-svelte/icons/circle-check';
  import AlertTriangle from 'lucide-svelte/icons/triangle-alert';
  import Headphones from 'lucide-svelte/icons/headphones';
  import KeyRound from 'lucide-svelte/icons/key-round';
  import Monitor from 'lucide-svelte/icons/monitor';

  export let open = false;

  type Step = 'welcome' | 'playback' | 'credentials' | 'done';
  let step: Step = 'welcome';
  let diagnostics: DiagnosticsReport | null = null;
  let loadingDiagnostics = false;

  async function loadDiagnostics(): Promise<void> {
    if (diagnostics || loadingDiagnostics) return;
    loadingDiagnostics = true;
    try {
      diagnostics = await DiagnosticService.getDiagnostics();
    } catch (error: unknown) {
      toast.add(error instanceof Error ? error.message : 'Não foi possível coletar diagnósticos.', 'error');
    } finally {
      loadingDiagnostics = false;
    }
  }

  async function nextStep(): Promise<void> {
    if (step === 'welcome') step = 'playback';
    else if (step === 'playback') { step = 'credentials'; await loadDiagnostics(); }
    else if (step === 'credentials') step = 'done';
    else { open = false; try { await SettingsService.saveSetting('first_run_completed', '1'); } catch {} }
  }

  function skipOnboarding(): void {
    open = false;
    try { void SettingsService.saveSetting('first_run_completed', '1'); } catch {}
  }

  type DiagCheck = { label: string; ok: boolean | null };
  $: diagnosticsChecks = [
    { label: 'Resolução de mídia (yt-dlp)', ok: diagnostics?.checks?.yt_dlp?.ok ?? null },
    { label: 'PO Token Provider', ok: diagnostics?.checks?.pot_provider?.ok ?? null },
    { label: 'Conectividade de API', ok: diagnostics?.checks?.api?.ok ?? null },
  ] as DiagCheck[];

  onMount(() => {
    if (typeof window === 'undefined') return;
    if (window.localStorage.getItem('nanotube_first_run_seen') === '1') {
      open = false;
      return;
    }
    window.localStorage.setItem('nanotube_first_run_seen', '1');
  });
</script>

<Modal title="Bem-vindo ao HummTube" bind:open on:close={skipOnboarding}>
  <div class="flex flex-col gap-4 text-sm">
    {#if step === 'welcome'}
      <div class="flex flex-col items-center gap-3 py-2 text-center">
        <img
          src="/branding/logo-full.png"
          alt="HummTube"
          class="h-16 w-auto object-contain select-none drop-shadow-md"
        />
        <h3 class="text-base font-semibold text-foreground">Vamos configurar o essencial</h3>
        <p class="max-w-md text-xs text-muted">
          Este assistente verifica o player, a autenticação e o hardware em poucos passos. Você pode pular a qualquer momento.
        </p>
      </div>
    {:else if step === 'playback'}
      <h3 class="text-sm font-semibold text-foreground">Player e atalhos</h3>
      <p class="text-xs text-muted">
        Pressione <kbd class="rounded bg-surfaceHover px-1.5 py-0.5 font-mono text-[10px]">F</kbd> para entrar/sair de tela cheia,
        <kbd class="rounded bg-surfaceHover px-1.5 py-0.5 font-mono text-[10px]">M</kbd> para silenciar,
        <kbd class="rounded bg-surfaceHover px-1.5 py-0.5 font-mono text-[10px]">F10</kbd> para o Modo TV.
      </p>
      <p class="text-xs text-muted">
        No Modo TV, use as setas para navegar. A qualidade preferida respeita o limite configurado em
        <strong>Configurações &rarr; Sistema &rarr; Reprodução e qualidade</strong>.
      </p>
    {:else if step === 'credentials'}
      <h3 class="text-sm font-semibold text-foreground">Conta e credenciais</h3>
      <p class="text-xs text-muted">
        Conecte sua conta do YouTube para sincronizar inscrições e listas. Tokens ficam no Keyring do sistema;
        sem Keyring disponível, a sessão fica em memória com aviso visível.
      </p>
      <div class="rounded-xl border border-border bg-background/40 p-3">
        <div class="mb-2 flex items-center gap-2 text-xs text-foreground">
          <Monitor size={14} class="text-primary" /> Diagnóstico rápido
        </div>
        {#if loadingDiagnostics}
          <p class="text-xs text-muted">Coletando informações…</p>
        {:else}
          <ul class="flex flex-col gap-1.5 text-xs">
            {#each diagnosticsChecks as check}
              <li class="flex items-center gap-2">
                {#if check.ok === true}<CheckCircle2 size={14} class="text-emerald-400" />
                {:else if check.ok === false}<AlertTriangle size={14} class="text-amber-300" />
                {:else}<KeyRound size={14} class="text-muted" />{/if}
                <span>{check.label}: {check.ok === null ? 'verificar' : check.ok ? 'ok' : 'atenção'}</span>
              </li>
            {/each}
          </ul>
        {/if}
      </div>
    {:else if step === 'done'}
      <div class="flex flex-col items-center gap-2 py-2 text-center">
        <CheckCircle2 size={42} class="text-emerald-400" />
        <h3 class="text-base font-semibold text-foreground">Tudo pronto</h3>
        <p class="text-xs text-muted">Aproveite. Você pode reexecutar este assistente a qualquer momento em Configurações.</p>
      </div>
    {/if}
    <div class="mt-3 flex items-center justify-between border-t border-border pt-3">
      <Button variant="ghost" size="sm" on:click={skipOnboarding}>Pular</Button>
      <Button variant="primary" size="sm" on:click={nextStep}>
        {step === 'done' ? 'Começar a usar' : step === 'credentials' ? 'Verificar' : 'Próximo'}
      </Button>
    </div>
  </div>
</Modal>
