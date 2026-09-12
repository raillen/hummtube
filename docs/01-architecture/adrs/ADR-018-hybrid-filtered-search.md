# ADR-018: Busca Híbrida com Filtros Locais

## Contexto
O usuário precisa de respostas imediatas enquanto digita, além da capacidade de buscar no YouTube completo.

## Decisão
Implementar o `SearchService` híbrido:
1. Durante a digitação: busca instantânea local no índice SQLite FTS5 (vídeos conhecidos, histórico, playlists);
2. Ação explícita de "Buscar no YouTube" (Enter / botão): consulta remota via YouTube Data API v3 ou yt-dlp flat search, aplicando filtros de duração, data e tipo.
