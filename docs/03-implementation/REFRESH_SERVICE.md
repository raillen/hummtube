---
id: refresh-service
status: canonical
---

# Serviço de Atualização Incremental (Refresh)

## 1. Funcionamento
O serviço em `internal/sync/service.go` busca novos envios para todos os canais inscritos usando `errgroup` com limite de concorrência configurado (padrão: 3 workers). A Home mostra primeiro o catálogo local e solicita uma atualização em segundo plano na primeira montagem da sessão, salvo `sync.refresh_on_startup=false`. Chamadas simultâneas da mesma janela compartilham a promessa de atualização.

## 2. Invariantes
- Se um canal falhar por erro de rede ou quota, os outros continuam normalmente;
- Cada canal grava os vídeos antes de avançar seu marcador incremental; falha de gravação mantém os vídeos elegíveis para nova tentativa;
- A integração atual usa a resposta assíncrona de `RefreshSubscriptions`; eventos Wails `sync:progress`/`sync:completed` ainda não estão conectados;
- Após sucesso, o bridge frontend emite `nanotube-catalog-refreshed` e a Home substitui seu modelo completo, descartando respostas anteriores atrasadas; o catálogo existente permanece visível durante a atualização;
- Erros por canal permanecem em `Stats.ChannelErrors`; não há garantia de sincronização completa quando somente alguns canais falham. A gravação é incremental por canal, não uma transação global.
