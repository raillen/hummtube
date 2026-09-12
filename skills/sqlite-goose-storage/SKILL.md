---
name: sqlite-goose-storage
description: Guia de gerenciamento de banco SQLite, FTS5, migrações versionadas Goose e integridade local.
---

# SQLite + FTS5 + Goose Storage Skill

## Visão Geral
Esta skill aborda a persistência robusta, rápida e sem ORM pesado para dados locais, histórico, playlists e catálogo indexado.

## Padrões
1. **Driver**: `modernc.org/sqlite` (Pure Go, sem dependência CGO);
2. **FTS5**: Tabela virtual com porter tokenizer e prefix match para busca em tempo real com <15ms;
3. **Goose Migrations**: Arquivos `.sql` incrementais sequenciais (`00001_...sql` até `00014_...sql`);
4. **Integridade & Transações**: `foreign_keys=ON`, transações explícitas `BEGIN IMMEDIATE` para mutações críticas.
