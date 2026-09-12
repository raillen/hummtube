---
id: player
status: canonical
---

# Implementação do Player — NanoTube Web

## 1. Arquitetura do Player Web/Wails

O Player é implementado em `frontend/src/lib/components/player/VideoPlayer.svelte`. Ele abstrai o elemento HTML5 `<video>` e integra `HLS.js` para reprodução contínua e adaptativa.

```text
┌────────────────────────────────────────────────────────┐
│ VideoPlayer.svelte (Svelte 4)                          │
│ ┌────────────────────────────────────────────────────┐ │
│ │ <video> Element (Hardware Acceleration no WebView) │ │
│ └────────────────────────────────────────────────────┘ │
│ ├─ Controles Reativos (Play/Pause, Seek, Vol, Full)  │
│ ├─ Qualidade Automática, Variantes e Somente Áudio   │
│ ├─ DSP de Áudio (AudioContext Gain, Pitch, Speed)    │
│ ├─ Seletor de Legendas & Faixas de Áudio             │
│ └─ Temporizador de Sono (Sleep Timer)                │
└───────────────────────────▲────────────────────────────┘
                            │ PlaybackPlan (Streams, Variants, Headers)
               PlayerService.ResolveMedia()
```

Para `resolved-media`, os streams chegam ao WebView como URLs locais opacas do
proxy (`/api/media/<token>` no servidor web e
`http://127.0.0.1:<porta>/api/media/<token>` no desktop). O desktop usa uma
porta efêmera de loopback porque o protocolo customizado `wails://` do
WebKitGTK não preserva de forma confiável todos os headers HTTP usados pelo
pipeline de mídia; isso pode transformar MP4/H.264 válido em
`MEDIA_ERR_SRC_NOT_SUPPORTED`. O servidor local atende somente mídia com token
curto, preserva CORS, headers de negociação e Range e não publica RPC. Manifests
HLS também têm playlists, segmentos e chaves reescritos para o mesmo origin,
sem deixar o WebView buscar recursos externos diretamente. O proxy é somente
para o fluxo NanoTube; IPTV mantém seu contrato separado.

## 2. Recursos de Reprodução

- **Aceleração por Hardware**: Renderização direta com suporte do WebView (VA-API / GPU);
- **Controle de Velocidade**: 0.25x a 2.0x com preservação de tom de áudio (*pitch correction*);
- **Controle de Ganho & DSP**: Boost digital via Web Audio API (`GainNode`) de -12dB a +12dB;
- **Persistência de Progresso**: Salva posição a cada 5 segundos no SQLite e restaura automaticamente.
- **Miniplayer contínuo**: Alternar entre player expandido e miniplayer muda somente o layout e os controles; a mesma instância de `<video>` permanece montada, preservando imagem, áudio e posição. O miniplayer pode circular pelos quatro cantos com um botão dedicado; a posição é global, persistida e restaurada na próxima sessão sem recarregar o vídeo.
- **Qualidade de reprodução**: O botão de qualidade abre um menu com Automática, resoluções progressivas, resoluções adaptativas e Somente áudio. Uma variante que falha retorna para uma resolução combinada segura sem perder a posição atual.
- **Controle integrado e persistente**: qualidade usa o mesmo padrão visual dos demais controles inferiores. A barra inferior e o botão permanecem visíveis durante a resolução; antes do plano chegar, o botão mostra **Carregando** e fica desabilitado em vez de desaparecer. O menu possui `menuitemradio`, estado selecionado, foco por teclado e uma explicação quando o provedor devolve poucas resoluções.
- **Somente áudio**: quando o resolver encontra uma faixa direta compatível com o WebView, o seletor oferece **Somente áudio** e preserva posição, velocidade, volume e fila no mesmo elemento de mídia.
- **Carregamento HLS sob demanda**: o decoder `hls.js` é importado somente quando o plano selecionado contém um stream `.m3u8`. Streams MP4/M4A não carregam esse decoder; navegadores com suporte HLS nativo continuam podendo reproduzir a lista mesmo quando o módulo opcional não está disponível.
- **Tela cheia real**: o botão usa a Fullscreen API no elemento raiz do player e acompanha `fullscreenchange`. Se a API estiver indisponível ou for recusada pelo WebView, ativa um overlay de tela cheia dentro da aplicação; `Escape` encerra esse fallback.
- **Fila persistente**: vídeos podem ser adicionados a partir dos cartões, do cabeçalho, de playlists ou da página dedicada. A ordem e o estado ficam no SQLite, isolados por perfil e limitados a 100 itens. O evento `ended` marca o item atual e inicia o próximo pendente; remover após tocar é uma preferência explícita.
- **Gestão da fila**: a página **Fila** e a janela rápida oferecem drag-and-drop, mover para cima/baixo, limpeza seletiva, reabertura de itens tocados e conversão transacional em playlist. A fila segue em uma aba própria do painel do player, com layout de 10-foot UI.
- **Contexto do vídeo**: o painel de informações tem abas Sobre/Fila, mostra a descrição completa persistida no SQLite e abre a página dedicada de vídeos ou playlists do canal sem desmontar o player; a reprodução continua no miniplayer.
- **Retorno rápido**: o controle **Voltar à interface** reduz o player ao miniplayer sem desmontar o elemento `<video>`; fechar o player encerra a sessão, mas preserva a fila para a próxima abertura.

## 3. Resolução compatível com o player web

O serviço usa `NewWebCascadingResolver`: `yt-dlp/mweb` com PO Token válido → `yt-dlp/android` → `yt-dlp/ios` → Invidious. O primeiro estágio só entra na cadeia quando o provider configurado e o plugin são detectados. A estratégia `mpv-ytdl-hook` pertence ao player nativo e não pode ser entregue ao `<video>`, pois não produz `primary.url`.

O plano web pode conter uma faixa combinada/HLS ou um par adaptativo. No par adaptativo, o player mantém um único `<video>` visível e um `<audio>` auxiliar, sincronizados em play, pause, seek, velocidade e correção periódica de drift. Isso libera 720p, 1080p e superiores quando o yt-dlp ou Invidious realmente expõe essas URLs. Se a faixa adaptativa falhar, o player volta para a melhor variante combinada disponível.

No modo automático, o resolver normaliza a escolha do provedor para priorizar
vídeo MP4/H.264 e áudio M4A/AAC. O formato padrão do yt-dlp pode ser AV1/VP9 +
Opus, combinação que depende dos plugins GStreamer instalados e não funciona em
todo WebKitGTK nem no hardware Linux de referência. Quando existe H.264, o menu
não anuncia variantes AV1/VP9 como universalmente compatíveis. Se o elemento de
mídia ainda devolver `MEDIA_ERR_SRC_NOT_SUPPORTED` (código 4), o player tenta
uma variante combinada diferente e registra em memória as URLs que falharam
para não alternar em loop entre os mesmos streams.

Atualizações de tempo não reescrevem volume ou velocidade quando os valores já estão aplicados. A sincronização adaptativa evita seek redundante quando as faixas já estão alinhadas (tolerância de 10 ms em alinhamento explícito; correção periódica acima de 300 ms).

A seleção manual de HLS e o retorno a Automática usam `loadLevel`, sem limpar o buffer existente; a qualidade visual muda depois dos segmentos já armazenados. O carregamento em andamento pode ser interrompido pela biblioteca. Os limites de buffer permanecem nos valores existentes, sem ajuste específico para TV não medido.

Selecionar uma opção com o mesmo par de URLs de vídeo/áudio não recarrega a mídia. Uma URL diferente exige recarga no mesmo elemento; seleções rápidas preservam a posição pendente até os metadados chegarem. Promessas de reprodução e callbacks HLS de gerações anteriores são ignorados. A pausa permanece controlada pelo store, sem `autoplay` nativo que possa desfazê-la durante uma troca. Se a importação do decoder falhar, o player tenta HLS nativo quando disponível; caso contrário, pausa e informa o erro.

Validação frontend: `npm run check` e `npm run test:e2e -- --config playwright.vite.config.ts e2e/player-modals.spec.ts --workers=1`, com Vite ativo na porta 5173. A suíte cobre seleção HLS com a mesma fonte, trocas rápidas, pausa, posição e seleção redundante. As rotas de mídia são controladas pelos testes: não comprovam continuidade audiovisual com segmentos reais nem compatibilidade WebKitGTK. Expiração de tokens do proxy depende da correção backend separada.

O componente só recarrega a mídia quando `primary.url` muda. Atualizações de tempo, volume ou estado não recriam o stream. Respostas atrasadas de uma resolução anterior são descartadas por geração, e o elemento `<video>` permanece montado ao alternar para o miniplayer.

O resolver expõe `audio_only` somente para formatos carregáveis diretamente: M4A/MP4 com AAC, WebM com Opus/Vorbis, MP3 ou Ogg, entregues por HTTP(S) ou HLS. Protocolos segmentados que exigiriam Shaka, FFmpeg ou transcodificação não são oferecidos. Quando nenhuma faixa compatível existe, a opção fica ausente e o vídeo continua no modo automático.

O YouTube pode omitir formatos adaptativos por SABR ou exigir PO Token. Nessa situação, o menu mostra somente as resoluções realmente reproduzíveis, frequentemente 360p. Atualizar o yt-dlp, manter um runtime JavaScript detectável e configurar o PO Token Provider aumenta a chance de obter as resoluções maiores; o player não inventa opções sem uma URL válida.

Em uma instalação nova, o `yt-dlp-ejs` precisa acompanhar a versão do yt-dlp para que o runtime resolva desafios de assinatura. Como alternativa explícita, **Configurações → Sistema → Reprodução e qualidade** permite autorizar o download do solver EJS pelo próprio yt-dlp; essa opção é persistente, desligada por padrão e deve ser habilitada conscientemente.

No miniplayer, os controles são uma camada sobre a mesma mídia. Eles aparecem tanto ao posicionar o ponteiro quanto ao focar qualquer botão pelo teclado; a camada não substitui o vídeo por thumbnail e não interrompe o áudio.

Erros de reprodução exibem o motivo legível (código de mídia traduzido) com botão **Tentar novamente** (`player-retry-button`): o retry limpa a URL da lista de formatos falhos, preserva a posição pendente e recarrega o mesmo stream (HLS usa `startLoad` com contador de recuperação zerado).

O progresso é salvo no serviço correspondente à origem: YouTube em `playback_progress`, IPTV/VOD em `iptv_playback_progress`. O `AudioContext` só é conectado quando o ganho DSP é realmente ativado e só depois de conseguir `resume()`; em stream remoto sem CORS garantido o controle fica desabilitado, preservando a rota de áudio nativa e evitando silêncio.

Os controles de tray não mantêm um segundo estado de mídia. O backend emite comandos tipados para a janela (`play_pause`, anterior, próximo, seek, volume, mudo e abrir fila) e o frontend os direciona para a única instância do player. No Linux, o tray só é criado quando a sessão expõe `org.kde.StatusNotifierWatcher`; sem um host SNI/AppIndicator, a integração é desativada de forma explícita e a janela mantém o fechamento normal.
