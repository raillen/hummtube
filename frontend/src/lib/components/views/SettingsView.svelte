<script lang="ts">
  import { onMount } from 'svelte';
  import { SettingsService, AccountService } from '../../wailsjs/services';
  import { encryptBackupJSON, decryptBackupJSON, isEncryptedPayload } from '../../backup/crypto';
  import type { AccountInfo, Profile, SecretInventory, PlaybackRuntimeStatus } from '../../types';
  import Button from '../ui/Button.svelte';
  import Modal from '../ui/Modal.svelte';
  import ConfirmDialog from '../ui/ConfirmDialog.svelte';
  import DiagnosticsModal from './DiagnosticsModal.svelte';
  import ShortcutsModal from '../settings/ShortcutsModal.svelte';
  import LoginModal from '../auth/LoginModal.svelte';
  import { toast, theme, ACCENT_PRESETS, accentColor, applyAccentColor } from '../../stores/uiStores';
  import { applyDisplaySettings } from '../../stores/displayPreferences';
  import { availableLocales, setLocale } from '../../i18n';
  import Shield from 'lucide-svelte/icons/shield';
  import Activity from 'lucide-svelte/icons/activity';
  import Download from 'lucide-svelte/icons/download';
  import Upload from 'lucide-svelte/icons/upload';
  import Keyboard from 'lucide-svelte/icons/keyboard';
  import User from 'lucide-svelte/icons/user';
  import LogOut from 'lucide-svelte/icons/log-out';
  import CheckCircle2 from 'lucide-svelte/icons/circle-check';
  import Palette from 'lucide-svelte/icons/palette';
  import Moon from 'lucide-svelte/icons/moon';
  import Sun from 'lucide-svelte/icons/sun';
  import LogIn from 'lucide-svelte/icons/log-in';
  import Users from 'lucide-svelte/icons/users';
  import UserPlus from 'lucide-svelte/icons/user-plus';
  import Trash2 from 'lucide-svelte/icons/trash-2';
  import AlertTriangle from 'lucide-svelte/icons/triangle-alert';
  import Sliders from 'lucide-svelte/icons/sliders-vertical';
  import Monitor from 'lucide-svelte/icons/monitor';
  import KeyRound from 'lucide-svelte/icons/key-round';
  import RotateCw from 'lucide-svelte/icons/rotate-cw';
  import EyeOff from 'lucide-svelte/icons/eye-off';

  let showDiagnostics = false;
  let showShortcutsModal = false;
  let showImportModal = false;
  let account: AccountInfo | null = null;
  let profiles: Profile[] = [];
  let activeProfile: Profile | null = null;
  let activeProfileID = '';
  let newProfileName = '';
  let isProfileBusy = false;
  let settings: Record<string, string> = {};
  let showLoginModal = false;
  let importJsonData = '';
  let importStrategy: 'merge' | 'replace' | 'duplicate' = 'merge';
  let isImporting = false;
  let exportPassphrase = '';
  let showExportModal = false;
  let showImportDecryptModal = false;
  let importDecryptPassphrase = '';
  let fileInput: HTMLInputElement;
  let thumbnailSize = 'comfortable';
  let thumbnailQuality = 'balanced';
  let homePageSize = '24';
  let subscriptionsPageSize = '24';
  let traySeekSeconds = '10';
  let closeBehavior = 'ask';
  let trayEnabled = true;
  let hideShorts = false;
  let hideLives = false;
  let hideUpcoming = false;
  let hideMixes = false;
  let hideMembers = false;
  let remoteEJS = false;
  let maxVideoHeight = '0';
  let modestHardwareSuggestion = '';
  $: modestHardwareSuggestion = detectModestHardwareSuggestion();
  let selectedLocale = 'pt-BR';
  let secretInventory: SecretInventory = { secrets: [], audit: [] };
  let runtimeStatus: PlaybackRuntimeStatus | null = null;
  let runtimeBusy = false;
  let pendingConfirm:
    | { kind: 'rollback'; trigger: HTMLElement | null }
    | { kind: 'profile'; profile: Profile; trigger: HTMLElement | null }
    | null = null;
  type SettingsTab = 'accounts' | 'appearance' | 'content' | 'system' | 'security';
  let activeSettingsTab: SettingsTab = 'accounts';
  const settingsTabs: Array<{ id: SettingsTab; label: string }> = [
    { id: 'accounts', label: 'Contas e perfis' },
    { id: 'appearance', label: 'Aparência' },
    { id: 'content', label: 'Conteúdo' },
    { id: 'system', label: 'Sistema' },
    { id: 'security', label: 'Segurança e dados' },
  ];
  type VisibilitySetting = 'hide_shorts' | 'hide_lives' | 'hide_upcoming' | 'hide_mixes' | 'hide_members';
  $: visibilityOptions = [
    { id: 'hide_shorts' as VisibilitySetting, label: 'Shorts', checked: hideShorts },
    { id: 'hide_lives' as VisibilitySetting, label: 'Lives em andamento', checked: hideLives },
    { id: 'hide_upcoming' as VisibilitySetting, label: 'Vídeos agendados', checked: hideUpcoming },
    { id: 'hide_mixes' as VisibilitySetting, label: 'Mixes', checked: hideMixes },
    { id: 'hide_members' as VisibilitySetting, label: 'Conteúdo exclusivo para membros', checked: hideMembers },
  ];

  async function loadSettings() {
    try {
      const [loadedAccount, loadedSettings, loadedProfiles, loadedActiveProfile] = await Promise.all([
        AccountService.getAccount(),
        SettingsService.getSettings(),
        AccountService.listProfiles(),
        AccountService.getActiveProfile(),
      ]);
      account = loadedAccount;
      settings = loadedSettings || {};
      thumbnailSize = settings.thumbnail_size || 'comfortable';
      thumbnailQuality = settings.thumbnail_quality || 'balanced';
      homePageSize = settings.home_page_size || '24';
      subscriptionsPageSize = settings.subscriptions_page_size || '24';
      traySeekSeconds = settings.tray_seek_seconds || '10';
      closeBehavior = settings.close_behavior || 'ask';
      trayEnabled = settings['ui.tray_enabled'] !== '0';
      hideShorts = settings.hide_shorts === '1';
      hideLives = settings.hide_lives === '1';
      hideUpcoming = settings.hide_upcoming === '1';
      hideMixes = settings.hide_mixes === '1';
      hideMembers = settings.hide_members === '1';
      remoteEJS = settings['playback.remote_ejs'] === '1';
      maxVideoHeight = settings['playback.max_height'] || '0';
      selectedLocale = settings.locale || (typeof localStorage !== 'undefined' ? localStorage.getItem('nanotube_locale') || 'pt-BR' : 'pt-BR');
      setLocale(selectedLocale);
      applyDisplaySettings(settings);
      profiles = loadedProfiles || [];
      activeProfile = loadedActiveProfile;
      activeProfileID = loadedActiveProfile.id;
      secretInventory = (await SettingsService.getSecretInventory()) || { secrets: [], audit: [] };
      await loadRuntimeStatus();
    } catch (error: unknown) {
      console.error('Erro ao carregar configurações:', error);
      toast.add(error instanceof Error ? error.message : 'Falha ao carregar configurações', 'error');
    }
  }

  async function loadRuntimeStatus(): Promise<void> {
    try {
      runtimeStatus = await SettingsService.getPlaybackRuntime();
    } catch {
      runtimeStatus = null;
    }
  }

  async function activateRuntime(version: string): Promise<void> {
    if (!version || runtimeBusy) return;
    runtimeBusy = true;
    try {
      await SettingsService.activatePlaybackRuntime(version);
      await loadRuntimeStatus();
      toast.add(`Runtime ${version} ativado.`, 'success');
    } catch (error: unknown) {
      toast.add(error instanceof Error ? error.message : 'Não foi possível ativar o runtime.', 'error');
    } finally {
      runtimeBusy = false;
    }
  }

  async function rollbackRuntime(): Promise<void> {
    if (runtimeBusy || !runtimeStatus?.previous_version) return;
    pendingConfirm = { kind: 'rollback', trigger: document.activeElement instanceof HTMLElement ? document.activeElement : null };
  }

  async function applyRollback(): Promise<void> {
    runtimeBusy = true;
    try {
      await SettingsService.rollbackPlaybackRuntime();
      await loadRuntimeStatus();
      toast.add('Runtime anterior restaurado.', 'success');
    } catch (error: unknown) {
      toast.add(error instanceof Error ? error.message : 'Não foi possível restaurar o runtime anterior.', 'error');
    } finally {
      runtimeBusy = false;
    }
  }

  async function handleProfileSelect(event: Event) {
    const profileID = (event.currentTarget as HTMLSelectElement).value;
    if (!profileID || profileID === activeProfileID) return;
    isProfileBusy = true;
    try {
      await AccountService.setActiveProfile(profileID);
      await loadSettings();
      window.dispatchEvent(new Event('nanotube-profile-changed'));
      toast.add(`Perfil ${activeProfile?.name || ''} ativado`, 'success');
    } catch (error: unknown) {
      activeProfileID = activeProfile?.id || '';
      toast.add(error instanceof Error ? error.message : 'Falha ao trocar de perfil', 'error');
    } finally {
      isProfileBusy = false;
    }
  }

  async function handleCreateProfile() {
    const name = newProfileName.trim();
    if (!name) {
      toast.add('Informe um nome para o perfil', 'info');
      return;
    }
    isProfileBusy = true;
    try {
      await AccountService.createProfile(name);
      newProfileName = '';
      await loadSettings();
      window.dispatchEvent(new Event('nanotube-profile-changed'));
      toast.add(`Perfil ${activeProfile?.name || name} criado`, 'success');
    } catch (error: unknown) {
      toast.add(error instanceof Error ? error.message : 'Falha ao criar perfil', 'error');
    } finally {
      isProfileBusy = false;
    }
  }

  async function handleCreateGuestProfile() {
    isProfileBusy = true;
    try {
      await AccountService.createGuestProfile('Convidado');
      await loadSettings();
      window.dispatchEvent(new Event('nanotube-profile-changed'));
      toast.add('Sessão de convidado criada', 'success');
    } catch (error: unknown) {
      toast.add(error instanceof Error ? error.message : 'Falha ao criar sessão de convidado', 'error');
    } finally {
      isProfileBusy = false;
    }
  }

  async function handleDeleteProfile(profile: Profile) {
    if (profile.id === 'default') return;
    pendingConfirm = { kind: 'profile', profile, trigger: document.activeElement instanceof HTMLElement ? document.activeElement : null };
  }

  async function applyDeleteProfile(profile: Profile): Promise<void> {
    isProfileBusy = true;
    try {
      await AccountService.deleteProfile(profile.id);
      await loadSettings();
      window.dispatchEvent(new Event('nanotube-profile-changed'));
      toast.add(`Perfil ${profile.name} removido`, 'info');
    } catch (error: unknown) {
      toast.add(error instanceof Error ? error.message : 'Falha ao remover perfil', 'error');
    } finally {
      isProfileBusy = false;
    }
  }

  function handleStartLogin() {
    showLoginModal = true;
  }

  async function handleAddAccount(): Promise<void> {
    isProfileBusy = true;
    try {
      const profile = await AccountService.createProfile(`Conta ${profiles.filter((entry) => entry.kind !== 'guest').length + 1}`);
      await loadSettings();
      toast.add(`Perfil ${profile.name} criado. Conclua a autorização da conta.`, 'info');
      showLoginModal = true;
      window.dispatchEvent(new Event('nanotube-profile-changed'));
    } catch (error: unknown) {
      toast.add(error instanceof Error ? error.message : 'Não foi possível adicionar outra conta.', 'error');
    } finally {
      isProfileBusy = false;
    }
  }

  async function handleDisconnectAccount() {
    try {
      await AccountService.disconnectAccount();
      account = null;
      window.dispatchEvent(new Event('nanotube-profile-changed'));
      toast.add('Conta desconectada com sucesso', 'info');
    } catch (error: unknown) {
      toast.add(error instanceof Error ? error.message : 'Falha ao desconectar conta', 'error');
    }
  }

  async function handleExportData() {
    try {
      const data = await SettingsService.exportPersonalData();
      const blob = new Blob([data], { type: 'application/json' });
      const url = URL.createObjectURL(blob);
      const a = document.createElement('a');
      a.href = url;
      a.download = `nanotube-backup-${new Date().toISOString().slice(0, 10)}.json`;
      a.click();
      URL.revokeObjectURL(url);
      toast.add('Backup de dados pessoais exportado!', 'success');
    } catch (error: unknown) {
      toast.add(error instanceof Error ? error.message : 'Falha ao exportar dados', 'error');
    }
  }

  async function handleExportEncrypted() {
    if (exportPassphrase.length < 6) {
      toast.add('A senha precisa ter pelo menos 6 caracteres.', 'error');
      return;
    }
    try {
      const data = await SettingsService.exportPersonalData();
      const payload = await encryptBackupJSON(data, exportPassphrase);
      const blob = new Blob([payload], { type: 'application/json' });
      const url = URL.createObjectURL(blob);
      const a = document.createElement('a');
      a.href = url;
      a.download = `nanotube-backup-${new Date().toISOString().slice(0, 10)}.enc.json`;
      a.click();
      URL.revokeObjectURL(url);
      toast.add('Backup criptografado exportado.', 'success');
    } catch (error: unknown) {
      toast.add(error instanceof Error ? error.message : 'Falha ao exportar backup criptografado.', 'error');
    }
  }

  function handleFileSelect(e: Event) {
    const target = e.target as HTMLInputElement;
    const file = target.files?.[0];
    if (!file) return;
    const reader = new FileReader();
    reader.onload = (event) => {
      importJsonData = (event.target?.result as string) || '';
      if (isEncryptedPayload(importJsonData)) {
        showImportDecryptModal = true;
      } else {
        showImportModal = true;
      }
      if (fileInput) fileInput.value = '';
    };
    reader.readAsText(file);
  }

  async function handleConfirmImport() {
    if (!importJsonData.trim()) return;
    isImporting = true;
    try {
      await SettingsService.importPersonalData(importJsonData, importStrategy);
      toast.add('Backup restaurado com sucesso!', 'success');
      showImportModal = false;
      importJsonData = '';
      await loadSettings();
    } catch (error: unknown) {
      toast.add(error instanceof Error ? error.message : 'Erro ao importar backup', 'error');
    } finally {
      isImporting = false;
    }
  }

  async function handleDecryptImport() {
    if (importDecryptPassphrase.length < 1) {
      toast.add('Informe a senha do backup.', 'error');
      return;
    }
    isImporting = true;
    try {
      importJsonData = await decryptBackupJSON(importJsonData, importDecryptPassphrase);
      importDecryptPassphrase = '';
      showImportDecryptModal = false;
      showImportModal = true;
    } catch (error: unknown) {
      toast.add('Senha inválida ou arquivo corrompido.', 'error');
    } finally {
      isImporting = false;
    }
  }

  function setThemeMode(m: 'dark' | 'light') {
    $theme = m;
    document.documentElement.classList.toggle('light', m === 'light');
    void savePreference('theme', m);
  }

  function changeLocale(next: string): void {
    selectedLocale = setLocale(next);
    void savePreference('locale', selectedLocale);
  }

  async function savePreference(key: string, value: string): Promise<void> {
    try {
      await SettingsService.saveSetting(key, value);
      settings = { ...settings, [key]: value };
      applyDisplaySettings(settings);
      toast.add('Preferência salva.', 'success');
    } catch (error: unknown) {
      toast.add(error instanceof Error ? error.message : 'Não foi possível salvar a preferência.', 'error');
    }
  }

  function saveToggle(key: string, enabled: boolean): void {
    void savePreference(key, enabled ? '1' : '0');
  }

  function detectModestHardwareSuggestion(): string {
    if (typeof navigator === 'undefined') return '';
    const deviceMemory = (navigator as Navigator & { deviceMemory?: number }).deviceMemory;
    const hardwareConcurrency = navigator.hardwareConcurrency || 0;
    const lowMemory = typeof deviceMemory === 'number' && deviceMemory > 0 && deviceMemory <= 2;
    const lowCpu = hardwareConcurrency > 0 && hardwareConcurrency <= 2;
    if (lowMemory || lowCpu) {
      return 'Limite recomendado: 720p para hardware modesto.';
    }
    return '';
  }

  function applyModestHardwareDefault(): void {
    if (maxVideoHeight !== '0' || !modestHardwareSuggestion) return;
    maxVideoHeight = '720';
    void savePreference('playback.max_height', '720');
    toast.add('Limite de resolução aplicado: 720p (hardware modesto detectado).', 'info');
  }

  function changeVisibility(id: VisibilitySetting, enabled: boolean): void {
    if (id === 'hide_shorts') hideShorts = enabled;
    if (id === 'hide_lives') hideLives = enabled;
    if (id === 'hide_upcoming') hideUpcoming = enabled;
    if (id === 'hide_mixes') hideMixes = enabled;
    if (id === 'hide_members') hideMembers = enabled;
    saveToggle(id, enabled);
  }

  function formatCredentialDate(value?: string): string {
    if (!value) return 'Sem registro';
    const date = new Date(value);
    return Number.isNaN(date.getTime()) ? 'Data indisponível' : new Intl.DateTimeFormat('pt-BR', { dateStyle: 'medium', timeStyle: 'short' }).format(date);
  }

  onMount(() => {
    void loadSettings();
    const handleCredentialChange = () => { void loadSettings(); };
    window.addEventListener('nanotube-credentials-changed', handleCredentialChange);
    return () => window.removeEventListener('nanotube-credentials-changed', handleCredentialChange);
  });
</script>

<input
  type="file"
  accept=".json"
  bind:this={fileInput}
  on:change={handleFileSelect}
  class="hidden"
/>

<div class="flex flex-col gap-6 p-6 max-w-4xl mx-auto w-full">
  <div>
    <h1 class="text-xl font-bold text-foreground">Configurações & Ajustes</h1>
    <p class="text-xs text-muted mt-0.5">Preferências locais, autenticação, segurança, backup e diagnósticos</p>
  </div>

  <nav class="flex gap-1 overflow-x-auto rounded-xl border border-border bg-surface p-1" aria-label="Seções dos ajustes">
    {#each settingsTabs as tab (tab.id)}
      <button type="button" on:click={() => (activeSettingsTab = tab.id)} aria-current={activeSettingsTab === tab.id ? 'page' : undefined} class="whitespace-nowrap rounded-lg px-3 py-2 text-xs font-semibold tv-focusable {activeSettingsTab === tab.id ? 'bg-primary/15 text-primary' : 'text-muted hover:bg-surfaceHover hover:text-foreground'}">{tab.label}</button>
    {/each}
  </nav>

  <div class="flex flex-col gap-4">
    <!-- Local profiles -->
    <div class="p-5 bg-surface border border-border rounded-xl flex flex-col gap-4" class:hidden={activeSettingsTab !== 'accounts'}>
      <div class="flex items-start gap-3.5">
        <div class="p-2.5 bg-cyan-500/10 text-cyan-400 rounded-xl">
          <Users size={20} />
        </div>
        <div>
          <h3 class="text-sm font-semibold text-foreground">Perfis locais</h3>
          <p class="text-xs text-muted">Separe contas e sessões sem compartilhar credenciais entre perfis</p>
        </div>
      </div>

      <div class="grid grid-cols-1 sm:grid-cols-[minmax(0,1fr)_auto] gap-3">
        <label class="flex flex-col gap-1.5 text-xs font-semibold text-foreground" for="active-profile">
          Perfil ativo
          <select
            id="active-profile"
            value={activeProfileID}
            on:change={handleProfileSelect}
            disabled={isProfileBusy}
            class="bg-surfaceHover border border-border rounded-xl px-3 py-2 text-foreground text-xs focus:outline-none focus:border-primary disabled:opacity-50"
          >
            {#each profiles as profile}
              <option value={profile.id}>{profile.name}{profile.kind === 'guest' ? ' (convidado)' : ''}</option>
            {/each}
          </select>
        </label>
        <Button variant="secondary" size="sm" on:click={handleCreateGuestProfile} disabled={isProfileBusy}>
          <UserPlus size={14} /> Usar como convidado
        </Button>
      </div>

      <div class="flex flex-col sm:flex-row gap-2">
        <label class="sr-only" for="new-profile-name">Nome do novo perfil</label>
        <input
          id="new-profile-name"
          type="text"
          bind:value={newProfileName}
          maxlength="80"
          placeholder="Nome do novo perfil"
          class="flex-1 bg-surfaceHover border border-border rounded-xl px-3 py-2 text-foreground text-xs placeholder:text-muted focus:outline-none focus:border-primary"
        />
        <Button variant="primary" size="sm" on:click={handleCreateProfile} disabled={isProfileBusy || !newProfileName.trim()}>
          <UserPlus size={14} /> Criar perfil
        </Button>
        <Button variant="secondary" size="sm" on:click={handleAddAccount} disabled={isProfileBusy}>
          <UserPlus size={14} /> Adicionar conta
        </Button>
      </div>

      {#if activeProfile?.kind === 'guest'}
        <p role="status" class="flex items-start gap-2 text-[11px] text-amber-300 bg-amber-500/10 border border-amber-500/20 rounded-lg p-2.5">
          <AlertTriangle size={14} class="shrink-0 mt-0.5" />
          Este perfil é temporário. A conta, os vínculos locais e a sessão em memória serão descartados quando o aplicativo fechar ou reiniciar.
        </p>
      {/if}

      {#if profiles.length > 1}
        <div class="flex flex-wrap gap-2 pt-3 border-t border-border" aria-label="Perfis disponíveis">
          {#each profiles as profile}
            <div class="inline-flex items-center gap-1.5 bg-surfaceHover border border-border rounded-full pl-3 pr-1 py-1 text-[11px]">
              <span class={profile.id === activeProfileID ? 'text-primary font-semibold' : 'text-muted'}>{profile.name}</span>
              {#if profile.id !== 'default'}
                <button
                  type="button"
                  class="p-1 rounded-full text-muted hover:text-red-400 hover:bg-red-500/10 disabled:opacity-50"
                  on:click={() => handleDeleteProfile(profile)}
                  disabled={isProfileBusy}
                  aria-label={`Remover perfil ${profile.name}`}
                  title={`Remover perfil ${profile.name}`}
                >
                  <Trash2 size={12} />
                </button>
              {/if}
            </div>
          {/each}
        </div>
      {/if}
    </div>

    <!-- Google Account Connection -->
    <div class="p-5 bg-surface border border-border rounded-xl flex items-center justify-between" class:hidden={activeSettingsTab !== 'accounts'}>
      <div class="flex items-center gap-3.5">
        <div class="p-2.5 bg-red-500/10 text-red-400 rounded-xl">
          <User size={20} />
        </div>
        <div>
          <h3 class="text-sm font-semibold text-foreground">Conta do YouTube / Google</h3>
          {#if account?.email}
            <p class="text-xs text-muted">Conectado como <strong class="text-foreground">{account.email}</strong></p>
            <p class="text-[11px] text-muted mt-0.5">
              {account.credential_state === 'memory'
                ? 'Sessão temporária em memória'
                : account.credential_state === 'available'
                  ? 'Credencial protegida pelo Keyring'
                  : 'Credencial persistida indisponível'}
            </p>
            {#if account.warning}
              <p role="status" class="text-[11px] text-amber-300 mt-1 max-w-lg">{account.warning}</p>
            {/if}
          {:else}
            <p class="text-xs text-muted">Nenhuma conta vinculada (usando modo local anônimo e feeds RSS)</p>
          {/if}
        </div>
      </div>
      {#if account?.email}
        <Button variant="danger" size="sm" on:click={handleDisconnectAccount}>
          <LogOut size={14} /> Desconectar
        </Button>
      {:else}
        <Button variant="secondary" size="sm" on:click={handleStartLogin}>
          <LogIn size={14} /> Conectar Google
        </Button>
      {/if}
    </div>

    <!-- Appearance / Theme -->
    <div class="p-5 bg-surface border border-border rounded-xl flex flex-col gap-4" class:hidden={activeSettingsTab !== 'appearance'}>
      <div class="flex items-center justify-between">
        <div class="flex items-center gap-3.5">
          <div class="p-2.5 bg-indigo-500/10 text-indigo-400 rounded-xl">
            <Palette size={20} />
          </div>
          <div>
            <h3 class="text-sm font-semibold text-foreground">Aparência do Tema</h3>
            <p class="text-xs text-muted">Selecione o esquema de cores da interface</p>
          </div>
        </div>
        <div class="flex items-center gap-1.5 bg-surfaceHover p-1 rounded-xl border border-border">
          <button
            type="button"
            on:click={() => setThemeMode('dark')}
            class="flex items-center gap-1.5 px-3 py-1.5 rounded-lg text-xs font-semibold transition-all {$theme === 'dark' ? 'bg-primary text-white shadow' : 'text-muted hover:text-foreground'}"
          >
            <Moon size={14} /> Escuro
          </button>
          <button
            type="button"
            on:click={() => setThemeMode('light')}
            class="flex items-center gap-1.5 px-3 py-1.5 rounded-lg text-xs font-semibold transition-all {$theme === 'light' ? 'bg-primary text-white shadow' : 'text-muted hover:text-foreground'}"
          >
            <Sun size={14} /> Claro
          </button>
        </div>
      </div>

      <div class="pt-3 border-t border-border flex flex-col gap-1.5 sm:flex-row sm:items-center sm:justify-between">
        <div>
          <h4 class="text-xs font-semibold text-foreground">Idioma da interface</h4>
          <p class="text-[11px] text-muted">Os catálogos podem ser ampliados sem alterar o domínio do aplicativo.</p>
        </div>
        <label class="sr-only" for="interface-locale">Idioma da interface</label>
        <select id="interface-locale" bind:value={selectedLocale} on:change={(event) => changeLocale(event.currentTarget.value)} class="rounded-xl border border-border bg-surfaceHover px-3 py-2 text-xs text-foreground">
          {#each availableLocales as availableLocale}
            <option value={availableLocale}>{availableLocale === 'pt-BR' ? 'Português (Brasil)' : 'English (US)'}</option>
          {/each}
        </select>
      </div>

      <!-- Accent Color Palette Selector -->
      <div class="pt-3 border-t border-border flex flex-col sm:flex-row sm:items-center justify-between gap-3">
        <div>
          <h4 class="text-xs font-semibold text-foreground">Cor de Destaque (Accent Color)</h4>
          <p class="text-[11px] text-muted">Personalize a cor dos botões, anéis de foco e elementos ativos</p>
        </div>
        <div class="flex items-center gap-2 flex-wrap">
          {#each ACCENT_PRESETS as preset}
            <button
              type="button"
              on:click={() => applyAccentColor(preset.id)}
              title={preset.name}
              class="w-7 h-7 rounded-full flex items-center justify-center transition-all cursor-pointer tv-focusable relative {$accentColor === preset.id ? 'ring-2 ring-foreground scale-110' : 'opacity-80 hover:opacity-100'}"
              style="background-color: {preset.primary};"
            >
              {#if $accentColor === preset.id}
                <CheckCircle2 size={14} class="text-white drop-shadow" />
              {/if}
            </button>
          {/each}
        </div>
      </div>
    </div>

    <div class="p-5 bg-surface border border-border rounded-xl flex flex-col gap-4" class:hidden={activeSettingsTab !== 'content'}>
      <div class="flex items-start gap-3.5">
        <div class="p-2.5 bg-sky-500/10 text-sky-400 rounded-xl"><Sliders size={20} /></div>
        <div><h3 class="text-sm font-semibold text-foreground">Conteúdo e miniaturas</h3><p class="text-xs text-muted">Controle densidade visual, tráfego de imagens e tamanho dos lotes.</p></div>
      </div>
      <div class="grid grid-cols-1 gap-3 sm:grid-cols-2">
        <label class="flex flex-col gap-1.5 text-xs text-muted">Tamanho das miniaturas
          <select bind:value={thumbnailSize} on:change={() => savePreference('thumbnail_size', thumbnailSize)} class="rounded-xl border border-border bg-surfaceHover px-3 py-2 text-foreground">
            <option value="compact">Compacto</option><option value="comfortable">Confortável</option><option value="large">Grande</option>
          </select>
        </label>
        <label class="flex flex-col gap-1.5 text-xs text-muted">Qualidade das miniaturas
          <select bind:value={thumbnailQuality} on:change={() => savePreference('thumbnail_quality', thumbnailQuality)} class="rounded-xl border border-border bg-surfaceHover px-3 py-2 text-foreground">
            <option value="data_saver">Economia de dados</option><option value="balanced">Equilibrada</option><option value="high">Alta</option>
          </select>
        </label>
        <label class="flex flex-col gap-1.5 text-xs text-muted">Vídeos por lote na página inicial
          <select bind:value={homePageSize} on:change={() => savePreference('home_page_size', homePageSize)} class="rounded-xl border border-border bg-surfaceHover px-3 py-2 text-foreground">
            <option value="12">12</option><option value="24">24</option><option value="36">36</option><option value="48">48</option>
          </select>
        </label>
        <label class="flex flex-col gap-1.5 text-xs text-muted">Vídeos por lote nas inscrições
          <select bind:value={subscriptionsPageSize} on:change={() => savePreference('subscriptions_page_size', subscriptionsPageSize)} class="rounded-xl border border-border bg-surfaceHover px-3 py-2 text-foreground">
            <option value="12">12</option><option value="24">24</option><option value="36">36</option><option value="48">48</option>
          </select>
        </label>
      </div>
      <fieldset class="grid gap-2 border-t border-border pt-4 sm:grid-cols-2">
        <legend class="mb-2 text-xs font-semibold text-foreground">Ocultar globalmente</legend>
        {#each visibilityOptions as visibility (visibility.id)}
          <label class="flex items-center gap-2 rounded-lg border border-border bg-background/50 p-2.5 text-xs text-foreground">
            <input type="checkbox" checked={visibility.checked} on:change={(event) => changeVisibility(visibility.id, event.currentTarget.checked)} class="accent-primary" />
            <EyeOff size={14} class="text-muted" /> {visibility.label}
          </label>
        {/each}
      </fieldset>
    </div>

    <div class="p-5 bg-surface border border-border rounded-xl flex flex-col gap-4" class:hidden={activeSettingsTab !== 'system'}>
      <div class="flex items-start gap-3.5">
        <div class="p-2.5 bg-emerald-500/10 text-emerald-400 rounded-xl"><Monitor size={20} /></div>
        <div><h3 class="text-sm font-semibold text-foreground">Janela e bandeja do sistema</h3><p class="text-xs text-muted">Defina o fechamento e o intervalo usado pelos controles de avanço/retrocesso.</p></div>
      </div>
      <div class="grid grid-cols-1 gap-3 sm:grid-cols-2">
        <label class="flex items-center gap-2 rounded-xl border border-border bg-background/50 px-3 py-2 text-xs text-foreground sm:col-span-2">
          <input type="checkbox" bind:checked={trayEnabled} on:change={() => saveToggle('ui.tray_enabled', trayEnabled)} class="accent-primary" />
          Ativar bandeja do sistema
        </label>
        <label class="flex flex-col gap-1.5 text-xs text-muted">Ao fechar a janela
          <select bind:value={closeBehavior} on:change={() => savePreference('close_behavior', closeBehavior)} class="rounded-xl border border-border bg-surfaceHover px-3 py-2 text-foreground">
            <option value="ask">Perguntar</option><option value="minimize">Minimizar para a bandeja</option><option value="quit">Encerrar aplicativo</option>
          </select>
        </label>
        <label class="flex flex-col gap-1.5 text-xs text-muted">Avançar/retroceder na bandeja
          <select bind:value={traySeekSeconds} on:change={() => savePreference('tray_seek_seconds', traySeekSeconds)} class="rounded-xl border border-border bg-surfaceHover px-3 py-2 text-foreground">
            <option value="5">5 segundos</option><option value="10">10 segundos</option><option value="15">15 segundos</option><option value="30">30 segundos</option><option value="60">60 segundos</option>
          </select>
        </label>
      </div>
    </div>

    <!-- Playback / quality -->
    <div class="p-5 bg-surface border border-border rounded-xl flex flex-col gap-4" class:hidden={activeSettingsTab !== 'system'}>
      <div class="flex items-start gap-3.5">
        <div class="p-2.5 bg-violet-500/10 text-violet-400 rounded-xl"><Sliders size={20} /></div>
        <div>
          <h3 class="text-sm font-semibold text-foreground">Reprodução e qualidade</h3>
          <p class="text-xs text-muted">A resolução final depende dos formatos liberados pelo YouTube e pelo yt-dlp.</p>
        </div>
      </div>
      <div class="grid grid-cols-1 gap-3 sm:grid-cols-2">
        <label class="flex items-center gap-2 rounded-xl border border-border bg-background/50 px-3 py-2 text-xs text-foreground sm:col-span-2">
          <input type="checkbox" bind:checked={remoteEJS} on:change={() => saveToggle('playback.remote_ejs', remoteEJS)} class="accent-primary" />
          Permitir baixar o solver EJS quando ele não estiver instalado
        </label>
        <p class="text-[11px] leading-relaxed text-muted sm:col-span-2">
          O solver resolve desafios recentes do YouTube e pode liberar 720p/1080p. Quando ativado, o yt-dlp baixa código externo de sua fonte configurada; para máxima previsibilidade, prefira instalar o pacote compatível <code>yt-dlp-ejs</code> junto ao yt-dlp.
        </p>
        <label class="flex flex-col gap-1.5 text-xs text-muted">Limite de resolução
          <select bind:value={maxVideoHeight} on:change={() => savePreference('playback.max_height', maxVideoHeight)} class="rounded-xl border border-border bg-surfaceHover px-3 py-2 text-foreground">
            <option value="0">Automática (sem limite)</option><option value="360">Até 360p</option><option value="480">Até 480p</option><option value="720">Até 720p</option><option value="1080">Até 1080p</option><option value="1440">Até 1440p</option><option value="2160">Até 2160p</option>
          </select>
          {#if modestHardwareSuggestion}
            <span class="text-[10px] text-amber-300">{modestHardwareSuggestion}</span>
            {#if maxVideoHeight === '0'}
              <button type="button" class="self-start rounded-md bg-amber-500/20 px-2 py-1 text-[10px] font-semibold text-amber-200 hover:bg-amber-500/30 tv-focusable" on:click={applyModestHardwareDefault}>Aplicar 720p</button>
            {/if}
          {/if}
        </label>
      </div>
    </div>

    <section class="p-5 bg-surface border border-border rounded-xl flex flex-col gap-4" class:hidden={activeSettingsTab !== 'system'} aria-labelledby="playback-runtime-title">
      <div class="flex items-start gap-3.5">
        <div class="p-2.5 bg-orange-500/10 text-orange-300 rounded-xl"><RotateCw size={20} /></div>
        <div>
          <h3 id="playback-runtime-title" class="text-sm font-semibold text-foreground">Runtime de playback</h3>
          <p class="text-xs text-muted">Versões instaladas são ativadas atomicamente; nenhuma atualização é baixada sem consentimento.</p>
        </div>
      </div>
      {#if runtimeStatus}
        <div class="flex flex-wrap items-center gap-2 text-xs" aria-live="polite">
          <span class="rounded-full border border-border bg-background/50 px-2.5 py-1 text-foreground">Ativo: <strong>{runtimeStatus.active_version || 'sistema/PATH'}</strong></span>
          {#if runtimeStatus.previous_version}<span class="rounded-full border border-border bg-background/50 px-2.5 py-1 text-muted">Anterior: {runtimeStatus.previous_version}</span>{/if}
        </div>
        {#if runtimeStatus.installed_versions.length > 0}
          <div class="grid gap-2 sm:grid-cols-2">
            {#each runtimeStatus.installed_versions as version (version)}
              <div class="flex items-center justify-between gap-3 rounded-xl border border-border bg-background/50 px-3 py-2">
                <span class="truncate text-xs text-foreground">{version}</span>
                <Button size="sm" variant={version === runtimeStatus.active_version ? 'ghost' : 'secondary'} disabled={runtimeBusy || version === runtimeStatus.active_version} on:click={() => activateRuntime(version)}>{version === runtimeStatus.active_version ? 'Ativo' : 'Ativar'}</Button>
              </div>
            {/each}
          </div>
        {:else}
          <p class="rounded-lg border border-dashed border-border p-3 text-xs text-muted">Nenhum runtime gerenciado instalado. O HummTube usa o yt-dlp disponível no sistema.</p>
        {/if}
        <div class="flex justify-end border-t border-border pt-3">
          <Button size="sm" variant="secondary" disabled={runtimeBusy || !runtimeStatus.previous_version} on:click={rollbackRuntime}><RotateCw size={14} /> Restaurar versão anterior</Button>
        </div>
      {:else}
        <p class="rounded-lg border border-dashed border-border p-3 text-xs text-muted">Gerenciador de runtime indisponível neste ambiente.</p>
      {/if}
    </section>

    <!-- Diagnostics -->
    <div class="p-5 bg-surface border border-border rounded-xl flex items-center justify-between" class:hidden={activeSettingsTab !== 'system'}>
      <div class="flex items-center gap-3.5">
        <div class="p-2.5 bg-blue-500/10 text-blue-400 rounded-xl">
          <Activity size={20} />
        </div>
        <div>
          <h3 class="text-sm font-semibold text-foreground">Diagnósticos do Sistema</h3>
          <p class="text-xs text-muted">Verifique o status do yt-dlp, aceleração gráfica e runtime JS</p>
        </div>
      </div>
      <Button variant="secondary" size="sm" on:click={() => (showDiagnostics = true)}>
        Abrir Diagnóstico
      </Button>
    </div>

    <!-- Keyboard Shortcuts -->
    <div class="p-5 bg-surface border border-border rounded-xl flex items-center justify-between" class:hidden={activeSettingsTab !== 'system'}>
      <div class="flex items-center gap-3.5">
        <div class="p-2.5 bg-amber-500/10 text-amber-400 rounded-xl">
          <Keyboard size={20} />
        </div>
        <div>
          <h3 class="text-sm font-semibold text-foreground">Atalhos de Teclado & Modo TV</h3>
          <p class="text-xs text-muted">Consulte e personalize os atalhos de reprodução, volume e navegação</p>
        </div>
      </div>
      <Button variant="secondary" size="sm" on:click={() => (showShortcutsModal = true)}>
        Personalizar Atalhos
      </Button>
    </div>

    <!-- Backup & Restore -->
    <div class="p-5 bg-surface border border-border rounded-xl flex items-center justify-between" class:hidden={activeSettingsTab !== 'security'}>
      <div class="flex items-center gap-3.5">
        <div class="p-2.5 bg-green-500/10 text-green-400 rounded-xl">
          <Download size={20} />
        </div>
        <div>
          <h3 class="text-sm font-semibold text-foreground">Backup & Portabilidade Local</h3>
          <p class="text-xs text-muted">Exporte ou restaure suas playlists, notas, inscrições e histórico</p>
        </div>
      </div>
      <div class="flex items-center gap-2">
        <Button variant="secondary" size="sm" on:click={() => fileInput?.click()}>
          <Upload size={14} /> Importar
        </Button>
        <Button variant="secondary" size="sm" on:click={handleExportData}>
          <Download size={14} /> Exportar JSON
        </Button>
        <Button variant="primary" size="sm" on:click={() => showExportModal = true}>
          <Shield size={14} /> Exportar criptografado
        </Button>
      </div>
    </div>

    <div class:hidden={activeSettingsTab !== 'security'}><slot name="product-integrations" /></div>

    <!-- Security & Privacy -->
    <div class="p-5 bg-surface border border-border rounded-xl flex items-center justify-between" class:hidden={activeSettingsTab !== 'security'}>
      <div class="flex items-center gap-3.5">
        <div class="p-2.5 bg-purple-500/10 text-purple-400 rounded-xl">
          <Shield size={20} />
        </div>
        <div>
          <h3 class="text-sm font-semibold text-foreground">Privacidade & Credenciais</h3>
          <p class="text-xs text-muted">Tokens no Keyring quando disponível; credenciais OAuth são provisionadas fora do aplicativo</p>
        </div>
      </div>
      <span class="text-xs font-semibold px-2.5 py-1 rounded-full border flex items-center gap-1 {account?.session_persistence === 'memory' ? 'bg-amber-500/10 text-amber-300 border-amber-500/20' : 'bg-green-500/10 text-green-400 border-green-500/20'}">
        {#if account?.session_persistence === 'memory'}
          <AlertTriangle size={12} /> Sessão temporária
        {:else if account?.credential_state === 'available'}
          <CheckCircle2 size={12} /> Token no Keyring
        {:else}
          <Shield size={12} /> Sem token salvo
        {/if}
      </span>
    </div>

    <section class="rounded-xl border border-border bg-surface p-5" class:hidden={activeSettingsTab !== 'security'} aria-labelledby="secret-manager-title">
      <header class="mb-4 flex items-start gap-3">
        <span class="rounded-xl bg-purple-500/10 p-2.5 text-purple-400"><KeyRound size={20} /></span>
        <div><h3 id="secret-manager-title" class="text-sm font-semibold text-foreground">Gerenciador seguro de credenciais</h3><p class="text-xs text-muted">Somente metadados são exibidos. Tokens, API keys e secrets nunca atravessam a interface.</p></div>
      </header>
      {#if secretInventory.secrets.length === 0}
        <p class="rounded-lg border border-dashed border-border p-4 text-xs text-muted">Nenhuma credencial vinculada ao perfil ativo.</p>
      {:else}
        <div class="grid gap-2">
          {#each secretInventory.secrets as secret (secret.id)}
            <article class="flex flex-wrap items-center gap-3 rounded-xl border border-border bg-background/50 p-3">
              <KeyRound size={16} class="text-primary" />
              <div class="min-w-0 flex-1"><strong class="block truncate text-xs text-foreground">{secret.label || secret.provider}</strong><span class="text-[10px] text-muted">{secret.provider.toUpperCase()} · {secret.storage === 'keyring' ? 'Keyring do sistema' : secret.storage} · rotação: {formatCredentialDate(secret.rotated_at)} · expiração: {secret.expires_at ? formatCredentialDate(secret.expires_at) : 'não informada pelo provedor'}</span>{#if secret.warning}<p class="mt-1 text-[10px] text-amber-300">{secret.warning}</p>{/if}</div>
              <span class="rounded-full border border-border px-2 py-1 text-[10px] font-semibold {secret.state === 'available' ? 'text-emerald-400' : secret.state === 'memory' ? 'text-amber-300' : 'text-muted'}">{secret.state}</span>
              {#if secret.provider === 'google' && secret.can_rotate}<Button size="sm" variant="secondary" on:click={handleStartLogin}><RotateCw size={13} /> Rotacionar</Button>{/if}
              {#if secret.provider === 'google' && secret.can_revoke}<Button size="sm" variant="danger" on:click={handleDisconnectAccount}>Revogar</Button>{/if}
            </article>
          {/each}
        </div>
      {/if}
      <details class="mt-4 rounded-xl border border-border bg-background/40 p-3">
        <summary class="cursor-pointer text-xs font-semibold text-foreground tv-focusable">Auditoria local ({secretInventory.audit.length})</summary>
        <ol class="mt-3 grid max-h-52 gap-2 overflow-y-auto">
          {#each secretInventory.audit as event (event.id)}
            <li class="flex items-start justify-between gap-3 border-b border-border/60 pb-2 text-[10px]"><span class="text-foreground"><strong>{event.provider.toUpperCase()}</strong> · {event.action}{#if event.detail}<span class="block text-muted">{event.detail}</span>{/if}</span><time class="shrink-0 text-muted" datetime={event.occurred_at}>{formatCredentialDate(event.occurred_at)}</time></li>
          {/each}
        </ol>
      </details>
    </section>
  </div>
</div>

<LoginModal bind:open={showLoginModal} bind:account />

<!-- Diagnostics Modal -->
<DiagnosticsModal bind:open={showDiagnostics} />

<!-- Modal Export Criptografado -->
<Modal title="Exportar Backup Criptografado" bind:open={showExportModal} on:close={() => (showExportModal = false)}>
  <div class="flex flex-col gap-4 text-xs">
    <p class="text-muted leading-relaxed">
      Defina uma senha com no mínimo 6 caracteres. O arquivo usará criptografia AES-256-GCM com chave derivada via PBKDF2 (100.000 iterações). Guarde a senha com segurança.
    </p>
    <label class="flex flex-col gap-1 text-xs text-muted">
      Senha de criptografia
      <input
        type="password"
        bind:value={exportPassphrase}
        placeholder="Mínimo de 6 caracteres..."
        class="rounded-lg border border-border bg-surface px-3 py-2 text-foreground focus:border-primary focus:outline-none tv-focusable"
      />
    </label>
    <div class="flex justify-end gap-2 pt-2 border-t border-border">
      <Button variant="ghost" size="sm" on:click={() => (showExportModal = false)}>Cancelar</Button>
      <Button variant="primary" size="sm" on:click={async () => { await handleExportEncrypted(); showExportModal = false; exportPassphrase = ''; }} disabled={exportPassphrase.length < 6}>
        Exportar Criptografado
      </Button>
    </div>
  </div>
</Modal>

<!-- Modal Descriptografar Import -->
<Modal title="Descriptografar Backup" bind:open={showImportDecryptModal} on:close={() => (showImportDecryptModal = false)}>
  <div class="flex flex-col gap-4 text-xs">
    <p class="text-muted leading-relaxed">
      Este arquivo está protegido com senha. Informe a chave definida no momento da exportação:
    </p>
    <label class="flex flex-col gap-1 text-xs text-muted">
      Senha do arquivo
      <input
        type="password"
        bind:value={importDecryptPassphrase}
        placeholder="Digite a senha..."
        class="rounded-lg border border-border bg-surface px-3 py-2 text-foreground focus:border-primary focus:outline-none tv-focusable"
      />
    </label>
    <div class="flex justify-end gap-2 pt-2 border-t border-border">
      <Button variant="ghost" size="sm" on:click={() => (showImportDecryptModal = false)}>Cancelar</Button>
      <Button variant="primary" size="sm" on:click={handleDecryptImport} disabled={!importDecryptPassphrase.trim() || isImporting}>
        {isImporting ? 'Descriptografando...' : 'Desbloquear Backup'}
      </Button>
    </div>
  </div>
</Modal>

<!-- Import Backup Modal -->
<Modal title="Restaurar Backup de Dados" bind:open={showImportModal} on:close={() => (showImportModal = false)}>
  <div class="flex flex-col gap-4 text-xs">
    <p class="text-muted leading-relaxed">
      Arquivo de backup selecionado. Escolha a estratégia de importação para mesclar ou substituir os dados atuais:
    </p>

    <div class="flex flex-col gap-2">
      <label class="flex items-start gap-2.5 p-3 rounded-xl border border-border bg-surfaceHover/50 cursor-pointer">
        <input type="radio" bind:group={importStrategy} value="merge" class="mt-0.5 accent-primary" />
        <div>
          <strong class="text-foreground block">Mesclar (Recomendado)</strong>
          <span class="text-muted">Adiciona novos itens mantendo os dados existentes intactos.</span>
        </div>
      </label>

      <label class="flex items-start gap-2.5 p-3 rounded-xl border border-border bg-surfaceHover/50 cursor-pointer">
        <input type="radio" bind:group={importStrategy} value="replace" class="mt-0.5 accent-primary" />
        <div>
          <strong class="text-foreground block">Substituir Tudo</strong>
          <span class="text-muted">Limpa as playlists e histórico atuais antes de importar.</span>
        </div>
      </label>

      <label class="flex items-start gap-2.5 p-3 rounded-xl border border-border bg-surfaceHover/50 cursor-pointer">
        <input type="radio" bind:group={importStrategy} value="duplicate" class="mt-0.5 accent-primary" />
        <div>
          <strong class="text-foreground block">Duplicar Playlists</strong>
          <span class="text-muted">Cria novas playlists com novos identificadores.</span>
        </div>
      </label>
    </div>

    <div class="flex justify-end gap-2 pt-2 border-t border-border">
      <Button variant="ghost" size="sm" on:click={() => (showImportModal = false)}>Cancelar</Button>
      <Button variant="primary" size="sm" on:click={handleConfirmImport} disabled={isImporting}>
        {isImporting ? 'Importando...' : 'Confirmar Restauração'}
      </Button>
    </div>
  </div>
</Modal>

<!-- Confirm Dialog -->
<ConfirmDialog
  open={pendingConfirm !== null}
  trigger={pendingConfirm?.trigger ?? null}
  danger
  title={pendingConfirm?.kind === 'rollback' ? 'Reverter Runtime' : 'Remover Perfil'}
  description={pendingConfirm?.kind === 'rollback'
    ? `Voltar para o runtime ${runtimeStatus?.previous_version || 'anterior'}?`
    : `Remover o perfil ${pendingConfirm?.profile?.name || ''}? A conta e os vínculos locais deste perfil serão apagados.`}
  confirmLabel={pendingConfirm?.kind === 'rollback' ? 'Reverter' : 'Remover'}
  cancelLabel="Cancelar"
  on:confirm={async () => {
    const current = pendingConfirm;
    const profile = current && current.kind === 'profile' ? current.profile : null;
    const kind = current?.kind;
    pendingConfirm = null;
    if (kind === 'rollback') await applyRollback();
    else if (kind === 'profile' && profile) await applyDeleteProfile(profile);
  }}
  on:cancel={() => (pendingConfirm = null)}
/>

<!-- Shortcuts Customization Modal -->
<ShortcutsModal bind:open={showShortcutsModal} />
