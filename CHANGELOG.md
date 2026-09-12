# Changelog

Todas as alterações notáveis deste projeto são documentadas neste arquivo.
O formato baseia-se no [Keep a Changelog](https://keepachangelog.com/pt-BR/1.0.0/) e adere ao [Semantic Versioning](https://semver.org/lang/pt-BR/).

## [0.1.0] - 2026-09-11

### Adicionado
- Rebranding oficial do ecossistema para **HummTube** e **HummSuite** (**HummTube**, **HummIPTV** e **HummMusic**):
  - Novos pontos de entrada: `cmd/hummtube`, `cmd/hummiptv` e `cmd/hummmusic`, mantendo aliases e binários legados (`nanotube-web`, `nanoiptv`, `nanomusic`) para total compatibilidade.
  - Migração de dados transparente e resiliente: cópia automática de bancos SQLite legados (`nanotube-web.db` -> `hummtube.db`) e migração segura de credenciais do Keyring no primeiro boot.
  - Identidade visual atualizada em todo o frontend (Header, Onboarding, Player, Modais e Apps HummIPTV/HummMusic).
  - Atualização da versão base do projeto para `0.1.0` (backend Go, frontend Vite/Svelte, packaging e Taskfile).
- Inicialização da estrutura canônica do Prumo v0.5.
- Configuração do manifesto `prumo.json` e orquestração `.ai/`.
- Definição da hierarquia de documentação canônica em `docs/`.
- Contrato estrito de arquitetura em `docs/architecture/clean-code-contract.md`.
- Estratégia de testes exaustivos em `docs/development/testing-strategy.md` (unitários, integração, conformidade, segurança SAST/secrets, performance/stress, UI).
- Política de documentação mandatória com `README.md` explicativo em cada diretório do projeto.

### Corrigido
- Eliminação de tremulação/flicker visual na interface com o miniplayer aberto:
  - Criação de derived stores (`playerQueue`, `isMiniplayer`, `miniplayerPosition`, `hasActiveVideo`) em `playerStore.ts` para isolar reatividade de alta frequência (`timeupdate`) do estado estrutural do player.
  - Desacoplamento de `VideoCard.svelte` e `Header.svelte` do objeto monolítico `$playerStore`, evitando re-renderização em cascata de todos os cards da página a cada segundo de reprodução.
  - Remoção de classes Tailwind de animação de entrada com keyframes persistentes (`animate-in slide-in-from-bottom-5`) dos contêineres do miniplayer em `App.svelte` e `PlayerSurface.svelte`, aplicando `isolate transition-all duration-200` para transição suave de posicionamento entre cantos e isolamento de composição GPU.
  - Proteção contra micro-jitter e re-emissões redundantes em `setTime` e `setDuration`.
- Ajuste de empilhamento (z-index) e pointer events entre `LoadingOverlay`, `PlayerControls` e botão de retorno para permitir interatividade sem bloqueio nos testes E2E do Playwright.
- Remoção do atributo `role="status"` duplicado em `LoadingOverlay` para resolver conflito com o modal de DSP de áudio.
- Limpeza de flags de carregamento de stream (`isSwitchingStream`) nos eventos `canplay`, `playing` e `abort`.

