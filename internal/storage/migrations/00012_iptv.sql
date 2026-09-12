-- v0.7: catálogo IPTV/EPG local sem persistir URLs de stream ou credenciais.
-- O provider resolve o playback novamente a partir do item/source ID.
-- +goose Up
CREATE TABLE iptv_sources (
    id                TEXT PRIMARY KEY,
    name              TEXT NOT NULL DEFAULT '',
    format            TEXT NOT NULL DEFAULT 'm3u',
    playlist_endpoint TEXT NOT NULL DEFAULT '',
    guide_endpoint    TEXT NOT NULL DEFAULT '',
    credential_ref    TEXT NOT NULL DEFAULT '',
    enabled           INTEGER NOT NULL DEFAULT 1,
    last_sync_at      TEXT NOT NULL DEFAULT '',
    last_error        TEXT NOT NULL DEFAULT ''
);

CREATE TABLE iptv_items (
    id                 TEXT PRIMARY KEY,
    source_id          TEXT NOT NULL REFERENCES iptv_sources(id) ON DELETE CASCADE,
    kind               TEXT NOT NULL DEFAULT 'unknown',
    classification     TEXT NOT NULL DEFAULT 'unknown',
    title              TEXT NOT NULL DEFAULT '',
    raw_title          TEXT NOT NULL DEFAULT '',
    group_name         TEXT NOT NULL DEFAULT '',
    logo_url           TEXT NOT NULL DEFAULT '',
    epg_id             TEXT NOT NULL DEFAULT '',
    channel_number     TEXT NOT NULL DEFAULT '',
    language           TEXT NOT NULL DEFAULT '',
    country            TEXT NOT NULL DEFAULT '',
    season             INTEGER NOT NULL DEFAULT 0,
    episode            INTEGER NOT NULL DEFAULT 0,
    has_season         INTEGER NOT NULL DEFAULT 0,
    has_episode        INTEGER NOT NULL DEFAULT 0,
    first_seen_at      TEXT NOT NULL DEFAULT '',
    last_seen_at       TEXT NOT NULL DEFAULT ''
);

CREATE INDEX iptv_items_source_kind_idx ON iptv_items(source_id, kind, title);
CREATE INDEX iptv_items_source_group_idx ON iptv_items(source_id, group_name, title);

CREATE TABLE iptv_guide_channels (
    source_id TEXT NOT NULL REFERENCES iptv_sources(id) ON DELETE CASCADE,
    id        TEXT NOT NULL,
    name      TEXT NOT NULL DEFAULT '',
    logo_url  TEXT NOT NULL DEFAULT '',
    PRIMARY KEY (source_id, id)
);

CREATE TABLE iptv_guide_programs (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    source_id   TEXT NOT NULL,
    channel_id  TEXT NOT NULL,
    title       TEXT NOT NULL DEFAULT '',
    description TEXT NOT NULL DEFAULT '',
    start_at    TEXT NOT NULL,
    end_at      TEXT NOT NULL,
    FOREIGN KEY (source_id, channel_id)
        REFERENCES iptv_guide_channels(source_id, id) ON DELETE CASCADE
);

CREATE INDEX iptv_guide_programs_window_idx
    ON iptv_guide_programs(source_id, channel_id, start_at, end_at);

-- +goose Down
DROP TABLE IF EXISTS iptv_guide_programs;
DROP TABLE IF EXISTS iptv_guide_channels;
DROP TABLE IF EXISTS iptv_items;
DROP TABLE IF EXISTS iptv_sources;
