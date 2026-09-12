import { expect, test, type Page } from '@playwright/test';

interface ChannelScenario {
  subscriptionError?: string;
  subscriptionDelayMs?: number;
  recentSubscriptions?: Array<Record<string, unknown>>;
  channels?: Array<Record<string, unknown>>;
}

const subscribedVideo = {
  id: 'video-from-subscription',
  channel_id: 'subscribed-channel',
  title: 'Vídeo persistido da inscrição',
  published_at: '2026-08-28T12:00:00Z',
  duration: 600_000_000_000,
  thumbnail_url: ''
};

const subscribedChannel = {
  id: 'subscribed-channel',
  title: 'Canal Inscrito',
  subscribed: true
};

async function openSubscriptions(page: Page, scenario: ChannelScenario = {}) {
  await page.route('**/api/rpc', async (route) => {
    const request = route.request().postDataJSON() as { method?: string };
    const method = request.method || '';

    if (method === 'ListSubscriptionVideos' && scenario.subscriptionDelayMs) {
      await new Promise((resolve) => setTimeout(resolve, scenario.subscriptionDelayMs));
    }
    if (method === 'ListSubscriptionVideos' && scenario.subscriptionError) {
      await route.fulfill({
        contentType: 'application/json',
        body: JSON.stringify({ error: scenario.subscriptionError })
      });
      return;
    }

    const results: Record<string, unknown> = {
      ListSubscriptionVideos: {
        videos: scenario.recentSubscriptions ?? [{ ...subscribedVideo, channel_title: subscribedChannel.title }],
        categories: [],
        total: (scenario.recentSubscriptions ?? [subscribedVideo]).length,
        has_more: false
      },
      ListManagedChannels: (scenario.channels ?? [subscribedChannel]).map((channel) => ({ channel, tags: [] })),
      GetChannelFolders: [],
      GetFavoriteChannelIDs: {},
      GetFolderMembership: {},
      GetSettings: {},
      GetAccount: null,
      GetDiagnostics: { errors: [], warnings: [] },
      ListIPTVSources: []
    };

    await route.fulfill({
      contentType: 'application/json',
      body: JSON.stringify({ result: results[method] ?? null })
    });
  });

  await page.goto('/');
  await page.locator('aside').getByRole('button', { name: 'Inscrições' }).click();
}

test.describe('Inscrições', () => {
  test('exibe os vídeos persistidos dos canais inscritos', async ({ page }) => {
    await openSubscriptions(page);

    await expect(page.getByRole('heading', { name: 'Vídeos das inscrições', exact: true })).toBeVisible();
    await expect(page.getByRole('heading', { name: '1 vídeos encontrados' })).toBeVisible();
    await expect(page.getByRole('button', { name: /Reproduzir Vídeo persistido da inscrição de Canal Inscrito/i })).toBeVisible();
  });

  test('mantém o gerenciamento de canais e pastas acessível', async ({ page }) => {
    await openSubscriptions(page);
    await page.getByRole('button', { name: /Gerenciar canais/ }).click();

    await expect(page.getByRole('heading', { name: 'Gerenciar canais' })).toBeAttached();
    await expect(page.locator('main').getByRole('button', { name: /Nova pasta/i })).toBeVisible();
    await expect(page.locator('main').getByRole('button', { name: /Adicionar canal/i }).first()).toBeVisible();
    await expect(page.getByText('Canal Inscrito', { exact: true })).toBeVisible();
  });

  test('abre e fecha o modal de nova pasta de canais', async ({ page }) => {
    await openSubscriptions(page);
    await page.getByRole('button', { name: /Gerenciar canais/ }).click();
    await page.getByRole('button', { name: 'Nova pasta' }).click();

    await expect(page.getByRole('heading', { name: 'Criar pasta de canais' })).toBeVisible();
    await expect(page.getByPlaceholder(/Nome da pasta/i)).toBeVisible();
    await page.getByRole('button', { name: 'Cancelar' }).click();
    await expect(page.getByRole('heading', { name: 'Criar pasta de canais' })).not.toBeVisible();
  });

  test('explica o estado vazio quando o canal ainda não tem vídeos sincronizados', async ({ page }) => {
    await openSubscriptions(page, { recentSubscriptions: [] });

    await expect(page.getByRole('heading', { name: 'Nenhum vídeo sincronizado' })).toBeVisible();
    await expect(page.getByRole('button', { name: 'Atualizar feeds', exact: true })).toBeVisible();
  });

  test('mostra loading e erro recuperável do backend', async ({ page }) => {
    await openSubscriptions(page, { subscriptionDelayMs: 1_000, subscriptionError: 'SQLite temporariamente indisponível' });

    await expect(page.getByRole('status')).toContainText('Carregando vídeos');
    await expect(page.getByRole('alert')).toContainText('SQLite temporariamente indisponível');
    await expect(page.getByRole('button', { name: 'Tentar novamente' })).toBeVisible();
  });
});
