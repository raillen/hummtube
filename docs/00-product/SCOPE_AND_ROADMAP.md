---
id: scope-and-roadmap
status: canonical
---

# Escopo e Roadmap — NanoTube Web

## Visão Geral das Releases

```text
┌──────────────┐    ┌──────────────┐    ┌──────────────┐    ┌──────────────┐
│    v0.1.0    │ ─► │    v0.2.0    │ ─► │    v0.3.0    │ ─► │    v1.0.0    │
│ Wails3 Core  │    │ Hardening    │    │ UX/TV/Contas │    │ Release Linux│
└──────────────┘    └──────────────┘    └──────────────┘    └──────────────┘
```

### v0.1.0 — Wails3 + Svelte Foundation (Atual)
- Migração completa do núcleo Go para bindings de serviço Wails v3;
- Frontend SPA em Svelte 4 + Tailwind CSS + Lucide Icons; Svelte 5 permanece a decisão-alvo e a migração integra o hardening v0.2.0;
- Player HTML5/HLS reativo com integração ao `PlaybackResolver` (yt-dlp explícito, Invidious);
- Catálogo local, busca instantânea SQLite FTS5, Home determinística com algoritmo MMR;
- Autenticação OAuth2 PKCE e Keyring seguro;
- Playlists locais (manuais e inteligentes), notas e bookmarks;
- Sistema de backup atômico e importação/exportação de dados pessoais.

### v0.2.0 — Hardening de Playback, Pesquisa e Consumo (Preview técnica)
- preview Debian amd64 gerada em `build/nanotube-web_0.1.0_amd64.deb`; instalação limpa e matriz de distribuições ainda não homologadas;
- resolução pública anônima e sessão autenticada por perfil somente sob consentimento;
- transporte same-origin com `Range`, headers protegidos, renovação e `singleflight`;
- runtime yt-dlp/PO Token versionado, diagnosticável e isolado de plugins arbitrários;
- qualidade consciente do hardware e modo de baixo consumo;
- Data API-first para contas conectadas, cancelamento e cache curto de pesquisa;
- cache real de thumbnails e build exclusivo do NanoTube;
- migração verificada para Svelte 5, alinhando implementação e D-004.

### v0.3.0 — Experiência, Contas e Modo TV (Planejado)
- catálogo tipado de comandos compartilhado por atalhos, menus contextuais e modo TV;
- `Ctrl+K`, histórico de navegação e persistência coerente dos atalhos;
- filtros reativos com cancelamento real e feedback de carregamento, atualização, vazio e erro;
- base de internacionalização em JSON, textos da interface orquestrados e erros conhecidos por código;
- seletor rápido de perfis/contas no header e sessão de navegador isolada por perfil;
- cards selecionáveis, gerenciamento de canais e menus contextuais consistentes;
- modo TV navegável por foco, D-Pad, teclado e gamepad, com memória de rota e hierarquia de retorno;
- importação nativa e unidirecional da configuração OAuth do aplicativo somente se a proposta de segurança e a ADR correspondente forem aprovadas.

### v1.0.0 — Release Linux Homologada (Planejado)
- pacotes nativos Debian/Ubuntu, Fedora e Arch com dependências corretas;
- Playback Runtime separado, SBOM, checksums, assinatura e rollback;
- OAuth de produção e documentação de privacidade/exclusão de dados;
- benchmarks no Celeron 1037U e matriz X11/Wayland;
- AppImage e Flatpak somente se passarem a matriz específica de compatibilidade.

## Plano canônico

O detalhamento em fatias verticais, dependências, gates, riscos e rollbacks está em `docs/03-implementation/NANOTUBE_RELEASE_HARDENING_PLAN.md`.

## Limites desta etapa (Non-Goals & In-Scope)

### In-Scope Capabilities
- Executável desktop autocontido NanoTube via Wails v3 + Go + Svelte;
- Resolução de streams de áudio/vídeo via `PlaybackResolver` (yt-dlp explícito, PO Token e Invidious fallback);
- Catálogo e histórico locais no SQLite com busca instantânea FTS5;
- Autenticação OAuth2 PKCE com armazenamento seguro no Keyring do SO;
- Modo TV acessível via teclado e controle direcional (Spatial Navigation);
- Exportação e importação de backups cifrados (AES-256-GCM).

### Non-Goals
- Não construir um navegador web completo de propósito geral;
- Não suportar DRM/Widevine proprietário ou engenharia reversa de SABR;
- Não incluir IPTV ou reprodutor musical de áudio dedicado dentro do binário do NanoTube (atribuídos a NanoIPTV e NanoMusic);
- Não permitir importação insegura de cookies brutos ou exposição de tokens em plain text.

### Compatibility Constraints
- Plataforma primária: Linux (X11 e Wayland em Debian/Ubuntu, Arch, Fedora);
- Hardware de referência mínimo: Processador dual-core (Intel Celeron 1037U ou equivalente) e 2 GB de memória RAM;
- Compatibilidade secundária: Windows 10/11 x64;
- Restrições de dependências: SQLite embutido (driver pure-go), sem CGO mandatório no backend principal.
