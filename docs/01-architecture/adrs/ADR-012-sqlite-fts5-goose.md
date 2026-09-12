# ADR-012: SQLite + FTS5 + Goose Migrations

## Contexto
O armazenamento local necessita de alta confiabilidade, ACID, suporte a busca textual full-text e migrações de schema atômicas.

## Decisão
Utilizar `modernc.org/sqlite` (SQLite 100% Go sem CGO) com suporte a FTS5 integrado e gerenciar todas as migrações através do `github.com/pressly/goose/v3`.
