package storage

import (
	"context"
	"testing"

	"github.com/nanotube/nanotube-web/internal/domain"
)

func TestAccountRepository(t *testing.T) {
	db, err := OpenMemory(context.Background())
	if err != nil {
		t.Fatalf("OpenMemory: %v", err)
	}
	defer db.Close()
	if err := Migrate(db); err != nil {
		t.Fatalf("Migrate: %v", err)
	}

	ctx := context.Background()
	repo := NewAccountRepository(db)

	if _, ok, err := repo.Account(ctx); err != nil || ok {
		t.Fatalf("Account inicial: ok=%v err=%v, esperado sem conta", ok, err)
	}

	info := domain.AccountInfo{
		Provider: "google", ProviderSubject: "google-subject-1",
		Email: "user@example.com", ConnectedAt: "2026-08-13T12:00:00Z",
		SessionPersistence: domain.SessionPersistenceKeyring,
	}
	if err := repo.SaveAccount(ctx, info); err != nil {
		t.Fatalf("SaveAccount: %v", err)
	}

	got, ok, err := repo.Account(ctx)
	if err != nil {
		t.Fatalf("Account: %v", err)
	}
	if !ok || got.Email != info.Email || got.ConnectedAt != info.ConnectedAt {
		t.Fatalf("Account = %+v ok=%v, esperado %+v", got, ok, info)
	}

	info2 := domain.AccountInfo{
		Provider: "google", ProviderSubject: "google-subject-1",
		Email: "other@example.com", ConnectedAt: "2026-08-13T13:00:00Z",
		SessionPersistence: domain.SessionPersistenceKeyring,
	}
	if err := repo.SaveAccount(ctx, info2); err != nil {
		t.Fatalf("SaveAccount (update): %v", err)
	}
	got, _, _ = repo.Account(ctx)
	if got.Email != info2.Email {
		t.Fatalf("update falhou: got %q", got.Email)
	}

	if err := repo.ClearAccount(ctx); err != nil {
		t.Fatalf("ClearAccount: %v", err)
	}
	if _, ok, err := repo.Account(ctx); err != nil || ok {
		t.Fatalf("Account após clear: ok=%v err=%v", ok, err)
	}
}

func TestAccountRepositoryIsolatesProfiles(t *testing.T) {
	db, err := OpenMemory(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := Migrate(db); err != nil {
		t.Fatal(err)
	}
	repo := NewRepository(db)
	second, err := repo.CreateProfile(context.Background(), "Trabalho")
	if err != nil {
		t.Fatal(err)
	}

	defaultAccount := NewAccountRepository(db, DefaultProfileID)
	secondAccount := NewAccountRepository(db, second.ID)
	if err := defaultAccount.SaveAccount(context.Background(), domain.AccountInfo{
		Provider: "google", ProviderSubject: "subject-default", Email: "default@example.com",
		SessionPersistence: domain.SessionPersistenceKeyring,
	}); err != nil {
		t.Fatal(err)
	}
	if err := secondAccount.SaveAccount(context.Background(), domain.AccountInfo{
		Provider: "google", ProviderSubject: "subject-work", Email: "work@example.com",
		SessionPersistence: domain.SessionPersistenceKeyring,
	}); err != nil {
		t.Fatal(err)
	}

	gotDefault, ok, err := defaultAccount.Account(context.Background())
	if err != nil || !ok || gotDefault.Email != "default@example.com" {
		t.Fatalf("conta default = %+v ok=%v err=%v", gotDefault, ok, err)
	}
	gotSecond, ok, err := secondAccount.Account(context.Background())
	if err != nil || !ok || gotSecond.Email != "work@example.com" {
		t.Fatalf("conta trabalho = %+v ok=%v err=%v", gotSecond, ok, err)
	}

	if err := secondAccount.ClearAccount(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, ok, _ := defaultAccount.Account(context.Background()); !ok {
		t.Fatal("limpar conta do segundo perfil removeu a conta default")
	}
}

func TestAccountRepositoryClearsMemoryOnlySessionsAfterCrash(t *testing.T) {
	db, err := OpenMemory(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := Migrate(db); err != nil {
		t.Fatal(err)
	}
	repo := NewAccountRepository(db)
	if err := repo.SaveAccount(context.Background(), domain.AccountInfo{
		Provider: "google", ProviderSubject: "memory-subject", Email: "memory@example.com",
		SessionPersistence: domain.SessionPersistenceMemory,
	}); err != nil {
		t.Fatal(err)
	}
	if err := repo.ClearTransientAccounts(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, ok, err := repo.Account(context.Background()); err != nil || ok {
		t.Fatalf("sessão transitória após cleanup: ok=%v err=%v", ok, err)
	}
}

func TestAccountRepositoryRejectsNilDependencies(t *testing.T) {
	var repo *AccountRepository
	if _, _, err := repo.Account(context.Background()); err == nil {
		t.Fatal("Account em repository nulo deveria falhar")
	}
	if err := repo.SaveAccount(context.Background(), domain.AccountInfo{}); err == nil {
		t.Fatal("SaveAccount em repository nulo deveria falhar")
	}
	if err := repo.ClearAccount(context.Background()); err == nil {
		t.Fatal("ClearAccount em repository nulo deveria falhar")
	}
}
