import { test, expect, type Page } from '@playwright/test';

interface RpcRequest {
  service?: string;
  method?: string;
  args?: unknown[];
}

async function mockRpc(page: Page, onSaveTvMode?: (value: string) => void) {
  await page.route('**/api/rpc', async (route) => {
    const request = route.request().postDataJSON() as RpcRequest;
    const method = request.method || '';
    if (method === 'SaveSetting' && request.args?.[0] === 'ui.tv_mode') {
      onSaveTvMode?.(String(request.args?.[1] || ''));
      await route.fulfill({ json: { result: null } });
      return;
    }
    const result =
      method === 'GetSettings'
        ? {}
        : method === 'GetHome'
          ? {
              for_you: [
                {
                  id: 'tv-video-1',
                  title: 'Vídeo TV 1',
                  channel_id: 'tv-channel',
                  channel_title: 'Canal TV',
                  duration: 60000000000
                }
              ],
              topic_sections: [],
              continue_watching: [],
              recent_subscriptions: [],
              rediscovery: []
            }
          : method === 'GetAccount'
            ? null
            : null;
    await route.fulfill({ json: { result } });
  });
}

test.describe('Navegação TV e acessibilidade', () => {
  test('modo TV persiste configuração e usa setas sem roubar sliders', async ({ page }) => {
    let savedTvMode = '';
    await mockRpc(page, (value) => {
      savedTvMode = value;
    });
    await page.goto('/');

    await page.getByTitle('Modo TV (F10)').click();
    await expect.poll(() => page.evaluate(() => document.documentElement.classList.contains('tv-mode'))).toBe(true);
    await expect.poll(() => savedTvMode).toBe('1');
    await expect.poll(() => page.evaluate(() => localStorage.getItem('nanotube_tv_mode'))).toBe('1');

    await page.reload();
    await expect.poll(() => page.evaluate(() => document.documentElement.classList.contains('tv-mode'))).toBe(true);

    const searchInput = page.getByPlaceholder('Buscar no catálogo ou YouTube (Ctrl+K)...');
    await searchInput.focus();
    const focusedBefore = await page.evaluate(() => document.activeElement?.id);
    await page.keyboard.press('ArrowDown');
    expect(await page.evaluate(() => document.activeElement?.id)).toBe(focusedBefore);
  });

  test('Escape em modal retorna o foco ao gatilho', async ({ page }) => {
    await mockRpc(page);
    await page.goto('/');
    const loginButton = page.getByTitle(/Entrar/);
    await loginButton.click();
    await expect(page.getByText('Autenticação & Sincronização')).toBeVisible();
    await page.keyboard.press('Escape');
    await expect(page.getByText('Autenticação & Sincronização')).not.toBeVisible();
    await expect(loginButton).toBeFocused();
  });

  test('login em modo TV abre a aba de código do dispositivo', async ({ page }) => {
    await mockRpc(page);
    await page.goto('/');
    await page.getByTitle('Modo TV (F10)').click();
    await expect.poll(() => page.evaluate(() => document.documentElement.classList.contains('tv-mode'))).toBe(true);
    await page.getByTitle(/Entrar/).click();
    await expect(page.getByRole('button', { name: /Gerar Código para TV/ })).toBeVisible();
  });

  test('modo TV aplica layout leanback com alvos maiores e grade respirável', async ({ page }) => {
    await mockRpc(page);
    await page.goto('/?tv=1');
    await expect.poll(() => page.evaluate(() => document.documentElement.classList.contains('tv-mode'))).toBe(true);

    const sidebar = page.getByLabel('Navegação Lateral');
    await expect.poll(() => sidebar.evaluate((element) => element.getBoundingClientRect().width)).toBeLessThanOrEqual(76);
    await expect(page.locator('[data-component="sidebar-label"]').first()).toBeHidden();
    await expect(page.locator('[data-component="resource-usage-card"]')).toBeHidden();

    const cards = page.locator('[data-component="video-grid-cards"]');
    await expect(cards).toBeVisible();
    const cardAction = page.locator('[data-component="video-card-action"]').first();
    await expect.poll(() => cardAction.evaluate((element) => element.getBoundingClientRect().height)).toBeGreaterThanOrEqual(44);
  });

  test('cadeia Escape completa: fecha menu de contexto do card antes de navegar', async ({ page }) => {
    await mockRpc(page);
    await page.goto('/?tv=1');
    await expect.poll(() => page.evaluate(() => document.documentElement.classList.contains('tv-mode'))).toBe(true);

    const moreActions = page.getByRole('button', { name: /Mais ações para Vídeo TV 1/ });
    await expect(moreActions).toBeVisible();
    await moreActions.click();
    await expect(page.getByRole('menu', { name: 'Ações disponíveis' })).toBeVisible();

    await page.keyboard.press('Escape');
    await expect(page.getByRole('menu', { name: 'Ações disponíveis' })).not.toBeVisible();
    await expect(page.getByRole('group', { name: /Ações para Vídeo TV 1/ })).toBeVisible();
  });

  test('sugere 720p em hardware modesto e aplica ao clicar', async ({ page }) => {
    await page.addInitScript(() => {
      Object.defineProperty(navigator, 'deviceMemory', { value: 1, configurable: true });
      Object.defineProperty(navigator, 'hardwareConcurrency', { value: 1, configurable: true });
    });
    await page.route('**/api/rpc', async (route) => {
      const request = route.request().postDataJSON() as RpcRequest;
      const method = request.method || '';
      if (method === 'SaveSetting' && request.args?.[0] === 'playback.max_height') {
        await route.fulfill({ json: { result: null } });
        return;
      }
      const result =
        method === 'GetSettings'
          ? {}
          : method === 'GetAccount'
            ? null
            : method === 'GetHome'
              ? { for_you: [], topic_sections: [], continue_watching: [], recent_subscriptions: [], rediscovery: [] }
              : null;
      await route.fulfill({ json: { result } });
    });
    await page.goto('/?tv=1');
    await page.getByRole('button', { name: 'Ajustes' }).click();
    await page.getByRole('button', { name: 'Sistema' }).click();
    const applyButton = page.getByRole('button', { name: 'Aplicar 720p' });
    await expect(applyButton).toBeVisible();
    await applyButton.click();
    const select = page.locator('select').filter({ hasText: 'Automática' });
    await expect(select).toHaveValue('720');
  });
});
