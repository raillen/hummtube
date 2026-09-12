-- Spike Fase 0 / S0.3: valida driver SQLite (modernc), FTS5 e Goose.
-- Schema canônico (v0.2) será construído em cima destas migrations.
-- +goose Up
CREATE TABLE videos (
    id            TEXT PRIMARY KEY,
    channel_id    TEXT NOT NULL DEFAULT '',
    title         TEXT NOT NULL DEFAULT '',
    published_at  TEXT NOT NULL DEFAULT '',
    first_seen_at TEXT NOT NULL DEFAULT ''
);

CREATE VIRTUAL TABLE videos_fts USING fts5(title, description);

-- +goose Down
DROP TABLE IF EXISTS videos_fts;
DROP TABLE IF EXISTS videos;
