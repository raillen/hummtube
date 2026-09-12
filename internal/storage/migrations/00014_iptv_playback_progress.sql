-- v0.7: posição de reprodução local para VOD do NanoIPTV (filmes e episódios).
-- TV ao vivo não participa: stream sem duração não tem retomada.
-- +goose Up
CREATE TABLE iptv_playback_progress (
    item_id      TEXT PRIMARY KEY REFERENCES iptv_items(id) ON DELETE CASCADE,
    position_ms  INTEGER NOT NULL DEFAULT 0,
    duration_ms  INTEGER NOT NULL DEFAULT 0,
    completed    INTEGER NOT NULL DEFAULT 0,
    updated_at   TEXT NOT NULL DEFAULT ''
);

CREATE INDEX iptv_playback_progress_updated_idx
    ON iptv_playback_progress(updated_at, item_id);

-- +goose Down
DROP TABLE IF EXISTS iptv_playback_progress;
