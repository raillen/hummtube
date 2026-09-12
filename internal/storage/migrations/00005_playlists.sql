-- v0.5+ (M7/PLY-01): playlists locais e seus itens. Estado puramente local,
-- nada é enviado ao YouTube. Documento canônico: docs/03-implementation/STORAGE.md
-- +goose Up
CREATE TABLE playlists (
    id          TEXT PRIMARY KEY,
    name        TEXT NOT NULL DEFAULT '',
    description TEXT NOT NULL DEFAULT '',
    color       TEXT NOT NULL DEFAULT '',
    created_at  TEXT NOT NULL DEFAULT '',
    updated_at  TEXT NOT NULL DEFAULT ''
);

-- Itens de uma playlist. A duplicata de vídeo dentro da mesma playlist é
-- impedida pela PK composta; o vídeo pode existir mesmo sem linha no catálogo
-- (o item sobrevive à saída do vídeo, como em favorites).
CREATE TABLE playlist_items (
    playlist_id TEXT NOT NULL,
    video_id    TEXT NOT NULL,
    position    INTEGER NOT NULL DEFAULT 0,
    added_at    TEXT NOT NULL DEFAULT '',
    PRIMARY KEY (playlist_id, video_id),
    FOREIGN KEY (playlist_id) REFERENCES playlists(id) ON DELETE CASCADE
);

-- A ordenação de leitura é sempre por posição dentro de uma playlist.
CREATE INDEX idx_playlist_items_order ON playlist_items (playlist_id, position);

-- +goose Down
DROP TABLE IF EXISTS playlist_items;
DROP TABLE IF EXISTS playlists;