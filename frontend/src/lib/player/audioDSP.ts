export interface AudioDSPAvailability {
  isAvailable: boolean;
  unavailableReason: string;
}

interface AudioDSPEnvironment {
  documentOrigin: string;
  hasWebAudio: boolean;
  isManagedMediaSource: boolean;
}

const REMOTE_STREAM_REASON =
  'O ganho digital foi desativado para este stream remoto para preservar o áudio nativo quando o servidor não permite processamento Web Audio.';

/**
 * MediaElementAudioSource pode produzir silêncio, sem lançar erro, quando a
 * mídia remota não autoriza CORS. A preview só ativa o DSP quando a rota é
 * verificavelmente segura ou quando o HLS é controlado pelo MediaSource.
 */
export function evaluateAudioDSPAvailability(
  streamURL: string,
  environment: AudioDSPEnvironment,
): AudioDSPAvailability {
  if (!environment.hasWebAudio) {
    return {
      isAvailable: false,
      unavailableReason: 'O WebView deste sistema não oferece suporte ao processamento Web Audio.',
    };
  }

  if (!streamURL) {
    return {
      isAvailable: false,
      unavailableReason: 'Inicie uma mídia para configurar o ganho digital.',
    };
  }

  if (environment.isManagedMediaSource) {
    return { isAvailable: true, unavailableReason: '' };
  }

  try {
    const parsedStreamURL = new URL(streamURL, environment.documentOrigin);
    if (
      parsedStreamURL.origin === environment.documentOrigin ||
      parsedStreamURL.protocol === 'blob:' ||
      parsedStreamURL.protocol === 'data:' ||
      isLocalMediaProxyURL(parsedStreamURL.toString())
    ) {
      return { isAvailable: true, unavailableReason: '' };
    }
  } catch {
    return {
      isAvailable: false,
      unavailableReason: 'O endereço desta mídia não permite configurar o ganho digital com segurança.',
    };
  }

  return { isAvailable: false, unavailableReason: REMOTE_STREAM_REASON };
}

export function isLocalMediaProxyURL(streamURL: string): boolean {
  try {
    const parsedStreamURL = new URL(streamURL);
    return (
      parsedStreamURL.protocol === 'http:' &&
      (parsedStreamURL.hostname === '127.0.0.1' || parsedStreamURL.hostname === '[::1]') &&
      parsedStreamURL.pathname.startsWith('/api/media/')
    );
  } catch {
    return false;
  }
}

export function decibelsToLinearGain(decibels: number): number {
  const safeDecibels = Number.isFinite(decibels) ? Math.max(-12, Math.min(12, decibels)) : 0;
  return Math.pow(10, safeDecibels / 20);
}
