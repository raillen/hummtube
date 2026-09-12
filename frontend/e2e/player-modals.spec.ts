import { expect, test, type Page } from '@playwright/test';

const firstVideo = {
  id: 'player-video-1',
  channel_id: 'player-channel',
  channel_title: 'Canal do Player',
  title: 'Vídeo principal do player',
  published_at: '2026-08-28T12:00:00Z',
  duration: 60_000_000_000,
  thumbnail_url: '',
  external_url: 'https://youtube.test/watch?v=player-video-1'
};

const queuedVideo = {
  ...firstVideo,
  id: 'player-video-2',
  title: 'Próximo vídeo da fila',
  external_url: 'https://youtube.test/watch?v=player-video-2'
};

async function openPlayer(
  page: Page,
  disableNativeFullscreen = false,
  rejectAdaptiveAudioPlayback = false,
  keepPlaybackResolutionPending = false,
  useLoopbackMediaProxy = false,
  useHLS = false,
) {
  let queueItems: Array<Record<string, unknown>> = [];
  await page.addInitScript(({ shouldDisableNativeFullscreen, shouldRejectAdaptiveAudioPlayback }) => {
    localStorage.removeItem('nanotube_player_queue');
    (window as Window & { __playerPlayCalls?: number }).__playerPlayCalls = 0;
    HTMLMediaElement.prototype.play = function () {
      if (shouldRejectAdaptiveAudioPlayback && this instanceof HTMLAudioElement) {
        return Promise.reject(new DOMException('formato de áudio recusado', 'NotSupportedError'));
      }
      const testWindow = window as Window & { __playerPlayCalls?: number };
      testWindow.__playerPlayCalls = (testWindow.__playerPlayCalls || 0) + 1;
      return Promise.resolve();
    };
    if (shouldDisableNativeFullscreen) {
      Object.defineProperty(Element.prototype, 'requestFullscreen', {
        configurable: true,
        value: undefined
      });
    }
  }, { shouldDisableNativeFullscreen: disableNativeFullscreen, shouldRejectAdaptiveAudioPlayback: rejectAdaptiveAudioPlayback });

  await page.route('**/api/rpc', async (route) => {
    const request = route.request().postDataJSON() as { method?: string; args?: unknown[] };
    const method = request.method || '';
	if (method === 'ResolveMedia' && keepPlaybackResolutionPending) {
		await new Promise<void>(() => undefined);
		return;
	}
    if (method === 'GetQueue') {
      await route.fulfill({ contentType: 'application/json', body: JSON.stringify({ result: { items: queueItems, preferences: { autoplay: true, remove_played: false } } }) });
      return;
    }
    if (method === 'EnqueueVideo') {
      const video = request.args?.[0] as typeof queuedVideo;
      const item = { id: `queue-${video.id}`, video, position: queueItems.length, state: 'pending', added_at: new Date().toISOString() };
      queueItems = [...queueItems, item];
      await route.fulfill({ contentType: 'application/json', body: JSON.stringify({ result: item }) });
      return;
    }
    if (method === 'MarkQueueItemPlayed') {
      const itemID = request.args?.[0];
      queueItems = queueItems.map((item) => item.id === itemID ? { ...item, state: 'played' } : item);
      await route.fulfill({ contentType: 'application/json', body: JSON.stringify({ result: null }) });
      return;
    }
    const mediaBaseURL = useLoopbackMediaProxy
      ? 'http://127.0.0.1:45678/api/media'
      : 'https://media.test';
    const results: Record<string, unknown> = {
      GetHome: {
        continue_watching: [],
        for_you: [firstVideo, queuedVideo],
        topic_sections: [],
        recent_subscriptions: [],
        rediscovery: []
      },
      ResolveMedia: {
        mode: 'resolved-media',
        primary: { url: `${mediaBaseURL}/${useHLS ? 'master.m3u8' : 'player-video-720.mp4'}` },
        audio: useHLS ? undefined : { url: `${mediaBaseURL}/player-adaptive-audio.m4a` },
        audio_only: { url: `${mediaBaseURL}/player-audio.m4a` },
        variants: [
          { id: '720', label: '720p', height: 720, has_audio: false, stream: { url: `${mediaBaseURL}/${useHLS ? 'master.m3u8' : 'player-video-720.mp4'}` } },
          { id: '360', label: '360p', height: 360, has_audio: true, stream: { url: `${mediaBaseURL}/player.mp4` } }
        ],
        metadata: { video_id: firstVideo.id, title: firstVideo.title }
      },
      GetSettings: {},
      GetAccount: null,
      GetDiagnostics: { warnings: [], errors: [] },
      ListIPTVSources: [],
      RememberVideo: null,
      SaveProgress: null
    };
    await route.fulfill({
      contentType: 'application/json',
      body: JSON.stringify({ result: results[method] ?? null })
    });
  });

  // Mantém o carregamento pendente para o teste controlar os eventos de mídia.
  await page.route('https://media.test/**', () => new Promise<void>(() => undefined));
  await page.route('http://127.0.0.1:45678/api/media/**', () => new Promise<void>(() => undefined));
  await page.goto('/');
}

test.describe('Player de vídeo', () => {
  test('time updates do not rewrite unchanged media preferences', async ({ page }) => {
    await openPlayer(page);
    await page.getByRole('button', { name: /Reproduzir Vídeo principal do player/i }).click();
    const video = page.getByTestId('video-player-container').locator('video');
    await expect(video).toHaveAttribute('src', /720/);
    await page.evaluate(() => {
      for (const media of document.querySelectorAll('video, audio')) {
        for (const property of ['volume', 'playbackRate']) {
          const descriptor = Object.getOwnPropertyDescriptor(HTMLMediaElement.prototype, property)!;
          Object.defineProperty(media, property, {
            get() { return descriptor.get!.call(this); },
            set(value: number) {
              this.setAttribute('data-preference-writes', String(Number(this.getAttribute('data-preference-writes') || 0) + 1));
              descriptor.set!.call(this, value);
            },
          });
        }
      }
    });
    for (let index = 0; index < 3; index++) {
      await video.dispatchEvent('timeupdate');
      await page.evaluate(() => new Promise(requestAnimationFrame));
    }
    await expect(page.locator('[data-preference-writes]')).toHaveCount(0);
    const audio = page.getByTestId('adaptive-audio-track');
    await audio.evaluate((element) => {
      Object.defineProperty(element, 'readyState', { get: () => 1 });
      Object.defineProperty(element, 'currentTime', {
        get: () => 0,
        set: () => element.setAttribute('data-redundant-seek', 'true'),
      });
    });
    await video.dispatchEvent('playing');
    await expect(audio).not.toHaveAttribute('data-redundant-seek');
  });

	test('mantém o seletor de qualidade visível enquanto o vídeo é resolvido', async ({ page }) => {
		await openPlayer(page, false, false, true);
		await page.getByRole('button', { name: /Reproduzir Vídeo principal do player/i }).click();

		const qualityButton = page.getByTestId('player-quality-button');
		await expect(page.getByTestId('player-bottom-controls')).toHaveCSS('opacity', '1');
		await expect(qualityButton).toBeVisible();
		await expect(qualityButton).toBeDisabled();
		await expect(qualityButton).toHaveAttribute('aria-label', 'Qualidade do vídeo: Carregando');
		await expect(qualityButton).toHaveAttribute('data-state', 'unavailable');
	});

  test('mantém o mesmo vídeo no miniplayer e oferece retorno claro à interface', async ({ page }) => {
    await openPlayer(page);
    await page.getByRole('button', { name: /Reproduzir Vídeo principal do player/i }).click();

    const player = page.getByRole('region', { name: 'Player de Vídeo' });
    const video = player.locator('video');
    await expect(player).toHaveAttribute('data-player-mode', 'expanded');
    await expect(video).toHaveCount(1);
    await video.evaluate((element) => element.setAttribute('data-mount-token', 'same-video'));

    await video.dispatchEvent('loadedmetadata');
    await expect.poll(() => page.evaluate(() => (window as Window & { __playerPlayCalls?: number }).__playerPlayCalls || 0)).toBeGreaterThan(0);

    await page.getByRole('button', { name: /Voltar à interface e manter o vídeo/i }).click();
    await expect(player).toHaveAttribute('data-player-mode', 'mini');
    await expect(video).toHaveAttribute('data-mount-token', 'same-video');
    await expect(video).toBeVisible();
    const miniControls = player.getByTestId('miniplayer-controls');
    const expandPlayer = player.getByRole('button', { name: 'Expandir player' });
    await expect(miniControls).toHaveCSS('opacity', '0');
    await player.hover();
    await expect(miniControls).toHaveCSS('opacity', '1');
    await page.mouse.move(0, 0);
    await expandPlayer.focus();
    await expect(miniControls).toHaveCSS('opacity', '1');
    await expect(expandPlayer).toBeVisible();
  });

  test('move o miniplayer entre os quatro cantos e mantém a posição ao reabrir', async ({ page }) => {
    await page.addInitScript(() => localStorage.removeItem('nanotube_miniplayer_position'));
    await openPlayer(page);
    await page.getByRole('button', { name: /Reproduzir Vídeo principal do player/i }).click();

    const player = page.getByRole('region', { name: 'Player de Vídeo' });
    const video = player.locator('video');
    await video.dispatchEvent('loadedmetadata');
    await video.evaluate((element) => element.setAttribute('data-mount-token', 'miniplayer-move'));
    await page.getByRole('button', { name: /Voltar à interface e manter o vídeo/i }).click();
    await expect(player).toHaveAttribute('data-player-mode', 'mini');
    await expect(player).toHaveAttribute('data-miniplayer-position', 'bottom-right');
    const moveButton = player.getByTestId('miniplayer-move');
    await moveButton.focus();
    await expect(player.getByTestId('miniplayer-controls')).toHaveCSS('opacity', '1');

    for (const position of ['bottom-left', 'top-left', 'top-right', 'bottom-right']) {
      await moveButton.click();
      await expect(player).toHaveAttribute('data-miniplayer-position', position);
      await expect(video).toHaveAttribute('data-mount-token', 'miniplayer-move');
      await expect(moveButton).toBeFocused();
    }
    await expect.poll(() => page.evaluate(() => localStorage.getItem('nanotube_miniplayer_position'))).toBe('bottom-right');

    await page.reload();
    await page.getByRole('button', { name: /Reproduzir Vídeo principal do player/i }).click();
    await page.locator('video').dispatchEvent('loadedmetadata');
    await page.getByRole('button', { name: /Voltar à interface e manter o vídeo/i }).click();
    await expect(page.getByRole('region', { name: 'Player de Vídeo' })).toHaveAttribute('data-miniplayer-position', 'bottom-right');
  });

  test('expõe variantes de qualidade com fallback automático seguro', async ({ page }) => {
    await openPlayer(page);
    await page.getByRole('button', { name: /Reproduzir Vídeo principal do player/i }).click();
    await page.getByTestId('video-player-container').hover();

    const qualityButton = page.getByTestId('player-quality-button');
    await expect(qualityButton).toBeVisible();
    await qualityButton.click();
    const qualityMenu = page.getByRole('menu', { name: 'Selecionar qualidade do vídeo' });
    await expect(qualityMenu.getByRole('menuitemradio')).toHaveCount(4);
    await expect(qualityMenu.getByRole('menuitemradio', { name: 'Automática' })).toBeVisible();
    await expect(qualityMenu.getByRole('menuitemradio', { name: /720p.*adaptativos/ })).toBeVisible();
    await expect(qualityMenu.getByRole('menuitemradio', { name: '360p' })).toBeVisible();
    await expect(qualityMenu.getByRole('menuitemradio', { name: 'Somente áudio' })).toBeVisible();
    await qualityMenu.getByRole('menuitemradio', { name: '360p' }).click();
    await expect(qualityButton).toHaveAttribute('aria-label', 'Qualidade do vídeo: 360p');

    await page.locator('video').dispatchEvent('error');
    await expect(qualityButton).toHaveAttribute('aria-label', /automática|reproduzindo .* pediu automática/i);
    await expect(page.getByRole('alert')).toContainText('qualidade automática');
  });

  test('expõe menu de faixa de áudio e legenda com toggles', async ({ page }) => {
    await openPlayer(page);
    await page.getByRole('button', { name: /Reproduzir Vídeo principal do player/i }).click();
    await page.getByTestId('video-player-container').hover();

    const audioButton = page.getByRole('button', { name: 'Áudio padrão' });
    await expect(audioButton).toBeVisible();
    await audioButton.click();
    const audioMenu = page.getByRole('menu', { name: 'Selecionar faixa de áudio' });
    await expect(audioMenu.getByRole('menuitemradio').first()).toBeVisible();
  });

  test('sincroniza MediaSession com o vídeo ativo', async ({ page }) => {
    await openPlayer(page);
    await page.getByRole('button', { name: /Reproduzir Vídeo principal do player/i }).click();
    await page.getByTestId('video-player-container').hover();
    const media = await page.evaluate(() => ({
      title: navigator.mediaSession.metadata?.title || '',
      artist: navigator.mediaSession.metadata?.artist || '',
    }));
    expect(media.title).toContain('Vídeo principal do player');
    expect(media.artist).toContain('Canal');
  });

  test('HLS mantém a fonte ao selecionar níveis e retornar ao automático', async ({ page }) => {
    await openPlayer(page, false, false, false, false, true);
    await page.route('https://media.test/master.m3u8', (route) => route.fulfill({
      contentType: 'application/vnd.apple.mpegurl',
      headers: { 'Access-Control-Allow-Origin': '*' },
      body: '#EXTM3U\n#EXT-X-STREAM-INF:BANDWIDTH=500000,RESOLUTION=640x360,CODECS="avc1.42e01e,mp4a.40.2"\nlow.m3u8\n#EXT-X-STREAM-INF:BANDWIDTH=1500000,RESOLUTION=1280x720,CODECS="avc1.42e01e,mp4a.40.2"\nhigh.m3u8\n',
    }));
    await page.getByRole('button', { name: /Reproduzir Vídeo principal do player/i }).click();
    const video = page.locator('video');
    await expect(video).toHaveAttribute('src', /^blob:/);
    const source = await video.getAttribute('src');
    const qualityButton = page.getByTestId('player-quality-button');
    await qualityButton.click();
    await expect(page.getByRole('menuitemradio', { name: '720p', exact: true })).toBeVisible();
    await page.getByRole('menuitemradio', { name: '720p', exact: true }).click();
    for (const name of ['360p', 'Automática']) {
      await qualityButton.click();
      await page.getByRole('menuitemradio', { name, exact: true }).click();
      await expect(qualityButton).toHaveAttribute('aria-label', `Qualidade do vídeo: ${name}`);
      await expect(video).toHaveAttribute('src', source!);
    }
    await expect(page.getByRole('alert')).toHaveCount(0);
  });

  test('trocas rápidas preservam posição e pausa sem recarregar a mesma fonte', async ({ page }) => {
    await openPlayer(page);
    await page.getByRole('button', { name: /Reproduzir Vídeo principal do player/i }).click();
    const video = page.locator('video');
    await expect(video).toHaveAttribute('src', /720/);
    await video.dispatchEvent('loadedmetadata');
    await video.dispatchEvent('pause');
    await video.evaluate((element) => {
      let position = 24;
      Object.defineProperty(element, 'currentTime', {
        configurable: true,
        get: () => position,
        set: (value: number) => { position = value; },
      });
      Object.defineProperty(element, 'duration', { configurable: true, get: () => 60 });
      element.load = () => {
        position = 0;
        element.setAttribute('data-load-count', String(Number(element.getAttribute('data-load-count') || 0) + 1));
      };
    });
    const qualityButton = page.getByTestId('player-quality-button');
    for (const name of [/720p/, /^Automática$/]) {
      await qualityButton.click();
      await page.getByRole('menuitemradio', { name }).click();
    }
    await expect(video).not.toHaveAttribute('data-load-count');
    const playCalls = await page.evaluate(() => (window as Window & { __playerPlayCalls?: number }).__playerPlayCalls);
    for (const name of [/^360p$/, /^Somente áudio$/, /^Automática$/]) {
      await qualityButton.click();
      await page.getByRole('menuitemradio', { name }).click();
    }
    await video.dispatchEvent('loadedmetadata');
    await expect(video).toHaveAttribute('data-load-count', '3');
    await expect.poll(() => video.evaluate((element) => element.currentTime)).toBe(24);
    await expect.poll(() => page.evaluate(() => (window as Window & { __playerPlayCalls?: number }).__playerPlayCalls)).toBe(playCalls);
    await expect(page.getByTestId('adaptive-audio-track')).toHaveAttribute('src', /player-adaptive-audio/);
  });

  test('código 4 troca para stream combinado e não repete formatos que já falharam', async ({ page }) => {
    await openPlayer(page);
    await page.getByRole('button', { name: /Reproduzir Vídeo principal do player/i }).click();

    const video = page.locator('video');
    await expect(video).toHaveAttribute('src', 'https://media.test/player-video-720.mp4');
    await video.evaluate((element) => {
      Object.defineProperty(element, 'error', {
        configurable: true,
        value: { code: 4, message: '' }
      });
      element.dispatchEvent(new Event('error'));
    });

    await expect(video).toHaveAttribute('src', 'https://media.test/player.mp4');
    await expect(page.getByTestId('player-quality-button')).toHaveAttribute('aria-label', 'Qualidade do vídeo: 360p');
    await expect(page.getByRole('alert')).toContainText('formato ou codec não suportado');

    await video.dispatchEvent('error');
    await expect(video).toHaveAttribute('src', 'https://media.test/player.mp4');
    await expect(page.getByRole('alert')).toContainText('Nenhum formato compatível restante');

    const retryButton = page.getByTestId('player-retry-button');
    await expect(retryButton).toBeVisible();
    await retryButton.click();
    await expect(page.getByRole('alert')).toHaveCount(0);
  });

  test('permite navegar no seletor de qualidade com as teclas direcionais', async ({ page }) => {
    await openPlayer(page);
    await page.getByRole('button', { name: /Reproduzir Vídeo principal do player/i }).click();
    await page.getByTestId('video-player-container').hover();

    const qualityButton = page.getByTestId('player-quality-button');
    await qualityButton.click();
    const qualityMenu = page.getByRole('menu', { name: 'Selecionar qualidade do vídeo' });
    await expect(qualityMenu.getByRole('menuitemradio', { name: 'Automática' })).toBeFocused();

    await page.keyboard.press('End');
    const audioOnlyOption = qualityMenu.getByRole('menuitemradio', { name: 'Somente áudio' });
    await expect(audioOnlyOption).toBeFocused();
    await page.keyboard.press('ArrowDown');
    await expect(qualityMenu.getByRole('menuitemradio', { name: 'Automática' })).toBeFocused();
    await page.keyboard.press('ArrowUp');
    await expect(audioOnlyOption).toBeFocused();
    await page.keyboard.press('Enter');

    await expect(qualityButton).toHaveAttribute('aria-label', 'Qualidade do vídeo: Somente áudio');
    await expect(qualityButton).toBeFocused();
  });

  test('sincroniza áudio separado ao selecionar resolução adaptativa', async ({ page }) => {
    await openPlayer(page);
    await page.getByRole('button', { name: /Reproduzir Vídeo principal do player/i }).click();
    await page.getByTestId('video-player-container').hover();

    const qualityButton = page.getByTestId('player-quality-button');
    await qualityButton.click();
    await page.getByRole('menuitemradio', { name: /720p/ }).click();
    await expect(qualityButton).toHaveAttribute('aria-label', 'Qualidade do vídeo: 720p');
    await expect(page.locator('video')).toHaveAttribute('src', 'https://media.test/player-video-720.mp4');
    await expect(page.getByTestId('adaptive-audio-track')).toHaveAttribute('src', 'https://media.test/player-adaptive-audio.m4a');
  });

  test('retorna ao stream combinado quando a faixa adaptativa não pode iniciar', async ({ page }) => {
    await openPlayer(page, false, true);
    await page.getByRole('button', { name: /Reproduzir Vídeo principal do player/i }).click();

    const video = page.locator('video');
    await video.dispatchEvent('loadedmetadata');

    await expect(video).toHaveAttribute('src', 'https://media.test/player.mp4');
    await expect(page.getByTestId('adaptive-audio-track')).not.toHaveAttribute('src', /.+/);
    await expect(page.getByTestId('player-quality-button')).toHaveAttribute('aria-label', 'Qualidade do vídeo: 360p');
    await expect(page.getByRole('alert')).toContainText('voltou para 360p');
  });

  test('preserva o áudio nativo quando o stream remoto não permite DSP seguro', async ({ page }) => {
    await openPlayer(page);
    await page.getByRole('button', { name: /Reproduzir Vídeo principal do player/i }).click();
    await expect(page.locator('video')).toHaveAttribute('src', 'https://media.test/player-video-720.mp4');
    const playerContainer = page.getByTestId('video-player-container');
    await expect(playerContainer).toHaveAttribute('data-audio-mode', 'separate');
    await expect(page.getByTestId('adaptive-audio-track')).toHaveAttribute('src', 'https://media.test/player-adaptive-audio.m4a');
    await playerContainer.hover();
    await page.getByRole('button', { name: 'DSP de áudio e ganho digital' }).click();

    const gainControl = page.getByRole('slider', { name: 'Ganho digital' });
    await expect(gainControl).toBeDisabled();
    await expect(page.getByRole('status')).toContainText('preservar o áudio nativo');
    await expect.poll(() => page.getByTestId('adaptive-audio-track').evaluate((element) => (element as HTMLAudioElement).volume)).toBe(1);
  });

  test('autoriza CORS e DSP somente para o proxy HTTP local controlado', async ({ page }) => {
    await openPlayer(page, false, false, false, true);
    await page.getByRole('button', { name: /Reproduzir Vídeo principal do player/i }).click();

    const video = page.locator('video');
    await expect(video).toHaveAttribute('src', 'http://127.0.0.1:45678/api/media/player-video-720.mp4');
    await expect(video).toHaveAttribute('crossorigin', 'anonymous');
    await page.getByTestId('video-player-container').hover();
    await page.getByRole('button', { name: 'DSP de áudio e ganho digital' }).click();
    await expect(page.getByRole('slider', { name: 'Ganho digital' })).toBeEnabled();
  });

  test('troca para somente áudio e retorna ao stream principal', async ({ page }) => {
    await openPlayer(page);
    await page.getByRole('button', { name: /Reproduzir Vídeo principal do player/i }).click();
    await page.getByTestId('video-player-container').hover();

    const qualityButton = page.getByTestId('player-quality-button');
    await qualityButton.click();
    await page.getByRole('menuitemradio', { name: 'Somente áudio' }).click();
    await expect(qualityButton).toHaveAttribute('aria-label', 'Qualidade do vídeo: Somente áudio');
    await expect(page.getByTestId('video-player-container').locator('strong').filter({ hasText: 'Somente áudio' })).toBeVisible();
    await expect(page.locator('video')).toHaveAttribute('src', 'https://media.test/player-audio.m4a');

    await qualityButton.click();
    await page.getByRole('menuitemradio', { name: 'Automática' }).click();
    await expect(page.locator('video')).toHaveAttribute('src', 'https://media.test/player-video-720.mp4');
  });

  test('botão usa fullscreen nativo e acompanha fullscreenchange', async ({ page }) => {
    await openPlayer(page);
    await page.getByRole('button', { name: /Reproduzir Vídeo principal do player/i }).click();
    const playerContainer = page.getByTestId('video-player-container');
    await playerContainer.hover();

    await page.getByRole('button', { name: 'Entrar em tela cheia' }).click();
    await expect(playerContainer).toHaveAttribute('data-fullscreen-mode', 'native');
    await expect.poll(() => page.evaluate(() => Boolean(document.fullscreenElement))).toBe(true);

    await playerContainer.hover();
    await page.getByRole('button', { name: 'Sair da tela cheia' }).click();
    await expect(playerContainer).toHaveAttribute('data-fullscreen-mode', 'off');
  });

  test('usa fullscreen interno quando a API nativa não está disponível', async ({ page }) => {
    await openPlayer(page, true);
    await page.getByRole('button', { name: /Reproduzir Vídeo principal do player/i }).click();
    const playerContainer = page.getByTestId('video-player-container');
    await playerContainer.hover();

    await page.getByRole('button', { name: 'Entrar em tela cheia' }).click();
    await expect(playerContainer).toHaveAttribute('data-fullscreen-mode', 'in-app');
    await page.keyboard.press('Escape');
    await expect(playerContainer).toHaveAttribute('data-fullscreen-mode', 'off');
  });

  test('evento ended avança para o próximo item da fila', async ({ page }) => {
    await openPlayer(page);
    await page.getByRole('button', { name: /Adicionar Próximo vídeo da fila/i }).click();
    await page.getByRole('button', { name: /Reproduzir Vídeo principal do player/i }).click();

    await page.locator('video').dispatchEvent('ended');
    await expect(page.getByRole('region', { name: 'Player de Vídeo' })).toContainText(queuedVideo.title);
  });
});
