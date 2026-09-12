import { expect, test } from '@playwright/test';

test.describe('Aplicativo NanoMusic', () => {
  test.beforeEach(async ({ page }) => {
    await page.goto('/');
  });

  test('oferece navegação musical sem superfícies de IPTV ou inscrições', async ({ page }) => {
    await expect(page.getByText('HummMusic', { exact: true })).toBeVisible();
    await expect(page.getByRole('heading', { name: 'Modo Música' })).toBeVisible();
    await expect(page.getByRole('button', { name: /Fila/ })).toBeVisible();
    await expect(page.getByRole('button', { name: /Playlists/ })).toBeVisible();
    await expect(page.getByRole('button', { name: /Ajustes/ })).toBeVisible();
    await expect(page.getByText('HummIPTV')).toHaveCount(0);
    await expect(page.getByText('NanoIPTV')).toHaveCount(0);
    await expect(page.getByRole('button', { name: 'Inscrições' })).toHaveCount(0);
  });

  test('abre fila e ajustes próprios', async ({ page }) => {
    await page.getByRole('button', { name: /Fila/ }).click();
    await expect(page.getByRole('heading', { name: 'Fila de reprodução' })).toBeVisible();
    await page.getByRole('button', { name: /Ajustes/ }).click();
    await expect(page.getByRole('heading', { name: 'Configurações & Ajustes' })).toBeVisible();
    await page.getByRole('button', { name: 'Segurança e dados' }).click();
    await expect(page.getByRole('heading', { name: 'Last.fm' })).toBeVisible();
  });
});
