# HummTube / HummSuite Workspace

> Cliente YouTube ultra-leve, moderno e independente para Linux/Desktop, construído com **Wails v3**, **Svelte**, **Tailwind CSS**, **Lucide Icons**, **Go** e **SQLite**.

Este monorepo também contém os executáveis independentes **HummIPTV** e **HummMusic** (anteriormente NanoIPTV e NanoMusic), preparados para futura extração em repositórios próprios. Eles compartilham bibliotecas, não bancos, Keyrings ou superfícies RPC.

---

## 🎯 Visão do Projeto

O **HummTube** (anteriormente NanoTube Web) é a evolução web-native moderna do cliente desktop YouTube. Mantém um núcleo robusto, determinístico e eficiente em **Go** com **SQLite FTS5**, oferecendo uma interface reativa, elegante e moderna em **Svelte + Tailwind CSS + Lucide Icons** embutida através do **Wails v3**.

### 🌟 Destaques da Stack

- **Backend**: Go (Wails v3, `modernc.org/sqlite`, Goose migrations, `golang.org/x/sync`, `golang.org/x/oauth2`).
- **Frontend**: Svelte (Vite SPA) + Tailwind CSS + Lucide Svelte (`lucide-svelte`).
- **Player**: Player HTML5/HLS moderno com aceleração de hardware nativa no WebView / backend streaming proxy e integração com `PlaybackResolver` (yt-dlp explícito, POT Provider, Invidious fallback).
- **Armazenamento**: SQLite local com migrações versionadas (Goose), busca textual full-text (FTS5), playlists manuais e inteligentes, histórico, notas e bookmarks.
- **Privacidade & Controle**: Sem anúncios, sem algoritmos opacos, feed local próprio ("Para Você"), ranking explicável e offline-first.
- **10-Foot UI (Modo TV)**: Interface adaptada para grandes telas e navegação espacial direcional por controle remoto / teclado.

---

## 🚀 Como Iniciar

### Pré-requisitos
- Go 1.22+
- Node.js 18+ e npm
- Wails v3 CLI (`go install github.com/wailsapp/wails/v3/cmd/wails3@latest`)
- `yt-dlp` instalado no sistema (opcionalmente Deno ou Node para desafios JS / POT)

### Executar em Desenvolvimento
```bash
# O Wails v3 inicia o Vite/HMR, observa o Go e reinicia o backend
task dev

# Aplicativos separados da HummSuite
task dev:hummiptv  # ou task dev:nanoiptv
task dev:hummmusic # ou task dev:nanomusic

# Porta Vite alternativa
task dev VITE_PORT=9245
```

### Executar Testes & Verificação
```bash
task verify
```

### Gerar Pacotes / Build de Produção
```bash
task build # gera bin/hummtube, bin/hummiptv e bin/hummmusic (com symlinks/cópias de compatibilidade)
```

---

## 📚 Documentação

- [`ATLAS.md`](./ATLAS.md): Ponto de entrada canônico da arquitetura e documentação.
- [`AGENTS.md`](./AGENTS.md): Regras de desenvolvimento, fluxo agêntico e diretrizes de engenharia.
- [`docs/`](./docs/): Documentação canônica dividida por domínios de produto, arquitetura, UI/UX, implementação, qualidade, segurança e governança.
