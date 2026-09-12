---
id: player-loading-pipeline
status: canonical
---

# Pipeline de Loading do Player e Feedback Visual

## 1. Contexto

Usuários reportaram dois problemas na abertura de um vídeo:

1. **Demora perceptível** entre o clique e o início da reprodução.
2. **Vídeo anterior continua tocando** (áudio e vídeo) até o novo resolver e iniciar.
3. **Nenhum indicador visual** de que o novo vídeo está em carregamento.

A auditoria do pipeline (`VideoPlayer.svelte`, `playerStore.ts`) confirmou as causas:

- `playerStore.loadVideo` apenas marca `isLoading: true`, mas o `<video>` mantém o `src` anterior. O `loadPlaybackPlan` só acontece depois de `PlayerService.resolveMedia` retornar, então o vídeo anterior só é pausado quando o novo plano já está pronto.
- Nenhum componente escuta `isLoading` para mostrar overlay.
- Não há `poster` reativo: tela preta entre clique e `canplay`.
- `HLS.js` eventos de progresso (`progress`, `stalled`, `canplay`) não são expostos para UI.
- Não há cancelamento de transferências in-flight quando o usuário troca de vídeo.

## 2. Pesquisa Externa

### 2.1 web.dev — Media Source Extensions

Referência: https://web.dev/articles/media-mse-basics

MSE move segmentos de áudio/vídeo para o `<video>`. Pontos relevantes:

- Antes de tocar, o pipeline precisa de `HAVE_FUTURE_DATA` (3) — pelo menos dois frames disponíveis.
- Cada `appendBuffer()` no `SourceBuffer` deve checar `updating` antes de nova escrita.
- `MediaError.message` é a fonte primária para diagnóstico de falhas.

### 2.2 MDN — `<video>` element

Referência: https://developer.mozilla.org/en-US/docs/Web/HTML/Element/video

Eventos críticos para o pipeline de loading, em ordem:

```
loadstart → progress → loadedmetadata → loadeddata → canplay → canplaythrough → playing
                                  ↑ stalled    ↑ suspend ↑
```

- `progress` dispara periodicamente durante o download.
- `stalled` indica que o download parou de progredir.
- `suspend` indica que o download foi suspenso.
- `waiting` é equivalente a "buffering".
- `canplay` é o ponto mínimo para iniciar reprodução sem buffering.
- `canplaythrough` garante que o vídeo pode ser reproduzido até o final sem interrupção.

Atributos relevantes:

- `preload="metadata"` baixa apenas metadados antes da reprodução.
- `preload="auto"` baixa tudo (usar com parcimônia em hardware modesto).
- `poster` exibe uma imagem antes do `currentSrc` estar pronto.
- `crossorigin` é necessário quando o stream vem de origem diferente (proxy local exige `anonymous`).

### 2.3 MDN — HTMLMediaElement.readyState

Referência: https://developer.mozilla.org/en-US/docs/Web/API/HTMLMediaElement/readyState

Cinco estados constantes:

```
HAVE_NOTHING      (0)  nada carregado
HAVE_METADATA     (1)  metadados prontos
HAVE_CURRENT_DATA  (2)  frame atual pronto
HAVE_FUTURE_DATA   (3)  ≥ 2 frames prontos
HAVE_ENOUGH_DATA   (4)  pode reproduzir até o fim
```

A melhor prática é mostrar skeleton a partir de (0) e conteúdo a partir de (1).

### 2.4 Prática comum em YouTube, Netflix e MUX

Os três serviços compartilham um padrão:

1. Mostram `poster` (frame título) imediatamente.
2. Spinner central durante `loadstart` → `canplay`.
3. `seek bar` determinístico com buffer esperado.
4. Cancelam transferências in-flight via `AbortController` quando o usuário troca de vídeo.
5. Mantêm thumbnail anterior com overlay em vez de tela preta.

## 3. Causa Raiz e Decisões

| Problema | Decisão |
|----------|---------|
| Vídeo antigo continua tocando | Pausar e resetar `videoElement.src` antes do `resolveMedia` no `loadVideo` |
| Tela preta durante loading | Adicionar `poster` reativo do novo vídeo antes do `resolveMedia` |
| Sem feedback visual | Renderizar `LoadingOverlay` com spinner + poster borrado + título |
| Sem indicador de progresso do buffer | Escutar eventos `loadstart`/`progress`/`stalled`/`canplay` e expor `bufferingProgress: 0..1` |
| Requests zumbis ao trocar | Usar `AbortController` para cancelar fetches in-flight |
| Acesso a método depreciado | Não usar `HTMLMediaElement.audioTracks`/`videoTracks` (já removidos da especificação em favor de MediaTrack APIs) |

## 4. Plano de Implementação

### P0 — Core (Sprint 1)

- **P0-1** Pausar e resetar `videoElement.src` no início do `loadVideo`.
- **P0-2** Adicionar `poster` da thumb do novo vídeo no `loadVideo` (antes de `resolveMedia`).
- **P0-3** Emitir evento `player:loading` com `videoId` para a UI.
- **P0-4** Renderizar `LoadingOverlay` (spinner, poster borrado, título, buffer) sobre o player.
- **P0-5** Escutar `loadstart`/`progress`/`stalled`/`suspend`/`canplay` e expor `bufferingProgress: 0..1` no store.
- **P0-6** Usar `AbortController` para cancelar fetch de poster e resolução anterior.

### P1 — Otimizações (Sprint 2)

- **P1-1** Pré-resolver `playbackPlan` em `hover`/`focus` do card (300 ms antes do clique).
- **P1-2** `preload="auto"` para HLS, `preload="metadata"` para MP4.
- **P1-3** Verificar se backend Go serializa `ResolveMedia` + `RememberVideo`; paralelizar.
- **P1-4** Cache LRU de `PlaybackPlan` por `videoId` no frontend (TTL 5 min).
- **P1-5** Confirmar keep-alive HTTP/2 no proxy local.
- **P1-6** Buffer HLS.js mínimo de 5s para evitar buracos.

### P2 — UX Aprimorada (Sprint 3)

- **P2-1** Skeleton com `video.title` e `video.channel_title`.
- **P2-2** Mostrar tempo decorrido no loading.
- **P2-3** Botão "Cancelar" durante `ResolveMedia` quando tempo > 5s.
- **P2-4** Distinguir três estados: `resolve` (rede) / `buffer` (dados) / `ready`.

## 5. Tradeoffs

| Decisão | Opção escolhida | Justificativa |
|---------|-----------------|---------------|
| Cancelar poster ao trocar | `AbortController` em fetch | Reduz requests zumbis |
| Cache de plano | TTL 5 min em memória | Reduz TTFI sem persistir tokens opacos no disco |
| Pré-resolução em hover | Habilitar para perfis autenticados e guest (bypass só se Data API forçada por perfil) | Custo de quota é o mesmo para todos |
| Loading overlay | Spinner + título + poster + barra de buffer | Reduz o "tela preta" psicológico |
| Mostrar buffer real do HLS | Sim | Transparência para o usuário |

## 6. Rastreabilidade de Riscos

- **R-001 (quebra de extração YouTube)**: pré-resolução em hover pode amplificar carga no yt-dlp. Mitigação: bypass em `prefers-reduced-data` e TTL curto no cache.
- **R-002 (PO Token)**: cache de plano pode retornar plano antigo após PO Token expirar. Mitigação: checar `expires_at` no momento de usar e forçar re-resolver se expirou.
- **R-006 (memória WebView)**: `preload="auto"` consome banda e memória. Mitigação: medir em hardware modesto e desabilitar se violar budget.
- **P-001 (segurança OAuth)**: cache de plano em memória não persiste, mas enquanto vivo expõe URLs do proxy local. Janela curta (5 min) reduz superfície.

## 7. Métricas de Validação

- Tempo `click → HAVE_METADATA` (medir antes e depois).
- Tempo `click → primeira frame visível`.
- Teste E2E: "click em vídeo 1 → click em vídeo 2 antes de 1 iniciar" deve pausar 1 em < 100 ms e mostrar overlay com título de 2.
- Teste E2E: overlay com `aria-busy="true"` durante `loadstart` e removido em `canplay`.
- Teste E2E: `AbortController` cancela fetch anterior (verificar com spy).

## 8. Referências

- web.dev Media Source Extensions: https://web.dev/articles/media-mse-basics
- MDN `<video>`: https://developer.mozilla.org/en-US/docs/Web/HTML/Element/video
- MDN HTMLMediaElement.readyState: https://developer.mozilla.org/en-US/docs/Web/API/HTMLMediaElement/readyState
- HLS.js API: https://hlsjs.video-dev.org/api-docs/
- Registro de Riscos: `docs/06-governance/RISK_REGISTER.md`
- Roadmap: `docs/00-product/SCOPE_AND_ROADMAP.md`
