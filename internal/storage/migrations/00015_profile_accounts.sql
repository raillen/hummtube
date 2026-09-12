-- v0.8: identidade e sessão OAuth isoladas por perfil local.
-- Dados legados de `settings` são preservados no perfil determinístico
-- `default`; refresh tokens continuam fora do SQLite e são migrados no
-- bootstrap da aplicação pelo adaptador de keyring.
-- +goose Up
ALTER TABLE profiles ADD COLUMN kind TEXT NOT NULL DEFAULT 'persistent'
    CHECK (kind IN ('persistent', 'guest'));

INSERT OR IGNORE INTO profiles (id, name, created_at, kind)
VALUES ('default', 'Padrão', strftime('%Y-%m-%dT%H:%M:%SZ', 'now'), 'persistent');

CREATE TABLE profile_accounts (
    profile_id          TEXT PRIMARY KEY REFERENCES profiles(id) ON DELETE CASCADE,
    provider            TEXT NOT NULL DEFAULT 'google',
    provider_subject    TEXT NOT NULL,
    email               TEXT NOT NULL DEFAULT '',
    connected_at        TEXT NOT NULL DEFAULT '',
    session_persistence TEXT NOT NULL DEFAULT 'keyring'
        CHECK (session_persistence IN ('keyring', 'memory')),
    UNIQUE (provider, provider_subject)
);

-- A versão anterior conhecia apenas e-mail. O identificador `legacy-email:`
-- é determinístico e mantém o vínculo até o próximo OAuth substituir pelo
-- subject autoritativo devolvido pelo Google.
INSERT OR REPLACE INTO profile_accounts
    (profile_id, provider, provider_subject, email, connected_at, session_persistence)
SELECT
    'default',
    'google',
    'legacy-email:' || lower(email.value),
    email.value,
    COALESCE(connected.value, ''),
    'keyring'
FROM settings AS email
LEFT JOIN settings AS connected ON connected.key = 'account.connected_at'
WHERE email.key = 'account.email' AND trim(email.value) <> '';

DELETE FROM settings WHERE key IN ('account.email', 'account.connected_at');
INSERT INTO settings (key, value) VALUES ('ui.active_profile', 'default')
ON CONFLICT(key) DO UPDATE SET value = excluded.value;

CREATE INDEX profile_accounts_provider_subject_idx
    ON profile_accounts(provider, provider_subject);

-- +goose Down
INSERT INTO settings (key, value)
SELECT 'account.email', email FROM profile_accounts WHERE profile_id = 'default'
ON CONFLICT(key) DO UPDATE SET value = excluded.value;

INSERT INTO settings (key, value)
SELECT 'account.connected_at', connected_at FROM profile_accounts WHERE profile_id = 'default'
ON CONFLICT(key) DO UPDATE SET value = excluded.value;

DROP INDEX IF EXISTS profile_accounts_provider_subject_idx;
DROP TABLE IF EXISTS profile_accounts;
ALTER TABLE profiles DROP COLUMN kind;
