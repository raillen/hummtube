# Prumo — NanoTube Web

> Roteador canônico de intenção para humanos e agentes no ecossistema Prumo v0.5.

## Estado Atual do Projeto
- [Project State](../PROJECT_STATE.md) — Fase ativa, recuperação e próximas ações
- [`prumo.json`](../prumo.json) — Manifesto canônico do projeto (protocolo v3)
- [`ATLAS.md`](../ATLAS.md) — Índice canônico de navegação por domínio e perguntas
- [Contratos de Documentação](contracts/bindings.json) — Mapeamento formal de contratos Prumo

---

## 1. Quero Usar o Produto (User)
- [Guia do Usuário](08-user/USER_GUIDE.md) — Navegação, pastas locais e reprodução
- [Atalhos de Teclado](08-user/SHORTCUTS.md) — Mapeamento completo de comandos de teclado e Modo TV
- [Visão do Produto](00-product/PRODUCT.md) — Proposta de valor, diferenciais e público-alvo

---

## 2. Quero Desenvolver / Contribuir (Developer)
- [Arquitetura Geral](01-architecture/ARCHITECTURE.md) — Clean Architecture, Ports & Adapters e limites de produto
- [Fluxos de Dados](01-architecture/DATA_FLOWS.md) — Ciclo de vida da mídia, cache e sincronização
- [Contratos e Interfaces](01-architecture/CONTRACTS.md) — Portas de serviços Go e bindings Wails v3
- [Stack Tecnológica](03-implementation/STACK.md) — Tecnologias backend (Go/SQLite) e frontend (Svelte/Tailwind)
- [Implementação da UI](03-implementation/UI_IMPLEMENTATION.md) — Componentes Svelte, stores reativas e temas
- [Implementação do Player](03-implementation/PLAYER.md) — Player HTML5/HLS integrado ao Wails
- [Resolução de Playback](03-implementation/PLAYBACK_BACKENDS.md) — `PlaybackResolver` (yt-dlp, PO Token, Invidious)
- [Armazenamento e Migrações](03-implementation/STORAGE.md) — SQLite, índices FTS5 e migrações Goose
- [Busca Instantânea](03-implementation/SEARCH.md) — Busca local FTS5 híbrida com Data API
- [Algoritmo de Recomendações](03-implementation/RECOMMENDATIONS.md) — MMR explicável, afinidades e saturação

---

## 3. Quero Testar e Garantir Qualidade (QA & Testing)
- [Estratégia de Testes](04-quality/TEST_STRATEGY.md) — Pirâmide de testes (unitários, concorrência `-race`, integração)
- [Testes de UI/UX e Acessibilidade](04-quality/UI_UX_TESTS.md) — Testes Vitest, Playwright e WCAG
- [Critérios de Pronto (DoD)](04-quality/DEFINITION_OF_DONE.md) — Definição estrita de conclusão de tarefas
- [Benchmarks de Hardware](04-quality/HARDWARE_BENCHMARKS.md) — Metas de performance no hardware de referência (Celeron / 2GB RAM)
- [Budgets de Desempenho](03-implementation/PERFORMANCE_BUDGET.md) — Limites de memória, latência e IPC

---

## 4. Quero Operar, Empacotar e Distribuir (Operations)
- [Operações e Releases](07-operations/OPERATIONS.md) — Ciclo de release, tarefas Taskfile e matrix de smoke test
- [Empacotamento Linux](07-operations/PACKAGING_AND_DISTRIBUTION.md) — Distribuição (.deb, Arch, AppImage, RPM) e ciclo de instalação
- [Diagnósticos e Solução de Problemas](07-operations/TROUBLESHOOTING.md) — Runbooks, recuperação e logs estruturados

---

## 5. Segurança, Privacidade e Confiança (Security)
- [Contrato de Segurança](05-security/SECURITY.md) — CSP, Keyring nativo, zero vazamento de tokens e proxy local
- [Modelo de Ameaças (STRIDE)](05-security/THREAT_MODEL.md) — Vetores de ataque e controles mitigadores
- [Notas de Conformidade](05-security/COMPLIANCE_NOTES.md) — Auditoria de dependências, SBOM e licenças

---

## 6. Governança e Decisões Arquiteturais (Governance)
- [Registro de Decisões (ADRs)](09-decisions/DECISION_REGISTER.md) — Catálogo de ADRs aprovadas
- [Papéis de Agentes](06-governance/AGENT_ROLES.md) — Especializações e limites de atuação
- [Governança de Documentação](06-governance/DOC_GOVERNANCE.md) — Regras de sincronização e autoridade documental

---

## 7. Instruções para Agentes de IA
1. Leia `prumo.json`, `PROJECT_STATE.md` e este `docs/PRUMO.md`;
2. Identifique o Goal ativo em `.ai/goals/` e seu plano de execução;
3. Opere sob **Lean Progressive Context (LPC)**: consulte apenas os documentos e símbolos pertinentes à tarefa;
4. Respeite as regras de Clean Architecture: o domínio nunca importa adaptadores externos;
5. Valide implementações com testes exaustivos (`task verify`, `prumo validate`, `prumo doctor`);
6. Mantenha os contratos de documentação sincronizados via `prumo docs audit` e `prumo docs readiness`.
