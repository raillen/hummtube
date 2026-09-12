-- M9/QOL-03: organização local de canais (favoritos + pastas de inscrições).
-- Estado puramente local, nada é enviado ao YouTube.
-- Documento canônico: docs/03-implementation/STORAGE.md
-- +goose Up
-- Canais favoritos (estrela). O canal continua inscrito normalmente; favorito é
-- apenas um marcador local.
CREATE TABLE channel_favorites (
    channel_id TEXT PRIMARY KEY,
    created_at TEXT NOT NULL DEFAULT ''
);

-- Pastas locais que agrupam inscrições.
CREATE TABLE channel_folders (
    id         TEXT PRIMARY KEY,
    name       TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL DEFAULT ''
);

-- Um canal em uma pasta. A PK composta impede o mesmo canal duas vezes na
-- mesma pasta; o canal pode existir em várias pastas.
CREATE TABLE channel_folder_items (
    folder_id  TEXT NOT NULL,
    channel_id TEXT NOT NULL,
    created_at TEXT NOT NULL DEFAULT '',
    PRIMARY KEY (folder_id, channel_id),
    FOREIGN KEY (folder_id) REFERENCES channel_folders(id) ON DELETE CASCADE
);

CREATE INDEX idx_channel_folder_items_folder ON channel_folder_items (folder_id, channel_id);

-- +goose Down
DROP TABLE IF EXISTS channel_folder_items;
DROP TABLE IF EXISTS channel_folders;
DROP TABLE IF EXISTS channel_favorites;
