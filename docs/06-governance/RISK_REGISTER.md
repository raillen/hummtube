---
id: risk-register
status: canonical
---

# Registro de Riscos — NanoTube Web

Status permitidos: `Open`, `Mitigated`, `Accepted` e `Closed`. Uma release estável não pode ter risco P0/P1 em `Open`.

| ID | Risco | Prioridade | Status | Owner | Marco | Mitigação | Evidência para sair de `Open` |
|---|---|---|---|---|---|---|---|
| R-001 | Quebra de extração pelo YouTube | P1 | Open | Implementer (Player) | M3 | Runtime versionado, rollback e smoke corpus; não manter SABR/extrator próprio | Corpus passa e rollback do runtime é exercitado |
| R-002 | Bloqueio por falta de PO Token | P1 | Open | Implementer (Player) | M3 | Provider fixado e diagnosticável; fallback combinado seguro | Matriz ausente/válido/expirado aprovada |
| R-003 | Plugin/config externo acessar cookies | P0 | Open | Security Reviewer | M1 | Ignorar configuração global, plugins herméticos e versão mínima autenticada | Testes negativos de configuração/plugin passam |
| R-004 | Fallback contornar conteúdo restrito | P0 | Open | Security Reviewer | M1 | Falhas terminais e nenhum Invidious após restrição/tentativa autenticada | Testes de cadeia provam ausência de fallback indevido |
| R-005 | Proxy local ampliar SSRF/DNS rebinding | P0 | Open | Security Reviewer | M2 | Sessões opacas, destinos pinados, redirects validados, limites e fuzz | Threat model e suíte SSRF/race/fuzz aprovados |
| R-006 | Consumo de memória pelo WebView | P1 | Open | Reviewer | M4 | Medir árvore completa, limitar buffers, lazy loading e modo econômico | Baseline e p95 cumprem budgets em hardware-alvo |
| R-007 | Falha de Keyring em ambiente sem desktop | P2 | Open | Implementer (Backend) | M1 | Fallback em memória com aviso e falha fechada para persistência | Testes com Keyring ausente aprovados |
| R-008 | Scrobble revelar hábitos de reprodução | P1 | Open | Security Reviewer | M5 | Last.fm opt-in, revogável, desabilitado em guest e session key no SecretStore | Testes de consentimento, revogação e redaction passam |
| R-009 | Pesquisa remota lenta ou concorrente | P1 | Open | Implementer (Backend) | M4 | Data API-first, cancelamento, uma busca por perfil, TTL e `singleflight` | Latência/cancelamento cumprem gates do plano |
| R-010 | Quota da Data API insuficiente | P1 | Open | Architect | M4 | Busca explícita, cache, filtros locais e diagnóstico agregado | Cenário diário documentado cabe na quota disponível |
| R-011 | Pacote incluir dependências/assets incorretos | P1 | Open | Release Verifier | M3 | Build exclusivo, instalação limpa, SBOM, checksums e assinatura | Matriz de instalação e inspeção de artefato passam |
| R-012 | OAuth impedir distribuição pública | P1 | Open | Architect | M3 | Projeto de produção, menor escopo, privacidade e verificação antecipada | OAuth de produção e fluxo de exclusão homologados |
| R-013 | Distribuição conflitar com termos ou licenças | P0 | Open | Architect | M3 | Revisão jurídica/política, notices, nenhuma redistribuição de mídia e componentes auditados | Parecer registrado e inventário de licenças aprovado |
