import { test, expect } from '@playwright/test';

test.describe('Modal de Autenticação & Modos de Login', () => {
  test.beforeEach(async ({ page }) => {
    await page.route('**/api/rpc', async (route) => {
      const request = route.request().postDataJSON() as { method?: string };
      if (request.method === 'GetAccount') {
        await route.fulfill({ contentType: 'application/json', body: JSON.stringify({ result: null }) });
        return;
      }
      await route.continue();
    });
    await page.goto('/');
  });

  test('deve abrir o modal de autenticação ao clicar em Entrar', async ({ page }) => {
    const loginBtn = page.getByTitle(/Entrar/);
    if (await loginBtn.isVisible()) {
      await loginBtn.click();
      await expect(page.getByText('Autenticação & Sincronização')).toBeVisible();

      // Verifica presença das 4 abas
      await expect(page.getByRole('button', { name: 'Google OAuth' })).toBeVisible();
      await expect(page.getByRole('button', { name: 'Modo TV Code' })).toBeVisible();
      await expect(page.getByRole('button', { name: 'Importar (Sem Login)' })).toBeVisible();
      await expect(page.getByRole('button', { name: 'Cookies' })).toBeVisible();
    }
  });

  test('deve navegar pelas abas do modal de autenticação', async ({ page }) => {
    const loginBtn = page.getByTitle(/Entrar/);
    if (await loginBtn.isVisible()) {
      await loginBtn.click();

      // 1. Aba Google OAuth
      await page.getByRole('button', { name: 'Google OAuth' }).click();
      await expect(page.getByRole('button', { name: /Entrar com o Google/ })).toBeVisible();

      // 2. Aba Modo TV Code
      await page.getByRole('button', { name: 'Modo TV Code' }).click();
      await expect(page.getByRole('button', { name: /Gerar Código para TV/ })).toBeVisible();

      // 3. Aba Importar (Sem Login)
      await page.getByRole('button', { name: 'Importar (Sem Login)' }).click();
      await expect(page.getByRole('button', { name: /Selecionar Arquivo de Inscrições/ })).toBeVisible();

      // 4. Aba Cookies
      await page.getByRole('button', { name: 'Cookies' }).click();
      await expect(page.locator('#browser-cookies-select')).toBeVisible();
      await expect(page.locator('#browser-cookie-keyring-select')).toBeVisible();
      await expect(page.getByRole('button', { name: /Salvar Configuração de Cookies/ })).toBeVisible();

      // Fechar modal
      await page.getByRole('button', { name: 'Fechar modal' }).click();
      await expect(page.getByText('Autenticação & Sincronização')).not.toBeVisible();
    }
  });

  test('deve restaurar e salvar navegador com o keyring selecionado', async ({ page }) => {
    let savedCookieSource = '';
    await page.route('**/api/rpc', async (route) => {
      const request = route.request().postDataJSON() as { method?: string; args?: unknown[] };
      if (request.method === 'GetSettings') {
        await route.fulfill({
          contentType: 'application/json',
          body: JSON.stringify({ result: { 'playback.cookies_browser': 'brave+gnomekeyring' } }),
        });
        return;
      }
      if (request.method === 'SetBrowserCookies') {
        savedCookieSource = String(request.args?.[0] || '');
        await route.fulfill({ contentType: 'application/json', body: JSON.stringify({ result: null }) });
        return;
      }
      await route.continue();
    });

    await page.getByTitle(/Entrar/).click();
    await page.getByRole('button', { name: 'Cookies' }).click();
    await expect(page.locator('#browser-cookies-select')).toHaveValue('brave');
    await expect(page.locator('#browser-cookie-keyring-select')).toHaveValue('gnomekeyring');
    await page.locator('#browser-cookie-keyring-select').selectOption('kwallet6');
    await page.getByRole('button', { name: /Salvar Configuração de Cookies/ }).click();
    expect(savedCookieSource).toBe('brave+kwallet6');
  });
});
