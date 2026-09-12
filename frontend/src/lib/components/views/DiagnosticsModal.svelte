<script lang="ts">
  import Modal from '../ui/Modal.svelte';
  import Button from '../ui/Button.svelte';
  import { SettingsService } from '../../wailsjs/services';
  import type { DiagnosticsReport } from '../../types';

  export let open = false;
  let report: DiagnosticsReport | null = null;
  let isLoading = true;
  let loadError = '';
	let copyFeedback = '';

  async function loadReport() {
    isLoading = true;
    loadError = '';
    try {
      report = await SettingsService.getDiagnostics();
    } catch (error: unknown) {
      loadError = error instanceof Error ? error.message : 'Não foi possível coletar os diagnósticos.';
    } finally {
      isLoading = false;
    }
  }

	async function copyDiagnostics() {
		if (!report) return;
		const lines = [
			`HummTube ${report.version}`,
			`yt-dlp: ${report.ytdlp_version || 'indisponível'}`,
			`JS: ${report.js_runtime || 'indisponível'}`,
			`Extração: ${report.extraction_config || 'indisponível'}`,
			...(report.recent_logs || []),
		];
		try {
			await navigator.clipboard.writeText(lines.join('\n'));
			copyFeedback = 'Diagnóstico copiado.';
		} catch {
			copyFeedback = 'Não foi possível copiar automaticamente.';
		}
	}

  $: if (open) {
    loadReport();
  }
</script>

<Modal title="Diagnóstico do Sistema" bind:open on:close={() => (open = false)}>
  <div class="flex flex-col gap-4 text-xs font-mono" aria-busy={isLoading}>
    <div class="flex items-center justify-center py-1">
      <img src="/branding/logo-full.png" alt="HummTube" class="h-10 w-auto object-contain select-none" />
    </div>
    {#if loadError}<p role="alert" class="rounded-lg border border-red-500/30 bg-red-500/10 p-3 text-red-300">{loadError}</p>{/if}
    <div class="p-3 bg-surfaceHover/50 rounded-lg flex flex-col gap-1.5 border border-border">
      <div class="flex justify-between"><span class="text-muted">Versão:</span><span class="text-foreground">{report?.version || (isLoading ? 'Coletando…' : 'Indisponível')}</span></div>
      <div class="flex justify-between"><span class="text-muted">Stack:</span><span class="text-foreground">{report?.framework || (isLoading ? 'Coletando…' : 'Indisponível')}</span></div>
      <div class="flex justify-between"><span class="text-muted">Go:</span><span class="text-foreground">{report?.go_version || (isLoading ? 'Coletando…' : 'Indisponível')}</span></div>
      <div class="flex justify-between"><span class="text-muted">yt-dlp:</span><span class="text-foreground">{report?.ytdlp_version || (isLoading ? 'Coletando…' : 'Indisponível')}</span></div>
      <div class="flex justify-between"><span class="text-muted">JS Runtime:</span><span class="text-foreground">{report?.js_runtime || (isLoading ? 'Coletando…' : 'Indisponível')}</span></div>
      <div class="flex justify-between gap-3"><span class="text-muted">EJS:</span><span class="text-right text-foreground">{report?.ejs_status || (isLoading ? 'Coletando…' : 'Indisponível')}</span></div>
      <div class="flex justify-between gap-3"><span class="text-muted">PO Token:</span><span class="text-right text-foreground">{report?.pot_provider || (isLoading ? 'Coletando…' : 'Indisponível')} {report?.pot_mode ? `(${report.pot_mode})` : ''}</span></div>
      <div class="flex justify-between gap-3"><span class="text-muted">Manifesto playback:</span><span class="text-right text-foreground">{report?.playback_manifest || (isLoading ? 'Coletando…' : 'Não configurado')}</span></div>
      <div class="flex justify-between gap-3"><span class="text-muted">Extração:</span><span class="break-all text-right text-foreground">{report?.extraction_config || (isLoading ? 'Coletando…' : 'Indisponível')}</span></div>
      <div class="flex justify-between"><span class="text-muted">SQLite:</span><span class="text-foreground">{report?.sqlite_version || (isLoading ? 'Coletando…' : 'Indisponível')}</span></div>
      <div class="flex justify-between"><span class="text-muted">Keyring:</span><span class="text-green-400">{report?.keyring || (isLoading ? 'Coletando…' : 'Indisponível')}</span></div>
    </div>
    {#if report?.extraction_hint}
      <p role="status" class="rounded-lg border border-amber-400/30 bg-amber-400/10 p-3 font-sans text-amber-200">{report.extraction_hint}</p>
    {/if}
    {#if report?.metrics}
      <section class="rounded-lg border border-border bg-surfaceHover/50 p-3 font-sans" aria-labelledby="diagnostics-metrics-title">
        <h3 id="diagnostics-metrics-title" class="mb-2 text-xs font-semibold text-foreground">Métricas desta sessão</h3>
        <div class="grid grid-cols-2 gap-x-4 gap-y-1.5 text-[11px]">
          <span class="text-muted">Buscas</span><strong class="text-foreground">{report.metrics.search_requests}</strong>
          <span class="text-muted">Erros de busca</span><strong class="text-foreground">{report.metrics.search_errors}</strong>
          <span class="text-muted">Reproduções resolvidas</span><strong class="text-foreground">{report.metrics.playback_requests}</strong>
          <span class="text-muted">Cache de playback</span><strong class="text-foreground">{report.metrics.playback_cache_hits}</strong>
          <span class="text-muted">Maior busca</span><strong class="text-foreground">{(report.metrics.search_latency_max_us / 1000).toFixed(0)} ms</strong>
          <span class="text-muted">Maior resolução</span><strong class="text-foreground">{(report.metrics.playback_latency_max_us / 1000).toFixed(0)} ms</strong>
        </div>
      </section>
    {/if}
    {#if report?.warnings?.length}
      <ul aria-label="Avisos do diagnóstico" class="list-disc space-y-1 rounded-lg border border-amber-400/20 bg-amber-400/5 p-3 pl-7 font-sans text-amber-200">
        {#each report.warnings as warning}<li>{warning}</li>{/each}
      </ul>
    {/if}
	<section class="rounded-lg border border-border bg-black/40 p-3 font-sans" aria-labelledby="diagnostics-log-title">
		<div class="mb-2 flex items-center justify-between gap-3">
			<h3 id="diagnostics-log-title" class="text-xs font-semibold text-foreground">Últimos eventos do aplicativo</h3>
			{#if report?.log_file}
				<span class="truncate text-[10px] text-muted" title={report.log_file}>{report.log_file}</span>
			{/if}
		</div>
		<pre class="max-h-48 overflow-auto whitespace-pre-wrap break-words text-[10px] leading-relaxed text-slate-300">{report?.recent_logs?.length ? report.recent_logs.join('\n') : isLoading ? 'Carregando eventos…' : 'Nenhum evento recente registrado.'}</pre>
	</section>
	{#if copyFeedback}<p class="text-right font-sans text-[11px] text-muted" role="status">{copyFeedback}</p>{/if}
	<div class="flex justify-end gap-2 pt-2 border-t border-border">
		<Button variant="secondary" size="sm" disabled={!report} on:click={copyDiagnostics}>Copiar diagnóstico</Button>
      <Button variant="ghost" size="sm" on:click={() => (open = false)}>Fechar</Button>
    </div>
  </div>
</Modal>
