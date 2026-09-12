import { test, expect } from '@playwright/test';

test.describe('Aplicativo NanoIPTV', () => {
  test.beforeEach(async ({ page }) => {
    page.on('console', msg => console.log('PAGE LOG:', msg.text()));
    page.on('pageerror', err => console.log('PAGE ERROR:', err));
    await page.goto('/');
  });

  test('deve exibir cabeçalho do HummIPTV e abas de categorias', async ({ page }) => {
    await expect(page.getByRole('heading', { name: 'HummIPTV' })).toBeVisible();

    // Abas de categorias
    await expect(page.getByRole('button', { name: /TV Ao Vivo/i })).toBeVisible();
    await expect(page.getByRole('button', { name: /Filmes VOD/i })).toBeVisible();
    await expect(page.getByRole('button', { name: /Séries VOD/i })).toBeVisible();
    await expect(page.getByRole('button', { name: /Favoritos/i })).toBeVisible();
    await expect(page.getByRole('button', { name: /Guia EPG/i })).toBeVisible();
    await expect(page.getByRole('button', { name: /Continuar Assistindo/i })).toBeVisible();
  });

  test('deve alternar entre as abas do NanoIPTV', async ({ page }) => {
    // 1. Filmes VOD
    await page.getByRole('button', { name: /Filmes VOD/i }).click();
    await page.waitForTimeout(200);

    // 2. Séries VOD
    await page.getByRole('button', { name: /Séries VOD/i }).click();
    await page.waitForTimeout(200);

    // 3. Guia EPG
    await page.getByRole('button', { name: /Guia EPG/i }).click();
    await page.waitForTimeout(200);

    // 4. Favoritos
    await page.getByRole('button', { name: /Favoritos/i }).click();
    await page.waitForTimeout(200);

    // 5. Continuar Assistindo
    await page.getByRole('button', { name: /Continuar Assistindo/i }).click();
    await page.waitForTimeout(200);

    // 6. Voltar para TV Ao Vivo
    await page.getByRole('button', { name: /TV Ao Vivo/i }).click();
  });

  test('deve abrir e fechar o modal de gerenciamento de listas IPTV', async ({ page }) => {
    const manageBtn = page.getByRole('button', { name: /Listas IPTV/i });
    await expect(manageBtn).toBeVisible();
    await page.waitForTimeout(300);
    await manageBtn.click();

    await expect(page.getByRole('heading', { name: 'Gerenciar Fontes IPTV' })).toBeVisible();

    // Botão de fechar modal
    await page.getByRole('button', { name: 'Fechar', exact: true }).click();
    await expect(page.getByRole('heading', { name: 'Gerenciar Fontes IPTV' })).not.toBeVisible();
  });
});
