<script lang="ts">
  import { onMount } from 'svelte';
  import Music2 from 'lucide-svelte/icons/music-2';
  import ExternalLink from 'lucide-svelte/icons/external-link';
  import CheckCircle2 from 'lucide-svelte/icons/circle-check';
  import type { LastFMStatus } from '../../lib/types';
  import { LastFMService } from '../../lib/wailsjs/services';
  import { toast } from '../../lib/stores/uiStores';
  import Button from '../../lib/components/ui/Button.svelte';

  let status: LastFMStatus | null = null;
  let authorizationPending = false;
  let busy = false;

  onMount(() => {
    void LastFMService.getStatus().then((loadedStatus) => (status = loadedStatus)).catch((error: unknown) => {
      toast.add(error instanceof Error ? error.message : 'Não foi possível consultar o Last.fm.', 'error');
    });
  });

  async function startAuthorization(): Promise<void> {
    busy = true;
    try {
      const authorizationURL = await LastFMService.startAuthorization();
      window.open(authorizationURL, '_blank', 'noopener,noreferrer');
      authorizationPending = true;
      toast.add('Autorize o NanoMusic no Last.fm e depois conclua a conexão.', 'info');
    } catch (error: unknown) {
      toast.add(error instanceof Error ? error.message : 'Não foi possível iniciar o Last.fm.', 'error');
    } finally {
      busy = false;
    }
  }

  async function completeAuthorization(): Promise<void> {
    busy = true;
    try {
      status = await LastFMService.completeAuthorization();
      window.dispatchEvent(new Event('nanotube-credentials-changed'));
      authorizationPending = false;
      toast.add('Last.fm conectado ao NanoMusic.', 'success');
    } catch (error: unknown) {
      toast.add(error instanceof Error ? error.message : 'A autorização ainda não foi concluída.', 'error');
    } finally {
      busy = false;
    }
  }

  async function disconnect(): Promise<void> {
    busy = true;
    try {
      await LastFMService.disconnect();
      status = { configured: status?.configured ?? false, connected: false };
      window.dispatchEvent(new Event('nanotube-credentials-changed'));
      authorizationPending = false;
      toast.add('Last.fm desconectado.', 'info');
    } catch (error: unknown) {
      toast.add(error instanceof Error ? error.message : 'Não foi possível desconectar o Last.fm.', 'error');
    } finally {
      busy = false;
    }
  }
</script>

<section class="flex flex-col gap-4 rounded-xl border border-border bg-surface p-5 sm:flex-row sm:items-center sm:justify-between" aria-labelledby="lastfm-settings-heading">
  <div class="flex items-start gap-3.5">
    <div class="rounded-xl bg-red-500/10 p-2.5 text-red-400"><Music2 size={20} /></div>
    <div>
      <h3 id="lastfm-settings-heading" class="text-sm font-semibold text-foreground">Last.fm</h3>
      {#if status?.connected}
        <p class="text-xs text-muted">Scrobble ativo como <strong class="text-foreground">{status.username}</strong>.</p>
      {:else}
        <p class="text-xs text-muted">Integração opcional do NanoMusic; a session key fica somente no Keyring próprio.</p>
      {/if}
      {#if status?.warning}<p role="status" class="mt-1 text-[11px] text-amber-300">{status.warning}</p>{/if}
    </div>
  </div>
  <div class="flex flex-wrap gap-2">
    {#if authorizationPending}
      <Button size="sm" on:click={completeAuthorization} disabled={busy}><CheckCircle2 size={14} /> Concluir conexão</Button>
    {:else if status?.connected}
      <Button variant="secondary" size="sm" on:click={startAuthorization} disabled={busy}><ExternalLink size={14} /> Reautorizar</Button>
      <Button variant="danger" size="sm" on:click={disconnect} disabled={busy}>Desconectar</Button>
    {:else}
      <Button size="sm" on:click={startAuthorization} disabled={busy || !status?.configured}><ExternalLink size={14} /> Autorizar Last.fm</Button>
    {/if}
  </div>
</section>
