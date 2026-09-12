package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/nanotube/nanotube-web/internal/domain"
)

const maxSubscriptionPageSize = 60

// SubscriptionVideos pagina e filtra antes do LIMIT para que "Carregar mais"
// seja estável e não esconda resultados que deveriam passar pelo filtro.
func (r *Repository) SubscriptionVideos(ctx context.Context, query domain.SubscriptionVideoQuery) (domain.SubscriptionVideoPage, error) {
	if query.Limit <= 0 {
		query.Limit = 24
	}
	if query.Limit > maxSubscriptionPageSize {
		query.Limit = maxSubscriptionPageSize
	}
	if query.Offset < 0 {
		query.Offset = 0
	}
	where := []string{"c.subscribed = 1"}
	args := make([]any, 0, 8)
	if query.ChannelID != "" {
		where = append(where, "v.channel_id = ?")
		args = append(args, strings.TrimSpace(query.ChannelID))
	}
	if query.Category != "" {
		where = append(where, "LOWER(v.category) = LOWER(?)")
		args = append(args, strings.TrimSpace(query.Category))
	}
	if !query.From.IsZero() {
		where = append(where, "v.published_at >= ?")
		args = append(args, fmtTime(query.From))
	}
	if !query.To.IsZero() {
		where = append(where, "v.published_at < ?")
		args = append(args, fmtTime(query.To))
	}
	switch query.Content {
	case domain.FeedRegular:
		where = append(where, "LOWER(COALESCE(v.live_status, '')) NOT IN ('is_live','live','is_upcoming','upcoming','post_live','was_live','completed')")
	case domain.FeedLive:
		where = append(where, "LOWER(COALESCE(v.live_status, '')) IN ('is_live','live','is_upcoming','upcoming','post_live','was_live','completed')")
	}
	switch query.Watch {
	case domain.SubscriptionWatchWatched:
		where = append(where, "p.video_id IS NOT NULL")
	case domain.SubscriptionWatchUnwatched:
		where = append(where, "p.video_id IS NULL")
	}

	fromSQL := ` FROM videos v
		JOIN channels c ON c.id = v.channel_id
		LEFT JOIN playback_progress p ON p.video_id = v.id
		WHERE ` + strings.Join(where, " AND ")
	var total int
	if err := r.db.QueryRowContext(ctx, `SELECT COUNT(*)`+fromSQL, args...).Scan(&total); err != nil {
		return domain.SubscriptionVideoPage{}, fmt.Errorf("inscrições: contar vídeos: %w", err)
	}
	pageArgs := append(append([]any{}, args...), query.Limit, query.Offset)
	rows, err := r.db.QueryContext(ctx, `
		SELECT `+videoColumns+`, c.title`+fromSQL+`
		ORDER BY v.published_at DESC, v.id
		LIMIT ? OFFSET ?`, pageArgs...)
	if err != nil {
		return domain.SubscriptionVideoPage{}, fmt.Errorf("inscrições: listar vídeos: %w", err)
	}
	defer rows.Close()
	videos := make([]domain.Video, 0, query.Limit)
	for rows.Next() {
		video, err := scanSubscriptionVideo(rows)
		if err != nil {
			return domain.SubscriptionVideoPage{}, err
		}
		videos = append(videos, video)
	}
	if err := rows.Err(); err != nil {
		return domain.SubscriptionVideoPage{}, fmt.Errorf("inscrições: listar vídeos: %w", err)
	}
	categories, err := r.SubscriptionCategories(ctx)
	if err != nil {
		return domain.SubscriptionVideoPage{}, err
	}
	return domain.SubscriptionVideoPage{
		Videos: videos, Categories: categories, Total: total,
		Offset: query.Offset, Limit: query.Limit, HasMore: query.Offset+len(videos) < total,
	}, nil
}

func scanSubscriptionVideo(scanner interface{ Scan(...any) error }) (domain.Video, error) {
	var video domain.Video
	var publishedAt, firstSeenAt, lastSeenAt string
	var durationSeconds int64
	if err := scanner.Scan(
		&video.ID, &video.ChannelID, &video.Title, &video.Description, &video.DescriptionExcerpt,
		&publishedAt, &durationSeconds, &video.Category, &video.ThumbnailURL,
		&video.LiveStatus, &firstSeenAt, &lastSeenAt, &video.ChannelTitle,
	); err != nil {
		return domain.Video{}, fmt.Errorf("inscrições: ler vídeo: %w", err)
	}
	video.PublishedAt = parseTime(publishedAt)
	video.Duration = time.Duration(durationSeconds) * time.Second
	video.FirstSeenAt = parseTime(firstSeenAt)
	video.LastSeenAt = parseTime(lastSeenAt)
	return video, nil
}

func (r *Repository) SubscriptionCategories(ctx context.Context) ([]string, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT DISTINCT v.category FROM videos v
		JOIN channels c ON c.id = v.channel_id
		WHERE c.subscribed = 1 AND TRIM(v.category) <> ''
		ORDER BY LOWER(v.category)`)
	if err != nil {
		return nil, fmt.Errorf("inscrições: listar categorias: %w", err)
	}
	defer rows.Close()
	categories := make([]string, 0)
	for rows.Next() {
		var category string
		if err := rows.Scan(&category); err != nil {
			return nil, fmt.Errorf("inscrições: ler categoria: %w", err)
		}
		categories = append(categories, category)
	}
	return categories, rows.Err()
}

// ManagedChannels calcula categoria dominante e último vídeo visto sem
// duplicar essas projeções no schema.
func (r *Repository) ManagedChannels(ctx context.Context) ([]domain.ManagedChannel, error) {
	profileID, err := r.ActiveProfileID(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT `+channelCols+`, c.subscribed_at,
			COALESCE((SELECT v.category FROM videos v
				WHERE v.channel_id = c.id AND TRIM(v.category) <> ''
				GROUP BY v.category ORDER BY COUNT(*) DESC, MAX(v.published_at) DESC LIMIT 1), ''),
			COALESCE((SELECT MAX(p.updated_at) FROM playback_progress p
				JOIN videos watched ON watched.id = p.video_id WHERE watched.channel_id = c.id), '')
		FROM channels c WHERE c.subscribed = 1 ORDER BY LOWER(c.title), c.id`)
	if err != nil {
		return nil, fmt.Errorf("canais: listar gerenciamento: %w", err)
	}
	defer rows.Close()
	channels := make([]domain.ManagedChannel, 0)
	for rows.Next() {
		var managed domain.ManagedChannel
		var subscribed, lastSync, subscribedAt, lastWatchedAt intOrString
		if err := rows.Scan(
			&managed.Channel.ID, &managed.Channel.Title, &subscribed,
			&managed.Channel.UploadsPlaylistID, &lastSync,
			&managed.Channel.LastKnownVideoID, &managed.Channel.LastError, &managed.Channel.ThumbnailURL,
			&subscribedAt, &managed.Category, &lastWatchedAt,
		); err != nil {
			return nil, fmt.Errorf("canais: ler gerenciamento: %w", err)
		}
		managed.Channel.Subscribed = subscribed.Int != 0
		managed.Channel.LastSyncAt = parseTime(lastSync.String)
		managed.SubscribedAt = parseTime(subscribedAt.String)
		managed.LastWatchedAt = parseTime(lastWatchedAt.String)
		channels = append(channels, managed)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("canais: listar gerenciamento: %w", err)
	}
	tags, err := r.channelTagsByChannel(ctx, profileID)
	if err != nil {
		return nil, err
	}
	for index := range channels {
		channels[index].Tags = tags[channels[index].Channel.ID]
		if channels[index].Tags == nil {
			channels[index].Tags = []string{}
		}
	}
	return channels, nil
}

// intOrString aceita INTEGER/TEXT sem depender de conversões implícitas do driver.
type intOrString struct {
	Int    int64
	String string
}

func (value *intOrString) Scan(source any) error {
	switch typed := source.(type) {
	case int64:
		value.Int = typed
	case string:
		value.String = typed
	case []byte:
		value.String = string(typed)
	case nil:
	default:
		return fmt.Errorf("tipo SQLite inesperado: %T", source)
	}
	return nil
}

func (r *Repository) channelTagsByChannel(ctx context.Context, profileID string) (map[string][]string, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT channel_id, tag FROM channel_tags WHERE profile_id = ? ORDER BY LOWER(tag)`, profileID)
	if err != nil {
		return nil, fmt.Errorf("canais: listar tags: %w", err)
	}
	defer rows.Close()
	tags := make(map[string][]string)
	for rows.Next() {
		var channelID, tag string
		if err := rows.Scan(&channelID, &tag); err != nil {
			return nil, fmt.Errorf("canais: ler tags: %w", err)
		}
		tags[channelID] = append(tags[channelID], tag)
	}
	return tags, rows.Err()
}

func (r *Repository) SetChannelTags(ctx context.Context, channelID string, tags []string) error {
	profileID, err := r.ActiveProfileID(ctx)
	if err != nil {
		return err
	}
	normalized := make(map[string]string)
	for _, tag := range tags {
		trimmed := strings.TrimSpace(tag)
		if trimmed == "" || len([]rune(trimmed)) > 40 {
			continue
		}
		normalized[strings.ToLower(trimmed)] = trimmed
	}
	if len(normalized) > 20 {
		return errors.New("canais: limite de 20 tags por canal")
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("canais: iniciar tags: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(ctx, `DELETE FROM channel_tags WHERE profile_id = ? AND channel_id = ?`, profileID, channelID); err != nil {
		return fmt.Errorf("canais: remover tags: %w", err)
	}
	keys := make([]string, 0, len(normalized))
	for key := range normalized {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO channel_tags (profile_id, channel_id, tag, created_at) VALUES (?, ?, ?, ?)`,
			profileID, channelID, normalized[key], fmtTime(time.Now())); err != nil {
			return fmt.Errorf("canais: salvar tag: %w", err)
		}
	}
	return tx.Commit()
}

// BulkUnsubscribeChannels aplica a ação destrutiva em uma única transação.
func (r *Repository) BulkUnsubscribeChannels(ctx context.Context, channelIDs []string) error {
	if len(channelIDs) == 0 {
		return errors.New("canais: nenhuma inscrição selecionada")
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("canais: iniciar desinscrição: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	for _, channelID := range uniqueNonEmpty(channelIDs) {
		if _, err := tx.ExecContext(ctx, `UPDATE channels SET subscribed = 0 WHERE id = ?`, channelID); err != nil {
			return fmt.Errorf("canais: desinscrever lote: %w", err)
		}
	}
	return tx.Commit()
}

func (r *Repository) BulkFavoriteChannels(ctx context.Context, channelIDs []string) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("canais: iniciar favoritos: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	for _, channelID := range uniqueNonEmpty(channelIDs) {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO channel_favorites (channel_id, created_at) VALUES (?, ?)
			ON CONFLICT(channel_id) DO NOTHING`, channelID, fmtTime(time.Now())); err != nil {
			return fmt.Errorf("canais: favoritar lote: %w", err)
		}
	}
	return tx.Commit()
}

func (r *Repository) BulkAddChannelsToFolder(ctx context.Context, folderID string, channelIDs []string) error {
	if strings.TrimSpace(folderID) == "" {
		return errors.New("canais: pasta vazia")
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("canais: iniciar pasta: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	for _, channelID := range uniqueNonEmpty(channelIDs) {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO channel_folder_items (folder_id, channel_id, created_at) VALUES (?, ?, ?)
			ON CONFLICT(folder_id, channel_id) DO NOTHING`, folderID, channelID, fmtTime(time.Now())); err != nil {
			return fmt.Errorf("canais: adicionar lote à pasta: %w", err)
		}
	}
	return tx.Commit()
}

func uniqueNonEmpty(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}

var _ sql.Scanner = (*intOrString)(nil)
