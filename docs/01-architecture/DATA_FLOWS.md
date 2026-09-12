---
id: data-flows
status: canonical
---

# Fluxos de Dados — NanoTube Web

## 1. Fluxo de Reprodução de Mídia

```text
[Usuário clica no Card]
         │
         ▼
[Frontend Svelte: playerStore.loadVideo(id)]
         │
         ▼ (Wails v3 IPC)
[Go Backend: PlayerService.ResolveMedia()]
         │
         ▼
[CascadingResolver]
   ├── 1. ExplicitYtDlpResolver (attestation PO Token / clients android, ios, mweb)
   └── 2. InvidiousResolver (fallback com proteção SSRF)
         │
         ▼
[MediaProxy valida destino e emite token opaco]
   ├── Web: /api/media/<token>
   └── Desktop: http://127.0.0.1:<porta efêmera>/api/media/<token>
         │
         ▼ (PlaybackPlan retornado sem URL assinada remota)
[Frontend Svelte Video Player]
         ├── Carrega stream direto ou HLS.js
         ├── Inicia playback com aceleração por hardware
         └── Registra progresso a cada 5s via PlayerService.SaveProgress()
```

---

## 2. Fluxo de Sincronização Incremental (Botão Atualizar)

```text
[Usuário clica em "Atualizar"]
         │
         ▼
[CatalogService.RefreshSubscriptions()]
         │
         ├── 1. Lê canais inscritos da base SQLite local
         ├── 2. Fan-out concorrente limitado (errgroup) buscando feeds RSS / YouTube Data API
         ├── 3. Filtra vídeos já existentes no catálogo (LastKnownVideoID / PublishedAt)
         ├── 4. Grava novos vídeos no SQLite e indexa no FTS5
         ├── 5. Atualiza tabela de tópicos e afinidades de canal
         ├── 6. Roda RecommendationEngine v2 (MMR guloso)
         └── 7. Emite evento Wails `catalog:refreshed` para re-renderizar Home instantaneamente
```
