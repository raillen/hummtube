---
id: product
status: canonical
decision_class: mixed
---

# Produto — NanoTube Web

## Visão

NanoTube é um cliente desktop moderno, leve e elegante dedicado ao YouTube, construído com **Wails v3 + Svelte + Tailwind CSS + Lucide Icons + Go + SQLite**.
Ele entrega uma experiência de usuário visualmente moderna, fluida e reativa com mínimo consumo de recursos computacionais, eliminando os excessos de navegadores convencionais e clientes baseados em Electron.

### Ganhos Principais:
1. **Baixo Consumo e Alta Responsividade**: WebView nativa orquestrada por Wails v3 sem overhead de Chromium completo em processo dedicado pesado;
2. **Controle Total & Sem Distrações**: Home local própria ("Para Você"), sem anúncios, sem feeds infinitos algorítmicos opacos, com ranking explicável e personalização de tópicos;
3. **Resiliência Arquitetural**: Playback desacoplado através de `PlaybackResolver` (yt-dlp explícito, PO Token Attestation, Invidious fallback);
4. **Design Moderno & Acessível**: Interface declarativa com Svelte, Tailwind CSS, ícones Lucide, suporte a temas (dark/light) e Modo TV (10-foot UI com navegação direcional espacial).

---

## Público-Alvo

- Usuários Linux e desktop geral que buscam um cliente YouTube rápido, limpo e focado no conteúdo;
- Dispositivos modestos (~2 GB RAM) e máquinas modernas que valorizam eficiência energética e desempenho;
- Usuários que desejam organizar suas inscrições em pastas locais, criar playlists inteligentes e ter controle sobre seus dados pessoais offline;
- Usuários de Home Theater / HTPC via Modo TV com navegação por teclado / controle remoto D-Pad.

---

## Diferenciais

- **Home Própria**: Baseada exclusivamente em inscrições locais, afinidades de canais/tópicos e histórico;
- **Sincronização Incremental**: Botão **Atualizar** que busca novos vídeos em segundo plano sem congelar a UI;
- **Player Reativo Integrado**: Player HTML5/HLS moderno com controles completos (play/pause, seek, volume, ganho de áudio, velocidade 0.25x-2.0x, legendas, faixas de áudio, temporizador de sono);
- **Busca Híbrida**: Busca instantânea local (SQLite FTS5) + busca remota sob demanda com filtros avançados;
- **Playlists Inteligentes & Manuais**: Filtragem dinâmica por regras locais versionadas e organização em pastas;
- **Backup & Privacidade**: Exportação/importação de dados pessoais, snapshots atômicos criptografados (AES-256-GCM) e armazenamento seguro de tokens OAuth via PKCE e Keyring do SO.

IPTV e a experiência musical dedicada pertencem, respectivamente, aos aplicativos **NanoIPTV** e **NanoMusic**. Eles compartilham a base de engenharia no monorepo, não a navegação ou os dados do NanoTube.

---

## Success Boundaries
- Consumo de memória em repouso < 150 MB (Linux WebView);
- Latência de busca instantânea local < 15 ms via SQLite FTS5;
- Cold start < 800 ms em hardware de referência (Intel Celeron / 2 GB RAM);
- Interface 100% navegável via teclado e D-Pad (Modo TV);
- Zero telemetria invasiva ou vazamento de credenciais locais.
