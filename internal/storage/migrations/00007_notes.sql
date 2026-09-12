-- PLY-02/QOL-01: notas e marcadores de tempo locais por vídeo.
-- Estado puramente local, nada é enviado ao YouTube.
-- Documento canônico: docs/03-implementation/STORAGE.md
-- +goose Up
-- Nota livre do usuário sobre um vídeo (uma por vídeo).
CREATE TABLE video_notes (
    video_id   TEXT PRIMARY KEY,
    text       TEXT NOT NULL DEFAULT '',
    updated_at TEXT NOT NULL DEFAULT ''
);

-- Marcadores de tempo: posições que valem a pena revisitar, com rótulo.
-- Sem FK para `videos`: como favoritos, o estado persiste mesmo se o vídeo
-- sair do catálogo local (e funciona para vídeos nunca catalogados).
CREATE TABLE video_bookmarks (
    id          TEXT PRIMARY KEY,
    video_id    TEXT NOT NULL,
    position_ms INTEGER NOT NULL DEFAULT 0,
    label       TEXT NOT NULL DEFAULT '',
    created_at  TEXT NOT NULL DEFAULT ''
);

-- A única consulta é por vídeo, ordenada por posição.
CREATE INDEX idx_video_bookmarks_video ON video_bookmarks (video_id, position_ms);

-- +goose Down
DROP TABLE IF EXISTS video_bookmarks;
DROP TABLE IF EXISTS video_notes;
