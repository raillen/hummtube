<script lang="ts">
  /* svelte-ignore a11y-no-noninteractive-element-interactions */
  import { onMount, onDestroy } from 'svelte';
  import type Hls from 'hls.js';
  import Headphones from 'lucide-svelte/icons/headphones';
  import type { PlaybackPlan } from '../../types';
  import {
    AUTO_QUALITY_ID,
    AUDIO_ONLY_QUALITY_ID,
    buildHLSQualityOptions,
    buildPlanQualityOptions,
    planVariantForOption,
    type PlayerQualityOption,
  } from '../../player/quality';
  import { decibelsToLinearGain, evaluateAudioDSPAvailability, isLocalMediaProxyURL } from '../../player/audioDSP';
  import { playerStore } from '../../stores/playerStore';
  import { savePlaybackProgress, scrobbleTrack } from '$product-player-hooks';
  import { translate } from '../../i18n';
  import MiniPlayer from '../layout/MiniPlayer.svelte';
  import PlayerControls from './PlayerControls.svelte';
  import LoadingOverlay from './LoadingOverlay.svelte';

  let videoElement: HTMLVideoElement;
  let adaptiveAudioElement: HTMLAudioElement;
  let playerContainer: HTMLDivElement;
  let hlsInstance: Hls | null = null;
  let audioContext: AudioContext | null = null;
  let videoMediaSource: MediaElementAudioSourceNode | null = null;
  let adaptiveAudioMediaSource: MediaElementAudioSourceNode | null = null;
  let connectedDSPSource: MediaElementAudioSourceNode | null = null;
  let bypassDSPSource: MediaElementAudioSourceNode | null = null;
  let gainNode: GainNode | null = null;
  let dspSetupPromise: Promise<void> | null = null;
  let dspRuntimeUnavailableReason = '';
  let adaptiveAudioPlayPromise: Promise<boolean> | null = null;
  let adaptiveAudioFailureStreamURL = '';
  let isSwitchingStream = false;
  let progressInterval: ReturnType<typeof setInterval> | null = null;
  let loadedStreamUrl = '';
  let loadedAudioStreamUrl = '';
  let usesSeparateAudio = false;
  let loadedPlaybackPlan: PlaybackPlan | null = null;
  let resumeAppliedToUrl = '';
  let pendingResumeAt: number | null = null;
  let qualityOptions: PlayerQualityOption[] = [];
  let selectedQualityID = AUTO_QUALITY_ID;
  let activeQualityLabel = 'Automática';
  let failedStreamURLs = new Set<string>();
  let hlsRecoveryAttempts = 0;
  let streamLoadGeneration = 0;
  let isDestroyed = false;
  let isNativeFullscreen = false;
  let isInAppFullscreen = false;
  let lastScrobbledVideoID = '';
  let scrobblePendingVideoID = '';
  let isDSPAvailable = false;
  let dspUnavailableReason = 'Inicie uma mídia para configurar o ganho digital.';
  let activeAudioTrackID = '';
  let activeSubtitleURL = '';
  let audioTracks: { id: string; label: string; language?: string }[] = [];
  let subtitles: { language: string; label?: string; url: string; format?: string }[] = [];
  let isControlsIdle = false;
  let controlsIdleTimer: ReturnType<typeof setTimeout> | null = null;
  let lastActiveControlsElement: HTMLElement | null = null;
  $: subtitleLabelByURL = (url: string): string => {
    const found = subtitles.find((sub) => sub.url === url);
    return found?.label || found?.language || url;
  };
  $: isFullscreen = isNativeFullscreen || isInAppFullscreen;
  $: isAudioOnly = selectedQualityID === AUDIO_ONLY_QUALITY_ID;
  $: audibleMediaElement = usesSeparateAudio ? adaptiveAudioElement : videoElement;

  // Carregamento de stream quando playbackPlan muda
  $: if (videoElement && adaptiveAudioElement && $playerStore.playbackPlan && $playerStore.playbackPlan !== loadedPlaybackPlan) {
    loadPlaybackPlan($playerStore.playbackPlan);
  }

  $: if (videoElement) {
    if ($playerStore.isPaused) {
      if (!videoElement.paused) videoElement.pause();
      if (adaptiveAudioElement && !adaptiveAudioElement.paused) adaptiveAudioElement.pause();
    } else if (
      !$playerStore.isPaused &&
      videoElement.paused &&
      videoElement.readyState >= HTMLMediaElement.HAVE_FUTURE_DATA
    ) {
      playVideo();
    }
  }

  $: if (videoElement) {
    if (videoElement.playbackRate !== $playerStore.speed) videoElement.playbackRate = $playerStore.speed;
    applyMediaAudioPreferences();
  }

  $: if (audibleMediaElement && ($playerStore.gain !== 0 || gainNode)) {
    void setupAudioDSP();
  }

  $: if (gainNode) {
    // Converte dB para ganho linear: linear = 10^(dB/20)
    const linearGain = decibelsToLinearGain($playerStore.gain);
    gainNode.gain.value = linearGain;
  }

  function isHLSStream(streamUrl: string): boolean {
    try {
      return new URL(streamUrl).pathname.toLowerCase().endsWith('.m3u8');
    } catch {
      return /\.m3u8(?:$|[?#])/i.test(streamUrl);
    }
  }

  function refreshAudioDSPAvailability(): void {
    const availability = evaluateAudioDSPAvailability(loadedStreamUrl, {
      documentOrigin: window.location.origin,
      hasWebAudio: Boolean(window.AudioContext || (window as Window & typeof globalThis & { webkitAudioContext?: typeof AudioContext }).webkitAudioContext),
      isManagedMediaSource: Boolean(hlsInstance && isHLSStream(loadedStreamUrl)),
    });
    isDSPAvailable = availability.isAvailable && !dspRuntimeUnavailableReason;
    dspUnavailableReason = dspRuntimeUnavailableReason || availability.unavailableReason;
    if (!isDSPAvailable && $playerStore.gain !== 0) playerStore.setGain(0);
  }

  function applyMediaAudioPreferences(): void {
    if (!videoElement) return;
    const videoVolume = usesSeparateAudio ? 0 : ($playerStore.isMuted ? 0 : $playerStore.volume);
    if (videoElement.volume !== videoVolume) videoElement.volume = videoVolume;
    if (!adaptiveAudioElement) return;
    if (adaptiveAudioElement.playbackRate !== $playerStore.speed) adaptiveAudioElement.playbackRate = $playerStore.speed;
    const audioVolume = usesSeparateAudio && !$playerStore.isMuted ? $playerStore.volume : 0;
    if (adaptiveAudioElement.volume !== audioVolume) adaptiveAudioElement.volume = audioVolume;
  }

  async function playVideo() {
    const generation = streamLoadGeneration;
    const isCurrent = () => !isDestroyed && generation === streamLoadGeneration;
    try {
      // A configuração do DSP pode ter sido feita enquanto o contexto estava
      // suspenso pelo autoplay policy. Tentar aqui mantém o áudio audível e
      // usa o gesto de reprodução como última oportunidade de desbloqueio.
      if ($playerStore.gain !== 0 && isDSPAvailable && !gainNode) await setupAudioDSP();
      if (audioContext?.state === 'suspended') {
        await audioContext.resume().catch(() => undefined);
      }
      if (!isCurrent() || $playerStore.isPaused) return;
      await videoElement.play();
      if (!isCurrent()) return;
      if ($playerStore.isPaused) {
        videoElement.pause();
        return;
      }
      if (usesSeparateAudio && adaptiveAudioElement) {
        const audioStarted = await playAdaptiveAudio();
        if (!audioStarted || !isCurrent() || $playerStore.isPaused) return;
      }
    } catch (error: unknown) {
      if (!isCurrent() || $playerStore.isPaused) return;
      if (error instanceof DOMException && error.name === 'AbortError') return;
      const reason = error instanceof Error ? error.message : 'reprodução bloqueada';
      videoElement.pause();
      adaptiveAudioElement?.pause();
      playerStore.pause();
      playerStore.setError(`Não foi possível iniciar o vídeo: ${reason}`);
    }
  }

  function loadPlaybackPlan(plan: PlaybackPlan) {
    loadedPlaybackPlan = plan;
    selectedQualityID = AUTO_QUALITY_ID;
    activeQualityLabel = 'Automática';
    failedStreamURLs = new Set<string>();
    qualityOptions = buildPlanQualityOptions(plan);
    audioTracks = (plan.audio_tracks || []).map((track) => ({
      id: track.id,
      label: track.label,
      language: track.language,
    }));
    if (audioTracks.length === 0 && plan.audio?.url) {
      audioTracks = [{ id: 'default', label: 'Áudio padrão' }];
    }
    activeAudioTrackID = audioTracks[0]?.id || '';
    subtitles = (plan.subtitles || []).map((sub) => ({
      language: sub.language,
      label: sub.language,
      url: sub.url,
      format: sub.format,
    }));
    activeSubtitleURL = '';
    pendingResumeAt = $playerStore.resumeAt;
    lastScrobbledVideoID = '';
    scrobblePendingVideoID = '';
    loadStream(plan.primary?.url || '', true, plan.audio?.url || '');
  }

  function syncActiveQualityFromHls(): void {
    if (!hlsInstance) {
      activeQualityLabel = 'Automática';
      return;
    }
    const level = hlsInstance.levels?.[hlsInstance.currentLevel];
    if (!level) {
      activeQualityLabel = 'Automática';
      return;
    }
    const height = level.height;
    activeQualityLabel = height ? `${height}p` : `${Math.round((level.bitrate || 0) / 1000)} kbps`;
  }

  function pauseAndClearPlayer(): void {
    if (!videoElement) return;
    try {
      videoElement.pause();
      // P0-1: limpa src para cortar imediatamente o vídeo anterior.
      videoElement.removeAttribute('src');
      videoElement.load();
    } catch {}
  }

  // P0-5: handlers de eventos do player para o progresso de buffer.
  function computeBufferingProgress(): number {
    if (!videoElement || !videoElement.duration || !isFinite(videoElement.duration)) return 0;
    const buffered = videoElement.buffered;
    if (!buffered || buffered.length === 0) return 0;
    const end = buffered.end(buffered.length - 1);
    return Math.max(0, Math.min(1, end / videoElement.duration));
  }

  function handleLoadStart(): void {
    playerStore.setLoadingPhase('buffering');
  }

  function handleProgress(): void {
    playerStore.setBufferingProgress(computeBufferingProgress());
  }

  // P1-2: preload dinâmico baseado no tipo de stream
  // HLS (streaming) se beneficia de buffer antecipado, enquanto MP4 (progressivo)
  // economiza banda ao baixar apenas metadados até o usuário tocar.
  function updatePreloadPolicy(streamUrl: string): void {
    if (!videoElement) return;
    const isHLS = streamUrl.toLowerCase().includes('.m3u8');
    videoElement.preload = isHLS ? 'auto' : 'metadata';
  }

  function handleCanPlay(): void {
    isSwitchingStream = false;
    playerStore.markReady();
  }

  function handleStalled(): void {
    playerStore.setLoadingPhase('buffering');
  }

  function handleWaiting(): void {
    playerStore.setLoadingPhase('buffering');
  }

  function handleSuspend(): void {
    // networkState 3 (NETWORK_NO_SOURCE) — operação normal, não mexer
    if (videoElement.networkState === 3) return;
    playerStore.setLoadingPhase('buffering');
  }

  function handleAbortPlayer(): void {
    isSwitchingStream = false;
    pauseAndClearPlayer();
  }

  async function loadStream(streamUrl: string, force = false, audioStreamUrl = '') {
    if (!streamUrl || (!force && streamUrl === loadedStreamUrl && audioStreamUrl === loadedAudioStreamUrl)) return;

    const requestGeneration = ++streamLoadGeneration;
    const resumeAt = pendingResumeAt ?? (Number.isFinite(videoElement.currentTime) ? videoElement.currentTime : 0);
    videoElement.crossOrigin = isLocalMediaProxyURL(streamUrl) ? 'anonymous' : null;
    loadedStreamUrl = streamUrl;
    isSwitchingStream = true;
    updatePreloadPolicy(streamUrl);
    dspRuntimeUnavailableReason = '';
    adaptiveAudioFailureStreamURL = '';
    configureAdaptiveAudio(audioStreamUrl, resumeAt);
    resumeAppliedToUrl = '';
    hlsRecoveryAttempts = 0;
    playerStore.setError(null);
    if (hlsInstance) {
      hlsInstance.destroy();
      hlsInstance = null;
    }
    // A disponibilidade depende da instância HLS atual. Atualize depois de
    // destruir a anterior para não marcar o fallback HLS nativo como MSE.
    refreshAudioDSPAvailability();

    if (isHLSStream(streamUrl)) {
      let HlsPlayer: typeof import('hls.js').default;
      try {
        // HLS é usado apenas quando um stream HLS realmente entra no player;
        // manter o decoder fora do bundle inicial reduz o cold start da UI.
        HlsPlayer = (await import('hls.js')).default;
      } catch {
        if (isDestroyed || requestGeneration !== streamLoadGeneration) return;
        if (videoElement.canPlayType('application/vnd.apple.mpegurl')) {
          videoElement.src = streamUrl;
          videoElement.load();
        } else {
          isSwitchingStream = false;
          playerStore.pause();
          playerStore.setError('Não foi possível carregar o suporte a HLS.');
        }
        return;
      }
      if (isDestroyed || requestGeneration !== streamLoadGeneration || loadedStreamUrl !== streamUrl) return;

      if (!HlsPlayer.isSupported()) {
        videoElement.src = streamUrl;
        videoElement.load();
        return;
      }

      hlsInstance = new HlsPlayer({ 
        enableWorker: true,
        maxBufferLength: 30,
        maxBufferSize: 60 * 1000 * 1000,
      });
      refreshAudioDSPAvailability();
      hlsInstance.loadSource(streamUrl);
      hlsInstance.attachMedia(videoElement);
      hlsInstance.on(HlsPlayer.Events.MANIFEST_PARSED, () => {
        if (isDestroyed || requestGeneration !== streamLoadGeneration) return;
        hlsRecoveryAttempts = 0;
        const isPrimaryStream = loadedPlaybackPlan?.primary.url === streamUrl;
        const hlsQualities = buildHLSQualityOptions(
          hlsInstance?.levels || [],
          Boolean(loadedPlaybackPlan?.audio_only?.url),
        );
        if (isPrimaryStream && selectedQualityID !== AUDIO_ONLY_QUALITY_ID && hlsQualities.length > 0) {
          qualityOptions = hlsQualities;
          selectedQualityID = AUTO_QUALITY_ID;
        }
        syncActiveQualityFromHls();
        if (!$playerStore.isPaused) void playVideo();
      });
      hlsInstance.on(HlsPlayer.Events.LEVEL_SWITCHED, () => {
        syncActiveQualityFromHls();
      });
      hlsInstance.on(HlsPlayer.Events.ERROR, (_event, details) => {
        if (isDestroyed || requestGeneration !== streamLoadGeneration || !details.fatal) return;
        if (selectedQualityID !== AUTO_QUALITY_ID) {
          fallbackToAutomaticQuality(`A qualidade selecionada falhou (${details.details}).`);
          return;
        }
        if (hlsRecoveryAttempts >= 1 || !hlsInstance) {
          playerStore.setError(`O stream HLS falhou (${details.details}).`);
          return;
        }
        hlsRecoveryAttempts += 1;
        if (details.type === HlsPlayer.ErrorTypes.NETWORK_ERROR) {
          hlsInstance.startLoad();
        } else if (details.type === HlsPlayer.ErrorTypes.MEDIA_ERROR) {
          hlsInstance.recoverMediaError();
        } else {
          playerStore.setError(`O stream HLS falhou (${details.details}).`);
        }
      });
    } else {
      videoElement.src = streamUrl;
      videoElement.load();
    }
  }

  function handleQualityChange(optionID: string) {
    if (!loadedPlaybackPlan || optionID === selectedQualityID) return;

    if (optionID === AUTO_QUALITY_ID) {
      selectedQualityID = AUTO_QUALITY_ID;
      activeQualityLabel = 'Automática';
      playerStore.setError(null);
      if (loadedStreamUrl === loadedPlaybackPlan.primary.url && hlsInstance) {
        hlsInstance.loadLevel = -1;
        return;
      }
      switchPlanStream(loadedPlaybackPlan.primary.url, false, loadedPlaybackPlan.audio?.url || '');
      return;
    }

    if (optionID === AUDIO_ONLY_QUALITY_ID) {
      const audioOnlyStream = loadedPlaybackPlan.audio_only;
      if (!audioOnlyStream?.url) {
        fallbackToAutomaticQuality('O formato somente áudio não está mais disponível.');
        return;
      }
      selectedQualityID = AUDIO_ONLY_QUALITY_ID;
      activeQualityLabel = 'Somente áudio';
      playerStore.setError(null);
      switchPlanStream(audioOnlyStream.url, false, '');
      return;
    }

    if (optionID.startsWith('hls:') && hlsInstance) {
      const level = Number.parseInt(optionID.slice('hls:'.length), 10);
      if (Number.isInteger(level) && level >= 0 && level < hlsInstance.levels.length) {
        hlsInstance.loadLevel = level;
        selectedQualityID = optionID;
        activeQualityLabel = hlsInstance.levels[level]?.height ? `${hlsInstance.levels[level]?.height}p` : 'HLS';
        playerStore.setError(null);
      } else {
        fallbackToAutomaticQuality('A qualidade solicitada não está mais disponível.');
      }
      return;
    }

    const variant = planVariantForOption(loadedPlaybackPlan, optionID);
    if (!variant) {
      fallbackToAutomaticQuality('A qualidade solicitada não está mais disponível.');
      return;
    }
    selectedQualityID = optionID;
    activeQualityLabel = variant.label || optionID;
    playerStore.setError(null);
    const adaptiveAudioURL = variant.has_audio === false
      ? loadedPlaybackPlan.audio?.url || loadedPlaybackPlan.audio_only?.url || ''
      : '';
    if (variant.has_audio === false && !adaptiveAudioURL) {
      fallbackToAutomaticQuality('A faixa de áudio desta resolução não está disponível.');
      return;
    }
    switchPlanStream(variant.stream.url, false, adaptiveAudioURL);
  }

  function switchPlanStream(streamUrl: string, force = false, audioStreamUrl = '') {
    if (!force && streamUrl === loadedStreamUrl && audioStreamUrl === loadedAudioStreamUrl) return;
    pendingResumeAt ??= Number.isFinite(videoElement.currentTime) ? videoElement.currentTime : 0;
    loadStream(streamUrl, force, audioStreamUrl);
  }

  function fallbackToAutomaticQuality(message: string) {
    if (!loadedPlaybackPlan) {
      playerStore.setError(message);
      return;
    }

    if (usesSeparateAudio && switchToMuxedFallback(message)) return;

    if (selectedQualityID === AUTO_QUALITY_ID) {
      playerStore.setError(message);
      return;
    }

    if (failedStreamURLs.has(loadedPlaybackPlan.primary.url)) {
      isSwitchingStream = false;
      playerStore.pause();
      playerStore.setError(`${message} Nenhum formato compatível restante pôde ser carregado.`);
      return;
    }

    selectedQualityID = AUTO_QUALITY_ID;
    if (hlsInstance && loadedStreamUrl === loadedPlaybackPlan.primary.url) {
      hlsInstance.loadLevel = -1;
      hlsInstance.startLoad();
      playerStore.setError(`${message} O modo automático foi restaurado.`);
      return;
    }

    switchPlanStream(loadedPlaybackPlan.primary.url, true, loadedPlaybackPlan.audio?.url || '');
    playerStore.setError(`${message} Voltando para a qualidade automática.`);
  }

  function switchToMuxedFallback(message: string): boolean {
    const muxedFallback = loadedPlaybackPlan?.variants?.find((variant) => (
      variant.has_audio !== false &&
      Boolean(variant.stream.url) &&
      variant.stream.url !== loadedStreamUrl &&
      !failedStreamURLs.has(variant.stream.url)
    ));
    if (!muxedFallback) return false;
    selectedQualityID = `plan:${muxedFallback.id}`;
    switchPlanStream(muxedFallback.stream.url, true, '');
    playerStore.setError(`${message} O player voltou para ${muxedFallback.label}.`);
    return true;
  }

  function configureAdaptiveAudio(streamUrl: string, resumeAt: number): void {
    if (!adaptiveAudioElement) return;
    const normalizedURL = streamUrl.trim();
    adaptiveAudioElement.pause();
    adaptiveAudioPlayPromise = null;
    usesSeparateAudio = normalizedURL !== '';
    applyMediaAudioPreferences();

    if (!usesSeparateAudio) {
      loadedAudioStreamUrl = '';
      adaptiveAudioElement.removeAttribute('src');
      adaptiveAudioElement.load();
      return;
    }

    if (normalizedURL !== loadedAudioStreamUrl) {
      loadedAudioStreamUrl = normalizedURL;
      adaptiveAudioElement.crossOrigin = isLocalMediaProxyURL(normalizedURL) ? 'anonymous' : null;
      adaptiveAudioElement.src = normalizedURL;
      adaptiveAudioElement.load();
    }
    if (resumeAt > 0 && adaptiveAudioElement.readyState >= HTMLMediaElement.HAVE_METADATA) {
      adaptiveAudioElement.currentTime = resumeAt;
    }
  }

  function synchronizeAdaptiveAudio(force = false): void {
    if (!usesSeparateAudio || !adaptiveAudioElement || adaptiveAudioElement.readyState < HTMLMediaElement.HAVE_METADATA) return;
    const drift = Math.abs(adaptiveAudioElement.currentTime - videoElement.currentTime);
    if (drift > (force ? 0.01 : 0.3)) adaptiveAudioElement.currentTime = videoElement.currentTime;
  }

  function handleAdaptiveAudioLoaded(): void {
    synchronizeAdaptiveAudio(true);
    if (!$playerStore.isPaused && !videoElement.paused) void playAdaptiveAudio();
  }

  function handleAdaptiveAudioError(): void {
    if (!usesSeparateAudio || !loadedPlaybackPlan) return;
    handleAdaptiveAudioFailure('A faixa de áudio adaptativa falhou.');
  }

  function changeAudioTrack(trackID: string): void {
    if (!loadedPlaybackPlan?.audio_tracks || trackID === activeAudioTrackID) return;
    const track = loadedPlaybackPlan.audio_tracks.find((entry) => entry.id === trackID);
    if (!track) return;
    activeAudioTrackID = trackID;
    loadedAudioStreamUrl = '';
    adaptiveAudioFailureStreamURL = '';
    if (usesSeparateAudio || trackID !== '') {
      configureAdaptiveAudio(track.stream.url, 0);
      if (!$playerStore.isPaused) void playAdaptiveAudio();
    }
  }

  function changeSubtitle(subtitleURL: string): void {
    activeSubtitleURL = subtitleURL;
    if (!videoElement) return;
    const trackElements = Array.from(videoElement.querySelectorAll('track'));
    for (const trackEl of trackElements) {
      const isMatch = trackEl.src === subtitleURL && subtitleURL !== '';
      if (trackEl.track) {
        trackEl.track.mode = isMatch ? 'showing' : 'hidden';
      }
    }
    if (subtitleURL !== '') {
      ensureSubtitleTrack(subtitleURL);
    }
  }

  function ensureSubtitleTrack(url: string): void {
    if (!videoElement) return;
    const existing = Array.from(videoElement.querySelectorAll('track')).find((track: HTMLTrackElement) => track.src === url);
    if (existing) return;
    const playbackPlan = loadedPlaybackPlan;
    const subtitle = playbackPlan?.subtitles?.find((entry) => entry.url === url);
    if (!subtitle) return;
    const track = document.createElement('track');
    track.kind = 'subtitles';
    track.src = url;
    track.srclang = subtitle.language;
    track.label = subtitle.label || subtitle.language;
    track.default = true;
    videoElement.appendChild(track);
  }

  function playAdaptiveAudio(): Promise<boolean> {
    if (!usesSeparateAudio || !adaptiveAudioElement) return Promise.resolve(true);
    if (adaptiveAudioPlayPromise) return adaptiveAudioPlayPromise;

    const requestedGeneration = streamLoadGeneration;
    const isCurrent = () => !isDestroyed && requestedGeneration === streamLoadGeneration;
    adaptiveAudioPlayPromise = (async () => {
      try {
        synchronizeAdaptiveAudio(true);
        await adaptiveAudioElement.play();
        if (!isCurrent()) return false;
        if ($playerStore.isPaused) {
          adaptiveAudioElement.pause();
          return false;
        }
        return true;
      } catch (error: unknown) {
        if (!isCurrent() || $playerStore.isPaused) return false;
        if (error instanceof DOMException && error.name === 'AbortError') return false;
        const reason = error instanceof Error ? error.message : 'reprodução bloqueada';
        handleAdaptiveAudioFailure(`A faixa de áudio adaptativa não pôde ser iniciada (${reason}).`);
        return false;
      } finally {
        if (isCurrent()) adaptiveAudioPlayPromise = null;
      }
    })();
    return adaptiveAudioPlayPromise;
  }

  function handleAdaptiveAudioFailure(message: string): void {
    if (!usesSeparateAudio || adaptiveAudioFailureStreamURL === loadedAudioStreamUrl) return;
    adaptiveAudioFailureStreamURL = loadedAudioStreamUrl;
    adaptiveAudioElement.pause();
    if (!switchToMuxedFallback(message)) {
      isSwitchingStream = false;
      playerStore.pause();
      playerStore.setError(`${message} Não existe uma resolução combinada de fallback.`);
    }
  }

  function handleVideoSeeking(): void {
    synchronizeAdaptiveAudio(true);
  }

  function handleVideoWaiting(): void {
    if (usesSeparateAudio) adaptiveAudioElement.pause();
  }

  function handleVideoPlaying(): void {
    isSwitchingStream = false;
    if (!usesSeparateAudio || $playerStore.isPaused) return;
    synchronizeAdaptiveAudio(true);
    void playAdaptiveAudio();
  }

  async function setupAudioDSP(): Promise<void> {
    const mediaElement = usesSeparateAudio ? adaptiveAudioElement : videoElement;
    if (!mediaElement) return;
    if (dspSetupPromise) return dspSetupPromise;
    dspSetupPromise = (async () => {
      let nextSource: MediaElementAudioSourceNode | null = null;
      try {
        const audioWindow = window as Window & typeof globalThis & { webkitAudioContext?: typeof AudioContext };
        const AudioCtx = window.AudioContext || audioWindow.webkitAudioContext;
        if (!AudioCtx) {
          dspRuntimeUnavailableReason = 'O WebView deste sistema não oferece suporte ao processamento Web Audio.';
          refreshAudioDSPAvailability();
          playerStore.setGain(0);
          return;
        }
        if (!audioContext) audioContext = new AudioCtx();
        // Não roteie o elemento para um contexto que ainda não pode tocar:
        // MediaElementSource silencia a saída nativa enquanto o contexto está
        // suspenso. Se o resume falhar, o vídeo continua usando áudio normal.
        if (audioContext.state === 'suspended') {
          await audioContext.resume();
        }
        if (audioContext.state !== 'running') {
          dspRuntimeUnavailableReason = 'O WebView bloqueou o processamento de áudio. O áudio nativo foi preservado.';
          refreshAudioDSPAvailability();
          playerStore.setGain(0);
          return;
        }
        nextSource = usesSeparateAudio
          ? (adaptiveAudioMediaSource ||= audioContext.createMediaElementSource(adaptiveAudioElement))
          : (videoMediaSource ||= audioContext.createMediaElementSource(videoElement));
        if (!gainNode) {
          gainNode = audioContext.createGain();
          gainNode.connect(audioContext.destination);
        }
        if (connectedDSPSource !== nextSource) {
          connectedDSPSource?.disconnect();
          if (bypassDSPSource === nextSource) {
            nextSource.disconnect();
            bypassDSPSource = null;
          }
          nextSource.connect(gainNode);
          connectedDSPSource = nextSource;
        }
        gainNode.gain.setValueAtTime(decibelsToLinearGain($playerStore.gain), audioContext.currentTime);
      } catch (error) {
        console.warn('DSP WebAudio não suportado ou bloqueado:', error);
        // Depois que createMediaElementSource é chamado, o navegador deixa de
        // enviar esse elemento diretamente aos alto-falantes. Em qualquer
        // falha posterior, reconecte a fonte ao destino sem ganho para nunca
        // transformar um ajuste de DSP em vídeo mudo.
        const fallbackAudioContext = audioContext;
        if (nextSource && fallbackAudioContext && fallbackAudioContext.state !== 'closed') {
          try {
            nextSource.disconnect();
            nextSource.connect(fallbackAudioContext.destination);
            bypassDSPSource = nextSource;
            connectedDSPSource = null;
          } catch (fallbackError) {
            console.warn('Não foi possível restaurar a rota de áudio sem DSP:', fallbackError);
          }
        }
        gainNode?.disconnect();
        gainNode = null;
        dspRuntimeUnavailableReason = 'O ganho digital falhou neste WebView. O player manteve a rota de áudio sem ganho.';
        refreshAudioDSPAvailability();
        playerStore.setGain(0);
        if (audioContext?.state === 'closed') audioContext = null;
      } finally {
        dspSetupPromise = null;
      }
    })();
    return dspSetupPromise;
  }

  function handleTimeUpdate() {
    if (!videoElement) return;
    synchronizeAdaptiveAudio();
    playerStore.setTime(videoElement.currentTime);
    playerStore.setDuration(videoElement.duration || 0);
    if (isScrobbleThresholdReached(videoElement.currentTime, videoElement.duration)) {
      scrobbleCurrentTrack();
    }
  }

  function isScrobbleThresholdReached(playedSeconds: number, durationSeconds: number): boolean {
    if (!Number.isFinite(playedSeconds) || !Number.isFinite(durationSeconds) || durationSeconds < 30) return false;
    return playedSeconds >= Math.min(durationSeconds / 2, 4 * 60);
  }

  function handleLoadedMetadata() {
    if (!videoElement || !loadedStreamUrl) return;
    const requestedResumeAt = pendingResumeAt ?? $playerStore.resumeAt;
    if (resumeAppliedToUrl !== loadedStreamUrl && requestedResumeAt > 0 && Number.isFinite(videoElement.duration)) {
      videoElement.currentTime = Math.min(requestedResumeAt, Math.max(0, videoElement.duration - 1));
      synchronizeAdaptiveAudio(true);
      resumeAppliedToUrl = loadedStreamUrl;
    }
    pendingResumeAt = null;
    isSwitchingStream = false;
    if (!$playerStore.isPaused) playVideo();
  }

  function handleVideoPause(): void {
    if (!isSwitchingStream) playerStore.pause();
  }

  function handleMediaError() {
    const mediaError = videoElement?.error;
    const nativeMessage = mediaError?.message || '';
    const code = mediaError?.code || mediaErrorCodeFromMessage(nativeMessage);
    if (code === 1 && isSwitchingStream) return;

    if (loadedStreamUrl) failedStreamURLs.add(loadedStreamUrl);
    const reason = mediaErrorDescription(code, nativeMessage);
    const message = `O player não conseguiu carregar esta qualidade (${reason}).`;
    if (switchToMuxedFallback(message)) return;
    fallbackToAutomaticQuality(message);
  }

  function retryPlayback(): void {
    if (!$playerStore.currentVideo || isSwitchingStream) return;
    const failedUrl = loadedStreamUrl;
    if (failedUrl) failedStreamURLs.delete(failedUrl);
    playerStore.setError(null);
    isSwitchingStream = true;
    if (hlsInstance && failedUrl === loadedPlaybackPlan?.primary.url) {
      hlsRecoveryAttempts = 0;
      hlsInstance.startLoad();
      return;
    }
    loadStream(failedUrl || loadedPlaybackPlan?.primary.url || '', true, loadedAudioStreamUrl);
  }

  function mediaErrorCodeFromMessage(message: string): number {
    const match = message.match(/(?:c[oó]digo|code)\s*([1-4])/i);
    return match ? Number.parseInt(match[1], 10) : 0;
  }

  function mediaErrorDescription(code: number, nativeMessage: string): string {
    switch (code) {
      case 1:
        return translate('player.error.codes.1');
      case 2:
        return translate('player.error.codes.2');
      case 3:
        return translate('player.error.codes.3');
      case 4:
        return translate('player.error.codes.4');
      default:
        return nativeMessage.trim() || 'erro de mídia desconhecido';
    }
  }

  function saveCurrentProgress(completed: boolean) {
    const video = $playerStore.currentVideo;
    if (!videoElement || !video || $playerStore.sourceKind === 'direct') return;
    const positionMs = completed ? 0 : Math.floor(videoElement.currentTime * 1000);
    const durationMs = completed ? 0 : Math.floor((videoElement.duration || 0) * 1000);
    savePlaybackProgress(video.id, $playerStore.sourceKind, positionMs, durationMs, completed)
      .catch(() => playerStore.setError('Não foi possível salvar o progresso de reprodução.'));
  }

  function handleEnded() {
    scrobbleCurrentTrack();
    saveCurrentProgress(true);
    if ($playerStore.queue.length > 0 && $playerStore.autoplay) {
      playerStore.playNext();
    } else {
      playerStore.pause();
    }
  }

  function scrobbleCurrentTrack(): void {
    const video = $playerStore.currentVideo;
    if (!video || lastScrobbledVideoID === video.id || scrobblePendingVideoID === video.id) return;
    const playedMs = Math.floor((videoElement?.currentTime || 0) * 1000);
    const durationMs = Math.floor((videoElement?.duration || 0) * 1000);
    scrobblePendingVideoID = video.id;
    void scrobbleTrack(video, $playerStore.sourceKind, playedMs, durationMs)
      .then(() => { lastScrobbledVideoID = video.id; })
      .catch(() => undefined)
      .finally(() => { if (scrobblePendingVideoID === video.id) scrobblePendingVideoID = ''; });
  }

  async function togglePlayerFullscreen() {
    if (isInAppFullscreen) {
      isInAppFullscreen = false;
      return;
    }
    if (document.fullscreenElement === playerContainer) {
      await document.exitFullscreen().catch(() => undefined);
      return;
    }
    if (!document.fullscreenEnabled || typeof playerContainer.requestFullscreen !== 'function') {
      isInAppFullscreen = true;
      return;
    }
    try {
      await playerContainer.requestFullscreen({ navigationUI: 'hide' });
      isNativeFullscreen = document.fullscreenElement === playerContainer;
      if (!isNativeFullscreen) isInAppFullscreen = true;
    } catch {
      isInAppFullscreen = true;
    }
  }

  function syncNativeFullscreenState() {
    isNativeFullscreen = document.fullscreenElement === playerContainer;
    if (isNativeFullscreen) isInAppFullscreen = false;
  }

  function enableInAppFullscreenFallback() {
    if (!document.fullscreenElement) isInAppFullscreen = true;
  }

  function exitInAppFullscreenOnEscape(event: KeyboardEvent) {
    if (event.key === 'Escape' && isInAppFullscreen) isInAppFullscreen = false;
  }

  function handleFullscreenCommand() {
    void togglePlayerFullscreen();
  }

  const IDLE_HIDE_MS = 3000;

  function armControlsIdleTimer(): void {
    if (controlsIdleTimer) clearTimeout(controlsIdleTimer);
    controlsIdleTimer = setTimeout(() => {
      isControlsIdle = true;
    }, IDLE_HIDE_MS);
  }

  function cancelControlsIdleTimer(): void {
    if (controlsIdleTimer) {
      clearTimeout(controlsIdleTimer);
      controlsIdleTimer = null;
    }
    isControlsIdle = false;
  }

  function rememberActiveControls(target: EventTarget | null): void {
    if (target instanceof HTMLElement && playerContainer?.contains(target)) {
      lastActiveControlsElement = target;
    }
  }

  function handleControlsActivity(): void {
    cancelControlsIdleTimer();
    armControlsIdleTimer();
  }

  function handleControlsPointerLeave(): void {
    armControlsIdleTimer();
  }

  function handleControlsPointerEnter(): void {
    cancelControlsIdleTimer();
  }

  function handleContainerKey(event: KeyboardEvent): void {
    rememberActiveControls(event.target);
    if (isControlsIdle) {
      cancelControlsIdleTimer();
      armControlsIdleTimer();
    }
  }

  function handleContainerClick(event: MouseEvent): void {
    rememberActiveControls(event.target);
    armControlsIdleTimer();
  }

  function restoreIdleFocus(): void {
    if (!isControlsIdle) return;
    const target = lastActiveControlsElement;
    if (!target || !playerContainer?.contains(target)) {
      lastActiveControlsElement = playerContainer?.querySelector<HTMLElement>('[data-component="player-control-button"]') ?? null;
    }
    if (lastActiveControlsElement && typeof lastActiveControlsElement.focus === 'function') {
      lastActiveControlsElement.focus({ preventScroll: true });
    }
  }

  function syncMediaSession(): void {
    if (typeof navigator === 'undefined' || !('mediaSession' in navigator)) return;
    const video = $playerStore.currentVideo;
    if (!video) {
      navigator.mediaSession.metadata = null;
      return;
    }
    try {
      navigator.mediaSession.metadata = new MediaMetadata({
        title: video.title,
        artist: video.channel_title || 'Canal',
        album: 'HummTube',
        artwork: video.thumbnail_url ? [{ src: video.thumbnail_url, sizes: '512x512', type: 'image/jpeg' }] : [],
      });
      navigator.mediaSession.playbackState = $playerStore.isPaused ? 'paused' : 'playing';
      navigator.mediaSession.setActionHandler('play', () => { playerStore.play(); });
      navigator.mediaSession.setActionHandler('pause', () => { playerStore.pause(); });
      navigator.mediaSession.setActionHandler('seekbackward', (details) => {
        if (videoElement) videoElement.currentTime = Math.max(0, videoElement.currentTime - (details.seekOffset || 10));
      });
      navigator.mediaSession.setActionHandler('seekforward', (details) => {
        if (videoElement) videoElement.currentTime = Math.min(videoElement.duration || 0, videoElement.currentTime + (details.seekOffset || 10));
      });
      navigator.mediaSession.setActionHandler('previoustrack', () => { playerStore.playPrevious(); });
      navigator.mediaSession.setActionHandler('nexttrack', () => { playerStore.playNext(); });
    } catch {
      // Navegadores que não suportam todos os handlers não quebram
    }
  }

  $: if ($playerStore.currentVideo || $playerStore.isPaused !== undefined) {
    syncMediaSession();
  }

  onMount(() => {
    document.addEventListener('fullscreenchange', syncNativeFullscreenState);
    document.addEventListener('fullscreenerror', enableInAppFullscreenFallback);
    document.addEventListener('keydown', exitInAppFullscreenOnEscape);
    window.addEventListener('nanotube-toggle-player-fullscreen', handleFullscreenCommand);
    window.addEventListener('nanotube-player-abort', handleAbortPlayer);
    progressInterval = setInterval(() => {
      if (videoElement && !videoElement.paused && $playerStore.currentVideo) {
        saveCurrentProgress(false);
      }
    }, 5000);
  });

  onDestroy(() => {
    isDestroyed = true;
    document.removeEventListener('fullscreenchange', syncNativeFullscreenState);
    document.removeEventListener('fullscreenerror', enableInAppFullscreenFallback);
    document.removeEventListener('keydown', exitInAppFullscreenOnEscape);
    window.removeEventListener('nanotube-toggle-player-fullscreen', handleFullscreenCommand);
    window.removeEventListener('nanotube-player-abort', handleAbortPlayer);
    if (hlsInstance) hlsInstance.destroy();
    if (progressInterval) clearInterval(progressInterval);
    if (controlsIdleTimer) clearTimeout(controlsIdleTimer);
    adaptiveAudioElement?.pause();
    connectedDSPSource?.disconnect();
    bypassDSPSource?.disconnect();
    gainNode?.disconnect();
    saveCurrentProgress(false);
    if (audioContext) audioContext.close();
    if (typeof navigator !== 'undefined' && 'mediaSession' in navigator) {
      try {
        navigator.mediaSession.metadata = null;
        navigator.mediaSession.playbackState = 'none';
      } catch {}
    }
  });
</script>

<div
  bind:this={playerContainer}
  class={isInAppFullscreen
    ? 'fixed inset-0 z-[100] flex h-screen w-screen items-center justify-center overflow-hidden bg-black group select-none'
    : 'relative flex h-full w-full items-center justify-center overflow-hidden bg-black group select-none'}
  data-fullscreen-mode={isNativeFullscreen ? 'native' : isInAppFullscreen ? 'in-app' : 'off'}
  data-audio-mode={usesSeparateAudio ? 'separate' : 'embedded'}
  data-controls-idle={isControlsIdle ? 'true' : 'false'}
  data-testid="video-player-container"
  role="region"
  aria-label="Contêiner do player"
  on:mousemove={handleControlsActivity}
  on:mouseenter={handleControlsPointerEnter}
  on:mouseleave={handleControlsPointerLeave}
  on:click={handleContainerClick}
  on:keydown={handleContainerKey}
>
  <!-- Video Element -->
  <video
    bind:this={videoElement}
    on:loadstart={handleLoadStart}
    on:progress={handleProgress}
    on:canplay={handleCanPlay}
    on:stalled={handleStalled}
    on:waiting={handleWaiting}
    on:suspend={handleSuspend}
    on:timeupdate={handleTimeUpdate}
    on:loadedmetadata={handleLoadedMetadata}
    on:error={handleMediaError}
    on:ended={handleEnded}
    on:seeking={handleVideoSeeking}
    on:playing={handleVideoPlaying}
    on:pause={handleVideoPause}
    on:click={playerStore.togglePlay}
    class="w-full h-full object-contain cursor-pointer"
    preload="metadata"
    playsinline
    poster={$playerStore.posterUrl}
  >
    <track kind="captions" />
  </video>

  <audio
    bind:this={adaptiveAudioElement}
    on:loadedmetadata={handleAdaptiveAudioLoaded}
    on:error={handleAdaptiveAudioError}
    preload="metadata"
    aria-hidden="true"
    data-testid="adaptive-audio-track"
  />

  {#if isAudioOnly}
    <div class="pointer-events-none absolute inset-0 flex flex-col items-center justify-center gap-3 bg-gradient-to-b from-black via-slate-950 to-black text-white" aria-live="polite">
      <div class="rounded-full bg-white/10 p-5 text-primary shadow-xl">
        <Headphones size={38} />
      </div>
      <strong class="text-sm">Somente áudio</strong>
      <span class="max-w-sm truncate px-4 text-xs text-white/60">{$playerStore.currentVideo?.title || ''}</span>
    </div>
  {/if}

  {#if ($playerStore.isLoading || isSwitchingStream) && $playerStore.loadingPhase !== 'ready' && !$playerStore.isMiniplayer}
    <LoadingOverlay isVisible />
  {/if}

  {#if $playerStore.isMiniplayer}
    <MiniPlayer />
  {:else}
    <PlayerControls
      {videoElement}
      {qualityOptions}
      {selectedQualityID}
      {activeQualityLabel}
      {audioTracks}
      {activeAudioTrackID}
      onAudioTrackChange={changeAudioTrack}
      {subtitles}
      {activeSubtitleURL}
      onSubtitleChange={changeSubtitle}
      onQualityChange={handleQualityChange}
      {isFullscreen}
      onToggleFullscreen={togglePlayerFullscreen}
      {isDSPAvailable}
      {dspUnavailableReason}
    />
  {/if}

  {#if $playerStore.error}
    <div class="absolute inset-x-4 top-16 z-50 flex items-center justify-between gap-3 rounded-xl border border-red-500/40 bg-black/85 p-3 text-sm text-red-200 backdrop-blur-md" role="alert">
      <span class="min-w-0 flex-1">{$playerStore.error}</span>
      <button
        type="button"
        on:click={retryPlayback}
        class="shrink-0 rounded-lg bg-red-600/80 px-3 py-1.5 text-xs font-semibold text-white hover:bg-red-600 focus:outline-none focus:ring-2 focus:ring-white/70 tv-focusable"
        data-testid="player-retry-button"
      >
        Tentar novamente
      </button>
    </div>
  {/if}
</div>

<style>
  [data-controls-idle='true'] :global([data-testid='player-controls-layer']) {
    opacity: 0;
    pointer-events: none;
    transition: opacity 0.3s ease;
  }
</style>
