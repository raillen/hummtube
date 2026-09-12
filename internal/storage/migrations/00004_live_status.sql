-- v0.5.1: estado de transmissão (live_status) no catálogo local.
-- Alimentado pela API oficial (snippet.liveBroadcastContent) e pela busca
-- yt-dlp (live_status). É o que permite filtrar lives na Home e em
-- Inscrições sem chamadas extras. Documento canônico: docs/03-implementation/STORAGE.md
-- +goose Up
ALTER TABLE videos ADD COLUMN live_status TEXT NOT NULL DEFAULT '';

-- +goose Down
ALTER TABLE videos DROP COLUMN live_status;