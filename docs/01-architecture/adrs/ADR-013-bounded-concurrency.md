# ADR-013: Concorrência Delimitada com errgroup e singleflight

## Contexto
Operações de rede e I/O não devem criar goroutines ilimitadas nem processar requisições idênticas em paralelo.

## Decisão
Utilizar `golang.org/x/sync/errgroup` com limite de concorrência (`SetLimit`) para fan-out de sincronização e `golang.org/x/sync/singleflight` para coalescer requisições idênticas (ex: download de mesma miniatura ou resolução de mesmo vídeo).
