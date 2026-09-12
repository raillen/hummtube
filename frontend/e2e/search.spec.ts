import { test, expect } from '@playwright/test';

test.describe('Pesquisa no Catálogo & YouTube', () => {
  test.beforeEach(async ({ page }) => {
    await page.goto('/');
  });

  test('deve executar pesquisa e transicionar para a aba de resultados', async ({ page }) => {
    let resourceTypes: string[] = [];
    await page.route('**/api/rpc', async (route) => {
      const request = route.request().postDataJSON() as { method?: string; args?: Array<{ resource_types?: string[] }> };
      if (request.method === 'Search') {
        resourceTypes = request.args?.[0]?.resource_types || [];
        await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ result: { items: [], source: 'yt-dlp' } }) });
        return;
      }
      await route.continue();
    });
    const searchInput = page.getByPlaceholder('Buscar no catálogo ou YouTube (Ctrl+K)...');
    await searchInput.fill('Svelte');
    await searchInput.press('Enter');

    await expect(page.getByRole('heading', { name: 'Pesquisa avançada' })).toBeVisible();
    await expect(page.getByLabel('Termo de pesquisa')).toHaveValue('Svelte');
    expect(resourceTypes).toEqual(['video', 'channel', 'playlist']);
  });

  test('envia request tipado e abre resultados de vídeo, canal e playlist', async ({ page }) => {
    await page.route('**/api/rpc', async (route) => {
      const request = route.request().postDataJSON() as { method?: string };
      if (request.method === 'SearchLocalFTS') {
        await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ result: [] }) });
        return;
      }
      if (request.method === 'Search') {
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify({ result: {
            items: [
              { id: 'video-1', channel_id: 'channel-1', channel_title: 'Canal', title: 'Linux em PC antigo', published_at: new Date().toISOString(), duration: 60e9, thumbnail_url: '' },
              { id: 'channel-1', channel_id: 'channel-1', resource_type: 'channel', title: 'Canal Linux', published_at: new Date().toISOString(), duration: 0, thumbnail_url: '' },
              { id: 'PL_search', channel_id: 'channel-1', resource_type: 'playlist', title: 'Playlist Linux', published_at: new Date().toISOString(), duration: 0, thumbnail_url: '' },
            ],
            next_page_token: '24', source: 'youtube-api', notices: [],
          } }),
        });
        return;
      }
      if (request.method === 'GetRemotePlaylistPage') {
        await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ result: {
          playlist_id: 'PL_search', videos: [{ id: 'video-1', channel_id: 'channel-1', channel_title: 'Canal', title: 'Linux em PC antigo', published_at: new Date().toISOString(), duration: 60e9, thumbnail_url: '' }],
        } }) });
        return;
      }
      await route.continue();
    });

    await page.locator('aside').getByRole('button', { name: 'Pesquisa' }).click();
    const advancedSearch = page.getByRole('search', { name: 'Pesquisa no catálogo e YouTube' });
    await advancedSearch.getByLabel('Termo de pesquisa').fill('linux');
    await advancedSearch.getByRole('button', { name: 'Playlists' }).click();
    await advancedSearch.getByRole('button', { name: 'Canais' }).click();
    await advancedSearch.getByRole('button', { name: /^Buscar$/ }).click();

    await expect(page.getByText('YouTube Data API + NanoRank local')).toBeVisible();
    await expect(page.getByRole('heading', { name: 'Playlist Linux' })).toBeVisible();
    await expect(page.getByRole('heading', { name: 'Canal Linux' })).toBeVisible();
    await page.getByRole('button', { name: 'Abrir', exact: true }).click();
    await expect(page.getByRole('heading', { name: 'Playlist Linux' })).toBeVisible();
    await expect(page.getByRole('button', { name: 'Adicionar tudo à fila' })).toBeVisible();
  });

  test('preserva a pesquisa ao sair e voltar pela navegação', async ({ page }) => {
    await page.route('**/api/rpc', async (route) => {
      const request = route.request().postDataJSON() as { method?: string };
      if (request.method === 'SearchLocalFTS') {
        await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ result: [] }) });
        return;
      }
      if (request.method === 'Search') {
        await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ result: {
          items: [{ id: 'kept', channel_id: 'channel', channel_title: 'Canal', title: 'Resultado preservado', published_at: new Date().toISOString(), duration: 60e9, thumbnail_url: '' }], source: 'yt-dlp',
        } }) });
        return;
      }
      await route.continue();
    });

    const globalSearch = page.getByPlaceholder('Buscar no catálogo ou YouTube (Ctrl+K)...');
    await globalSearch.fill('consulta anterior');
    await globalSearch.press('Enter');
    await expect(page.getByText('Resultado preservado')).toBeVisible();
    await page.locator('aside').getByRole('button', { name: 'Biblioteca' }).click();
    await page.getByRole('button', { name: 'Voltar', exact: true }).click();
    await expect(page.getByLabel('Termo de pesquisa')).toHaveValue('consulta anterior');
    await expect(page.getByText('Resultado preservado')).toBeVisible();
  });
});
