---
id: nanotube-release-hardening-plan
status: in-progress
owner: Architect
target: v0.2.0-v1.0.0
last_reviewed: 2026-08-31
---

# Plano de Hardening, Experiência, Performance e Distribuição — NanoTube

> Marco antecipado (2026-08-31): a preview Debian amd64 `0.1.0` foi gerada com playback web, seletor de qualidade, fallback adaptativo, DSP seguro e cadeia yt-dlp hermética. Este marco não substitui a homologação de instalação limpa, assinatura, SBOM e matriz Linux previstas abaixo.

> Progresso de release (2026-08-31): o workspace passou a ter preflight explícito de ferramentas (`task tooling:check`), AppDir AppImage restrito ao NanoTube com validação estrutural e geração/validação de SBOM CycloneDX 1.5. Esses controles tornam o processo reproduzível, mas não fecham NT-011/NT-012/NT-018: assinatura, provenance, instalação limpa e matriz Linux continuam pendentes.

> Estado verificado (2026-08-31): o gate automatizado passou (`task verify`: Go race/vet, svelte-check, build e 45 E2E), o benchmark local foi registrado em `build/benchmarks/` e AppDir, SBOM e preview Debian foram validados. Este ciclo também entregou isolamento de cookies por perfil, alternância de perfis no header, métricas agregadas de playback/pesquisa, fundação `pt-BR`/`en-US` de i18n, Ctrl+K, menus contextuais, navegação TV de baixo consumo, DevTools bloqueado por padrão em release e o transporte local opaco de mídia (NT-006). A infraestrutura continua em progresso: migração Svelte 5 (NT-002), canal remoto de atualização/instalação do runtime (NT-010), assinatura/provenance e homologação em VMs Linux ainda não foram concluídos. A base local de instalação atômica, ativação e rollback do runtime já está coberta por testes e exposta no módulo de configurações, mas permanece sem distribuição remota. O proxy local está integrado ao `ResolveMedia`; sua aceitação completa ainda requer fuzz/stress de fronteiras e matriz de WebKit.

## 1. Resultado esperado

Entregar o NanoTube como cliente desktop YouTube instalável, previsível e leve, com:

- reprodução pública anônima e reprodução autenticada usada somente quando necessária;
- 720p/1080p adaptativo quando o YouTube e o runtime realmente oferecerem os formatos;
- fallback combinado de 360p sem travar a sessão;
- pesquisa externa rápida para contas conectadas e degradada de forma explícita para visitantes;
- filtros reativos com feedback consistente, cancelamento real e estado preservado durante atualizações;
- contas, ações contextuais e modo TV navegáveis pelo mesmo catálogo de comandos e sistema de foco;
- textos da interface preparados para tradução sem acoplar o domínio a um idioma;
- pacotes Linux reproduzíveis, com dependências, licenças e runtime de playback verificáveis;
- consumo medido no hardware de referência, incluindo processos WebKit e GPU;
- caminho de rollback para cada mudança estrutural.

Este documento é um plano. Uma capacidade só passa a ser descrita como atual nos documentos canônicos depois que seu código, testes e benchmarks forem concluídos.

## 1.1. Documentos de controle

- [Roadmap do produto](../00-product/SCOPE_AND_ROADMAP.md)
- [Registro de decisões](../09-decisions/DECISION_REGISTER.md)
- [Registro de riscos](../06-governance/RISK_REGISTER.md)
- [Orçamento de performance](PERFORMANCE_BUDGET.md)
- [Estratégia de testes](../04-quality/TEST_STRATEGY.md)
- [Segurança](../05-security/SECURITY.md) e [modelo de ameaças](../05-security/THREAT_MODEL.md)
- [Empacotamento e distribuição](../07-operations/PACKAGING_AND_DISTRIBUTION.md)
- [Referências externas](../06-governance/EXTERNAL_REFERENCES.md)

## 2. Escopo e limites

### Dentro do escopo

1. segurança e isolamento do yt-dlp, plugins, cookies e headers de mídia;
2. transporte same-origin de streams resolvidos, sem transcodificação;
3. seleção de qualidade consciente de codec, hardware, FPS e consumo;
4. runtime de playback versionado, diagnosticável e atualizável de forma explícita;
5. cancelamento, cache e seleção eficiente de provider na pesquisa;
6. cache real de thumbnails e redução de trabalho em segundo plano;
7. build e empacotamento exclusivos do NanoTube;
8. OAuth de produção, SBOM, provenance, assinatura e matriz Linux;
9. benchmarks reais no Celeron 1037U e em um equipamento moderno;
10. gerenciamento seguro de perfis, configuração OAuth do aplicativo e consentimento de sessão do navegador;
11. catálogo de comandos, atalhos, menus contextuais e histórico de navegação;
12. estados assíncronos, seleção em lote e experiência TV 10-foot;
13. catálogos de texto traduzíveis e contratos de erro estáveis.

### Fora do escopo

- qualquer alteração funcional no NanoIPTV ou NanoMusic;
- implementar SABR ou manter um extrator YouTube próprio;
- operar um proxy remoto de vídeo ou pesquisa;
- embutir uma API key pública da YouTube Data API no executável;
- transcodificar mídia durante a reprodução;
- migrar o player principal para libmpv sem spike e nova decisão arquitetural;
- transformar o NanoTube em um host genérico de plugins.
- importar tokens OAuth de usuário, valores de cookies, arquivos `cookies.txt` ou paths arbitrários pela UI;
- duplicar a aplicação em uma segunda SPA específica para TV;
- exigir um segundo idioma para concluir a infraestrutura de internacionalização da v0.3.0.

### Premissas de execução sujeitas aos gates

Estas premissas orientam a ordem de trabalho, mas só se tornam decisões arquiteturais confirmadas depois dos respectivos testes, benchmarks e ADRs.

- o player WebView continua sendo o player principal;
- o backend mantém headers, cookies e URLs assinadas fora do RPC;
- mpv pode ser avaliado apenas como fallback externo opcional;
- Shaka Player permanece um spike reversível, não uma dependência aprovada;
- pacotes nativos são o canal primário; AppImage e Flatpak são posteriores;
- a Data API é o caminho preferencial de pesquisa quando existe conta conectada;
- yt-dlp permanece o caminho público para visitante e fallback explícito;
- falhas `Restricted`, `DRM`, privado ou membros são terminais e não seguem para Invidious.
- a importação de configuração OAuth do aplicativo permanece bloqueada até P-007 receber ADR e testes de segurança;
- os mesmos comandos de domínio alimentam ações rápidas, atalhos, menus contextuais e apresentação TV;
- o modo TV reutiliza domínio e estado do desktop, alterando apresentação, foco e densidade;
- `pt-BR` permanece o fallback e outros idiomas são carregados sob demanda.

## 3. Arquitetura alvo

```text
Pesquisa autenticada ─────────► YouTube Data API ──► cache curto ──► NanoRank local
Pesquisa visitante ───────────► yt-dlp público ────► cache curto ──► NanoRank local

Play público ─► resolver anônimo ─► StreamSession opaca ─► proxy local Range ─► <video>
                      │
                      ├─ restrito ─► consentimento ─► resolver autenticado por perfil
                      │                                      │
                      │                                      └─ falha ─► Abrir no YouTube
                      └─ indisponível ─► fallback técnico permitido

NanoTube ─► Playback Runtime Manifest
               ├─ yt-dlp fixado
               ├─ runtime JS fixado
               ├─ EJS/provider fixado
               └─ hashes, assinatura, licença e rollback
```

### Invariantes

- existe somente uma instância ativa de reprodução;
- vídeo público nunca recebe cookies de navegador;
- cookies nunca são enviados a Invidious;
- uma restrição de conteúdo não pode ser convertida em sucesso por outro provider;
- o frontend recebe somente uma URL local opaca, nunca `Cookie` ou `Authorization`;
- o proxy aceita somente sessões criadas pelo backend e destinos previamente validados;
- o runtime não carrega configuração ou plugins arbitrários do usuário;
- somente uma busca remota fica ativa por perfil;
- caches autenticados são isolados por perfil e possuem TTL;
- nenhum atualizador executa código remoto sem consentimento, integridade e rollback.

## 4. Orçamentos e gates

| Área | Gate de aceite |
|---|---|
| Inicialização | cold start menor que 1,5 s no hardware de referência |
| Repouso | CPU p95 menor que 1% e RSS/PSS da árvore menor que 180 MB |
| Playback | árvore menor que 350 MB em 1080p; 720p30 sem frames continuamente descartados no Celeron |
| Bundle | entrada inicial menor que 150 KB gzip; HLS/Shaka continuam lazy |
| Pesquisa local | FTS5 p95 menor que 15 ms |
| Pesquisa oficial | p95 fria menor que 1,5 s e repetição aquecida menor que 300 ms |
| Pesquisa visitante | meta provisória p95 menor que 4 s na primeira página; validar no hardware-alvo |
| Cancelamento | requisição anterior e subprocesso encerrados em até 250 ms após cancelamento |
| Concorrência | no máximo uma busca externa ativa por perfil e uma resolução idêntica em andamento |
| Thumbnails | zero download simultâneo duplicado; hit aquecido maior que 80% no cenário de navegação |
| Segurança | nenhuma credencial, URL assinada ou header sensível atravessa RPC, logs, backup ou diagnóstico |
| Interação | toda entidade coberta executa a mesma ação por botão, menu contextual, teclado e TV; foco retorna ao gatilho |
| Filtros | somente a consulta remota mais recente permanece ativa; conteúdo anterior continua visível durante atualização |
| TV | Home → Canal → Player → Fila → Voltar funciona sem mouse em 1280×720 e 1920×1080 |
| i18n | catálogos têm paridade de chaves, fallback válido, pseudolocalização sem overflow crítico e `lang` sincronizado |
| Distribuição | instalação limpa e smoke em Debian/Ubuntu, Fedora e Arch; SBOM e checksums publicados |

Metas de rede dependem da conexão e do upstream. Os relatórios devem separar tempo local, DNS, conexão, provider, enriquecimento e renderização.

## 5. Ordem de implementação

```text
M0 Baseline, escopo e Svelte 5
 ├─ NT-001 Medição e telemetria local
 └─ NT-002 Build exclusivo do NanoTube
          │
          ├─► M1 Segurança de playback ─► M2 Transporte e qualidade ─► M3 Runtime/distribuição ─┐
          │                                                                                   │
          └─► M4 Pesquisa, thumbnails e consumo ─► M5 Experiência, contas e TV ────────────────┤
                                                                                              │
                                                                                              ▼
                                                                                         M6 Homologação
                                                                                              │
                                                                                              └─► M7 Spikes opcionais
```

M1 a M3 formam a trilha crítica de playback e distribuição. M4 pode avançar em paralelo depois de M0. M5 inicia por suas fundações depois de NT-002 e consome as garantias de cancelamento, perfis e baixo consumo de M1/M4. A matriz P0 bloqueia qualquer beta pública; M3, M4, M5 e M6 bloqueiam a classificação como release estável. M7 não bloqueia a entrega.

### Responsabilidade por marco

| Marco | Responsável | Revisão obrigatória |
|---|---|---|
| M0 | Architect + Reviewer | Documentation Maintainer |
| M1 | Implementer (Backend) + Implementer (Player) | Security Reviewer + Tester |
| M2 | Implementer (Player) | Security Reviewer + Reviewer + Tester |
| M3 | Release Verifier | Security Reviewer + Documentation Maintainer |
| M4 | Implementer (Backend) + Implementer (Frontend) | Reviewer + Tester |
| M5 | Implementer (Frontend) + Implementer (Backend) | UX/Design System Reviewer + Security Reviewer + Tester |
| M6 | Release Verifier | Security Reviewer + Reviewer + Documentation Maintainer |
| M7 | Architect | Reviewer; novo ADR somente se adotado |

### Prioridades de execução

Prioridade expressa ordem e capacidade de bloquear uma entrega; não substitui a severidade registrada para cada risco.

| Prioridade | Gate | Fatias |
|---|---|---|
| P0 — segurança e fundação | bloqueia qualquer beta pública | NT-001 a NT-006, NT-009 e NT-013 |
| P1 — experiência funcional | bloqueia a v0.3.0 e a release estável | NT-007, NT-008, NT-014 a NT-016, NT-019 a NT-023 e NT-025 a NT-027 |
| P2 — distribuição e capacidade avançada | executada depois de P0/P1; NT-010/011/017/018 ainda bloqueiam a v1.0.0 | NT-010 a NT-012, NT-017, NT-018 e NT-024 quando P-007 for aceita |
| P3 — experimental | não bloqueia release | NT-X01 e NT-X02 |

Os IDs são estáveis e identificam fatias; a numeração não define a ordem de execução. Ordem recomendada dentro das prioridades:

1. fechar baseline, migração Svelte 5 e fronteiras de cookies/playback;
2. implementar cancelamento, qualidade, cache e baixo consumo;
3. retirar textos obsoletos e criar as fundações de i18n, comandos e estados assíncronos;
4. corrigir filtros, navegação, perfis e feedbacks antes de ampliar componentes;
5. padronizar seleção, canais e ações contextuais;
6. concluir o modo TV sobre o sistema compartilhado de foco;
7. homologar runtime, pacotes, hardware e rollback;
8. executar importação OAuth somente depois da ADR e dos gates de segurança; avaliar spikes por último.

## 6. Fatias verticais

### M0 — Baseline e isolamento do produto

#### NT-001 — Harness de performance e estágios de latência

**Objetivo:** medir antes de otimizar e impedir que o processo Go esconda o custo do WebKit/GPU.

**Módulos prováveis:** `internal/diagnostics`, `Taskfile.yml`, `docs/03-implementation/PERFORMANCE_BUDGET.md`, `docs/04-quality/HARDWARE_BENCHMARKS.md`.

**Entrega:**

- criar `task bench` e `task bench:hardware`;
- medir árvore de processos, RSS/PSS, CPU, tempo até interface e primeiro frame;
- instrumentar pesquisa por estágio sem registrar query, token ou dados pessoais;
- guardar resultados datados por hardware e ambiente.

**Aceite:** baseline reproduzível no Celeron e em um quad-core moderno; três aquecimentos e pelo menos 30 execuções medidas para métricas locais; relatório informa mediana, p95, dispersão e ambiente. Métricas remotas usam pelo menos 20 amostras distribuídas no tempo e separam a latência do upstream.

**Testes:** parser de métricas, ausência de dados sensíveis, execução sem ferramentas opcionais com aviso recuperável.

**Rollback:** métricas permanecem atrás de flag diagnóstica e não alteram o fluxo normal.

#### NT-002 — Build e assets exclusivos do NanoTube

**Objetivo:** remover NanoIPTV, NanoMusic e assets obsoletos do artefato NanoTube.

**Módulos prováveis:** `Taskfile.yml`, `frontend/package.json`, `frontend/vite.config.ts`, `assets.go`, `packaging/**`.

**Entrega:**

- tasks `build:nanotube`, `test:nanotube` e `package:nanotube` independentes;
- migrar o frontend instalado de Svelte 4 para o Svelte 5 já definido por D-004, ou bloquear o marco se a prova de compatibilidade exigir revisar essa decisão;
- limpar somente `dist/nanotube` antes do build sem tocar nos outros produtos;
- embutir e empacotar apenas o target NanoTube;
- remover a cópia externa duplicada do frontend quando o binário embutido for suficiente;
- desabilitar DevTools em release;
- remover IPTV das descrições e metadados do NanoTube.

**Aceite:** `frontend/package.json` e lockfile usam Svelte 5 compatível; `svelte-check`, testes e build passam sem modo de compatibilidade indefinido; pacote não contém paths `nanoiptv`/`nanomusic`, assets órfãos ou DevTools habilitado; smoke desktop e `--server` continuam funcionando.

**Rollback:** manter task legada `build:all` somente para o monorepo, sem ser chamada pelo pipeline NanoTube.

### M1 — Segurança de playback e conteúdo restrito

#### NT-003 — Falhas terminais e resolução anônima primeiro

**Objetivo:** impedir fallback que contorne uma restrição e reduzir exposição da conta.

**Módulos prováveis:** `internal/playback/cascading.go`, `failure.go`, `internal/services/services.go`.

**Entrega:**

- classificar `Restricted`, `DRM`, privado, membros e indisponibilidade definitiva como terminais;
- separar resolvedor público e resolvedor autenticado;
- nunca anexar cookies na primeira tentativa de vídeo público;
- não chamar Invidious depois de tentativa autenticada ou falha de restrição;
- limitar retries e aplicar backoff/circuit breaker para 429.

**Aceite:** um provider posterior não é chamado depois de falha terminal; vídeo público não produz `--cookies-from-browser`; erro final oferece ação compreensível.

**Testes:** regressão de fallback terminal, exatamente uma tentativa autenticada, 429 e cancelamento.

**Rollback:** feature flag seleciona temporariamente a cadeia anterior apenas em builds de desenvolvimento, nunca em release.

#### NT-004 — Runtime yt-dlp hermético e headers fora do RPC

**Objetivo:** impedir que configuração ou plugin externo execute código enquanto o subprocesso acessa sessão do navegador.

**Módulos prováveis:** `internal/playback/ytdl.go`, `subprocess.go`, `extractor_runtime.go`, `internal/domain/ports.go`.

**Entrega:**

- invocar yt-dlp ignorando configurações externas;
- desabilitar diretórios globais de plugins e permitir somente diretório controlado;
- registrar versão e origem do runtime, nunca seus segredos;
- remover headers arbitrários do contrato web ou aplicar allowlist no backend;
- bloquear `Cookie`, `Authorization`, `Set-Cookie` e `Proxy-Authorization` no bridge.

**Aceite:** configuração de teste contendo comando e plugin malicioso não é executada; header sensível injetado no JSON não chega ao frontend.

**Testes:** subprocesso falso, plugin-malícia fixture, fuzz do parser JSON e sanitização.

**Rollback:** runtime do sistema pode permanecer fallback anônimo; playback autenticado falha fechado se a procedência não for aceitável.

#### NT-005 — PlaybackSession por perfil e consentimento

**Objetivo:** impedir que o perfil OAuth A use silenciosamente a sessão de navegador B.

**Módulos prováveis:** `internal/domain`, `internal/storage`, `internal/services`, `LoginModal.svelte`, `SettingsView.svelte`.

**Entrega:**

- modelo `PlaybackSessionMetadata` por `profile_id` sem valores de cookies;
- sessão de navegador desligada por padrão, sem herança para guest;
- descoberta segura de perfis de navegador como IDs opacos, sem paths arbitrários;
- ações Configurar, Testar, Desativar e Esquecer;
- prompt contextual antes da única tentativa autenticada;
- logout OAuth não altera silenciosamente a preferência de cookies e a UI explica que são mecanismos distintos;
- recomendação explícita de perfil de navegador dedicado ao YouTube.

**Aceite:** alternar perfil alterna ou desativa a sessão correspondente; guest nunca usa sessão persistida; backup não contém material autenticador.

**Testes:** migração da preferência global, isolamento por perfil, logout/exclusão, Keyring indisponível e E2E do consentimento.

**Rollback:** preferências novas podem ser ignoradas sem perder OAuth ou biblioteca; cookies continuam no navegador.

### M2 — Transporte e qualidade

#### NT-006 — StreamSession opaca e proxy local de mídia

**Objetivo:** aplicar headers e `Range` no backend sem expor credenciais ou transcodificar.

**Módulos prováveis:** novo `internal/mediaproxy`, `internal/server`, `internal/playback`, `internal/domain`, `VideoPlayer.svelte`.

**Entrega:**

- registro em memória de sessões com ID aleatório, perfil, origem, headers permitidos e expiração;
- endpoint same-origin que aceita somente sessão válida e método permitido;
- suporte a `GET`, `HEAD`, `Range`, cancelamento e streaming com backpressure;
- validação de esquema, hostname, DNS/IP, redirects e origem a cada conexão;
- limite de headers, resposta e conexões; nenhuma persistência de URL assinada;
- `PlaybackPlan` web passa a expor URL local opaca.

**Aceite:** seek funciona; respostas 206 preservam ranges; fechar/trocar vídeo cancela upstream; SSRF e DNS rebinding falham fechados; CPU não indica transcodificação.

**Testes:** servidor upstream fake, ranges inválidos, redirect, IP privado, expiração, concorrência, slowloris, cancelamento, `go test -race` e fuzz dos boundaries.

**Rollback:** flag `media_proxy` permite retornar ao transporte direto apenas quando nenhum header especial for necessário.

**Estado atual (2026-08-31):** implementado em `internal/playback/media_proxy.go` e registrado pelo
`ResolveMedia`. O endpoint `/api/media/<token>` mantém URL/headers apenas em memória, usa TTL curto,
limita sessões, aceita `GET`/`HEAD`/`Range`, força transporte sem compressão, filtra headers de resposta,
valida esquema e destinos públicos a cada resolução DNS/conexão e rejeita redirecionamentos privados ou
excessivos. Manifests HLS limitados têm playlists, segmentos e chaves HTTP reescritos para tokens locais.
Testes com upstream fake cobrem Range, expiração, rejeição de destino privado, variantes, HLS e
redirecionamento inseguro. Ainda faltam fuzz/stress de slowloris e homologação de seek em WebKit nas
distribuições alvo para fechar o aceite integral.

#### NT-007 — Renovação, cache e coalescência de resolução

**Objetivo:** evitar subprocessos duplicados e recuperar URLs temporárias expiradas.

**Módulos prováveis:** `internal/playback`, `internal/services`, `playerStore.ts`, `VideoPlayer.svelte`.

**Entrega:**

- cache em memória por vídeo, perfil, modo autenticado e política de formato;
- TTL limitado por `expires_at`, com margem de segurança;
- `singleflight` para resolução idêntica;
- uma renovação automática após 403/URL expirada, preservando posição;
- invalidação em troca de perfil, sessão, runtime ou qualidade.

**Aceite:** duas solicitações simultâneas executam um extractor; stream expirado renova uma vez; loop de retry é impossível.

**Testes:** relógio falso, cache hit/miss, expiração, troca de perfil e falha parcial.

**Rollback:** desabilitar cache mantém proxy e segurança intactos.

#### NT-008 — Qualidade consciente do hardware

**Objetivo:** oferecer mais resoluções sem escolher codec caro para o equipamento.

**Módulos prováveis:** `internal/domain/ports.go`, `explicit_ytdlp.go`, `ytdl.go`, `frontend/src/lib/player/quality.ts`, `VideoPlayer.svelte`, `SettingsView.svelte`.

**Entrega:**

- variantes com codec, container, FPS, bitrate, HDR e dimensões;
- perfis Automático compatível, Econômico, Melhor qualidade e Manual;
- `MediaCapabilities.decodingInfo()` e fallback determinístico;
- Celeron prioriza H.264/AAC, SDR e 30 FPS; AV1 nunca é padrão sem decode eficiente;
- HLS com `capLevelToPlayerSize`, reação a frames descartados e buffers limitados;
- opções manuais continuam mostrando somente streams reproduzíveis.

**Aceite:** 360p, 720p e 1080p são selecionáveis quando presentes; automático não mantém formato com queda contínua de frames; troca preserva posição/áudio/fila.

**Testes:** fixtures H.264/VP9/AV1, 30/60 FPS, HDR, HLS, faixa separada e ausência de áudio.

**Rollback:** perfil Compatível usa a seleção conservadora anterior; metadados novos são opcionais no contrato.

### M3 — Runtime e distribuição

#### NT-009 — Manifesto e diagnóstico do Playback Runtime

**Objetivo:** tornar procedência e compatibilidade observáveis antes de instalar ou atualizar componentes.

**Módulos prováveis:** `internal/playback/extractor_runtime.go`, `internal/diagnostics`, schema de manifesto, `DiagnosticsModal.svelte`.

**Entrega:**

- manifesto versionado com yt-dlp, JS runtime, EJS/provider, hashes, licenças e compatibilidade;
- diagnóstico `ok`, `ausente`, `incompatível`, `desatualizado` ou `não confiável`;
- versão mínima de segurança para permitir cookies;
- nenhuma edição de versão ou caminho arbitrário pela UI.

**Aceite:** runtime desconhecido pode reproduzir anonimamente conforme política, mas não recebe cookies; manifesto adulterado é rejeitado.

**Testes:** schema, assinatura/hash, downgrade, versão incompatível e redaction.

**Rollback:** sistema continua usando yt-dlp anônimo do PATH com aviso explícito.

#### NT-010 — Pacote versionado e atualização explícita do runtime

**Objetivo:** desacoplar correções de extração do ciclo de release da interface.

**Dependência:** NT-009 e ADR que superseda a prioridade irrestrita do runtime do sistema.

**Entrega:**

- subpacote `nanotube-playback-runtime` para canais nativos;
- instalação em path imutável e conhecido;
- atualização atômica, rollback para versão anterior e canal estável testado;
- ação explícita na UI para verificar/instalar quando o canal não for gerenciado pelo sistema;
- notices, fontes correspondentes e SBOM dos componentes redistribuídos.

**Aceite:** instalação limpa reproduz 720p/1080p do corpus permitido sem depender de yt-dlp preexistente; atualização quebrada retorna à versão anterior.

**Testes:** pacote em container/VM limpa, corrupção, interrupção no meio, downgrade e permissões.

**Rollback:** remover o subpacote retorna ao modo anônimo do runtime do sistema; dados pessoais não são tocados.

**Estado atual (2026-08-31):** a base local já instala somente componentes declarados em manifesto com hash verificado, publica a versão por rename atômico, expõe ativação/rollback e mostra o estado (sem caminhos locais) em `Settings`. Ainda faltam o canal remoto assinado, download/atualização explícitos, notices/SBOM por componente redistribuído e homologação em instalação limpa.

#### NT-011 — Pacotes nativos e OAuth de produção

**Objetivo:** produzir uma instalação pública coerente com o runtime real.

**Módulos prováveis:** `packaging/deb`, `packaging/rpm`, `packaging/arch`, CI de release, `internal/auth`, documentação legal/operacional.

**Entrega:**

- dependências GTK/WebKitGTK corretas por ABI e distro;
- Go, Node e Wails coerentes com lockfiles e build;
- checksums obrigatórios, artefatos assinados, SBOM e provenance;
- projeto OAuth de produção separado de desenvolvimento;
- homepage, termos, política de privacidade, exclusão de dados e preparação para verificação Google;
- BYO OAuth permanece modo avançado.

**Aceite:** instalação/upgrade/remoção em Debian/Ubuntu, Fedora e Arch; login PKCE em navegador externo; pacote sem secrets de desenvolvimento.

**Testes:** VMs limpas, assinatura, upgrade preservando banco, uninstall preservando/removendo dados conforme opção documentada.

**Rollback:** manter release anterior publicada e migrações backward-safe; runtime e app podem voltar independentemente.

#### NT-012 — Gate para AppImage e Flatpak

**Objetivo:** não anunciar portabilidade antes de provar WebKit, runtime, Keyring e sessão de navegador.

**Entrega:** matriz AppImage em hosts com/sem WebKitGTK compatível; spike Flatpak com portals, Keyring e perfil dedicado.

**Aceite:** somente promover um formato se instalação limpa, playback, OAuth, tray e atualização funcionarem sem permissões amplas injustificadas.

**Rollback:** formatos reprovados permanecem experimentais e fora da página principal de download.

### M4 — Pesquisa, thumbnails e consumo

#### NT-013 — Cancelamento real e uma busca ativa

**Objetivo:** impedir que filtros em tempo real acumulem requests e subprocessos.

**Módulos prováveis:** `services.ts`, `SearchView.svelte`, `internal/server`, `internal/services`, `internal/search`.

**Entrega:**

- `AbortController` atravessa fetch e cancela o `r.Context()`;
- subprocesso recebe cancelamento e encerra a árvore;
- uma busca ativa por perfil e `singleflight` por request normalizado;
- filtros locais reaplicam a página em memória sem rede;
- filtros remotos usam debounce e cancelam o request anterior.

**Aceite:** alternar filtros rapidamente não deixa mais de um yt-dlp; cancelamento cumpre o gate de 250 ms.

**Testes:** E2E de digitação/filtros, subprocesso lento, navegação para fora e troca de perfil.

**Rollback:** desabilitar coalescência mantém o cancelamento; a UI pode voltar ao submit manual.

#### NT-014 — Data API-first, cache curto e estado por IDs

**Objetivo:** reduzir tempo até o primeiro resultado para contas conectadas.

**Módulos prováveis:** `internal/search/service.go`, `internal/youtubeapi/provider.go`, `internal/services/services.go`, `internal/storage`.

**Entrega:**

- Data API como provider padrão quando a conta está disponível;
- cache LRU/TTL por perfil, request e token de página;
- `fields` mínimo nas chamadas oficiais;
- enriquecimento de vídeos e playlists em paralelo com limite;
- buscar afinidade/favorito/fila/histórico apenas para IDs retornados;
- corrigir paginação cumulativa do yt-dlp ou manter janela pública cacheada;
- quota e cache hit visíveis em diagnóstico agregado, sem query.

**Aceite:** cache hit não usa quota/rede; busca conectada não abre yt-dlp; ordem do provider é preservada antes do NanoRank.

**Testes:** provider fake, TTL, perfil, página, quota, race, resposta parcial e fallback visitante.

**Rollback:** flag de seleção permite voltar ao provider público sem alterar contratos.

#### NT-015 — Cache real de thumbnails

**Objetivo:** cumprir a arquitetura documentada e reduzir rede/decode repetido.

**Módulos prováveis:** novo adapter de `ThumbnailStore`, `internal/server`, `VideoCard.svelte`, settings e poda.

**Entrega:**

- cache content-addressed por SHA-256 da URL;
- ETag/Cache-Control, LRU em disco e limite padrão de 250 MB;
- dois downloads concorrentes no perfil econômico e `singleflight` por URL;
- `decoding="async"`, `fetchpriority="low"`, `srcset/sizes` adequados;
- poda assíncrona e tolerante a corrupção.

**Aceite:** segunda visita funciona a partir do cache; arquivos corrompidos são refeitos; scroll não cria long task recorrente.

**Testes:** HTTP fake, ETag, corrupção, concorrência, poda e offline.

**Rollback:** URL remota direta continua como fallback sem bloquear cards.

#### NT-016 — Modo de baixo consumo e ciclo de vida

**Objetivo:** reduzir CPU ociosa e trabalho gráfico não essencial.

**Módulos prováveis:** `SpatialNav.ts`, layout, player, tokens CSS, settings e desktop lifecycle.

**Entrega:**

- polling de gamepad somente no modo TV, com eventos de conexão e pausa em janela oculta;
- suspender timers/prefetch não essenciais em background;
- perfil visual de baixo consumo reduz blur, sombras e animações;
- throttle de atualizações de tempo que não precisam ocorrer a cada evento;
- `content-visibility` em seções longas e virtualização apenas acima de limite medido.

**Aceite:** idle CPU cumpre orçamento; modo TV continua navegável; `prefers-reduced-motion` é respeitado.

**Testes:** timers falsos, visibility change, gamepad mock, E2E teclado/TV e regressão visual.

**Rollback:** configuração visual e polling TV podem ser restaurados separadamente.

### M5 — Experiência, contas e modo TV

#### NT-019 — Higiene de conteúdo da Home e pesquisa

**Objetivo:** remover informação sem valor sem alterar os contratos de recomendação ou playlist.

**Dependência:** NT-002; pode ser entregue junto da primeira migração de textos do NT-020.

**Módulos prováveis:** `VideoCard.svelte`, `HomeView.svelte`, `SearchView.svelte` e testes E2E de Home/pesquisa.

**Entrega:**

- remover dos cards da Home o bloco visual “Motivos da recomendação”;
- preservar `recommendation_reasons` no domínio e o painel geral de transparência do feed;
- remover da pesquisa o aviso sobre duração total de playlists;
- preservar título, thumbnail e quantidade de vídeos da playlist;
- não alterar filtros de duração aplicáveis a resultados de vídeo.

**Aceite:** cards da Home não exibem chips de motivo; playlists pesquisadas não mostram duração nem o aviso removido; contratos Go/RPC permanecem compatíveis.

**Testes:** Home com fixture contendo motivos e pesquisa com resultado de playlist e contagem conhecida.

**Rollback:** restaurar somente a apresentação; nenhum dado ou schema é removido.

#### NT-020 — Fundação de internacionalização e catálogo de textos

**Objetivo:** retirar textos de interface do código e permitir tradução incremental sem perder estado ou aumentar o startup de forma relevante.

**Dependência:** NT-002. Deve preceder a reformulação ampla de componentes de NT-021 a NT-027.

**Módulos prováveis:** novo `frontend/src/lib/i18n`, `frontend/src/locales`, layout, componentes compartilhados, DTOs de erro, settings e validadores de build.

**Entrega:**

- catálogos JSON por locale e domínio, com `pt-BR` embutido como fallback;
- fachada tipada `t(key, params)` independente da biblioteca escolhida;
- interpolação, pluralização e formatação por `Intl`;
- preferência `ui.locale` por perfil e atualização de `document.documentElement.lang`;
- carregamento sob demanda de idiomas adicionais;
- migração de textos visíveis, `title`, placeholder, ARIA, toast, loading, menu e atalho;
- códigos estáveis e parâmetros sanitizados para erros conhecidos do backend;
- conteúdo externo do YouTube e erros técnicos desconhecidos permanecem dados, não chaves de tradução.

**Aceite:** nenhuma string de interface planejada fica fora do inventário; troca de locale não reinicia o app nem perde rota, filtros ou player; ausência de chave cai de forma observável para `pt-BR`.

**Testes:** paridade de chaves, parâmetros, plural 0/1/N, datas e números, fallback, locale persistido, pseudolocalização e snapshots de overflow em desktop/TV.

**Rollback:** a fachada continua operando somente com `pt-BR`; catálogos adicionais podem ser removidos sem tocar dados do usuário.

#### NT-021 — Catálogo de comandos, atalhos e histórico de navegação

**Objetivo:** tornar cada ação executável de forma consistente por botão, atalho, menu contextual e modo TV.

**Dependências:** NT-002 e NT-020.

**Módulos prováveis:** novo `frontend/src/lib/commands`, `shortcutsStore.ts`, `navigationHistory.ts`, `App.svelte`, `Header.svelte`, modal de atalhos e `docs/08-user/SHORTCUTS.md`.

**Entrega:**

- registro tipado com ID, label traduzível, contexto, disponibilidade, atalho padrão e executor;
- `Ctrl+K` abre ou foca a busca global por referência/evento estável, inclusive quando outro campo está focado quando for seguro;
- `/` pode ser alias quando nenhum editor estiver ativo; `Ctrl+F` deixa de ser dependência oculta do fluxo;
- comandos de voltar/avançar, ajustes, sincronização, ajuda, fila e transporte do player;
- detecção de colisões e teclas reservadas durante personalização;
- uma fonte persistente de atalhos, com fallback em memória explícito;
- DevTools restrito a build de desenvolvimento;
- histórico preserva rota, pesquisa, filtros, paginação, scroll e foco recuperável.

**Aceite:** todo atalho documentado possui executor e teste; `Ctrl+K` funciona na Home, pesquisa, player e formulários; voltar à pesquisa restaura o estado anterior.

**Testes:** unitários do normalizador/colisões, E2E de cada comando, modal ativo, input em edição, histórico e persistência após reinício.

**Rollback:** comandos podem manter somente os atalhos essenciais; a navegação por botões continua usando o mesmo executor.

#### NT-022 — Filtros reativos e estados assíncronos da interface

**Objetivo:** atualizar feeds em tempo real sem acumular consultas, apagar conteúdo útil ou perder foco.

**Dependências:** NT-013, NT-020 e o contrato de cancelamento do backend.

**Módulos prováveis:** `SearchView.svelte`, `HomeView.svelte`, `SubscriptionsView.svelte`, `ChannelManagementView.svelte`, `services.ts`, novos componentes assíncronos e testes E2E.

**Entrega:**

- dependências reativas explícitas, sem ocultar filtros dentro de funções não rastreáveis;
- estado tipado `idle`, `initialLoading`, `refreshing`, `loadingMore`, `success`, `empty` e `error`;
- filtros locais aplicados imediatamente e filtros remotos com debounce/cancelamento;
- somente a resposta da geração mais recente pode alterar a view;
- paginação reiniciada quando filtros mudam e “carregar mais” preservado quando aplicável;
- períodos da Home aplicados a todas as seções cobertas ou controle ocultado onde não houver suporte;
- semântica documentada de união para seleção múltipla de datas e tipos;
- conteúdo anterior preservado durante refresh com indicador “Atualizando…”;
- componentes leves de skeleton, progresso inline, vazio, erro persistente e retry;
- `aria-busy`, `role=status`, `aria-live` moderado e foco preservado.

**Aceite:** alterar filtros dispara no máximo uma consulta remota ativa; resultado antigo permanece utilizável até o novo chegar; Home, inscrições, busca e canais respondem sem refresh manual.

**Testes:** cliques rápidos, resposta fora de ordem, cancelamento de yt-dlp, troca de rota/perfil, reset de página, restauração de filtros, loading/vazio/erro/retry e acessibilidade.

**Rollback:** filtros remotos podem retornar temporariamente ao submit explícito sem remover o estado assíncrono compartilhado.

#### NT-023 — Seletor de perfis e contas no header

**Objetivo:** oferecer troca e criação de perfis no ponto de uso sem duplicar o gerenciamento completo dos Ajustes.

**Dependências:** NT-005, NT-020 e comandos explícitos por `profile_id`.

**Módulos prováveis:** `internal/domain`, `internal/services`, bindings, novo store de perfis/contas, `Header.svelte`, `LoginModal.svelte`, `SettingsView.svelte` e E2E de autenticação.

**Entrega:**

- DTO `ProfileSummary` sem segredos com perfil, tipo, e-mail de apresentação e estado da credencial;
- store único para header, login e Ajustes;
- switcher rápido com perfil ativo, trocar, adicionar conta, criar guest e abrir gerenciamento;
- ações de login, logout, revogação e exclusão recebem o `profile_id` alvo, sem depender de troca implícita;
- perfil pendente só é confirmado após OAuth bem-sucedido ou é limpo no cancelamento;
- troca de perfil cancela operações escopadas e recarrega stores pessoais de modo coordenado;
- a política de manter ou interromper o player na troca fica explícita e testada.

**Aceite:** alternar rapidamente entre duas contas e guest não cruza token, cookies, fila, favoritos ou estado visual pessoal; header e Ajustes mostram o mesmo estado.

**Testes:** concorrência durante login/logout, perfil inexistente, Keyring indisponível, cancelamento do OAuth, restart e E2E do switcher por teclado/TV.

**Rollback:** o botão do header volta a abrir o gerenciamento completo, mantendo store e operações explícitas por perfil.

#### NT-024 — Importação nativa da configuração OAuth do aplicativo

**Objetivo:** permitir BYO OAuth sem expor Client Secret à SPA, ao bridge ou aos logs.

**Gate obrigatório:** P-007 aceita e ADR publicada. Se a proposta for rejeitada, esta fatia termina como `REJECTED` e o provisionamento externo atual permanece.

**Dependências:** NT-004, NT-009 e NT-020.

**Módulos prováveis:** `internal/auth`, novo importador de configuração, seletor nativo Wails, inventário de credenciais, Ajustes, threat model e testes de segurança.

**Entrega:**

- aceitar somente configuração OAuth Google de aplicativo instalado;
- diálogo nativo e handle one-shot; o frontend nunca usa `FileReader` nem recebe conteúdo/path completo;
- abertura sem seguir symlink, `fstat`, owner/permissões e limite aproximado de 64 KiB;
- parser estrito, sem campos desconhecidos/duplicados/trailing, endpoints Google allowlisted e redirect loopback;
- rejeitar tokens de usuário, cookies, bloco `web`, endpoint customizado e payload genérico;
- escrita atômica em diretório `0700`, arquivo `0600`, revalidação e rollback;
- resposta limitada a `source_kind`, estado, escopo global e erro sanitizado;
- auditoria sem conteúdo secreto ou path completo.

**Aceite:** Client ID, Client Secret, token, cookies e path nunca aparecem em RPC, diagnóstico, backup ou log; ambiente externo com precedência é indicado sem falsa confirmação de instalação.

**Testes:** fuzz do parser, endpoint malicioso, symlink, owner/permissão, payload grande, replay do handle, Origin externo/vazio, interrupção de escrita, precedência e rollback.

**Rollback:** remover o arquivo gerenciado restaura a fonte externa anterior; contas e tokens de usuários não são modificados.

#### NT-025 — Seleção reutilizável e gerenciamento de canais

**Objetivo:** tornar multiseleção clara no desktop e eficiente por teclado/D-Pad, sem conflito com reprodução.

**Dependências:** NT-020 a NT-022.

**Módulos prováveis:** `VideoCard.svelte`, `VideoGrid.svelte`, `LibraryView.svelte`, `ChannelManagementView.svelte`, componentes de seleção e APIs em lote.

**Entrega:**

- contrato reutilizável `selectionMode`, `selected` e mudança de seleção no card;
- input nativo com alvo mínimo de 44×44 px, check, overlay, borda e estado textual/ARIA;
- modo explícito de seleção e barra persistente com contador, selecionar visíveis e estado indeterminado;
- `Enter` reproduz no modo normal e `Espaço` seleciona no modo de seleção;
- canal como um foco principal em TV, com avatar, status e ações secundárias;
- filtros de canais em painel recolhível e dock de ações em lote;
- operações backend em lote para favorito, pasta, tags e desinscrição;
- confirmação resumida para ações destrutivas e restauração de foco após remoção.

**Aceite:** vídeos de biblioteca e canais são selecionáveis sem mouse; checkbox nunca inicia playback; 500 canais continuam navegáveis sem travamento perceptível.

**Testes:** seleção por mouse/teclado/D-Pad, selecionar visíveis, exclusão/desinscrição, atualização da lista, foco, dark/light/TV e limites de volume.

**Rollback:** cada view mantém seleção local e pode ocultar a barra em lote sem alterar os comandos backend.

#### NT-026 — Menu contextual completo por entidade

**Objetivo:** oferecer as mesmas ações para a mesma entidade em todas as telas do NanoTube, excluindo IPTV.

**Dependências:** NT-021 e NT-025.

**Módulos prováveis:** catálogo de comandos, novo `ContextMenuHost`, trigger reutilizável, `ContextMenu.svelte`, cards, busca, biblioteca, playlists, canais e fila.

**Entrega:**

- matriz de capacidades para vídeo, canal, playlist, fila, histórico, favorito e resultado de pesquisa;
- um resolver de ações visíveis, habilitadas, marcadas e destrutivas por entidade/contexto;
- menu compartilhado com ícone, atalho, separador, estado e confirmação quando necessária;
- botão direito, tecla Menu e `Shift+F10` em todos os gatilhos cobertos;
- setas, Home, End, Enter, Espaço, Escape e retorno do foco;
- `aria-haspopup`, estado expandido e relação gatilho/menu;
- action sheet central para TV usando os mesmos comandos;
- “adicionar/criar playlist” e “salvar/adicionar playlist sem duplicados” permanecem operações únicas do domínio.

**Aceite:** a mesma entidade oferece ações equivalentes em Home, busca, inscrições, biblioteca, canal e fila; controles convencionais e formulários não recebem menu artificial.

**Testes:** matriz por entidade, todos os métodos de abertura, foco/overlay, ação indisponível, async duplo, confirmação destrutiva e D-Pad.

**Rollback:** ações rápidas permanecem disponíveis; o host pode ser desativado sem duplicar lógica de domínio.

#### NT-027 — Gerenciador de foco e experiência 10-foot

**Objetivo:** transformar o modo TV em uma apresentação completa e previsível, não apenas uma classe visual.

**Dependências:** NT-016 e NT-020 a NT-026.

**Módulos prováveis:** `SpatialNav.ts`, novo `FocusManager`, store de modo de interação, layout, modais, menus, player, tokens CSS e suíte E2E TV.

**Entrega:**

- modos de interação desktop, teclado e TV com preferência persistida por perfil e `?tv=1` como override;
- escopos empilháveis para rota, modal, menu, action sheet e player;
- zonas e vizinhos determinísticos, memória de foco/scroll por rota e restauração por entidade;
- controles nativos e widgets compostos não têm setas interceptadas enquanto são editados;
- hierarquia Voltar: fechar overlay, fechar painel/player, voltar ao detalhe, histórico e confirmação de saída;
- D-Pad e analógico com deadzone/repetição, botões A/B/Menu e polling somente quando necessário;
- tokens TV para tipografia, alvos, espaçamento, overscan, foco e movimento reduzido;
- header, sidebar, canais, playlists, biblioteca, fila, busca e player adaptados sem duplicar domínio;
- nenhum comando essencial depende de hover.

**Aceite:** Home → busca/canal → player → fila → voltar funciona sem mouse; inputs/selects continuam editáveis; foco nunca fica oculto ou perdido após refresh.

**Testes:** teclado, controle remoto e gamepad; focus trap/restauração; 1280×720 e 1920×1080; dark/light; reduced motion; regressão visual e orçamento de CPU ociosa.

**Rollback:** o modo desktop permanece independente; o motor TV pode ser desativado por preferência sem alterar dados ou comandos.

### M6 — Homologação e release

#### NT-017 — Matriz de qualidade e hardware

**Objetivo:** provar compatibilidade, estabilidade e consumo antes da homologação.

**Módulos prováveis:** `docs/04-quality/HARDWARE_BENCHMARKS.md`, `Taskfile.yml`, fixtures de playback, suíte E2E e scripts de diagnóstico.

**Entrega:** executar um corpus público e versionado, registrar app/runtime/WebKit/driver/kernel, coletar tempo até primeiro frame, rebuffer, frames descartados, CPU e RSS/PSS da árvore completa.

Executar corpus sem credenciais privadas contendo:

- muxado 360p, adaptativo 720p/1080p, 1440p/4K quando disponível;
- H.264, VP9, AV1, 30/60 FPS, SDR/HDR;
- VOD curto/longo, live, estreia, áudio separado e somente áudio;
- troca de qualidade, seek repetido, miniplayer, suspensão e retomada;
- PO Token ausente/válido/expirado, URL expirada e headers distintos;
- conteúdo público, restrito sem acesso e restrito com conta de teste autorizada;
- X11/Wayland, Celeron, Intel moderno e pelo menos uma GPU AMD.

O corpus guarda somente IDs públicos apropriados para teste; cookies e contas nunca entram no repositório.

**Aceite:** três aquecimentos e pelo menos 30 execuções por cenário local crítico; nenhum P0/P1 de playback aberto; 720p30 cumpre o orçamento no Celeron; resultados fora do orçamento possuem decisão explícita de bloqueio ou redução de suporte.

**Testes:** repetibilidade do harness, redaction, ambientes X11/Wayland, troca de rede, suspensão/retomada e comparação com o baseline NT-001.

**Rollback:** manter a versão estável anterior e a política de qualidade Compatível; não promover hardware/formato reprovado como suportado.

#### NT-018 — Release candidate e rollback

**Aceite final:**

- `task verify` limpo;
- testes de segurança negativos e fuzz sem regressão;
- budgets registrados no hardware de referência;
- packages instalados e atualizados em VMs limpas;
- runtime anterior restaurado em exercício de rollback;
- migração de banco testada com backup real redigido;
- documentação de usuário, troubleshooting, segurança, distribuição e Atlas atualizadas;
- nenhum risco P0/P1 com status `Open` no registro de riscos.

## 7. M7 — Spikes exploratórios

### NT-X01 — Shaka Player

Executar somente depois de NT-008. Comparar contra HLS.js e o par `<video>/<audio>` atual:

- compatibilidade MSE no WebKitGTK suportado;
- necessidade de sintetizar manifesto ou proxy adicional;
- drift, seek, ABR e troca de faixa;
- RAM, CPU, bundle lazy e tempo até primeiro frame;
- manutenção e comportamento com headers/PO Token.

**Gate de adoção:** melhora mensurável de estabilidade ou consumo sem exigir SABR próprio. Se o ganho não for claro, encerrar o spike e manter HLS.js.

### NT-X02 — mpv externo

Executar somente se, após NT-017, o WebView continuar incapaz de reproduzir uma classe importante de formatos suportados pelo runtime.

**Gate de adoção:** fallback opcional, processo controlado, IPC tipado, licença/distribuição auditada e nenhuma promessa de miniplayer/DSP integrado. Incorporar libmpv exige outro ADR e não faz parte deste plano.

## 8. ADRs e documentação por marco

Antes de estabilizar as respectivas fatias:

1. superseder ADR-010/017 com política de falhas terminais e anônimo-primeiro;
2. superseder ADR-021 com política de runtime versionado e fallback do sistema;
3. registrar ADR da `StreamSession`/proxy local e sua fronteira SSRF;
4. atualizar ADR-018 para Data API-first quando autenticado;
5. atualizar ADR-014 somente depois que o cache de thumbnail existir de fato;
6. registrar a importação OAuth nativa somente se P-007 e NT-024 forem aprovadas;
7. registrar o catálogo compartilhado de comandos/foco conforme P-008 antes de estabilizar NT-021/026/027;
8. registrar catálogos e códigos de erro conforme P-009 antes de concluir NT-020;
9. registrar Shaka/mpv apenas se o spike for adotado.

O índice de ADRs também deve registrar `proposed`, `accepted` e `superseded`, evitando que um ADR antigo continue parecendo vigente depois da confirmação de P-001 a P-009.

Documentos de comportamento atual não devem antecipar a implementação. Cada fatia atualiza no mesmo PR os documentos que governa.

## 9. Riscos e gatilhos de reavaliação

| Risco | Controle | Gatilho de reavaliação |
|---|---|---|
| YouTube muda PO Token/SABR | runtime desacoplado e corpus de smoke | duas quebras críticas no mesmo mês |
| Proxy local amplia SSRF | sessão opaca, DNS/IP pinado, allowlist e fuzz | qualquer bypass de destino/redirect |
| Runtime redistribuído amplia supply chain | assinatura, hash, SBOM, versão mínima e rollback | advisory crítico ou componente sem manutenção |
| Data API consome quota | cache, cancelamento, singleflight e busca explícita | quota diária insuficiente para uso normal |
| Codec automático aumenta CPU | MediaCapabilities e frames descartados | Celeron não sustenta 720p30 H.264 |
| WebKit varia entre distros | pacotes por ABI e matriz limpa | incompatibilidade em distro oficialmente suportada |
| OAuth impede distribuição ampla | projeto de produção e verificação antecipada | escopo reclassificado ou auditoria exige redesign |
| Distribuição conflita com termos ou licenças | revisão jurídica/política, notices e nenhuma redistribuição de mídia | mudança nos termos, licença incompatível ou exigência do provedor |
| Importação OAuth leva segredo ao WebView/RPC | seletor nativo, handle one-shot, parser estrito e redaction | qualquer valor secreto ou path completo observado fora do backend |
| Cookies atravessam perfis | consentimento e metadata por `profile_id`, guest sem herança | sessão A usada após ativar perfil B/guest |
| Navegação TV perde foco ou bloqueia formulários | escopos, zonas, memória por rota e testes 10-foot | foco inalcançável, invisível ou seta interceptada em input/menu |
| Catálogo de comandos diverge da UI | executor único e matriz por entidade | ação com semântica diferente entre botão, menu, atalho ou TV |
| Internacionalização quebra layouts/estado | fachada estável, pseudolocalização e lazy locale | troca de idioma reinicia view/player ou gera overflow crítico |
| Shaka/libmpv expandem demais o produto | spikes isolados com gate e rollback | dependência exige SABR próprio ou UI paralela |

## 10. Estado de conclusão

O programa só termina quando NT-001 a NT-027 estiverem `DONE` ou quando uma decisão registrada remover formalmente uma fatia. NT-024 pode terminar como `REJECTED` se P-007 não passar pelos gates, preservando o provisionamento externo seguro. NT-X01 e NT-X02 podem terminar como `REJECTED` com evidência; rejeitar um spike é um resultado válido.
