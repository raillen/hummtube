-- Descrição completa para o painel de informações do player.
-- +goose Up
ALTER TABLE videos ADD COLUMN description TEXT NOT NULL DEFAULT '';
DELETE FROM videos_fts;
INSERT INTO videos_fts (video_id, title, description, channel_name)
SELECT v.id, v.title, COALESCE(NULLIF(v.description, ''), v.description_excerpt), COALESCE(c.title, '')
FROM videos v LEFT JOIN channels c ON c.id = v.channel_id;

-- +goose Down
ALTER TABLE videos DROP COLUMN description;
