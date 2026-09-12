-- M7/PLY-05: regras versionadas para playlists inteligentes
-- +goose Up
ALTER TABLE playlists ADD COLUMN is_smart INTEGER NOT NULL DEFAULT 0;
CREATE TABLE playlist_rules (
    playlist_id TEXT PRIMARY KEY REFERENCES playlists(id) ON DELETE CASCADE,
    version     INTEGER NOT NULL DEFAULT 1,
    rule_json   TEXT NOT NULL DEFAULT '{}',
    updated_at  TEXT NOT NULL DEFAULT ''
);
CREATE TABLE smart_presets (
    id         TEXT PRIMARY KEY,
    name       TEXT NOT NULL,
    rule_json  TEXT NOT NULL DEFAULT '{}',
    version    INTEGER NOT NULL DEFAULT 1,
    created_at TEXT NOT NULL DEFAULT '',
    updated_at TEXT NOT NULL DEFAULT ''
);

-- +goose Down
DROP TABLE IF EXISTS smart_presets;
DROP TABLE IF EXISTS playlist_rules;
