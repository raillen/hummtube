import { test, expect } from '@playwright/test';

test.describe('Biblioteca & Estatísticas', () => {
  test.beforeEach(async ({ page }) => {
    await page.goto('/');
    await page.locator('aside').getByRole('button', { name: 'Biblioteca' }).click();
  });

  test('deve exibir cabeçalho da biblioteca e abas', async ({ page }) => {
    await expect(page.getByRole('heading', { name: 'Biblioteca Local' })).toBeVisible();

    await expect(page.getByRole('button', { name: /^Histórico \(/i })).toBeVisible();
    await expect(page.getByRole('button', { name: /Favoritos/i })).toBeVisible();
    await expect(page.getByRole('button', { name: /Estatísticas Pessoais/i })).toBeVisible();
  });

  test('deve alternar entre as seções da biblioteca', async ({ page }) => {
    // 1. Favoritos
    await page.getByRole('button', { name: /Favoritos/i }).click();
    await page.waitForTimeout(200);

    // 2. Estatísticas Pessoais
    await page.getByRole('button', { name: /Estatísticas Pessoais/i }).click();
    await page.waitForTimeout(300);

    // 3. Voltar para Histórico
    await page.getByRole('button', { name: /^Histórico \(/i }).click();
  });

  test('confirmação destrutiva usa diálogo focável TV com cancelamento', async ({ page }) => {
    await page.route('**/api/rpc', async (route) => {
      const request = route.request().postDataJSON() as { method?: string };
      const method = request.method || '';
      const result = method === 'GetHistory' ? [{ id: 'v-1', title: 'V1', channel_id: 'c', channel_title: 'Canal', published_at: '2026-09-01T00:00:00Z', duration: 60_000_000_000, thumbnail_url: '', external_url: 'https://youtube.test/watch?v=v-1' }] : method === 'GetFavorites' ? [] : method === 'GetLocalStats' ? {} : method === 'GetSettings' ? {} : null;
      await route.fulfill({ json: { result } });
    });
    await page.goto('/');
    await page.locator('aside').getByRole('button', { name: 'Biblioteca' }).click();

    await page.getByRole('button', { name: /Limpar tudo/i }).click();
    const dialog = page.getByTestId('confirm-dialog');
    await expect(dialog).toBeVisible();
    await expect(page.getByTestId('confirm-dialog-confirm')).toBeFocused();
    await page.keyboard.press('Escape');
    await expect(dialog).not.toBeVisible();
  });
});
