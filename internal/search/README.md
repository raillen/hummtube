# Mecanismo de Busca (`internal/search/`)

## O que é este diretório?
Serviço de busca híbrida rápida e inteligente.

## Para que serve?
Executa consultas sub-milissegundo no índice FTS5 local e faz queries remotas paginadas via YouTube Data API v3 com cancelamento via `context.Context`.
