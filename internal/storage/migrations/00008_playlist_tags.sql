-- M7/PLY-04: tags por playlist (puramente local, sem impacto em catálogo).
-- +goose Up
CREATE TABLE playlist_tags (
    playlist_id TEXT NOT NULL REFERENCES playlists(id) ON DELETE CASCADE,
    tag         TEXT NOT NULL,
    PRIMARY KEY (playlist_id, tag COLLATE NOCASE)
);
CREATE INDEX idx_playlist_tags_tag ON playlist_tags(tag COLLATE NOCASE);

-- +goose Down
DROP INDEX IF EXISTS idx_playlist_tags_tag;
DROP TABLE IF EXISTS playlist_tags;
