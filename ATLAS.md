# ATLAS — HummTube Web

> Ponto de entrada canônico para humanos, agentes e LLMs no monorepo transitório da **HummSuite** (anteriormente NanoSuite).

## 1. Objetivo

HummTube é um cliente desktop YouTube ultra-leve, modular e reativo construído com **Wails v3 + Svelte + Tailwind CSS + Lucide Icons + Go + SQLite**.
Ele preserva o motor desacoplado de dados, catálogo e resolução de mídia, fornecendo uma interface rica, acessível e personalizável sem a sobrecarga de um navegador completo tradicional ou Electron.

O mesmo workspace hospeda **HummIPTV** e **HummMusic** como aplicativos executáveis independentes, conforme D-033 e D-035. Cada produto possui bundle, allowlist RPC, SQLite e Keyring próprios e está preparado para futura extração em repositório separado (com compatibilidade e migração transparente dos legados NanoTube, NanoIPTV e NanoMusic).

## 2. Documentos canônicos por pergunta

| Pergunta | Documento |
|---|---|
| O que é o produto? | `docs/00-product/PRODUCT.md` |
| O que entra em cada release? | `docs/00-product/SCOPE_AND_ROADMAP.md` |
| Plano da suíte (hub TV + apps)? | `docs/00-product/NANOSUITE_PLAN.md` |
| Requisitos obrigatórios? | `docs/00-product/REQUIREMENTS.md` |
| Arquitetura geral? | `docs/01-architecture/ARCHITECTURE.md` |
| Fluxos de dados? | `docs/01-architecture/DATA_FLOWS.md` |
| Contratos/ports & Wails bindings? | `docs/01-architecture/CONTRACTS.md` |
| UI/UX & Design System? | `docs/02-ui-ux/UI_UX.md` |
| Telas/wireframes e navegação? | `docs/02-ui-ux/FLOWS_AND_WIREFRAMES.md` |
| Acessibilidade & Modo TV? | `docs/02-ui-ux/ACCESSIBILITY.md` |
| Stack técnica completa? | `docs/03-implementation/STACK.md` |
| Layout do repositório? | `docs/03-implementation/REPOSITORY_LAYOUT.md` |
| Como construir a UI em Svelte? | `docs/03-implementation/UI_IMPLEMENTATION.md` |
| Como implementar o Player Web/Wails? | `docs/03-implementation/PLAYER.md` |
| Como resolver playback? | `docs/03-implementation/PLAYBACK_BACKENDS.md` |
| Modernização de playback / PO Token? | `docs/03-implementation/YOUTUBE_PLAYBACK_MODERNIZATION.md` |
| Qual é o plano de hardening, experiência, performance e distribuição do NanoTube? | `docs/03-implementation/NANOTUBE_RELEASE_HARDENING_PLAN.md` |
| YouTube/OAuth & PKCE? | `docs/03-implementation/YOUTUBE_AND_AUTH.md` |
| Concorrência e ciclo de vida Wails? | `docs/03-implementation/CONCURRENCY.md` |
| Refresh do catálogo local? | `docs/03-implementation/REFRESH_SERVICE.md` |
| Recomendações v2 (MMR & afinidades)? | `docs/03-implementation/RECOMMENDATIONS.md` |
| Thumbnails e cache eficiente? | `docs/03-implementation/THUMBNAILS_AND_CACHE.md` |
| SQLite/FTS5/Goose migrations? | `docs/03-implementation/STORAGE.md` |
| Pesquisa instantânea local e remota? | `docs/03-implementation/SEARCH.md` |
| Limites entre NanoTube, NanoIPTV e NanoMusic? | `docs/00-product/NANOSUITE_PLAN.md` e `docs/01-architecture/adrs/ADR-022-nanosuite-product-separation.md` |
| NanoIPTV no Wails? | `docs/03-implementation/NANOIPTV.md` |
| Build, dev tooling e Tarefas? | `docs/03-implementation/BUILD_AND_TOOLING.md` |
| Política de dependências? | `docs/03-implementation/DEPENDENCY_POLICY.md` |
| Performance budgets & medição? | `docs/03-implementation/PERFORMANCE_BUDGET.md` |
| Diagnósticos do sistema? | `docs/03-implementation/DIAGNOSTICS_SPEC.md` |
| Como testar (Go + Frontend)? | `docs/04-quality/TEST_STRATEGY.md` |
| Como testar UI/UX e acessibilidade? | `docs/04-quality/UI_UX_TESTS.md` |
| Critérios de pronto (DoD)? | `docs/04-quality/DEFINITION_OF_DONE.md` |
| Benchmarks de referência? | `docs/04-quality/HARDWARE_BENCHMARKS.md` |
| Segurança, CSP e Sandbox Wails? | `docs/05-security/SECURITY.md` |
| Inventário e auditoria de credenciais? | `docs/01-architecture/adrs/ADR-023-credential-metadata-inventory.md` e `docs/05-security/SECURITY.md` |
| Modelo de ameaças? | `docs/05-security/THREAT_MODEL.md` |
| Quais são as notas de conformidade? | `docs/05-security/COMPLIANCE_NOTES.md` |
| Papéis e governança de agentes? | `docs/06-governance/AGENT_ROLES.md` |
| Rotas de IA e modelos? | `docs/06-governance/AI_ROUTING.md` |
| Como a documentação é governada? | `docs/06-governance/DOC_GOVERNANCE.md` |
| Quais riscos estão abertos? | `docs/06-governance/RISK_REGISTER.md` |
| Quais fontes externas sustentam o plano? | `docs/06-governance/EXTERNAL_REFERENCES.md` |
| Como operar o aplicativo? | `docs/07-operations/OPERATIONS.md` |
| Como empacotar e distribuir? | `docs/07-operations/PACKAGING_AND_DISTRIBUTION.md` |
| Como diagnosticar problemas? | `docs/07-operations/TROUBLESHOOTING.md` |
| Guia do usuário? | `docs/08-user/USER_GUIDE.md` |
| Quais são os atalhos? | `docs/08-user/SHORTCUTS.md` |
| Registro de decisões (ADRs)? | `docs/09-decisions/DECISION_REGISTER.md` |

## 3. Implementação em uma visão

```text
┌──────────────┐  ┌──────────────┐  ┌──────────────┐
│ HummTube     │  │ HummIPTV     │  │ HummMusic    │
│ YouTube SPA  │  │ M3U/EPG SPA  │  │ Music SPA    │
└──────┬───────┘  └──────┬───────┘  └──────┬───────┘
       │ RPC allowlist   │ RPC allowlist   │ RPC allowlist
┌──────▼─────────────────▼─────────────────▼──────┐
│ Runtime Wails/Go e bibliotecas compartilhadas   │
│ Player • Storage • Services • Security          │
└──────┬─────────────────┬─────────────────┬──────┘
       ▼                 ▼                 ▼
  hummtube.db       hummiptv.db       hummmusic.db
 Keyring próprio   Keyring próprio   Keyring próprio
```

## 4. Máquina de estado do desenvolvimento

`SPECIFIED → IMPLEMENTING → TESTED → BENCHMARKED → REVIEWED → DOCUMENTED → DONE`
