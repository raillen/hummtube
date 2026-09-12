-- Miniaturas de canais para os modos visuais do gerenciador.
-- +goose Up
ALTER TABLE channels ADD COLUMN thumbnail_url TEXT NOT NULL DEFAULT '';

-- +goose Down
ALTER TABLE channels DROP COLUMN thumbnail_url;
