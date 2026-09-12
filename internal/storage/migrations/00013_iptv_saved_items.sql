-- v0.7: lista local de itens IPTV.
-- A intenção é mantida por item_id, sem persistir URL de stream ou segredo.
-- +goose Up
CREATE TABLE iptv_saved_items (
    item_id    TEXT PRIMARY KEY,
    created_at TEXT NOT NULL DEFAULT ''
);

CREATE INDEX iptv_saved_items_created_idx
    ON iptv_saved_items(created_at, item_id);

-- +goose Down
DROP TABLE IF EXISTS iptv_saved_items;
