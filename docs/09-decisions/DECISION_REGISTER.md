---
id: decision-register
status: canonical
---

# Registro de Decisões — NanoTube Web

## Decisões Confirmadas

| ID | Decisão |
|---|---|
| D-001 | Produto é cliente desktop YouTube ultra-leve para Linux/Desktop com controle e sem distrações |
| D-002 | Go é a linguagem principal do backend e serviços de domínio |
| D-003 | Wails v3 é o framework desktop adotado para o runtime |
| D-004 | Svelte 5 (Vite SPA) é a biblioteca declarativa de componentes da interface |
| D-005 | Tailwind CSS é o padrão de estilização e temas visuais |
| D-006 | Lucide Icons (`lucide-svelte`) é o conjunto exclusivo de ícones da interface |
| D-007 | Instância única de reprodução ativa por vez |
| D-008 | Playback desacoplado via `PlaybackResolver` (yt-dlp explícito, PO Token Attestation, Invidious) |
| D-009 | OAuth2 PKCE para conta do YouTube, identidade estável e refresh token isolado por perfil no Keyring; ausência do Keyring permite apenas sessão em memória com aviso |
| D-010 | Home local própria e determinística baseada no Recommendation Engine v2 (MMR guloso) sem LLM |
| D-011 | Botão Atualizar para sincronização assíncrona e incremental com `errgroup` |
| D-012 | SQLite (`modernc.org/sqlite`) com Goose migrations e FTS5 para busca textual local |
| D-013 | Player de vídeo web reativo (HTML5/HLS.js) com aceleração de hardware nativa no WebView |
| D-014 | DSP de áudio integrado (ganho -12dB a +12dB, velocidade 0.25x-2.0x com pitch correction, normalização) |
| D-015 | Modo TV (10-Foot UI) com motor de navegação direcional espacial 2D |
| D-016 | Playlists manuais, inteligentes por regras JSON versionadas, tags e organização de canais em pastas |
| D-017 | Notas e marcadores de tempo locais por vídeo |
| D-018 | Backup e exportação/importação de dados pessoais com suporte a criptografia AES-256-GCM |
| D-019 | Histórico: NanoIPTV foi inicialmente integrado ao NanoTube; supersedido por D-033 |
| D-020 | Taskfile (`go-task`) como orquestrador único de build, testes, lint e gates de verificação |
| D-021 | Binário yt-dlp externo é dependência explícita e diagnosticável; nunca é baixado silenciosamente em runtime |
| D-022 | Fila de reprodução é persistida no SQLite, isolada por perfil e convertível transacionalmente em playlist |
| D-023 | Feed de inscrições e gerenciamento de canais são workspaces separados, com paginação/filtros e ações em lote |
| D-024 | Controles da bandeja enviam comandos à instância única do player; fechar pode perguntar, minimizar ou encerrar |
| D-025 | Histórico: Modo Música foi inicialmente um workspace do NanoTube; supersedido por D-033 |
| D-026 | Last.fm é integração beta opt-in; session key fica no Keyring por perfil e scrobble é validado por limiar no backend |
| D-027 | Busca/catálogo oficial usa o SDK YouTube Data API v3 com cliente OAuth capturado por perfil e erros remotos sanitizados |
| D-028 | Página de canal é uma rota de detalhe própria; listagem remota não persiste conteúdo até uma ação explícita e views são carregadas sob demanda |
| D-029 | O decoder HLS e os ícones Lucide são carregados granularmente; HLS só entra no runtime quando um stream `.m3u8` é reproduzido |
| D-030 | O player mantém descrição completa em `videos.description`, oferece painel Sobre/Fila orientado a TV e preserva áudio nativo quando o Web Audio não pode ser desbloqueado |
| D-031 | O player web aceita variantes adaptativas com um `<video>` visível e uma faixa `<audio>` auxiliar sincronizada; o menu de qualidade usa um botão integrado e mantém fallback para stream combinado |
| D-032 | O PO Token Provider continua uma dependência explícita, mas instalações completas em diretório gerenciado são detectadas automaticamente; cookies restritos permanecem no navegador e apenas seletores `navegador+keyring` validados são persistidos |
| D-033 | NanoTube, NanoIPTV e NanoMusic são produtos executáveis independentes no monorepo transitório, com bundles, políticas RPC, bancos SQLite e namespaces de Keyring próprios; ADR-022 |
| D-034 | O gerenciador de credenciais expõe somente metadados e auditoria local por perfil; valores secretos e identificadores internos do Keyring nunca atravessam o bridge |
| D-035 | Rebranding completo para HummTube / HummSuite (HummTube, HummIPTV, HummMusic) e versão base 0.1.0, mantendo migração transparente e retrocompatibilidade com namespaces, bancos e binários legados (`nanotube-web`, `nanoiptv`, `nanomusic`) |

## Propostas em validação

Estas propostas pertencem ao plano `nanotube-release-hardening-plan` e ainda não substituem ADRs confirmados.

| ID | Proposta | ADRs afetados | Gate para confirmação |
|---|---|---|---|
| P-001 | Manter o player WebView como principal e transportar streams com `StreamSession` opaca/proxy local sem transcodificação | Novo ADR; complementa ADR-009/013 | Segurança SSRF, `Range`, CPU e matriz 720p/1080p aprovados |
| P-002 | Usar resolução anônima primeiro e sessão de navegador somente sob consentimento, isolada por perfil | Supersede ADR-010/017 | Testes negativos de cookies/fallback e UX de desativação aprovados |
| P-003 | Distribuir Playback Runtime versionado separado do ciclo da interface | Supersede ADR-021 | Manifesto, assinatura, SBOM, licença e rollback aprovados |
| P-004 | Preferir Data API para busca autenticada e yt-dlp para visitante/fallback | Atualiza ADR-018 | Latência, cache e consumo de quota aprovados |
| P-005 | Priorizar pacotes Linux nativos; AppImage/Flatpak dependem de homologação própria | Novo ADR de distribuição | Instalação limpa e matriz de dependências aprovadas |
| P-006 | Avaliar Shaka e mpv apenas por spikes reversíveis; não implementar SABR próprio | Novo ADR somente se houver adoção | Benchmark demonstra ganho material sem ampliar manutenção estrutural |
| P-007 | Permitir importação nativa e unidirecional somente da configuração OAuth do aplicativo; tokens de usuário e cookies permanecem não importáveis | Complementa D-034 e supersede parcialmente o provisionamento exclusivamente externo da ADR-023 | Parser estrito, seleção nativa, escrita atômica `0600`, redaction, fuzz e testes negativos do bridge aprovados |
| P-008 | Unificar atalhos, ações rápidas, menus contextuais e navegação TV em um catálogo tipado de comandos com escopos de foco | Complementa D-015 e D-023 | Paridade de ações por entidade, colisões de atalhos e matriz teclado/D-Pad aprovadas |
| P-009 | Orquestrar textos da interface em catálogos JSON tipados e traduzir erros conhecidos por código estável, mantendo detalhes técnicos sanitizados | Novo ADR de internacionalização e contrato de erros | Paridade de chaves, fallback, pluralização, pseudolocalização e troca de idioma sem perda de estado aprovados |
