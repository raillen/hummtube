package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/nanotube/nanotube-web/internal/iptv"
)

const (
	defaultIPTVItemLimit = 200
	maxIPTVItemLimit     = 1000
)

var ErrSensitiveEndpoint = errors.New("endpoint contém credenciais inline")

var iptvEndpointPattern = regexp.MustCompile(`(?i)https?://\S+`)

// ListIPTVSources returns source configuration and synchronization status.
func (r *Repository) ListIPTVSources(ctx context.Context) ([]iptv.SourceState, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, name, format, playlist_endpoint, guide_endpoint, credential_ref,
		       enabled, last_sync_at, last_error
		FROM iptv_sources ORDER BY name, id`)
	if err != nil {
		return nil, fmt.Errorf("listar fontes IPTV: %w", err)
	}
	defer rows.Close()

	states := make([]iptv.SourceState, 0)
	for rows.Next() {
		state, err := scanIPTVSource(rows)
		if err != nil {
			return nil, fmt.Errorf("listar fontes IPTV scan: %w", err)
		}
		states = append(states, state)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("listar fontes IPTV rows: %w", err)
	}
	return states, nil
}

// GetIPTVSource returns one source configuration without exposing a secret.
func (r *Repository) GetIPTVSource(ctx context.Context, sourceID string) (iptv.SourceConfig, bool, error) {
	sourceID = strings.TrimSpace(sourceID)
	if sourceID == "" {
		return iptv.SourceConfig{}, false, fmt.Errorf("obter fonte IPTV: source ID vazio")
	}
	row := r.db.QueryRowContext(ctx, `
		SELECT id, name, format, playlist_endpoint, guide_endpoint, credential_ref, enabled
		FROM iptv_sources WHERE id = ?`, sourceID)
	var source iptv.SourceConfig
	var format string
	var enabled int
	if err := row.Scan(&source.ID, &source.Name, &format, &source.PlaylistURL,
		&source.GuideURL, &source.CredentialRef, &enabled); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return iptv.SourceConfig{}, false, nil
		}
		return iptv.SourceConfig{}, false, fmt.Errorf("obter fonte IPTV: %w", err)
	}
	source.Format = iptv.SourceFormat(format)
	source.Enabled = enabled != 0
	return source, true, nil
}

// GetIPTVItem returns persisted catalog metadata without a stream URL.
func (r *Repository) GetIPTVItem(ctx context.Context, sourceID, itemID string) (iptv.Item, bool, error) {
	sourceID = strings.TrimSpace(sourceID)
	itemID = strings.TrimSpace(itemID)
	if sourceID == "" || itemID == "" {
		return iptv.Item{}, false, fmt.Errorf("obter item IPTV: fonte e item são obrigatórios")
	}
	row := r.db.QueryRowContext(ctx, `
		SELECT i.id, i.source_id, s.name, i.kind, i.classification, i.title,
		       i.raw_title, i.group_name, i.logo_url, i.epg_id, i.channel_number,
		       i.language, i.country, i.season, i.episode, i.has_season,
		       i.has_episode
		FROM iptv_items i
		JOIN iptv_sources s ON s.id = i.source_id
		WHERE i.source_id = ? AND i.id = ?`, sourceID, itemID)
	var (
		item                                   iptv.Item
		kindValue, classificationValue         string
		season, episode, hasSeason, hasEpisode int
	)
	if err := row.Scan(
		&item.ID, &item.SourceID, &item.SourceName, &kindValue, &classificationValue,
		&item.Title, &item.RawTitle, &item.Group, &item.LogoURL, &item.EPGID,
		&item.ChannelNumber, &item.Language, &item.Country, &season, &episode,
		&hasSeason, &hasEpisode,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return iptv.Item{}, false, nil
		}
		return iptv.Item{}, false, fmt.Errorf("obter item IPTV: %w", err)
	}
	item.Kind = iptv.ContentKind(kindValue)
	item.Classification = iptv.ClassificationSource(classificationValue)
	item.Episode = iptv.EpisodeRef{
		Season: season, Episode: episode,
		HasSeason: hasSeason != 0, HasEpisode: hasEpisode != 0,
	}
	return item, true, nil
}

// SaveIPTVSource persists editable source metadata without changing the
// current catalog snapshot or synchronization status.
func (r *Repository) SaveIPTVSource(ctx context.Context, source iptv.SourceConfig) error {
	prepared, err := prepareIPTVSource(source)
	if err != nil {
		return err
	}
	_, err = r.db.ExecContext(ctx, `
		INSERT INTO iptv_sources
			(id, name, format, playlist_endpoint, guide_endpoint, credential_ref, enabled)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			name = excluded.name,
			format = excluded.format,
			playlist_endpoint = excluded.playlist_endpoint,
			guide_endpoint = excluded.guide_endpoint,
			credential_ref = excluded.credential_ref,
			enabled = excluded.enabled`,
		prepared.ID, prepared.Name, prepared.Format, prepared.PlaylistURL,
		prepared.GuideURL, prepared.CredentialRef, boolInt(prepared.Enabled))
	if err != nil {
		return fmt.Errorf("salvar fonte IPTV: %w", err)
	}
	return nil
}

// SetIPTVSourceError updates only the diagnostic status of a source. Endpoint
// text is redacted before persistence so a future adapter cannot leak URLs.
func (r *Repository) SetIPTVSourceError(ctx context.Context, sourceID, message string) error {
	sourceID = strings.TrimSpace(sourceID)
	if sourceID == "" {
		return fmt.Errorf("atualizar erro IPTV: source ID vazio")
	}
	message = iptvEndpointPattern.ReplaceAllString(strings.TrimSpace(message), "<endpoint>")
	if len(message) > 512 {
		message = message[:512]
	}
	_, err := r.db.ExecContext(ctx, `UPDATE iptv_sources SET last_error = ? WHERE id = ?`, message, sourceID)
	if err != nil {
		return fmt.Errorf("atualizar erro IPTV: %w", err)
	}
	return nil
}

// DeleteIPTVSource removes the source and its catalog/EPG snapshots through
// the foreign-key cascades.
func (r *Repository) DeleteIPTVSource(ctx context.Context, sourceID string) error {
	sourceID = strings.TrimSpace(sourceID)
	if sourceID == "" {
		return fmt.Errorf("remover fonte IPTV: source ID vazio")
	}
	if _, err := r.db.ExecContext(ctx, `DELETE FROM iptv_sources WHERE id = ?`, sourceID); err != nil {
		return fmt.Errorf("remover fonte IPTV: %w", err)
	}
	return nil
}

// ReplaceIPTVCatalog grava metadata do catálogo em uma transação. URLs de
// stream são deliberadamente descartadas: o provider deve resolvê-las no
// momento do playback usando a fonte e o ID do item.
func (r *Repository) ReplaceIPTVCatalog(ctx context.Context, source iptv.SourceConfig, playlist iptv.Playlist, syncedAt time.Time) error {
	now := normalizeSyncTime(syncedAt)
	prepared, err := prepareIPTVSource(source)
	if err != nil {
		return err
	}
	if err := validateIPTVItems(prepared.ID, playlist.Items); err != nil {
		return err
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("iptv begin catalog: %w", err)
	}
	defer tx.Rollback()

	if err := upsertIPTVSourceTx(ctx, tx, prepared, now); err != nil {
		return err
	}

	if err := replaceIPTVItemsTx(ctx, tx, prepared.ID, playlist.Items, now); err != nil {
		return err
	}
	return tx.Commit()
}

// ListIPTVItems retorna metadata persistida. StreamURL permanece vazio por
// desenho; a camada de provider faz a resolução efêmera para reprodução.
func (r *Repository) ListIPTVItems(ctx context.Context, sourceID string, kind iptv.ContentKind, limit int) ([]iptv.Item, error) {
	return r.listIPTVItems(ctx, strings.TrimSpace(sourceID), kind, "", limit, false)
}

// ListAllIPTVItems returns the normalized catalog across enabled and disabled
// sources. The caller chooses whether disabled sources remain visible.
func (r *Repository) ListAllIPTVItems(ctx context.Context, kind iptv.ContentKind, limit int) ([]iptv.Item, error) {
	return r.listIPTVItems(ctx, "", kind, "", limit, false)
}

// ListIPTVSavedItems returns the local list joined with the current catalog.
// Stale intentions naturally disappear from the result when a provider no
// longer publishes an item, without deleting the user's local state eagerly.
func (r *Repository) ListIPTVSavedItems(ctx context.Context, limit int) ([]iptv.Item, error) {
	return r.listIPTVItems(ctx, "", "", "", limit, true)
}

// SetIPTVItemSaved changes only local organization state.
func (r *Repository) SetIPTVItemSaved(ctx context.Context, itemID string, saved bool) error {
	itemID = strings.TrimSpace(itemID)
	if itemID == "" {
		return fmt.Errorf("salvar item IPTV: ID vazio")
	}
	if saved {
		_, err := r.db.ExecContext(ctx, `
			INSERT INTO iptv_saved_items (item_id, created_at)
			VALUES (?, ?) ON CONFLICT(item_id) DO NOTHING`, itemID, fmtTime(time.Now().UTC()))
		if err != nil {
			return fmt.Errorf("salvar item IPTV: %w", err)
		}
		return nil
	}
	if _, err := r.db.ExecContext(ctx, `DELETE FROM iptv_saved_items WHERE item_id = ?`, itemID); err != nil {
		return fmt.Errorf("remover item IPTV salvo: %w", err)
	}
	return nil
}

// SearchIPTVItems performs a bounded local search over title, group and source
// name. It intentionally uses LIKE over the normalized snapshot for this MVP;
// an FTS5 table can be introduced later with a measured migration.
func (r *Repository) SearchIPTVItems(ctx context.Context, query string, kind iptv.ContentKind, limit int) ([]iptv.Item, error) {
	return r.listIPTVItems(ctx, "", kind, strings.TrimSpace(query), limit, false)
}

func (r *Repository) listIPTVItems(ctx context.Context, sourceID string, kind iptv.ContentKind, queryText string, limit int, savedOnly bool) ([]iptv.Item, error) {
	if limit <= 0 {
		limit = defaultIPTVItemLimit
	}
	if limit > maxIPTVItemLimit {
		limit = maxIPTVItemLimit
	}

	query := `
		SELECT i.id, i.source_id, s.name, i.kind, i.classification, i.title,
		       i.raw_title, i.group_name, i.logo_url, i.epg_id, i.channel_number,
		       i.language, i.country, i.season, i.episode, i.has_season,
		       i.has_episode
		FROM iptv_items i
		JOIN iptv_sources s ON s.id = i.source_id
		WHERE 1 = 1`
	if savedOnly {
		query = `
			SELECT i.id, i.source_id, s.name, i.kind, i.classification, i.title,
			       i.raw_title, i.group_name, i.logo_url, i.epg_id, i.channel_number,
			       i.language, i.country, i.season, i.episode, i.has_season,
			       i.has_episode
			FROM iptv_saved_items saved
			JOIN iptv_items i ON i.id = saved.item_id
			JOIN iptv_sources s ON s.id = i.source_id
			WHERE 1 = 1`
	}
	args := []any{}
	if sourceID != "" {
		query += " AND i.source_id = ?"
		args = append(args, sourceID)
	}
	if kind != "" {
		query += " AND i.kind = ?"
		args = append(args, kind)
	}
	if queryText != "" {
		query += " AND (LOWER(i.title) LIKE LOWER(?) ESCAPE '\\' OR LOWER(i.group_name) LIKE LOWER(?) ESCAPE '\\' OR LOWER(s.name) LIKE LOWER(?) ESCAPE '\\')"
		pattern := escapeLikePattern(queryText)
		args = append(args, pattern, pattern, pattern)
	}
	query += " ORDER BY i.kind, i.title, i.id LIMIT ?"
	args = append(args, limit)

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("listar itens IPTV: %w", err)
	}
	defer rows.Close()

	items := make([]iptv.Item, 0)
	for rows.Next() {
		var (
			item                                   iptv.Item
			kindValue, classificationValue         string
			season, episode, hasSeason, hasEpisode int
		)
		if err := rows.Scan(
			&item.ID, &item.SourceID, &item.SourceName, &kindValue, &classificationValue,
			&item.Title, &item.RawTitle, &item.Group, &item.LogoURL, &item.EPGID,
			&item.ChannelNumber, &item.Language, &item.Country, &season, &episode,
			&hasSeason, &hasEpisode,
		); err != nil {
			return nil, fmt.Errorf("listar itens IPTV scan: %w", err)
		}
		item.Kind = iptv.ContentKind(kindValue)
		item.Classification = iptv.ClassificationSource(classificationValue)
		item.Episode = iptv.EpisodeRef{
			Season: season, Episode: episode,
			HasSeason: hasSeason != 0, HasEpisode: hasEpisode != 0,
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("listar itens IPTV rows: %w", err)
	}
	return items, nil
}

// ListIPTVGroups returns distinct group names for a content kind.
func (r *Repository) ListIPTVGroups(ctx context.Context, kind iptv.ContentKind) ([]string, error) {
	query := `SELECT DISTINCT group_name FROM iptv_items WHERE group_name != ''`
	args := []any{}
	if kind != "" {
		query += ` AND kind = ?`
		args = append(args, string(kind))
	}
	query += ` ORDER BY group_name`
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("listar grupos IPTV: %w", err)
	}
	defer rows.Close()

	groups := make([]string, 0)
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, fmt.Errorf("listar grupos IPTV scan: %w", err)
		}
		if trimmed := strings.TrimSpace(name); trimmed != "" {
			groups = append(groups, trimmed)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("listar grupos IPTV rows: %w", err)
	}
	return groups, nil
}

// ListIPTVItemsPaginated returns paginated and filtered items along with total item count and total pages.
func (r *Repository) ListIPTVItemsPaginated(ctx context.Context, filter iptv.ItemFilter) (iptv.PageResult, error) {
	pageSize := filter.PageSize
	if pageSize <= 0 {
		pageSize = 50
	}
	if pageSize > 200 {
		pageSize = 200
	}
	page := filter.Page
	if page <= 0 {
		page = 1
	}
	offset := (page - 1) * pageSize

	baseWhere := " WHERE 1 = 1"
	args := []any{}
	if filter.SourceID != "" {
		baseWhere += " AND i.source_id = ?"
		args = append(args, filter.SourceID)
	}
	if filter.Kind != "" {
		baseWhere += " AND i.kind = ?"
		args = append(args, string(filter.Kind))
	}
	if filter.Group != "" {
		baseWhere += " AND i.group_name = ?"
		args = append(args, filter.Group)
	}
	if filter.Query != "" {
		baseWhere += " AND (LOWER(i.title) LIKE LOWER(?) ESCAPE '\\' OR LOWER(i.group_name) LIKE LOWER(?) ESCAPE '\\' OR LOWER(s.name) LIKE LOWER(?) ESCAPE '\\')"
		pat := escapeLikePattern(filter.Query)
		args = append(args, pat, pat, pat)
	}

	fromClause := " FROM iptv_items i JOIN iptv_sources s ON s.id = i.source_id"
	if filter.SavedOnly {
		fromClause = " FROM iptv_saved_items saved JOIN iptv_items i ON i.id = saved.item_id JOIN iptv_sources s ON s.id = i.source_id"
	}

	countQuery := "SELECT COUNT(*)" + fromClause + baseWhere
	var totalCount int
	if err := r.db.QueryRowContext(ctx, countQuery, args...).Scan(&totalCount); err != nil {
		return iptv.PageResult{}, fmt.Errorf("contar itens IPTV: %w", err)
	}

	totalPages := (totalCount + pageSize - 1) / pageSize
	if totalPages == 0 {
		totalPages = 1
	}

	selectQuery := `
		SELECT i.id, i.source_id, s.name, i.kind, i.classification, i.title,
		       i.raw_title, i.group_name, i.logo_url, i.epg_id, i.channel_number,
		       i.language, i.country, i.season, i.episode, i.has_season,
		       i.has_episode` + fromClause + baseWhere + " ORDER BY i.title, i.id LIMIT ? OFFSET ?"

	selectArgs := append(append([]any{}, args...), pageSize, offset)
	rows, err := r.db.QueryContext(ctx, selectQuery, selectArgs...)
	if err != nil {
		return iptv.PageResult{}, fmt.Errorf("listar itens IPTV paginados: %w", err)
	}
	defer rows.Close()

	items := make([]iptv.Item, 0, pageSize)
	for rows.Next() {
		var (
			item                                   iptv.Item
			kindValue, classificationValue         string
			season, episode, hasSeason, hasEpisode int
		)
		if err := rows.Scan(
			&item.ID, &item.SourceID, &item.SourceName, &kindValue, &classificationValue,
			&item.Title, &item.RawTitle, &item.Group, &item.LogoURL, &item.EPGID,
			&item.ChannelNumber, &item.Language, &item.Country, &season, &episode,
			&hasSeason, &hasEpisode,
		); err != nil {
			return iptv.PageResult{}, fmt.Errorf("listar itens IPTV paginados scan: %w", err)
		}
		item.Kind = iptv.ContentKind(kindValue)
		item.Classification = iptv.ClassificationSource(classificationValue)
		item.Episode = iptv.EpisodeRef{
			Season: season, Episode: episode,
			HasSeason: hasSeason != 0, HasEpisode: hasEpisode != 0,
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return iptv.PageResult{}, fmt.Errorf("listar itens IPTV paginados rows: %w", err)
	}

	return iptv.PageResult{
		Items:      items,
		TotalCount: totalCount,
		Page:       page,
		PageSize:   pageSize,
		TotalPages: totalPages,
	}, nil
}

// ListIPTVGuide returns programs overlapping a time window, joined with
// channel metadata for rendering the EPG. A zero-width window (from == to)
// is valid and means "programs covering that instant"; only a zero `to`
// defaults to 24 hours after `from`.
func (r *Repository) ListIPTVGuide(ctx context.Context, sourceID string, from, to time.Time, limit int) ([]iptv.GuideEntry, error) {
	if from.IsZero() {
		from = time.Now().UTC()
	}
	if to.IsZero() {
		to = from.Add(24 * time.Hour)
	}
	if limit <= 0 {
		limit = 200
	}
	if limit > 1000 {
		limit = 1000
	}
	query := `
		SELECT p.source_id, p.channel_id, c.name, c.logo_url, p.title,
		       p.description, p.start_at, p.end_at
		FROM iptv_guide_programs p
		JOIN iptv_guide_channels c ON c.source_id = p.source_id AND c.id = p.channel_id
		WHERE p.start_at < ? AND p.end_at > ?`
	args := []any{fmtTime(to), fmtTime(from)}
	if strings.TrimSpace(sourceID) != "" {
		query += " AND p.source_id = ?"
		args = append(args, strings.TrimSpace(sourceID))
	}
	query += " ORDER BY p.start_at, c.name, p.title LIMIT ?"
	args = append(args, limit)
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("listar guia IPTV: %w", err)
	}
	defer rows.Close()
	entries := make([]iptv.GuideEntry, 0)
	for rows.Next() {
		var entry iptv.GuideEntry
		var startAt, endAt string
		if err := rows.Scan(&entry.SourceID, &entry.ChannelID, &entry.ChannelName, &entry.LogoURL,
			&entry.Program.Title, &entry.Program.Description, &startAt, &endAt); err != nil {
			return nil, fmt.Errorf("listar guia IPTV scan: %w", err)
		}
		entry.Program.ChannelID = entry.ChannelID
		entry.Program.Start = parseTime(startAt)
		entry.Program.End = parseTime(endAt)
		entries = append(entries, entry)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("listar guia IPTV rows: %w", err)
	}
	return entries, nil
}

type rowScanner interface {
	Scan(...any) error
}

// ImportIPTVCatalog substitui o catálogo da fonte consumindo itens em
// lotes, sem acumular a playlist inteira em memória. Tudo acontece em uma
// única transação: um lote ruim aborta e o snapshot anterior permanece.
// Documento canônico: docs/03-implementation/NANOIPTV.md ("importação
// incremental").
func (r *Repository) ImportIPTVCatalog(ctx context.Context, source iptv.SourceConfig, syncedAt time.Time, batches func(yield func([]iptv.Item) error) error) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("iptv import begin: %w", err)
	}
	defer tx.Rollback()

	if err := upsertIPTVSourceTx(ctx, tx, source, syncedAt); err != nil {
		return fmt.Errorf("iptv import source: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM iptv_items WHERE source_id = ?`, source.ID); err != nil {
		return fmt.Errorf("iptv import clear: %w", err)
	}

	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO iptv_items (
			id, source_id, kind, classification, title, raw_title,
			group_name, logo_url, epg_id, channel_number, language, country,
			season, episode, has_season, has_episode, first_seen_at, last_seen_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			source_id = excluded.source_id,
			kind = excluded.kind,
			classification = excluded.classification,
			title = excluded.title,
			raw_title = excluded.raw_title,
			group_name = excluded.group_name,
			logo_url = excluded.logo_url,
			epg_id = excluded.epg_id,
			channel_number = excluded.channel_number,
			language = excluded.language,
			country = excluded.country,
			season = excluded.season,
			episode = excluded.episode,
			has_season = excluded.has_season,
			has_episode = excluded.has_episode,
			last_seen_at = excluded.last_seen_at`)
	if err != nil {
		return fmt.Errorf("iptv import prepare: %w", err)
	}
	defer stmt.Close()

	stamp := fmtTime(syncedAt)
	insertBatch := func(batch []iptv.Item) error {
		for i := range batch {
			item := &batch[i]
			item.StreamURL = ""
			if _, err := stmt.ExecContext(ctx,
				item.ID, source.ID, string(item.Kind), string(item.Classification),
				item.Title, item.RawTitle, item.Group, item.LogoURL, item.EPGID,
				item.ChannelNumber, item.Language, item.Country,
				item.Episode.Season, item.Episode.Episode,
				boolInt(item.Episode.HasSeason), boolInt(item.Episode.HasEpisode),
				stamp, stamp,
			); err != nil {
				return fmt.Errorf("inserir item IPTV %s: %w", item.ID, err)
			}
		}
		return nil
	}
	if err := batches(insertBatch); err != nil {
		return fmt.Errorf("iptv import stream: %w", err)
	}

	nowText := fmtTime(syncedAt)
	if _, err := tx.ExecContext(ctx, `UPDATE iptv_sources SET last_sync_at = ?, last_error = '' WHERE id = ?`, nowText, source.ID); err != nil {
		return fmt.Errorf("iptv import meta: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("iptv import commit: %w", err)
	}
	return nil
}

func scanIPTVSource(rows rowScanner) (iptv.SourceState, error) {
	var state iptv.SourceState
	var format, lastSync string
	var enabled int
	if err := rows.Scan(&state.Config.ID, &state.Config.Name, &format,
		&state.Config.PlaylistURL, &state.Config.GuideURL, &state.Config.CredentialRef,
		&enabled, &lastSync, &state.LastError); err != nil {
		return iptv.SourceState{}, err
	}
	state.Config.Format = iptv.SourceFormat(format)
	state.Config.Enabled = enabled != 0
	state.LastSyncAt = parseTime(lastSync)
	return state, nil
}

// ReplaceIPTVGuide substitui o EPG da fonte de maneira atômica. Programas
// precisam referenciar canais presentes no mesmo snapshot.
func (r *Repository) ReplaceIPTVGuide(ctx context.Context, sourceID string, channels []iptv.GuideChannel, programs []iptv.GuideProgram) error {
	sourceID = strings.TrimSpace(sourceID)
	if sourceID == "" {
		return fmt.Errorf("substituir EPG IPTV: source ID vazio")
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("iptv begin guide: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, `DELETE FROM iptv_guide_programs WHERE source_id = ?`, sourceID); err != nil {
		return fmt.Errorf("limpar programas IPTV: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM iptv_guide_channels WHERE source_id = ?`, sourceID); err != nil {
		return fmt.Errorf("limpar canais IPTV: %w", err)
	}

	channelStmt, err := tx.PrepareContext(ctx, `
		INSERT INTO iptv_guide_channels (source_id, id, name, logo_url)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(source_id, id) DO UPDATE SET
			name = excluded.name,
			logo_url = excluded.logo_url`)
	if err != nil {
		return fmt.Errorf("preparar canais IPTV: %w", err)
	}
	for _, channel := range channels {
		if strings.TrimSpace(channel.ID) == "" || strings.TrimSpace(channel.Name) == "" {
			channelStmt.Close()
			return fmt.Errorf("canais IPTV: ID e nome são obrigatórios")
		}
		if _, err := channelStmt.ExecContext(ctx, sourceID, channel.ID, channel.Name, channel.LogoURL); err != nil {
			channelStmt.Close()
			return fmt.Errorf("inserir canal IPTV %s: %w", channel.ID, err)
		}
	}
	if err := channelStmt.Close(); err != nil {
		return fmt.Errorf("fechar canais IPTV: %w", err)
	}

	programStmt, err := tx.PrepareContext(ctx, `
		INSERT INTO iptv_guide_programs
			(source_id, channel_id, title, description, start_at, end_at)
		VALUES (?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return fmt.Errorf("preparar programas IPTV: %w", err)
	}
	for _, program := range programs {
		if err := validateGuideProgram(program); err != nil {
			programStmt.Close()
			return err
		}
		if _, err := programStmt.ExecContext(ctx, sourceID, program.ChannelID, program.Title, program.Description, fmtTime(program.Start), fmtTime(program.End)); err != nil {
			programStmt.Close()
			return fmt.Errorf("inserir programa IPTV: %w", err)
		}
	}
	if err := programStmt.Close(); err != nil {
		return fmt.Errorf("fechar programas IPTV: %w", err)
	}

	return tx.Commit()
}

func (r *Repository) CurrentIPTVPrograms(ctx context.Context, sourceID, channelID string, at time.Time) ([]iptv.GuideProgram, error) {
	sourceID = strings.TrimSpace(sourceID)
	channelID = strings.TrimSpace(channelID)
	if sourceID == "" || channelID == "" {
		return nil, fmt.Errorf("consultar EPG IPTV: source e channel ID são obrigatórios")
	}
	if at.IsZero() {
		at = time.Now().UTC()
	}
	instant := fmtTime(at)
	rows, err := r.db.QueryContext(ctx, `
		SELECT channel_id, title, description, start_at, end_at
		FROM iptv_guide_programs
		WHERE source_id = ? AND channel_id = ? AND start_at <= ? AND end_at > ?
		ORDER BY start_at`, sourceID, channelID, instant, instant)
	if err != nil {
		return nil, fmt.Errorf("consultar EPG IPTV: %w", err)
	}
	defer rows.Close()

	programs := make([]iptv.GuideProgram, 0)
	for rows.Next() {
		var (
			program        iptv.GuideProgram
			startAt, endAt string
		)
		if err := rows.Scan(&program.ChannelID, &program.Title, &program.Description, &startAt, &endAt); err != nil {
			return nil, fmt.Errorf("consultar EPG IPTV scan: %w", err)
		}
		program.Start = parseTime(startAt)
		program.End = parseTime(endAt)
		programs = append(programs, program)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("consultar EPG IPTV rows: %w", err)
	}
	return programs, nil
}

func prepareIPTVSource(source iptv.SourceConfig) (iptv.SourceConfig, error) {
	source.ID = strings.TrimSpace(source.ID)
	source.Name = strings.TrimSpace(source.Name)
	if source.ID == "" {
		return iptv.SourceConfig{}, fmt.Errorf("fonte IPTV: ID vazio")
	}
	if source.Name == "" {
		source.Name = source.ID
	}
	if source.Format == "" {
		source.Format = iptv.SourceFormatM3U
	}
	if source.Format != iptv.SourceFormatM3U {
		return iptv.SourceConfig{}, fmt.Errorf("fonte IPTV: formato %q não suportado", source.Format)
	}
	var err error
	if source.PlaylistURL, err = persistableEndpoint(source.PlaylistURL); err != nil {
		return iptv.SourceConfig{}, fmt.Errorf("fonte IPTV playlist: %w", err)
	}
	if source.GuideURL, err = persistableEndpoint(source.GuideURL); err != nil {
		return iptv.SourceConfig{}, fmt.Errorf("fonte IPTV guia: %w", err)
	}
	return source, nil
}

func persistableEndpoint(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", nil
	}
	parsed, err := url.Parse(value)
	if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return "", fmt.Errorf("endpoint HTTP(S) inválido")
	}
	if parsed.User != nil {
		return "", ErrSensitiveEndpoint
	}
	for key := range parsed.Query() {
		switch strings.ToLower(key) {
		case "user", "username", "pass", "password", "token", "auth", "key", "api_key", "apikey", "credential":
			return "", ErrSensitiveEndpoint
		}
	}
	return value, nil
}

func upsertIPTVSourceTx(ctx context.Context, tx *sql.Tx, source iptv.SourceConfig, syncedAt time.Time) error {
	_, err := tx.ExecContext(ctx, `
		INSERT INTO iptv_sources
			(id, name, format, playlist_endpoint, guide_endpoint, credential_ref,
			 enabled, last_sync_at, last_error)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, '')
		ON CONFLICT(id) DO UPDATE SET
			name = excluded.name,
			format = excluded.format,
			playlist_endpoint = excluded.playlist_endpoint,
			guide_endpoint = excluded.guide_endpoint,
			credential_ref = excluded.credential_ref,
			enabled = excluded.enabled,
			last_sync_at = excluded.last_sync_at,
			last_error = ''`,
		source.ID, source.Name, source.Format, source.PlaylistURL, source.GuideURL,
		source.CredentialRef, boolInt(source.Enabled), fmtTime(syncedAt))
	if err != nil {
		return fmt.Errorf("upsert fonte IPTV %s: %w", source.ID, err)
	}
	return nil
}

func replaceIPTVItemsTx(ctx context.Context, tx *sql.Tx, sourceID string, items []iptv.Item, now time.Time) error {
	if len(items) == 0 {
		if _, err := tx.ExecContext(ctx, `DELETE FROM iptv_items WHERE source_id = ?`, sourceID); err != nil {
			return fmt.Errorf("limpar itens IPTV: %w", err)
		}
		return nil
	}

	insert, err := tx.PrepareContext(ctx, `
		INSERT INTO iptv_items
			(id, source_id, kind, classification, title, raw_title, group_name,
			 logo_url, epg_id, channel_number, language, country, season, episode,
			 has_season, has_episode, first_seen_at, last_seen_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			source_id = excluded.source_id,
			kind = excluded.kind,
			classification = excluded.classification,
			title = excluded.title,
			raw_title = excluded.raw_title,
			group_name = excluded.group_name,
			logo_url = excluded.logo_url,
			epg_id = excluded.epg_id,
			channel_number = excluded.channel_number,
			language = excluded.language,
			country = excluded.country,
			season = excluded.season,
			episode = excluded.episode,
			has_season = excluded.has_season,
			has_episode = excluded.has_episode,
			first_seen_at = CASE WHEN iptv_items.first_seen_at = '' THEN excluded.first_seen_at ELSE iptv_items.first_seen_at END,
			last_seen_at = excluded.last_seen_at`)
	if err != nil {
		return fmt.Errorf("preparar itens IPTV: %w", err)
	}
	for _, item := range items {
		if _, err := insert.ExecContext(ctx,
			item.ID, sourceID, item.Kind, item.Classification, item.Title, item.RawTitle,
			item.Group, item.LogoURL, item.EPGID, item.ChannelNumber, item.Language,
			item.Country, item.Episode.Season, item.Episode.Episode,
			boolInt(item.Episode.HasSeason), boolInt(item.Episode.HasEpisode),
			fmtTime(now), fmtTime(now)); err != nil {
			insert.Close()
			return fmt.Errorf("inserir item IPTV %s: %w", item.ID, err)
		}
	}
	if err := insert.Close(); err != nil {
		return fmt.Errorf("fechar itens IPTV: %w", err)
	}

	if _, err := tx.ExecContext(ctx, `CREATE TEMP TABLE iptv_sync_item_ids (id TEXT PRIMARY KEY)`); err != nil {
		return fmt.Errorf("criar índice temporário IPTV: %w", err)
	}
	defer func() {
		_, _ = tx.ExecContext(ctx, `DROP TABLE IF EXISTS iptv_sync_item_ids`)
	}()

	seen := make(map[string]struct{}, len(items))
	for _, item := range items {
		seen[item.ID] = struct{}{}
	}
	ids, err := tx.PrepareContext(ctx, `INSERT OR IGNORE INTO iptv_sync_item_ids (id) VALUES (?)`)
	if err != nil {
		return fmt.Errorf("preparar índice temporário IPTV: %w", err)
	}
	for id := range seen {
		if _, err := ids.ExecContext(ctx, id); err != nil {
			ids.Close()
			return fmt.Errorf("indexar item IPTV: %w", err)
		}
	}
	if err := ids.Close(); err != nil {
		return fmt.Errorf("fechar índice temporário IPTV: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM iptv_items
		WHERE source_id = ?
		  AND NOT EXISTS (SELECT 1 FROM iptv_sync_item_ids sync_ids WHERE sync_ids.id = iptv_items.id)`, sourceID); err != nil {
		return fmt.Errorf("remover itens IPTV obsoletos: %w", err)
	}
	return nil
}

func validateIPTVItems(sourceID string, items []iptv.Item) error {
	for _, item := range items {
		if strings.TrimSpace(item.ID) == "" {
			return fmt.Errorf("catálogo IPTV: item sem ID")
		}
		if item.SourceID != "" && item.SourceID != sourceID {
			return fmt.Errorf("catálogo IPTV: item %s pertence a outra fonte", item.ID)
		}
	}
	return nil
}

func validateGuideProgram(program iptv.GuideProgram) error {
	if strings.TrimSpace(program.ChannelID) == "" || strings.TrimSpace(program.Title) == "" {
		return fmt.Errorf("programa IPTV: canal e título são obrigatórios")
	}
	if program.Start.IsZero() || program.End.IsZero() || !program.End.After(program.Start) {
		return fmt.Errorf("programa IPTV: janela de tempo inválida")
	}
	return nil
}

func normalizeSyncTime(value time.Time) time.Time {
	if value.IsZero() {
		return time.Now().UTC()
	}
	return value.UTC()
}
