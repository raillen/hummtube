import { test, expect } from '@playwright/test';

test.describe('Playlists & Organização', () => {
  test.beforeEach(async ({ page }) => {
    await page.goto('/');
    await page.locator('aside').getByRole('button', { name: 'Playlists' }).click();
  });

  test('deve exibir cabeçalho de playlists e botão de criar', async ({ page }) => {
    await expect(page.getByRole('heading', { name: 'Playlists Locais' })).toBeVisible();
    await expect(page.locator('main').getByRole('button', { name: /Nova Playlist/i })).toBeVisible();
  });

  test('deve abrir e fechar o modal de criação de playlist', async ({ page }) => {
    const createBtn = page.locator('main').getByRole('button', { name: /Nova Playlist/i });
    await expect(createBtn).toBeVisible();
    await createBtn.click();

    await expect(page.getByText('Criar Nova Playlist')).toBeVisible();
    await expect(page.getByPlaceholder('Nome da playlist...')).toBeVisible();

    // Fechar modal
    await page.getByRole('button', { name: 'Cancelar' }).click();
    await expect(page.getByText('Criar Nova Playlist')).not.toBeVisible();
  });

  test('abre playlist remota, adiciona tudo à fila e materializa uma cópia local', async ({ page }) => {
    const video = { id: 'remote-video', channel_id: 'channel', channel_title: 'Canal', title: 'Vídeo remoto', published_at: new Date().toISOString(), duration: 90e9, thumbnail_url: '' };
    let queueItems: Array<Record<string, unknown>> = [];
    await page.route('**/api/rpc', async (route) => {
      const request = route.request().postDataJSON() as { method?: string; args?: unknown[] };
      if (request.method === 'GetQueue') {
        await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ result: { items: queueItems, preferences: { autoplay: true, remove_played: false } } }) });
        return;
      }
      if (request.method === 'EnqueueVideo') {
        const queuedVideo = request.args?.[0] as typeof video;
        const item = { id: `queue-${queuedVideo.id}`, video: queuedVideo, position: queueItems.length, state: 'pending', added_at: new Date().toISOString() };
        queueItems = [...queueItems, item];
        await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ result: item }) });
        return;
      }
      if (request.method === 'GetRemotePlaylistPage') {
        await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ result: { playlist_id: 'PL_fixture', videos: [video] } }) });
        return;
      }
      if (request.method === 'ImportRemotePlaylist') {
        await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ result: {
          playlist: { id: 'local-import', name: 'Playlist do YouTube', is_smart: false, item_count: 1, created_at: new Date().toISOString(), updated_at: new Date().toISOString() }, videos: [video],
        } }) });
        return;
      }
      if (request.method === 'GetPlaylist') {
        await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ result: {
          playlist: { id: 'local-import', name: 'Playlist do YouTube', is_smart: false, item_count: 1, created_at: new Date().toISOString(), updated_at: new Date().toISOString() }, videos: [video],
        } }) });
        return;
      }
      await route.continue();
    });

    await page.reload();
    await page.locator('aside').getByRole('button', { name: 'Playlists' }).click();

    await page.getByLabel('ID ou URL de playlist do YouTube').fill('PL_fixture');
    await page.getByRole('button', { name: 'Abrir remota' }).click();
    await expect(page.getByText('Remota')).toBeVisible();
    await page.getByRole('button', { name: 'Adicionar tudo à fila' }).click();
    await expect(page.getByRole('button', { name: /Abrir fila de reprodução \(1 vídeos\)/ })).toBeVisible();
    await page.getByRole('button', { name: 'Salvar ou combinar' }).click();
    await expect(page.getByRole('heading', { name: 'Salvar playlist' })).toBeVisible();
    await page.getByRole('button', { name: 'Salvar como nova' }).click();
    await expect(page.getByRole('button', { name: 'Excluir playlist' })).toBeVisible();
  });
});
