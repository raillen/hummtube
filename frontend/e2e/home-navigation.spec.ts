import { test, expect } from '@playwright/test';

for (const startupEnabled of [true, false]) {
  test(`startup refresh preference ${startupEnabled} and Header invalidation`, async ({ page }) => {
    let refreshes = 0;
    let releaseRefresh: () => void = () => {};
    const refreshGate = new Promise<void>((resolve) => { releaseRefresh = resolve; });
    await page.route('**/api/rpc', async (route) => {
      const { method } = route.request().postDataJSON() as { method: string };
      if (method === 'RefreshSubscriptions') {
        refreshes++;
        await refreshGate;
      }
      const result = method === 'GetSettings' ? { 'sync.refresh_on_startup': String(startupEnabled) }
        : method === 'GetHome' ? {
          for_you: [{ id: 'refresh-video', title: refreshes ? 'Updated catalog' : 'Cached catalog', channel_id: 'refresh-channel', channel_title: 'Refresh channel', duration: 60000000000 }],
          topic_sections: [], continue_watching: [], recent_subscriptions: [], rediscovery: [],
        } : null;
      await route.fulfill({ json: { result } });
    });
    await page.goto('/');
    await expect(page.getByRole('button', { name: /Reproduzir Cached catalog/ })).toBeVisible();
    if (startupEnabled) await expect.poll(() => refreshes).toBe(1);
    else expect(refreshes).toBe(0);
    await page.getByTitle('Atualizar inscrições (Ctrl+R)').click();
    await expect.poll(() => refreshes).toBe(1);
    releaseRefresh();
    await expect(page.getByRole('button', { name: /Reproduzir Updated catalog/ })).toBeVisible();
    expect(refreshes).toBe(1);
  });
}

test.describe('Navegação Principal & Header', () => {
  test.beforeEach(async ({ page }) => {
    page.on('console', msg => console.log('PAGE LOG:', msg.text()));
    page.on('pageerror', err => console.log('PAGE ERROR:', err));
    await page.goto('/');
  });

  test('deve carregar a página inicial com título e elementos do Header', async ({ page }) => {
    await expect(page.locator('header')).toBeVisible();
    await expect(page.locator('header').getByAltText('HummTube')).toBeVisible();

    const searchInput = page.getByPlaceholder('Buscar no catálogo ou YouTube (Ctrl+K)...');
    await expect(searchInput).toBeVisible();

    await expect(page.getByTitle('Atualizar inscrições (Ctrl+R)')).toBeVisible();
    await expect(page.getByTitle('Alternar tema')).toBeVisible();
    await expect(page.getByTitle('Modo TV (F10)')).toBeVisible();
    await expect(page.getByTitle(/Entrar|Conectado/)).toBeVisible();
  });

  test('deve alternar entre tema Claro e Escuro', async ({ page }) => {
    const themeBtn = page.getByTitle('Alternar tema');
    await themeBtn.click();

    const isLight = await page.evaluate(() => document.documentElement.classList.contains('light'));
    expect(isLight).toBe(true);

    await themeBtn.click();
    const isDark = await page.evaluate(() => !document.documentElement.classList.contains('light'));
    expect(isDark).toBe(true);
  });

  test('deve focar a busca global com Ctrl+K', async ({ page }) => {
    const searchInput = page.getByPlaceholder('Buscar no catálogo ou YouTube (Ctrl+K)...');
    await page.locator('header').getByTitle('Alternar tema').click();
    await page.keyboard.press('Control+k');
    await expect(searchInput).toBeFocused();
  });

  test('deve ativar e desativar o Modo TV (10-Foot UI)', async ({ page }) => {
    const tvBtn = page.getByTitle('Modo TV (F10)');
    await tvBtn.click();

    const isTv = await page.evaluate(() => document.documentElement.classList.contains('tv-mode'));
    expect(isTv).toBe(true);

    await tvBtn.click();
    const isNotTv = await page.evaluate(() => !document.documentElement.classList.contains('tv-mode'));
    expect(isNotTv).toBe(true);
  });

  test('deve carregar o próximo lote da Home sem resetar a contagem', async ({ page }) => {
    const videos = Array.from({ length: 40 }, (_, index) => ({
      id: `home-video-${index}`,
      channel_id: 'home-channel',
      channel_title: 'Canal Home',
      title: `Vídeo Home ${index}`,
      published_at: new Date(Date.now() - index * 60_000).toISOString(),
      duration: 60_000_000_000,
      thumbnail_url: ''
    }));
    await page.route('**/api/rpc', async (route) => {
      const request = route.request().postDataJSON() as { method?: string };
      if (request.method === 'GetHome') {
        await route.fulfill({ contentType: 'application/json', body: JSON.stringify({ result: { continue_watching: [], for_you: videos, topic_sections: [], recent_subscriptions: [], rediscovery: [] } }) });
        return;
      }
      if (request.method === 'GetSettings') {
        await route.fulfill({ contentType: 'application/json', body: JSON.stringify({ result: { home_page_size: '24', subscriptions_page_size: '24' } }) });
        return;
      }
      await route.continue();
    });
    await page.reload();
    await expect(page.getByRole('heading', { name: 'Para Você' })).toBeVisible();
    await expect(page.getByRole('button', { name: 'Canal Home' }).first()).toBeVisible();
    const loadMore = page.getByRole('button', { name: 'Carregar mais' });
    await expect(loadMore).toBeVisible();
    await loadMore.click();
    await expect(page.getByRole('group', { name: /Ações para Vídeo Home 39/ })).toBeVisible();
  });

  test('deve navegar entre todas as abas da Sidebar', async ({ page }) => {
    const sidebar = page.locator('aside');
    const main = page.locator('main');

    // 1. Inscrições e gerenciamento de canais são workspaces separados
    await sidebar.getByRole('button', { name: 'Inscrições' }).click();
    await expect(main.getByRole('heading', { name: 'Vídeos das inscrições' })).toBeVisible();
    await sidebar.getByRole('button', { name: 'Gerenciar canais' }).click();
    await expect(main.getByRole('heading', { name: 'Gerenciar canais' })).toBeVisible();

    // 2. Playlists
    await sidebar.getByRole('button', { name: 'Playlists' }).click();
    await expect(main.getByRole('heading', { name: 'Playlists Locais' })).toBeVisible();

    await sidebar.getByRole('button', { name: 'Fila' }).click();
    await expect(main.getByRole('heading', { name: 'Fila de reprodução' })).toBeVisible();

    // 3. Biblioteca
    await sidebar.getByRole('button', { name: 'Biblioteca' }).click();
    await expect(main.getByRole('heading', { name: 'Biblioteca Local' })).toBeVisible();

    await expect(sidebar.getByRole('button', { name: 'Música' })).toHaveCount(0);
    await expect(sidebar.getByRole('button', { name: 'HummIPTV' })).toHaveCount(0);
    await expect(sidebar.getByRole('button', { name: 'NanoIPTV' })).toHaveCount(0);

    // 4. Ajustes
    await sidebar.getByRole('button', { name: 'Ajustes' }).click();
    await expect(main.getByRole('heading', { name: 'Configurações & Ajustes' })).toBeVisible();

    // 5. Voltar para Início
    await sidebar.getByRole('button', { name: 'Início' }).click();
  });
});
