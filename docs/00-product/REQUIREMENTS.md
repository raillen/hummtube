---
id: requirements
status: canonical
---

# Requisitos — NanoTube Web

## Funcionais — P0 (Essenciais)

- **FR-001** Abrir aplicação desktop moderna através de Wails v3.
- **FR-002** Reproduzir streams de vídeo via player reativo integrado com aceleração de hardware.
- **FR-003** Resolver fontes de playback através do `PlaybackResolver` (cascading fallback: yt-dlp explícito com POT Provider, Invidious e direct stream).
- **FR-004** Autenticação OAuth2 PKCE usando navegador do sistema e loopback local seguro.
- **FR-005** Sincronizar inscrições e metadados via YouTube Data API v3 oficial.
- **FR-006** Construir Home local inteligente com seções ("Para você", "Canais favoritos", "Não assistidos", "Vídeos longos").
- **FR-007** Botão **Atualizar** para sincronização incremental e assíncrona.
- **FR-008** Pesquisa híbrida instantânea (SQLite FTS5 local + busca remota sob demanda).
- **FR-009** Histórico local e retomada de posição de reprodução (*Resume*).
- **FR-010** Fila de reprodução (*Queue*) e miniplayer persistente.
- **FR-011** Cache eficiente de miniaturas com lazy loading e controle de quota em disco.
- **FR-012** Painel de diagnóstico do sistema (yt-dlp, runtime JS, codecs, aceleração).

## Funcionais — P1 (Experiência Avançada)

- **FR-013** Playlists locais manuais (criar, editar, reordenar por drag-and-drop, tags, cores).
- **FR-014** Playlists inteligentes com regras versionadas e predicados dinâmicos (título, canal, duração, data, live).
- **FR-015** Organização de canais em pastas locais e canais favoritos.
- **FR-016** Notas e marcadores de tempo (*Bookmarks*) em vídeos.
- **FR-017** Exportação/importação de dados pessoais e snapshots atômicos com criptografia AES-256-GCM.
- **FR-018** DSP de Áudio: controle de ganho/boost, velocidade variável (0.25x–2.0x), normalização de volume.
- **FR-019** Seleção de legendas e faixas de áudio dinâmicas.
- **FR-020** Modo TV (10-Foot UI) com navegação direcional espacial por teclado/D-Pad.
- **FR-021** Histórico, supersedido por D-033: IPTV pertence ao executável NanoIPTV e não à interface NanoTube.
- **FR-022** Temporizador de sono (*Sleep Timer*) configurável.
- **FR-023** Temas visuais (Escuro/Claro/Alto Contraste) via Tailwind CSS e Lucide Icons.

## Não Funcionais

- **NFR-PERF-001** Inicialização rápida e pegada de memória otimizada (~100-180 MB em repouso).
- **NFR-PERF-002** Apenas uma sessão de mídia decodificando ativamente.
- **NFR-RESP-001** Nenhuma operação de I/O, rede ou subprocesso bloqueia a thread principal da UI.
- **NFR-SEC-001** Tokens OAuth e credenciais sensíveis armazenados exclusivamente no Keyring do sistema operacional; nunca logados.
- **NFR-SEC-002** Proteção contra SSRF na resolução de streams e requisições Invidious.
- **NFR-DATA-001** Armazenamento em SQLite com migrações atômicas via Goose.
- **NFR-DATA-002** NanoTube, NanoIPTV e NanoMusic usam bancos e namespaces de Keyring distintos.
- **NFR-SEC-003** Cada executável publica somente a allowlist RPC do próprio produto, validando conjuntamente serviço e método.
