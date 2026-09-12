// Organização local de canais (M9/QOL-03): favoritos e pastas de inscrições.
// É estado puramente local: nada é enviado ao YouTube.
// Documento canônico: docs/03-implementation/STORAGE.md
package storage

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/nanotube/nanotube-web/internal/domain"
)

// ChannelFolderID gera um id curto e estável para uma pasta nova.
func ChannelFolderID() (string, error) {
	buf := make([]byte, 6)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("canais: gerar id: %w", err)
	}
	return "cf_" + hex.EncodeToString(buf), nil
}

// ---------------------------------------------------------------- favoritos

// AddChannelFavorite marca um canal como favorito local. Repetir não duplica.
func (r *Repository) AddChannelFavorite(ctx context.Context, channelID string) error {
	if channelID == "" {
		return errors.New("canais: canal id vazio")
	}
	if _, err := r.db.ExecContext(ctx, `
		INSERT INTO channel_favorites (channel_id, created_at) VALUES (?, ?)
		ON CONFLICT(channel_id) DO NOTHING`, channelID, fmtTime(time.Now())); err != nil {
		return fmt.Errorf("canais favoritos: %w", err)
	}
	return nil
}

// RemoveChannelFavorite tira a estrela de um canal. Remover o que não está
// marcado não é erro.
func (r *Repository) RemoveChannelFavorite(ctx context.Context, channelID string) error {
	if _, err := r.db.ExecContext(ctx,
		`DELETE FROM channel_favorites WHERE channel_id = ?`, channelID); err != nil {
		return fmt.Errorf("canais favoritos: %w", err)
	}
	return nil
}

// FavoriteChannelIDs devolve o conjunto de canais favoritos. A UI usa isso
// para decidir se o menu oferece "favoritar" ou "desfavoritar".
func (r *Repository) FavoriteChannelIDs(ctx context.Context) (map[string]bool, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT channel_id FROM channel_favorites`)
	if err != nil {
		return nil, fmt.Errorf("canais favoritos: %w", err)
	}
	defer rows.Close()

	ids := map[string]bool{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("canais favoritos scan: %w", err)
		}
		ids[id] = true
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("canais favoritos rows: %w", err)
	}
	return ids, nil
}

// ------------------------------------------------------------------ pastas

// CreateFolder cria uma pasta local de inscrições e devolve a entidade.
func (r *Repository) CreateFolder(ctx context.Context, name string) (domain.ChannelFolder, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return domain.ChannelFolder{}, errors.New("canais: nome de pasta vazio")
	}
	id, err := ChannelFolderID()
	if err != nil {
		return domain.ChannelFolder{}, err
	}
	now := fmtTime(time.Now())
	if _, err := r.db.ExecContext(ctx, `
		INSERT INTO channel_folders (id, name, created_at) VALUES (?, ?, ?)`,
		id, name, now); err != nil {
		return domain.ChannelFolder{}, fmt.Errorf("canais pastas: %w", err)
	}
	return domain.ChannelFolder{ID: id, Name: name, CreatedAt: parseTime(now)}, nil
}

// RenameFolder atualiza o nome de uma pasta.
func (r *Repository) RenameFolder(ctx context.Context, folderID, name string) error {
	name = strings.TrimSpace(name)
	if folderID == "" || name == "" {
		return errors.New("canais: pasta inválida")
	}
	res, err := r.db.ExecContext(ctx,
		`UPDATE channel_folders SET name = ? WHERE id = ?`, name, folderID)
	if err != nil {
		return fmt.Errorf("canais pastas: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return errors.New("canais pastas: pasta não encontrada")
	}
	return nil
}

// DeleteFolder remove a pasta e a associação com os canais (ON DELETE CASCADE).
func (r *Repository) DeleteFolder(ctx context.Context, folderID string) error {
	if _, err := r.db.ExecContext(ctx,
		`DELETE FROM channel_folders WHERE id = ?`, folderID); err != nil {
		return fmt.Errorf("canais pastas: %w", err)
	}
	return nil
}

// Folders lista as pastas, mais recentes primeiro.
func (r *Repository) Folders(ctx context.Context) ([]domain.ChannelFolder, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, name, created_at FROM channel_folders ORDER BY created_at DESC, id`)
	if err != nil {
		return nil, fmt.Errorf("canais pastas: %w", err)
	}
	defer rows.Close()

	var out []domain.ChannelFolder
	for rows.Next() {
		var f domain.ChannelFolder
		var created string
		if err := rows.Scan(&f.ID, &f.Name, &created); err != nil {
			return nil, fmt.Errorf("canais pastas scan: %w", err)
		}
		f.CreatedAt = parseTime(created)
		out = append(out, f)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("canais pastas rows: %w", err)
	}
	return out, nil
}

// AddChannelToFolder coloca um canal em uma pasta. Repetir não duplica nem
// erra (PK composta + DO NOTHING).
func (r *Repository) AddChannelToFolder(ctx context.Context, folderID, channelID string) error {
	if folderID == "" || channelID == "" {
		return errors.New("canais: pasta/canal vazio")
	}
	if _, err := r.db.ExecContext(ctx, `
		INSERT INTO channel_folder_items (folder_id, channel_id, created_at) VALUES (?, ?, ?)
		ON CONFLICT(folder_id, channel_id) DO NOTHING`,
		folderID, channelID, fmtTime(time.Now())); err != nil {
		return fmt.Errorf("canais pastas: %w", err)
	}
	return nil
}

// RemoveChannelFromFolder tira um canal de uma pasta. Não é erro quando o
// canal não está na pasta.
func (r *Repository) RemoveChannelFromFolder(ctx context.Context, folderID, channelID string) error {
	if _, err := r.db.ExecContext(ctx,
		`DELETE FROM channel_folder_items WHERE folder_id = ? AND channel_id = ?`,
		folderID, channelID); err != nil {
		return fmt.Errorf("canais pastas: %w", err)
	}
	return nil
}

// FolderMembership devolve, por pasta, os ids dos canais associados. A UI
// decide na memória qual pasta está aberta (sem reconsulta ao banco).
func (r *Repository) FolderMembership(ctx context.Context) (map[string][]string, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT folder_id, channel_id FROM channel_folder_items`)
	if err != nil {
		return nil, fmt.Errorf("canais pastas: %w", err)
	}
	defer rows.Close()

	members := map[string][]string{}
	for rows.Next() {
		var folderID, channelID string
		if err := rows.Scan(&folderID, &channelID); err != nil {
			return nil, fmt.Errorf("canais pastas scan: %w", err)
		}
		members[folderID] = append(members[folderID], channelID)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("canais pastas rows: %w", err)
	}
	return members, nil
}
