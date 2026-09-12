package storage

import (
	"context"
	"testing"
)

func TestCredentialAuditIsProfileScopedAndValidated(t *testing.T) {
	repo := openTestRepo(t)
	ctx := context.Background()
	other, err := repo.CreateProfile(ctx, "Outra conta")
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.RecordCredentialAudit(ctx, DefaultProfileID, "Google", "connected", "autorizada"); err != nil {
		t.Fatal(err)
	}
	if err := repo.RecordCredentialAudit(ctx, other.ID, "lastfm", "rotated", "renovada"); err != nil {
		t.Fatal(err)
	}
	if err := repo.RecordCredentialAudit(ctx, DefaultProfileID, "google", "token_value", "não permitido"); err == nil {
		t.Fatal("ação de auditoria arbitrária foi aceita")
	}

	events, err := repo.CredentialAudit(ctx, DefaultProfileID, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 || events[0].Provider != "google" || events[0].Action != "connected" {
		t.Fatalf("eventos do perfil padrão = %+v", events)
	}
	if events[0].ProfileID != DefaultProfileID {
		t.Fatalf("evento vazou de perfil: %+v", events[0])
	}
}
