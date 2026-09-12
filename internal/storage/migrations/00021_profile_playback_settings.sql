-- v0.8.2: preferências de playback que podem carregar credenciais passam a
-- pertencer ao perfil local. O conteúdo dos cookies continua no navegador;
-- SQLite armazena apenas a fonte escolhida e, opcionalmente, um path legado.
-- +goose Up
CREATE TABLE profile_settings (
    profile_id TEXT NOT NULL REFERENCES profiles(id) ON DELETE CASCADE,
    key        TEXT NOT NULL,
    value      TEXT NOT NULL DEFAULT '',
    PRIMARY KEY (profile_id, key)
);

CREATE INDEX profile_settings_key_idx ON profile_settings(key, profile_id);

INSERT OR IGNORE INTO profile_settings (profile_id, key, value)
SELECT 'default', key, value
FROM settings
WHERE key IN ('playback.cookies_browser', 'playback.cookies_file');

DELETE FROM settings
WHERE key IN ('playback.cookies_browser', 'playback.cookies_file');

-- +goose Down
INSERT INTO settings (key, value)
SELECT key, value
FROM profile_settings
WHERE profile_id = 'default'
  AND key IN ('playback.cookies_browser', 'playback.cookies_file')
ON CONFLICT(key) DO UPDATE SET value = excluded.value;

DROP INDEX IF EXISTS profile_settings_key_idx;
DROP TABLE IF EXISTS profile_settings;
