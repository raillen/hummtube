-- v0.4: favoritos / assistir depois (local, sem sincronização com o YouTube).
-- Documento canônico: docs/03-implementation/STORAGE.md
-- +goose Up
CREATE TABLE favorites (
    video_id   TEXT PRIMARY KEY,
    created_at TEXT NOT NULL DEFAULT ''
);

-- A listagem é sempre ordenada por quando foi salvo, então o índice cobre a
-- única consulta que existe.
CREATE INDEX idx_favorites_created_at ON favorites (created_at DESC);

-- +goose Down
DROP INDEX IF EXISTS idx_favorites_created_at;
DROP TABLE IF EXISTS favorites;
