import { defineConfig } from 'vite';
import { svelte } from '@sveltejs/vite-plugin-svelte';
import { resolve } from 'node:path';

const supportedTargets = ['hummtube', 'hummiptv', 'hummmusic', 'nanotube', 'nanoiptv', 'nanomusic'] as const;
const requestedTarget = process.env.NANOSUITE_APP || 'hummtube';
if (!supportedTargets.includes(requestedTarget as (typeof supportedTargets)[number])) {
  throw new Error(`NANOSUITE_APP inválido: ${requestedTarget}`);
}
const appTarget = requestedTarget as (typeof supportedTargets)[number];
const appEntrypoints = {
  hummtube: resolve(__dirname, 'src/App.svelte'),
  hummiptv: resolve(__dirname, 'src/apps/nanoiptv/App.svelte'),
  hummmusic: resolve(__dirname, 'src/apps/nanomusic/App.svelte'),
  nanotube: resolve(__dirname, 'src/App.svelte'),
  nanoiptv: resolve(__dirname, 'src/apps/nanoiptv/App.svelte'),
  nanomusic: resolve(__dirname, 'src/apps/nanomusic/App.svelte'),
};
const playerHookEntrypoints = {
  hummtube: resolve(__dirname, 'src/apps/nanotube/playerHooks.ts'),
  hummiptv: resolve(__dirname, 'src/apps/nanoiptv/playerHooks.ts'),
  hummmusic: resolve(__dirname, 'src/apps/nanomusic/playerHooks.ts'),
  nanotube: resolve(__dirname, 'src/apps/nanotube/playerHooks.ts'),
  nanoiptv: resolve(__dirname, 'src/apps/nanoiptv/playerHooks.ts'),
  nanomusic: resolve(__dirname, 'src/apps/nanomusic/playerHooks.ts'),
};

export default defineConfig({
  plugins: [svelte()],
  resolve: {
    alias: {
      '$product-app': appEntrypoints[appTarget],
      '$product-player-hooks': playerHookEntrypoints[appTarget],
    },
  },
  server: {
    host: '127.0.0.1',
    port: Number(process.env.WAILS_VITE_PORT) || 5173,
    strictPort: true,
    proxy: {
      '/api': {
        target: 'http://127.0.0.1:8999',
        changeOrigin: true
      }
    }
  },
  build: {
    target: 'esnext',
    outDir: `dist/${appTarget}`,
    // Preserva os marcadores que permitem ao go:embed compilar antes do
    // primeiro build de produção (por exemplo, em `task dev`).
    emptyOutDir: false,
    // HLS é lazy-loaded via import() dinâmico e fica fora do bundle inicial
    // (~185 kB gzip, baixado só em stream .m3u8). O aviso do Vite é ruído para
    // esse chunk opcional documentado; o orçamento inicial segue em PERFORMANCE_BUDGET.md.
    chunkSizeWarningLimit: 650,
  },
});
