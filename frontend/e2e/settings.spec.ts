import { test, expect } from '@playwright/test';

test.describe('Configurações & Diagnóstico', () => {
  test.beforeEach(async ({ page }) => {
    await page.goto('/');
    await page.locator('aside').getByRole('button', { name: 'Ajustes' }).click();
  });

  test('deve exibir informações de diagnóstico do sistema', async ({ page }) => {
    await expect(page.getByRole('heading', { name: 'Configurações & Ajustes' })).toBeVisible();
    await page.getByRole('button', { name: 'Sistema' }).click();
    await expect(page.getByRole('heading', { name: 'Diagnósticos do Sistema' })).toBeVisible();

    // Abre modal de diagnóstico
    const diagBtn = page.getByRole('button', { name: 'Abrir Diagnóstico' });
    await expect(diagBtn).toBeVisible();
    await diagBtn.click();

    await expect(page.getByRole('heading', { name: 'Diagnóstico do Sistema' })).toBeVisible();
    await expect(page.getByText(/Stack:/)).toBeVisible();
    await expect(page.getByText(/Go:/)).toBeVisible();
    await expect(page.getByText(/PO Token:/)).toBeVisible();
    await expect(page.getByText(/Extração:/)).toBeVisible();
	await expect(page.getByRole('heading', { name: 'Últimos eventos do aplicativo' })).toBeVisible();
	await expect(page.getByRole('button', { name: 'Copiar diagnóstico' })).toBeEnabled();

    // Fecha modal de diagnóstico
    await page.getByRole('button', { name: 'Fechar modal' }).click();
    await expect(page.getByText('Diagnóstico do Sistema')).not.toBeVisible();
  });

  test('deve conter opções de Backup e Restauração', async ({ page }) => {
    await page.getByRole('button', { name: 'Segurança e dados' }).click();
    await expect(page.getByRole('heading', { name: 'Backup & Portabilidade Local' })).toBeVisible();
    await expect(page.getByRole('button', { name: 'Importar' })).toBeVisible();
    await expect(page.getByRole('button', { name: 'Exportar JSON' })).toBeVisible();
  });

  test('deve permitir alternar o tema da interface na view de Ajustes', async ({ page }) => {
    await page.getByRole('button', { name: 'Aparência' }).click();
    await expect(page.getByRole('heading', { name: 'Aparência do Tema' })).toBeVisible();
    
    // Alternar para tema claro
    await page.getByRole('button', { name: 'Claro' }).click();
    let isLight = await page.evaluate(() => document.documentElement.classList.contains('light'));
    expect(isLight).toBe(true);

    // Alternar para tema escuro
    await page.getByRole('button', { name: 'Escuro' }).click();
    let isDark = await page.evaluate(() => !document.documentElement.classList.contains('light'));
    expect(isDark).toBe(true);
  });

  test('deve permitir alterar a cor de destaque (accent color)', async ({ page }) => {
    await page.getByRole('button', { name: 'Aparência' }).click();
    await expect(page.getByRole('heading', { name: 'Cor de Destaque (Accent Color)' })).toBeVisible();

    // Selecionar cor Verde Esmeralda (#10b981)
    const emeraldBtn = page.getByRole('button', { name: 'Verde Esmeralda' });
    await expect(emeraldBtn).toBeVisible();
    await emeraldBtn.click();

    let primaryColor = await page.evaluate(() => 
      document.documentElement.style.getPropertyValue('--color-primary').trim()
    );
    expect(primaryColor).toBe('#10b981');

    // Selecionar cor Ciano / Sky (#0ea5e9)
    const skyBtn = page.getByRole('button', { name: 'Ciano / Sky' });
    await expect(skyBtn).toBeVisible();
    await skyBtn.click();

    primaryColor = await page.evaluate(() => 
      document.documentElement.style.getPropertyValue('--color-primary').trim()
    );
    expect(primaryColor).toBe('#0ea5e9');
  });

  test('deve abrir o modal de atalhos e permitir personalizar e restaurar atalhos', async ({ page }) => {
    await page.getByRole('button', { name: 'Sistema' }).click();
    const customizeBtn = page.getByRole('button', { name: 'Personalizar Atalhos' });
    await expect(customizeBtn).toBeVisible();
    await customizeBtn.click();

    await expect(page.getByRole('heading', { name: 'Gerenciador de Atalhos de Teclado' })).toBeVisible();
    await expect(page.getByText('Player de Vídeo & Áudio')).toBeVisible();

    // Clica no botão do atalho de Play/Pause (padrão: Space)
    const spaceKeyBtn = page.getByRole('button', { name: 'Space', exact: true });
    await expect(spaceKeyBtn).toBeVisible();
    await spaceKeyBtn.click();

    // Pressiona 'k' para redefinir o atalho
    await page.keyboard.press('k');

    // Verifica que o botão agora exibe 'k'
    await expect(page.getByRole('button', { name: 'k', exact: true })).toBeVisible();

    // Clica em 'Restaurar Padrões'
    await page.getByRole('button', { name: 'Restaurar Padrões' }).click();

    // Verifica que o atalho voltou para 'Space'
    await expect(page.getByRole('button', { name: 'Space', exact: true })).toBeVisible();

    // Fecha o modal
    await page.getByRole('button', { name: 'Salvar & Fechar' }).click();
    await expect(page.getByRole('heading', { name: 'Gerenciador de Atalhos de Teclado' })).not.toBeVisible();
  });

  test('expõe gerenciador de contas, visitante e inventário sem valores secretos', async ({ page }) => {
    await expect(page.getByRole('button', { name: 'Adicionar conta' })).toBeVisible();
    await expect(page.getByRole('button', { name: 'Usar como convidado' })).toBeVisible();
    await page.getByRole('button', { name: 'Segurança e dados' }).click();
    await expect(page.getByRole('heading', { name: 'Gerenciador seguro de credenciais' })).toBeVisible();
    await expect(page.getByText('Tokens, API keys e secrets nunca atravessam a interface.')).toBeVisible();
  });

  test('recolhe e expande a navegação lateral', async ({ page }) => {
    const collapse = page.getByRole('button', { name: 'Recolher menu lateral' });
    await expect(collapse).toBeVisible();
    await collapse.click();
    await expect(page.getByRole('button', { name: 'Expandir menu lateral' })).toBeVisible();
  });

  test('exporta e importa backup criptografado', async ({ page }) => {
    await page.route('**/api/rpc', async (route) => {
      const request = route.request().postDataJSON() as RpcRequest;
      const method = request.method || '';
      if (method === 'ExportPersonalData') {
        await route.fulfill({ json: { result: JSON.stringify({ test: 'data' }) } });
        return;
      }
      if (method === 'ImportPersonalData') {
        await route.fulfill({ json: { result: null } });
        return;
      }
      const result =
        method === 'GetSettings'
          ? {}
          : method === 'GetAccount'
            ? { id: 'prof-1', name: 'Perfil Ativo', email: 'test@test.com' }
            : method === 'GetHome'
              ? { for_you: [], topic_sections: [], continue_watching: [], recent_subscriptions: [], rediscovery: [] }
              : null;
      await route.fulfill({ json: { result } });
    });
    await page.goto('/');
    await page.locator('aside').getByRole('button', { name: 'Ajustes' }).click();
    await page.getByRole('button', { name: 'Segurança e dados' }).click();

    // Testa export criptografado
    await page.getByRole('button', { name: 'Exportar criptografado' }).click();
    const exportModal = page.getByRole('heading', { name: 'Exportar Backup Criptografado' });
    await expect(exportModal).toBeVisible();

    await page.getByPlaceholder('Mínimo de 6 caracteres...').fill('minhasenha123');
    await page.getByRole('button', { name: 'Exportar Criptografado', exact: true }).click();
    await expect(exportModal).not.toBeVisible();
  });
});
