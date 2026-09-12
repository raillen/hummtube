---
id: repository-layout
status: canonical
---

# Layout do Repositório — NanoTube Web

```text
nanotube-web/
├── ATLAS.md                 # Ponto de entrada canônico
├── AGENTS.md                # Diretrizes e regras agênticas
├── Taskfile.yml             # Comandos unificados de build/test/dev
├── README.md                # Apresentação do projeto
├── go.mod / go.sum          # Módulo Go
├── agents/                  # Definições dos agentes especialistas
├── skills/                  # Skills especializadas do projeto
├── cmd/
│   ├── nanotube-web/        # Entrypoint do cliente YouTube
│   ├── nanoiptv/            # Entrypoint IPTV/M3U/EPG
│   └── nanomusic/           # Entrypoint música/podcasts
├── internal/
│   ├── app/                 # Configuração e diretórios do app
│   ├── auth/                # OAuth2 PKCE e loopback handler
│   ├── backup/              # Snapshots e exportação JSON AES-256-GCM
│   ├── diagnostics/         # Diagnóstico de sistema, codecs e extratores
│   ├── domain/              # Modelos puros e interfaces/ports
│   ├── iptv/                # Parser M3U, XMLTV EPG e streams
│   ├── playback/            # PlaybackResolver (yt-dlp, POT, Invidious)
│   ├── product/             # Identidade e política RPC de cada produto
│   ├── launcher/            # Bootstrap Wails/web compartilhado
│   ├── search/              # Busca híbrida (FTS5 + yt-dlp)
│   ├── services/            # Serviços Wails exportados para o frontend
│   ├── storage/             # Repositório SQLite e migrações Goose
│   │   └── migrations/      # 00001 a 00021 SQL migrations
│   ├── suggestions/         # Recommendation Engine v2 (MMR)
│   └── sync/                # Sincronização incremental RSS / YouTube API
├── frontend/
│   ├── package.json         # Dependências do frontend
│   ├── vite.config.ts       # Configuração do Vite
│   ├── svelte.config.js     # Configuração do Svelte
│   ├── tailwind.config.js   # Configuração do Tailwind CSS
│   └── src/
│       ├── main.ts          # Entrypoint SPA
│       ├── App.svelte       # Componente raiz
│       ├── apps/            # Raízes e hooks específicos por produto
│       └── lib/
│           ├── components/  # Layout, Player, Views, Video Cards
│           ├── stores/      # Estado reativo
│           ├── types/       # Tipos TypeScript
│           └── navigation/  # Navegação espacial Modo TV
└── docs/                    # Documentação canônica organizada por domínio
```

O Vite resolve `$product-app` e `$product-player-hooks` conforme `NANOSUITE_APP`, produzindo `dist/nanotube`, `dist/nanoiptv` e `dist/nanomusic`.
