package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/nanotube/nanotube-web/internal/domain"
)

// AccountRepository persiste somente metadados não sensíveis da conta do
// perfil indicado. Refresh tokens nunca entram no SQLite.
type AccountRepository struct {
	db        *sql.DB
	profileID string
}

// NewAccountRepository mantém `default` como compatibilidade para chamadores
// antigos, mas serviços multi-perfil devem sempre informar o profileID.
func NewAccountRepository(db *sql.DB, profileID ...string) *AccountRepository {
	id := DefaultProfileID
	if len(profileID) > 0 && strings.TrimSpace(profileID[0]) != "" {
		id = strings.TrimSpace(profileID[0])
	}
	return &AccountRepository{db: db, profileID: id}
}

func (r *AccountRepository) SaveAccount(ctx context.Context, info domain.AccountInfo) error {
	if err := r.validate(ctx); err != nil {
		return err
	}
	provider := strings.TrimSpace(info.Provider)
	if provider == "" {
		provider = "google"
	}
	subject := strings.TrimSpace(info.ProviderSubject)
	if subject == "" {
		return errors.New("salvar conta: provider_subject vazio")
	}
	persistence := info.SessionPersistence
	if persistence == "" {
		persistence = domain.SessionPersistenceKeyring
	}
	if persistence != domain.SessionPersistenceKeyring && persistence != domain.SessionPersistenceMemory {
		return fmt.Errorf("salvar conta: persistência de sessão inválida: %q", persistence)
	}
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO profile_accounts
			(profile_id, provider, provider_subject, email, connected_at, session_persistence)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(profile_id) DO UPDATE SET
			provider = excluded.provider,
			provider_subject = excluded.provider_subject,
			email = excluded.email,
			connected_at = excluded.connected_at,
			session_persistence = excluded.session_persistence`,
		r.profileID, provider, subject, strings.TrimSpace(info.Email), info.ConnectedAt, persistence)
	if err != nil {
		return fmt.Errorf("salvar conta do perfil %s: %w", r.profileID, err)
	}
	return nil
}

func (r *AccountRepository) Account(ctx context.Context) (domain.AccountInfo, bool, error) {
	if err := r.validate(ctx); err != nil {
		return domain.AccountInfo{}, false, err
	}
	var info domain.AccountInfo
	err := r.db.QueryRowContext(ctx, `
		SELECT profile_id, provider, provider_subject, email, connected_at, session_persistence
		FROM profile_accounts WHERE profile_id = ?`, r.profileID).
		Scan(&info.ProfileID, &info.Provider, &info.ProviderSubject, &info.Email,
			&info.ConnectedAt, &info.SessionPersistence)
	switch {
	case err == sql.ErrNoRows:
		return domain.AccountInfo{}, false, nil
	case err != nil:
		return domain.AccountInfo{}, false, fmt.Errorf("conta do perfil %s: %w", r.profileID, err)
	default:
		return info, true, nil
	}
}

func (r *AccountRepository) ClearAccount(ctx context.Context) error {
	if err := r.validate(ctx); err != nil {
		return err
	}
	_, err := r.db.ExecContext(ctx, `DELETE FROM profile_accounts WHERE profile_id = ?`, r.profileID)
	if err != nil {
		return fmt.Errorf("limpar conta do perfil %s: %w", r.profileID, err)
	}
	return nil
}

// ClearTransientAccounts remove metadados de sessões que existiam apenas em
// memória. É chamado no bootstrap para não exibir uma conta desconectada após
// encerramento inesperado.
func (r *AccountRepository) ClearTransientAccounts(ctx context.Context) error {
	if err := r.validate(ctx); err != nil {
		return err
	}
	_, err := r.db.ExecContext(ctx,
		`DELETE FROM profile_accounts WHERE session_persistence = ?`, domain.SessionPersistenceMemory)
	if err != nil {
		return fmt.Errorf("limpar sessões transitórias: %w", err)
	}
	return nil
}

func (r *AccountRepository) validate(ctx context.Context) error {
	if r == nil || r.db == nil {
		return errors.New("database indisponível")
	}
	if ctx == nil {
		return errors.New("contexto nulo")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if strings.TrimSpace(r.profileID) == "" {
		return errors.New("profileID vazio")
	}
	return nil
}
