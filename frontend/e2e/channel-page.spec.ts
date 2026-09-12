import { test, expect } from '@playwright/test';

const video = (id: string, title: string) => ({
  id, channel_id: 'UC_channel', channel_title: 'Canal de Teste', title,
  published_at: new Date().toISOString(), duration: 90e9, thumbnail_url: '',
});

test('abre canal, pagina vídeos e preserva o canal ao visitar uma playlist', async ({ page }) => {
  await page.route('**/api/rpc', async (route) => {
    const request = route.request().postDataJSON() as { method?: string; args?: unknown[] };
    const respond = (result: unknown) => route.fulfill({
      status: 200, contentType: 'application/json', body: JSON.stringify({ result }),
    });
    if (request.method === 'SearchLocalFTS') return respond([]);
    if (request.method === 'GetChannels') return respond([]);
    if (request.method === 'GetRemoteChannelVideosPage') {
      const token = String(request.args?.[1] || '');
      return respond({
        channel_id: 'UC_channel',
        videos: token ? [video('video-2', 'Segundo vídeo do canal')] : [video('video-1', 'Primeiro vídeo do canal')],
        next_page_token: token ? '' : '24',
      });
    }
    if (request.method === 'GetRemotePlaylistPage') {
      return respond({ playlist_id: 'PL_channel', videos: [video('playlist-video', 'Vídeo da playlist')] });
    }
    if (request.method === 'Search') {
      const options = (request.args?.[0] || {}) as { resource_types?: string[] };
      if (options.resource_types?.length === 1 && options.resource_types[0] === 'playlist') {
        return respond({
          items: [{
            ...video('PL_channel', 'Playlist do canal'), resource_type: 'playlist', duration: 0,
          }],
          source: 'youtube-api',
        });
      }
      return respond({
        items: [{
          ...video('UC_channel', 'Canal de Teste'), resource_type: 'channel', duration: 0,
        }],
        source: 'youtube-api',
      });
    }
    return route.continue();
  });

  await page.goto('/');
  await page.getByRole('complementary', { name: 'Navegação Lateral' }).getByRole('button', { name: 'Pesquisa' }).click();
  const search = page.getByRole('search', { name: 'Pesquisa no catálogo e YouTube' });
  await search.getByLabel('Termo de pesquisa').fill('canal');
  await search.getByRole('button', { name: 'Canais' }).click();
  await search.getByRole('button', { name: 'Vídeos' }).click();
  await search.getByRole('button', { name: /^Buscar$/ }).click();
  await page.getByRole('button', { name: 'Ver vídeos' }).click();

  await expect(page.getByRole('heading', { name: 'Canal de Teste' })).toBeVisible();
  await expect(page.getByRole('heading', { name: 'Primeiro vídeo do canal' })).toBeVisible();
  await page.getByRole('button', { name: 'Carregar mais' }).click();
  await expect(page.getByRole('heading', { name: 'Segundo vídeo do canal' })).toBeVisible();

  const channelNavigation = page.getByRole('navigation', { name: 'Conteúdo do canal' });
  await channelNavigation.getByRole('button', { name: 'Playlists', exact: true }).click();
  await page.getByRole('button', { name: /Playlist do canal/ }).click();
  await expect(page.getByRole('heading', { name: 'Playlist do canal' })).toBeVisible();
  await page.getByRole('button', { name: 'Voltar para playlists' }).click();
  await expect(page.getByRole('heading', { name: 'Canal de Teste' })).toBeVisible();
});
