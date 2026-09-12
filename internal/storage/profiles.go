package storage

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/nanotube/nanotube-web/internal/domain"
)

const DefaultProfileID = "default"

func ProfileID() (string, error) {
	return profileID("pf_")
}

func profileID(prefix string) (string, error) {
	buf := make([]byte, 6)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return prefix + hex.EncodeToString(buf), nil
}

func (r *Repository) CreateProfile(ctx context.Context, name string) (domain.Profile, error) {
	return r.createProfile(ctx, name, domain.ProfileKindPersistent, "pf_")
}

// CreateGuestProfile cria um perfil temporário. Ele existe no SQLite durante
// a sessão para que FKs e serviços compartilhem o mesmo contexto, mas é
// removido no próximo bootstrap se o processo não o descartar normalmente.
func (r *Repository) CreateGuestProfile(ctx context.Context, name string) (domain.Profile, error) {
	if strings.TrimSpace(name) == "" {
		name = "Convidado"
	}
	return r.createProfile(ctx, name, domain.ProfileKindGuest, "guest_")
}

func (r *Repository) createProfile(ctx context.Context, name string, kind domain.ProfileKind, prefix string) (domain.Profile, error) {
	if strings.TrimSpace(name) == "" {
		return domain.Profile{}, fmt.Errorf("nome vazio")
	}
	id, err := profileID(prefix)
	if err != nil {
		return domain.Profile{}, fmt.Errorf("gerar id de perfil: %w", err)
	}
	now := fmtTime(time.Now())
	_, err = r.db.ExecContext(ctx, `INSERT INTO profiles (id, name, created_at, kind) VALUES (?, ?, ?, ?)`,
		id, strings.TrimSpace(name), now, kind)
	if err != nil {
		return domain.Profile{}, err
	}
	return domain.Profile{ID: id, Name: strings.TrimSpace(name), Kind: kind, CreatedAt: parseTime(now)}, nil
}

func (r *Repository) Profiles(ctx context.Context) ([]domain.Profile, error) {
	if err := r.EnsureDefaultProfile(ctx); err != nil {
		return nil, err
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, name, kind, created_at FROM profiles
		ORDER BY CASE WHEN id = ? THEN 0 ELSE 1 END, created_at, id`, DefaultProfileID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.Profile
	for rows.Next() {
		var p domain.Profile
		var created string
		if err := rows.Scan(&p.ID, &p.Name, &p.Kind, &created); err != nil {
			return nil, err
		}
		p.CreatedAt = parseTime(created)
		out = append(out, p)
	}
	return out, rows.Err()
}

func (r *Repository) Profile(ctx context.Context, id string) (domain.Profile, error) {
	if strings.TrimSpace(id) == "" {
		return domain.Profile{}, fmt.Errorf("perfil: id vazio")
	}
	var profile domain.Profile
	var created string
	err := r.db.QueryRowContext(ctx,
		`SELECT id, name, kind, created_at FROM profiles WHERE id = ?`, id).
		Scan(&profile.ID, &profile.Name, &profile.Kind, &created)
	if err == sql.ErrNoRows {
		return domain.Profile{}, fmt.Errorf("perfil não encontrado")
	}
	if err != nil {
		return domain.Profile{}, err
	}
	profile.CreatedAt = parseTime(created)
	return profile, nil
}

func (r *Repository) RenameProfile(ctx context.Context, id, name string) error {
	if strings.TrimSpace(name) == "" {
		return fmt.Errorf("nome vazio")
	}
	res, err := r.db.ExecContext(ctx, `UPDATE profiles SET name = ? WHERE id = ?`, strings.TrimSpace(name), id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("perfil não encontrado")
	}
	return nil
}

func (r *Repository) DeleteProfile(ctx context.Context, id string) error {
	if id == DefaultProfileID {
		return fmt.Errorf("o perfil padrão não pode ser removido")
	}
	if _, err := r.Profile(ctx, id); err != nil {
		return err
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `
		UPDATE settings SET value = ?
		WHERE key = ? AND value = ?`, DefaultProfileID, SettingActiveProfile, id); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM profiles WHERE id = ?`, id); err != nil {
		return err
	}
	return tx.Commit()
}

func (r *Repository) ActiveProfile(ctx context.Context) (domain.Profile, error) {
	if err := r.EnsureDefaultProfile(ctx); err != nil {
		return domain.Profile{}, err
	}
	activeID, ok, err := r.Setting(ctx, SettingActiveProfile)
	if err == nil && ok && activeID != "" {
		if profile, profileErr := r.Profile(ctx, activeID); profileErr == nil {
			return profile, nil
		}
	}
	if err := r.SetSetting(ctx, SettingActiveProfile, DefaultProfileID); err != nil {
		return domain.Profile{}, err
	}
	return r.Profile(ctx, DefaultProfileID)
}

func (r *Repository) ActiveProfileID(ctx context.Context) (string, error) {
	profile, err := r.ActiveProfile(ctx)
	if err != nil {
		return "", err
	}
	return profile.ID, nil
}

func (r *Repository) SetActiveProfile(ctx context.Context, id string) error {
	if _, err := r.Profile(ctx, id); err != nil {
		return err
	}
	return r.SetSetting(ctx, SettingActiveProfile, id)
}

func (r *Repository) EnsureDefaultProfile(ctx context.Context) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO profiles (id, name, created_at, kind)
		VALUES (?, 'Padrão', ?, ?)
		ON CONFLICT(id) DO NOTHING`, DefaultProfileID, fmtTime(time.Now()), domain.ProfileKindPersistent)
	return err
}

// CleanupEphemeralProfiles remove guests abandonados por encerramento normal
// ou crash. Perfis persistentes e a seleção ativa válida são preservados.
func (r *Repository) CleanupEphemeralProfiles(ctx context.Context) error {
	if err := r.EnsureDefaultProfile(ctx); err != nil {
		return err
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `
		UPDATE settings SET value = ?
		WHERE key = ? AND value IN (SELECT id FROM profiles WHERE kind = 'guest')`,
		DefaultProfileID, SettingActiveProfile); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM profiles WHERE kind = 'guest'`); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO settings (key, value) VALUES (?, ?)
		ON CONFLICT(key) DO NOTHING`, SettingActiveProfile, DefaultProfileID); err != nil {
		return err
	}
	return tx.Commit()
}

func (r *Repository) AddProfileMember(ctx context.Context, profileID, kind, refID string) error {
	if profileID == "" || kind == "" || refID == "" {
		return fmt.Errorf("dados de membro de perfil inválidos")
	}
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO profile_members (profile_id, kind, ref_id)
		VALUES (?, ?, ?)
		ON CONFLICT(profile_id, kind, ref_id) DO NOTHING`,
		profileID, kind, refID)
	return err
}

func (r *Repository) RemoveProfileMember(ctx context.Context, profileID, kind, refID string) error {
	_, err := r.db.ExecContext(ctx, `
		DELETE FROM profile_members
		WHERE profile_id = ? AND kind = ? AND ref_id = ?`,
		profileID, kind, refID)
	return err
}

func (r *Repository) ProfileMembers(ctx context.Context, profileID, kind string) ([]string, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT ref_id FROM profile_members
		WHERE profile_id = ? AND kind = ?`,
		profileID, kind)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

func (r *Repository) ProfileHasMember(ctx context.Context, profileID, kind, refID string) (bool, error) {
	var count int
	err := r.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM profile_members
		WHERE profile_id = ? AND kind = ? AND ref_id = ?`,
		profileID, kind, refID).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}
