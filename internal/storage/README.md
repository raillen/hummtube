# Armazenamento e Persistência (`internal/storage/`)

## O que é este diretório?
Camada de persistência baseada em SQLite puro (`modernc.org/sqlite`) com migrações Goose e índices FTS5.

## Para que serve?
Persiste canais, vídeos do feed, histórico, notas e configurações locais, além de alimentar a busca instantânea full-text.
