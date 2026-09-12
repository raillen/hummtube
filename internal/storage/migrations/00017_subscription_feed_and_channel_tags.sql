-- Metadados locais para organização avançada de inscrições.
-- +goose Up
ALTER TABLE channels ADD COLUMN subscribed_at TEXT NOT NULL DEFAULT '';

CREATE TABLE channel_tags (
    profile_id TEXT NOT NULL REFERENCES profiles(id) ON DELETE CASCADE,
    channel_id TEXT NOT NULL REFERENCES channels(id) ON DELETE CASCADE,
    tag        TEXT NOT NULL COLLATE NOCASE,
    created_at TEXT NOT NULL,
    PRIMARY KEY (profile_id, channel_id, tag)
);
CREATE INDEX channel_tags_profile_tag_idx ON channel_tags(profile_id, tag, channel_id);

-- +goose Down
DROP INDEX IF EXISTS channel_tags_profile_tag_idx;
DROP TABLE IF EXISTS channel_tags;
ALTER TABLE channels DROP COLUMN subscribed_at;
