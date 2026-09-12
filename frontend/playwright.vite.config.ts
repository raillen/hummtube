import { defineConfig, devices } from '@playwright/test';

// Suíte focada no frontend atual. O Vite deve estar ativo em 5173; as rotas
// RPC e de mídia são controladas pelos próprios testes, sem iniciar os três
// binários da NanoSuite definidos no gate E2E completo.
export default defineConfig({
  testDir: './e2e',
  fullyParallel: true,
  reporter: 'list',
  use: {
    baseURL: 'http://127.0.0.1:5173',
    trace: 'on-first-retry',
    screenshot: 'only-on-failure',
  },
  projects: [
    {
      name: 'nanotube-vite',
      use: { ...devices['Desktop Chrome'] },
    },
  ],
});
