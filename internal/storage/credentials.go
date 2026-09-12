package storage

import (
	"context"
	"fmt"
	"strings"
	"time"
)

type CredentialAuditEvent struct {
	ID         int64     `json:"id"`
	ProfileID  string    `json:"profile_id"`
	Provider   string    `json:"provider"`
	Action     string    `json:"action"`
	OccurredAt time.Time `json:"occurred_at"`
	Detail     string    `json:"detail,omitempty"`
}

var allowedCredentialAuditActions = map[string]bool{
	"connected": true, "rotated": true, "revoked": true, "verification_failed": true,
}

func (r *Repository) RecordCredentialAudit(ctx context.Context, profileID, provider, action, detail string) error {
	profileID = strings.TrimSpace(profileID)
	provider = strings.ToLower(strings.TrimSpace(provider))
	action = strings.ToLower(strings.TrimSpace(action))
	if profileID == "" || provider == "" || !allowedCredentialAuditActions[action] {
		return fmt.Errorf("auditoria de credencial: evento inválido")
	}
	if len(detail) > 240 {
		runes := []rune(detail)
		if len(runes) > 240 {
			detail = string(runes[:240])
		}
	}
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO credential_audit (profile_id, provider, action, occurred_at, detail)
		VALUES (?, ?, ?, ?, ?)`, profileID, provider, action, fmtTime(time.Now()), strings.TrimSpace(detail))
	if err != nil {
		return fmt.Errorf("auditoria de credencial: %w", err)
	}
	return nil
}

func (r *Repository) CredentialAudit(ctx context.Context, profileID string, limit int) ([]CredentialAuditEvent, error) {
	if limit <= 0 || limit > 100 {
		limit = 30
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, profile_id, provider, action, occurred_at, detail
		FROM credential_audit WHERE profile_id = ?
		ORDER BY occurred_at DESC, id DESC LIMIT ?`, strings.TrimSpace(profileID), limit)
	if err != nil {
		return nil, fmt.Errorf("listar auditoria de credencial: %w", err)
	}
	defer rows.Close()
	events := make([]CredentialAuditEvent, 0, limit)
	for rows.Next() {
		var event CredentialAuditEvent
		var occurredAt string
		if err := rows.Scan(&event.ID, &event.ProfileID, &event.Provider, &event.Action, &occurredAt, &event.Detail); err != nil {
			return nil, fmt.Errorf("ler auditoria de credencial: %w", err)
		}
		event.OccurredAt = parseTime(occurredAt)
		events = append(events, event)
	}
	return events, rows.Err()
}
