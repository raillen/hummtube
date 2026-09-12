---
id: stack
status: canonical
---

# Stack Tecnológica — NanoTube Web

## Backend (Go)
- **Linguagem**: Go 1.22+
- **Framework Desktop**: Wails v3 (`github.com/wailsapp/wails/v3`)
- **Banco de Dados**: SQLite (`modernc.org/sqlite` pure-go driver) + FTS5
- **Migrações**: Goose (`github.com/pressly/goose/v3`)
- **Concorrência**: `golang.org/x/sync` (`errgroup`, `singleflight`)
- **Autenticação**: `golang.org/x/oauth2` + PKCE
- **API YouTube**: `google.golang.org/api/youtube/v3`
- **Armazenamento de Segredos**: `github.com/zalando/go-keyring`

## Frontend (Web / SPA)
- **Framework**: Svelte 5 / Svelte + Vite
- **Estilização**: Tailwind CSS v4 / v3 com CSS Variables
- **Ícones**: Lucide Svelte (`lucide-svelte`)
- **Player**: HTML5 Video API + `hls.js` para streaming adaptativo
- **Gerenciamento de Estado**: Svelte Runes & Stores reativos
- **Navegação TV**: Spatial Navigation Engine TypeScript pura

## Ferramentas de Build & Teste
- **Orquestrador**: Taskfile (`go-task`)
- **Testes Backend**: `go test`, `go vet`, `goleak`
- **Testes Frontend**: Vitest + Testing Library / Playwright
- **Linters**: `golangci-lint`, `svelte-check`, `eslint` / `prettier`
