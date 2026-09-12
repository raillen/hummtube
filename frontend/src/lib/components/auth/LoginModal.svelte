<script lang="ts">
  import { onDestroy } from 'svelte';
  import Modal from '../ui/Modal.svelte';
  import Button from '../ui/Button.svelte';
  import { AccountService, CatalogService, SettingsService } from '../../wailsjs/services';
  import type { AccountInfo, DeviceCodeInfo } from '../../types';
  import { toast, activeTab, isTvMode } from '../../stores/uiStores';
  import LogIn from 'lucide-svelte/icons/log-in';
  import LogOut from 'lucide-svelte/icons/log-out';
  import Cookie from 'lucide-svelte/icons/cookie';
  import CheckCircle2 from 'lucide-svelte/icons/circle-check';
  import Tv from 'lucide-svelte/icons/tv';
  import Key from 'lucide-svelte/icons/key';
  import Upload from 'lucide-svelte/icons/upload';
  import ExternalLink from 'lucide-svelte/icons/external-link';
  import Copy from 'lucide-svelte/icons/copy';
  import Check from 'lucide-svelte/icons/check';

  export let open = false;
  export let account: AccountInfo | null = null;

  type AuthTab = 'oauth' | 'device' | 'import' | 'cookies';
  const supportedCookieBrowsers = ['firefox', 'chrome', 'brave', 'chromium', 'edge', 'opera', 'vivaldi'] as const;
  const supportedCookieKeyrings = ['', 'gnomekeyring', 'kwallet6', 'kwallet5', 'kwallet', 'basictext'] as const;
  type CookieBrowser = (typeof supportedCookieBrowsers)[number];
  type CookieKeyring = (typeof supportedCookieKeyrings)[number];
  let currentTab: AuthTab = 'oauth';

  let isConnecting = false;
  let showCredsConfig = false;

  // Device Code Flow
  let deviceCodeInfo: DeviceCodeInfo | null = null;
  let isRequestingDevice = false;
  let copied = false;
  let loginPoll: ReturnType<typeof setInterval> | null = null;
  let loginTimeout: ReturnType<typeof setTimeout> | null = null;

  let loginWasOpen = false;
  $: if (open && !loginWasOpen) {
    loginWasOpen = true;
    if (!account?.email && $isTvMode && currentTab === 'oauth') currentTab = 'device';
  } else if (!open && loginWasOpen) {
    loginWasOpen = false;
  }

  function stopLoginPolling(): void {
    if (loginPoll) clearInterval(loginPoll);
    if (loginTimeout) clearTimeout(loginTimeout);
    loginPoll = null;
    loginTimeout = null;
  }

  onDestroy(stopLoginPolling);
  $: if (!open) stopLoginPolling();

  // Subscriptions Import
  let importFileInput: HTMLInputElement;
  let isImportingSubs = false;

  // Browser Cookies
  let selectedBrowser: CookieBrowser = 'firefox';
  let selectedCookieKeyring: CookieKeyring = '';
  let isSavingCookies = false;
  let cookieSettingsLoaded = false;

  function isCookieBrowser(value: string): value is CookieBrowser {
    return supportedCookieBrowsers.some((browser) => browser === value);
  }

  function isCookieKeyring(value: string): value is CookieKeyring {
    return supportedCookieKeyrings.some((keyring) => keyring === value);
  }

  function applyBrowserCookieSource(source: string) {
    const [browser = '', keyring = ''] = source.toLowerCase().split('+', 2);
    if (isCookieBrowser(browser)) selectedBrowser = browser;
    if (isCookieKeyring(keyring)) selectedCookieKeyring = keyring;
  }

  async function openCookiesTab() {
    currentTab = 'cookies';
    if (cookieSettingsLoaded) return;
    cookieSettingsLoaded = true;
    try {
      const settings = await SettingsService.getSettings();
      applyBrowserCookieSource(settings['playback.cookies_browser'] || '');
    } catch {
      // O formulário continua utilizável com a seleção padrão.
    }
  }

  async function pollLoginStatus(loginProfileID: string, timeoutMs: number, onConnected: () => void): Promise<void> {
    stopLoginPolling();
    const finish = () => {
      stopLoginPolling();
      isConnecting = false;
      deviceCodeInfo = null;
    };
    loginPoll = setInterval(async () => {
      try {
        const [acc, status] = await Promise.all([
          AccountService.getAccount(),
          AccountService.getLoginStatus(),
        ]);
        if (
          acc?.email &&
          acc.profile_id === loginProfileID &&
          status.state === 'connected' &&
          status.profile_id === loginProfileID
        ) {
          account = acc;
          window.dispatchEvent(new Event('nanotube-profile-changed'));
          window.dispatchEvent(new Event('nanotube-credentials-changed'));
          finish();
          onConnected();
          toast.add(`Conectado como ${acc.email}!`, 'success');
          if (acc.warning || status.warning) {
            toast.add(acc.warning || status.warning || '', 'info');
          }
          open = false;
        } else if (status.state === 'error') {
          finish();
          toast.add(status.error || 'A autenticação não foi concluída', 'error');
        }
      } catch (error: unknown) {
        finish();
        toast.add(error instanceof Error ? error.message : 'Falha ao consultar autenticação', 'error');
      }
    }, 2000);
    loginTimeout = setTimeout(() => {
      if (!loginPoll) return;
      finish();
      toast.add('Tempo esgotado aguardando a autorização', 'error');
    }, timeoutMs);
  }

  async function handleStartGoogleLogin() {
    stopLoginPolling();
    isConnecting = true;
    try {
      const loginProfile = await AccountService.getActiveProfile();
      await AccountService.startGoogleLogin();
      toast.add('Navegador aberto para login seguro!', 'info');
      await pollLoginStatus(loginProfile.id, 120000, () => {});
    } catch (e: unknown) {
      isConnecting = false;
      const msg = e instanceof Error ? e.message : 'Falha ao iniciar autenticação';
      if (msg.includes('credenciais') || msg.includes('client_secret')) {
        showCredsConfig = true;
        toast.add('Configure as credenciais OAuth fora do aplicativo.', 'info');
      } else {
        toast.add(msg, 'error');
      }
    }
  }

  async function handleStartDeviceLogin() {
    stopLoginPolling();
    isRequestingDevice = true;
    try {
      const loginProfile = await AccountService.getActiveProfile();
      deviceCodeInfo = await AccountService.startDeviceLogin();
      toast.add('Código gerado! Acesse google.com/device', 'info');
      await pollLoginStatus(loginProfile.id, (deviceCodeInfo?.expires_in || 1800) * 1000, () => {});
    } catch (e: unknown) {
      const msg = e instanceof Error ? e.message : 'Erro ao solicitar código de dispositivo';
      if (msg.includes('credenciais')) {
        showCredsConfig = true;
        currentTab = 'oauth';
      }
      toast.add(msg, 'error');
    } finally {
      isRequestingDevice = false;
    }
  }

  function handleCopyUserCode() {
    if (!deviceCodeInfo?.user_code) return;
    navigator.clipboard.writeText(deviceCodeInfo.user_code);
    copied = true;
    setTimeout(() => (copied = false), 2500);
    toast.add('Código copiado para a área de transferência!', 'success');
  }

  function handleFileSelect(e: Event) {
    const target = e.target as HTMLInputElement;
    const file = target.files?.[0];
    if (!file) return;
    const reader = new FileReader();
    reader.onload = async (event) => {
      const content = (event.target?.result as string) || '';
      isImportingSubs = true;
      try {
        const res = await AccountService.importSubscriptions(content);
        toast.add(`${res.imported} canais importados com sucesso!`, 'success');
        await CatalogService.refreshSubscriptions();
        open = false;
        $activeTab = 'channels';
      } catch (error: unknown) {
        toast.add(error instanceof Error ? error.message : 'Falha ao importar inscrições', 'error');
      } finally {
        isImportingSubs = false;
        if (importFileInput) importFileInput.value = '';
      }
    };
    reader.readAsText(file);
  }

  async function handleSaveBrowserCookies() {
    isSavingCookies = true;
    try {
      const cookieSource = selectedCookieKeyring
        ? `${selectedBrowser}+${selectedCookieKeyring}`
        : selectedBrowser;
      await AccountService.setBrowserCookies(cookieSource);
      toast.add('Sessão do navegador configurada para vídeos restritos.', 'success');
      open = false;
    } catch (error: unknown) {
      toast.add(error instanceof Error ? error.message : 'Falha ao configurar cookies', 'error');
    } finally {
      isSavingCookies = false;
    }
  }

  async function handleDisconnect() {
    try {
      await AccountService.disconnectAccount();
      account = null;
      window.dispatchEvent(new Event('nanotube-profile-changed'));
      window.dispatchEvent(new Event('nanotube-credentials-changed'));
      toast.add('Conta desconectada com sucesso', 'info');
    } catch (error: unknown) {
      toast.add(error instanceof Error ? error.message : 'Falha ao desconectar conta', 'error');
    }
  }
</script>

<input
  type="file"
  accept=".csv,.opml,.xml,.json"
  bind:this={importFileInput}
  on:change={handleFileSelect}
  class="hidden"
/>

<Modal title="Autenticação & Sincronização" bind:open on:close={() => (open = false)}>
  <div class="flex flex-col gap-4 text-xs">
    {#if account?.email}
      <!-- Connected State -->
      <div class="p-4 bg-surfaceHover/60 border border-border rounded-xl flex items-center justify-between">
        <div class="flex items-center gap-3">
          <div class="w-10 h-10 rounded-full bg-primary/20 text-primary flex items-center justify-center font-bold text-sm border border-primary/30">
            {account.email.charAt(0).toUpperCase()}
          </div>
          <div>
            <div class="flex items-center gap-1.5">
              <h4 class="font-bold text-foreground text-sm">{account.email}</h4>
              <CheckCircle2 size={14} class="text-green-400" />
            </div>
            <p class="text-[11px] text-muted">
              {account.credential_state === 'memory'
                ? 'Sessão temporária em memória'
                : account.credential_state === 'available'
                  ? 'Token protegido pelo Keyring nativo'
                  : 'Credencial persistida indisponível'}
            </p>
            {#if account.warning}
              <p role="status" class="text-[11px] text-amber-300 mt-1 max-w-sm">{account.warning}</p>
            {/if}
          </div>
        </div>

        <Button variant="danger" size="sm" on:click={handleDisconnect}>
          <LogOut size={14} /> Sair
        </Button>
      </div>
    {:else}
      <!-- Tabs Navigation -->
      <div class="flex items-center gap-1.5 bg-surfaceHover/60 p-1 rounded-xl border border-border">
        <button
          type="button"
          on:click={() => (currentTab = 'oauth')}
          class="flex-1 py-1.5 px-2 rounded-lg font-semibold text-center transition-all {currentTab === 'oauth' ? 'bg-primary text-white shadow' : 'text-muted hover:text-foreground'}"
        >
          Google OAuth
        </button>
        <button
          type="button"
          on:click={() => (currentTab = 'device')}
          class="flex-1 py-1.5 px-2 rounded-lg font-semibold text-center transition-all {currentTab === 'device' ? 'bg-primary text-white shadow' : 'text-muted hover:text-foreground'}"
        >
          Modo TV Code
        </button>
        <button
          type="button"
          on:click={() => (currentTab = 'import')}
          class="flex-1 py-1.5 px-2 rounded-lg font-semibold text-center transition-all {currentTab === 'import' ? 'bg-primary text-white shadow' : 'text-muted hover:text-foreground'}"
        >
          Importar (Sem Login)
        </button>
        <button
          type="button"
          on:click={openCookiesTab}
          class="flex-1 py-1.5 px-2 rounded-lg font-semibold text-center transition-all {currentTab === 'cookies' ? 'bg-primary text-white shadow' : 'text-muted hover:text-foreground'}"
        >
          Cookies
        </button>
      </div>

      <!-- Tab Content: Google OAuth2 -->
      {#if currentTab === 'oauth'}
        <div class="flex flex-col gap-3">
          <p class="text-muted leading-relaxed">
            Conecte sua conta do YouTube para sincronizar suas inscrições e playlists. O token fica no Keyring do sistema quando disponível; caso contrário, a sessão permanece somente em memória e o aplicativo exibe um aviso.
          </p>

          <div class="flex justify-center my-2">
            <Button variant="primary" size="md" on:click={handleStartGoogleLogin} disabled={isConnecting} class="w-full max-w-sm">
              <LogIn size={16} /> {isConnecting ? 'Aguardando no navegador...' : 'Entrar com o Google'}
            </Button>
          </div>

          {#if showCredsConfig}
            <div class="p-3 bg-surfaceHover/50 border border-border rounded-xl flex flex-col gap-3 animate-in fade-in duration-150">
              <div class="flex items-center gap-2 font-bold text-foreground">
                <Key size={15} class="text-amber-400" />
                <span>Credenciais OAuth Google ausentes</span>
              </div>
              <p class="text-[11px] text-muted">
                Por segurança, o HummTube não recebe nem grava Client Secret pela interface. Provisione a credencial fora do app usando
                <code>NANOTUBE_GOOGLE_CLIENT_FILE</code> ou coloque o arquivo em
                <code>~/.config/nanotube-web/client_secret.json</code> com permissão restrita.
              </p>
            </div>
          {:else}
            <button
              type="button"
              on:click={() => (showCredsConfig = true)}
              class="text-[11px] text-muted hover:text-foreground underline text-center"
            >
              Como provisionar credenciais OAuth
            </button>
          {/if}
        </div>

      <!-- Tab Content: Device Code (Modo TV) -->
      {:else if currentTab === 'device'}
        <div class="flex flex-col gap-4">
          <p class="text-muted leading-relaxed">
            Ideal para <strong>Modo TV</strong> e controle remoto. Autorize em outro dispositivo (celular ou computador) sem digitar senhas no app:
          </p>

          {#if !deviceCodeInfo}
            <div class="flex justify-center my-2">
              <Button variant="primary" size="md" on:click={handleStartDeviceLogin} disabled={isRequestingDevice} class="w-full max-w-sm">
                <Tv size={16} /> {isRequestingDevice ? 'Gerando Código...' : 'Gerar Código para TV'}
              </Button>
            </div>
          {:else}
            <div class="p-4 bg-primary/10 border border-primary/30 rounded-xl flex flex-col items-center justify-center text-center gap-3">
              <span class="text-xs font-semibold text-muted">1. Acesse pelo celular ou PC:</span>
              <a
                href={deviceCodeInfo.verification_url}
                target="_blank"
                rel="noreferrer"
                class="font-mono text-base font-bold text-primary underline flex items-center gap-1"
              >
                {deviceCodeInfo.verification_url} <ExternalLink size={14} />
              </a>

              <span class="text-xs font-semibold text-muted mt-2">2. Digite este código:</span>
              <div class="flex items-center gap-2">
                <span class="text-2xl font-mono font-extrabold tracking-widest text-foreground bg-surface border border-border px-4 py-2 rounded-xl select-all">
                  {deviceCodeInfo.user_code}
                </span>
                <button
                  type="button"
                  on:click={handleCopyUserCode}
                  class="p-2.5 rounded-xl bg-surface border border-border hover:bg-surfaceHover text-foreground transition-colors"
                  title="Copiar código"
                >
                  {#if copied}
                    <Check size={16} class="text-green-400" />
                  {:else}
                    <Copy size={16} />
                  {/if}
                </button>
              </div>

              <span class="text-[11px] text-muted animate-pulse mt-1">
                Aguardando autorização no Google...
              </span>
            </div>
          {/if}
        </div>

      <!-- Tab Content: Import Subscriptions (No Login) -->
      {:else if currentTab === 'import'}
        <div class="flex flex-col gap-3">
          <p class="text-muted leading-relaxed">
            Importe suas inscrições de forma <strong>100% privada e sem login</strong>. O HummTube sincronizará os vídeos diretamente via RSS local.
          </p>

          <div class="grid grid-cols-1 sm:grid-cols-2 gap-2 text-[11px] text-muted">
            <div class="p-2.5 bg-surfaceHover/50 rounded-xl border border-border">
              <strong class="text-foreground block mb-0.5">Google Takeout</strong>
              <span>Arquivo <code>subscriptions.csv</code></span>
            </div>
            <div class="p-2.5 bg-surfaceHover/50 rounded-xl border border-border">
              <strong class="text-foreground block mb-0.5">OPML / NewPipe / FreeTube</strong>
              <span>Arquivos <code>.opml</code>, <code>.xml</code> ou <code>.json</code></span>
            </div>
          </div>

          <div class="flex justify-center mt-2">
            <Button
              variant="primary"
              size="md"
              on:click={() => importFileInput?.click()}
              disabled={isImportingSubs}
              class="w-full max-w-sm"
            >
              <Upload size={16} /> {isImportingSubs ? 'Importando canais...' : 'Selecionar Arquivo de Inscrições'}
            </Button>
          </div>
        </div>

      <!-- Tab Content: Browser Cookies -->
      {:else if currentTab === 'cookies'}
        <div class="flex flex-col gap-3">
          <p class="text-muted leading-relaxed">
            Utilize a sessão do seu navegador local para que o yt-dlp possa reproduzir <strong>vídeos com restrição de idade (+18)</strong> ou conteúdos de <strong>membros de canais</strong>:
          </p>

          <fieldset class="flex flex-col gap-3 rounded-xl border border-border bg-surfaceHover/30 p-3">
            <legend class="px-1 font-bold text-foreground text-xs">Origem da sessão local</legend>
            <label for="browser-cookies-select" class="flex flex-col gap-1 font-bold text-foreground text-xs">
              Navegador instalado
            <select
              id="browser-cookies-select"
              bind:value={selectedBrowser}
              class="bg-surface border border-border rounded-xl px-3 py-2 text-foreground text-xs focus:outline-none focus:border-primary"
            >
              <option value="firefox">Mozilla Firefox</option>
              <option value="chrome">Google Chrome</option>
              <option value="brave">Brave Browser</option>
              <option value="chromium">Chromium</option>
              <option value="edge">Microsoft Edge</option>
              <option value="opera">Opera</option>
              <option value="vivaldi">Vivaldi</option>
            </select>
            </label>

            <label for="browser-cookie-keyring-select" class="flex flex-col gap-1 font-bold text-foreground text-xs">
              Cofre de senhas do navegador
              <select
                id="browser-cookie-keyring-select"
                bind:value={selectedCookieKeyring}
                class="bg-surface border border-border rounded-xl px-3 py-2 text-foreground text-xs focus:outline-none focus:border-primary"
              >
                <option value="">Detectar automaticamente</option>
                <option value="gnomekeyring">GNOME Keyring / Secret Service</option>
                <option value="kwallet6">KWallet 6</option>
                <option value="kwallet5">KWallet 5</option>
                <option value="kwallet">KWallet</option>
                <option value="basictext">Armazenamento básico</option>
              </select>
            </label>
          </fieldset>

          <p class="rounded-lg border border-amber-400/25 bg-amber-400/10 p-2 text-[11px] leading-relaxed text-muted">
            Se aparecer “cookies não puderam ser descriptografados”, escolha o cofre usado pelo seu desktop. Em GNOME, IceWM com Secret Service ou ambientes semelhantes, normalmente é <strong>GNOME Keyring</strong>.
          </p>

          <div class="flex justify-end gap-2 mt-2">
            <Button variant="primary" size="sm" on:click={handleSaveBrowserCookies} disabled={isSavingCookies}>
              <Cookie size={14} /> Salvar Configuração de Cookies
            </Button>
          </div>
        </div>
      {/if}
    {/if}

    <div class="flex justify-end pt-2 border-t border-border">
      <Button variant="ghost" size="sm" on:click={() => (open = false)}>Fechar</Button>
    </div>
  </div>
</Modal>
