---
id: concurrency
status: canonical
---

# Concorrência e Cancelamento — NanoTube Web

## 1. Diretrizes de Concorrência

1. Toda operação de backend aceita `context.Context` e respeita cancelamentos de requisições do frontend;
2. `errgroup.Group` com `.SetLimit(N)` é utilizado para fan-out delimitado;
3. `singleflight.Group` previne download duplicado simultâneo de uma mesma miniatura ou resolução de mesmo stream;
4. Nenhuma goroutine é órfã; todo processo possui lifecycle monitorado.
