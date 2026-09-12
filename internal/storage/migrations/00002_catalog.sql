-- v0.2: schema canônico de catálogo/estado local.
-- Documento canônico: docs/03-implementation/STORAGE.md
-- +goose Up
ALTER TABLE videos ADD COLUMN description_excerpt TEXT NOT NULL DEFAULT '';
ALTER TABLE videos ADD COLUMN duration INTEGER NOT NULL DEFAULT 0;
ALTER TABLE videos ADD COLUMN category TEXT NOT NULL DEFAULT '';
ALTER TABLE videos ADD COLUMN thumbnail_url TEXT NOT NULL DEFAULT '';
ALTER TABLE videos ADD COLUMN last_seen_at TEXT NOT NULL DEFAULT '';

CREATE TABLE channels (
    id                  TEXT PRIMARY KEY,
    title               TEXT NOT NULL DEFAULT '',
    subscribed          INTEGER NOT NULL DEFAULT 0,
    uploads_playlist_id TEXT,
    last_sync_at        TEXT,
    last_known_video_id TEXT,
    last_error          TEXT
);

CREATE TABLE playback_progress (
    video_id    TEXT PRIMARY KEY,
    position_ms INTEGER NOT NULL DEFAULT 0,
    duration_ms INTEGER NOT NULL DEFAULT 0,
    updated_at  TEXT NOT NULL DEFAULT '',
    completed   INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE interest_topics (
    topic      TEXT PRIMARY KEY,
    score      INTEGER NOT NULL DEFAULT 0,
    updated_at TEXT NOT NULL DEFAULT ''
);

CREATE TABLE recommendation_feedback (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    video_id   TEXT,
    channel_id TEXT,
    topic      TEXT,
    action     TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL DEFAULT ''
);

CREATE TABLE queue_items (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    video_id    TEXT NOT NULL DEFAULT '',
    position_ms INTEGER NOT NULL DEFAULT 0,
    order_index INTEGER NOT NULL DEFAULT 0,
    opened_at   TEXT NOT NULL DEFAULT ''
);

CREATE TABLE settings (
    key   TEXT PRIMARY KEY,
    value TEXT NOT NULL DEFAULT ''
);

DROP TABLE IF EXISTS videos_fts;
CREATE VIRTUAL TABLE videos_fts USING fts5(video_id UNINDEXED, title, description, channel_name);

-- +goose Down
DROP TABLE IF EXISTS settings;
DROP TABLE IF EXISTS queue_items;
DROP TABLE IF EXISTS recommendation_feedback;
DROP TABLE IF EXISTS interest_topics;
DROP TABLE IF EXISTS playback_progress;
DROP TABLE IF EXISTS channels;
DROP TABLE IF EXISTS videos_fts;
CREATE VIRTUAL TABLE videos_fts USING fts5(title, description);
ALTER TABLE videos DROP COLUMN last_seen_at;
ALTER TABLE videos DROP COLUMN thumbnail_url;
ALTER TABLE videos DROP COLUMN category;
ALTER TABLE videos DROP COLUMN duration;
ALTER TABLE videos DROP COLUMN description_excerpt;
