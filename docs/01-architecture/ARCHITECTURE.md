---
id: architecture
status: canonical
---

# Arquitetura — NanoTube Web

## Estilo Arquitetural

Monorepo transitório de aplicações desktop modulares e reativas, estruturado em **Clean Architecture / Ports and Adapters**. NanoTube, NanoIPTV e NanoMusic usam entrypoints e superfícies públicas independentes sobre bibliotecas Go/Svelte compartilhadas.

```text
┌─────────────────────────────────────────────────────────────┐
│ Svelte 5 Frontend SPA (Vite + Tailwind CSS + Lucide Icons)  │
│ Bundles: NanoTube • NanoIPTV • NanoMusic                    │
│ Components: VideoCard • Player • Queue • Nav • Modais       │
│ Stores: playerState, activeQueue, settings, theme, tvMode   │
└──────────────────────────────┬──────────────────────────────┘
                               │ Wails v3 IPC / Go Bindings
┌──────────────────────────────▼──────────────────────────────┐
│ Wails v3 Service Bridge (Go)                                │
│ ├─ CatalogService       ├─ SettingsService                  │
│ ├─ PlayerService        ├─ BackupService                    │
│ ├─ SearchService        ├─ IPTVService (somente NanoIPTV)   │
│ └─ PlaylistService      └─ DiagnosticsService               │
└───────┬──────────────┬──────────────┬───────────────┬───────┘
        │              │              │               │
        ▼              ▼              ▼               ▼
 Provider Ports   Local Storage  Player / Stream  IPTV Core
        │              │              │               │
 YouTube Data v3  SQLite + FTS5   Cascading      M3U / XMLTV
 OAuth2 / PKCE    Goose Migr.     PlaybackResolver EPG Sync
 Keyring Secrets  Disk Cache      (yt-dlp, POT,   Keyring Creds
                                   Invidious)
```

---

## Camadas do Sistema

### 0. Limite de produto

- `internal/product.Spec` define identidade, dados, Keyring, bundle e allowlist RPC;
- o handler rejeita pares `service.method` desconhecidos ou fora da allowlist antes do dispatch;
- o runtime compartilhado não significa estado compartilhado: cada processo abre apenas seu banco próprio;
- NanoTube não carrega views de IPTV/Música; NanoIPTV e NanoMusic têm raízes Svelte próprias;
- ADR-022 registra migração inicial e estratégia de extração futura.

### 1. Frontend (Svelte 5 + Tailwind CSS + Lucide Icons)
- **Declarativo e Reativo**: Componentes Svelte tipados com TypeScript.
- **Design System Leve**: Tailwind CSS com variáveis de tema para suporte nativo a temas escuro, claro e Modo TV.
- **Ícones**: Lucide Icons integrados via `lucide-svelte`.
- **Navegação Espacial**: Módulo de navegação 2D para controle remoto / teclado no Modo TV.

### 2. Service Bridge (Wails v3)
- Conecta o frontend ao backend via chamadas tipadas assíncronas geradas automaticamente ou IPC estruturado.
- Gerencia eventos do ciclo de vida da janela, menus nativos, atalhos globais e bandeja do sistema (*System Tray*).

### 3. Application Services & Core Domain (Go)
- **Catalog & Sync**: Sincronização incremental com RSS e YouTube Data API v3.
- **Recommendation Engine v2**: Algoritmo MMR (Maximal Marginal Relevance) guloso com saturação de canal e afinidades temáticas.
- **Search Service**: Busca híbrida (SQLite FTS5 instantâneo + busca remota sob demanda).
- **Playback Resolver**: `CascadingResolver` orquestrando resolução de streams via yt-dlp explícito, attestation com PO Token Provider e Invidious fallback.
- **Storage & Migrations**: Repositório SQLite com migrações versionadas via Goose, sem ORM pesado.
- **Auth & Keyring**: OAuth2 PKCE e armazenamento de segredos no Keyring do SO.
- **Backup & Portabilidade**: Exportação atômica e snapshots criptografados via AES-256-GCM.

---

## Dependency Direction & System Boundaries
- O fluxo de dependência aponta rigorosamente para dentro (Clean Architecture / Ports & Adapters): Domain Entities -> Application Use Cases -> Infrastructure Adapters;
- Camadas externas (Wails v3 IPC, SQLite, chamadas de rede HTTP, subprocesso yt-dlp) são adaptadores periféricos descartáveis e testáveis via mocks;
- O estado canônico do aplicativo reside exclusivamente no banco de dados SQLite local correspondente (`nanotube-web.db`).

## External Integrations
- YouTube Data API v3 (cliente oficial via OAuth2 PKCE para catálogo, pesquisa e inscrições);
- Runtime yt-dlp e PO Token (Proof of Origin) Attestation Providers para extração de URLs de mídia;
- Instâncias Invidious como fallback automático cascading com validação estrita anti-SSRF;
- System Keyring via SecretStore (`go-keyring`) para custódia de tokens sensíveis.
