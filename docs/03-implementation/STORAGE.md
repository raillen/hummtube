---
id: storage
status: canonical
---

# Armazenamento e Migrações (SQLite + FTS5 + Goose)

## 1. Schema e Tabelas Principais

- `channels`: Canais conhecidos e inscritos (`id`, `title`, `thumbnail_url`, `subscribed`, `uploads_playlist_id`, `last_sync_at`);
- `videos`: Metadados locais de vídeos (`id`, `channel_id`, `title`, `description`, `description_excerpt`, `duration`, `published_at`, `thumbnail_url`, `live_status`);
- `videos_fts`: Tabela virtual SQLite FTS5 indexando títulos, descrição (completa quando disponível) e canais para busca instantânea;
- `playback_progress`: Posição atual (`position_ms`, `duration_ms`, `updated_at`, `completed`);
- `favorites`: Vídeos marcados como favoritos;
- `playlists` & `playlist_items`: Playlists manuais com ordenação e deduplicação;
- `smart_rules`: Regras JSON versionadas de playlists inteligentes;
- `channel_favorites`, `channel_folders`, `channel_folder_items`: Organização de canais;
- `video_notes` & `video_bookmarks`: Anotações e marcadores temporais;
- `iptv_sources`, `iptv_channels`, `iptv_saved_items`: Catálogo IPTV local;
- `profiles`, `profile_members`: perfis persistentes/guest e referências locais isoladas;
- `profile_accounts`: identidade não sensível e tipo de persistência da sessão por perfil;
- `playback_queue`, `playback_queue_preferences`: ordem, estado tocado e preferências da fila por perfil;
- `credential_audit`: metadados locais de conexão, rotação e revogação por perfil; nunca contém tokens, Client IDs ou secrets;
- `channel_tags`: categorias pessoais adicionais para organização dos canais;
- `interest_topics`: Pesos e afinidades calculados para o recommendation engine.
- `profile_settings`: preferências de playback que podem selecionar uma sessão
  do navegador, atualmente a origem de cookies. A tabela guarda somente a
  preferência, nunca o conteúdo dos cookies, e é isolada por `profile_id`.

## 2. Migrações Goose

Todas as alterações estruturais são executadas sequencialmente através de arquivos `.sql` versionados em `internal/storage/migrations/` (00001 a 00021).

A migration `00015_profile_accounts.sql` preserva a conta legada no perfil determinístico `default`, move os metadados de `settings` para `profile_accounts` e adiciona o tipo `persistent|guest`. Tokens não participam da migration SQLite: o bootstrap move a antiga chave global do Keyring somente depois de confirmar a gravação em `google.refresh_token/default`.

As migrations `00016_playback_queue.sql`, `00017_subscription_feed_and_channel_tags.sql` e `00018_video_description.sql` adicionam a fila persistente, preferências de avanço, data de inscrição, tags de canais e a descrição completa dos vídeos. A fila possui limite de 100 itens aplicado pelo repositório, unicidade de vídeo por perfil e normalização transacional das posições após remover ou reordenar. `description_excerpt` continua como projeção curta para cards; `description` é usada no painel do player e no índice FTS quando disponível.

As migrations `00019_credential_audit.sql` e `00020_channel_thumbnails.sql` adicionam, respectivamente, auditoria não sensível de credenciais por perfil e miniaturas persistentes de canais. A remoção de um perfil apaga seus eventos por cascade.

A migration `00021_profile_playback_settings.sql` move as preferências legadas
de cookies para `profile_settings/default` e remove as chaves globais. Ao
alternar de perfil, o resolver é reconstruído antes de qualquer reprodução;
perfis guest não podem salvar nem reutilizar uma sessão de navegador
persistida.

`Repository.ActiveProfile(ctx)` e `ActiveProfileID(ctx)` sempre garantem um perfil default válido. Novas tabelas que representem estado pessoal devem carregar `profile_id`, declarar a política de cascade e testar que leitura, logout ou remoção em um perfil não afetam outro.

## 3. Metadados antes de referências locais

Resultados vindos de pesquisa remota são persistidos em `videos` antes da resolução e reprodução. Isso mantém válidos os `JOINs` usados por histórico, favoritos e playlists depois de reiniciar o aplicativo. Uma gravação nunca é considerada bem-sucedida por fallback de UI: erros do SQLite atravessam o contrato RPC.

A importação IPTV também é transacional por lotes e usa atualização por conflito para preservar o catálogo diante de IDs duplicados do provedor.
