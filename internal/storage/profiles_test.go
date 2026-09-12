package storage

import (
	"context"
	"testing"

	"github.com/nanotube/nanotube-web/internal/domain"
)

func TestProfileCRUD(t *testing.T) {
	r := openTestRepo(t)
	ctx := context.Background()

	// Default profile initialization
	active, err := r.ActiveProfile(ctx)
	if err != nil {
		t.Fatalf("ActiveProfile default: %v", err)
	}
	if active.ID != DefaultProfileID || active.Name != "Padrão" || active.Kind != domain.ProfileKindPersistent {
		t.Fatalf("Nome do perfil padrão = %q, esperado 'Padrão'", active.Name)
	}

	// Create a new profile
	p2, err := r.CreateProfile(ctx, "Trabalho")
	if err != nil {
		t.Fatalf("CreateProfile: %v", err)
	}
	if p2.Name != "Trabalho" || p2.ID == "" {
		t.Fatalf("Perfil criado inválido: %+v", p2)
	}

	// List profiles
	profiles, err := r.Profiles(ctx)
	if err != nil {
		t.Fatalf("Profiles: %v", err)
	}
	if len(profiles) != 2 {
		t.Fatalf("len(profiles) = %d, esperado 2", len(profiles))
	}

	// Switch active profile
	if err := r.SetActiveProfile(ctx, p2.ID); err != nil {
		t.Fatalf("SetActiveProfile: %v", err)
	}
	active2, err := r.ActiveProfile(ctx)
	if err != nil {
		t.Fatalf("ActiveProfile pós switch: %v", err)
	}
	if active2.ID != p2.ID {
		t.Fatalf("ActiveProfile ID = %q, esperado %q", active2.ID, p2.ID)
	}

	// Rename profile
	if err := r.RenameProfile(ctx, p2.ID, "Estudos"); err != nil {
		t.Fatalf("RenameProfile: %v", err)
	}
	renamed, _ := r.ActiveProfile(ctx)
	if renamed.Name != "Estudos" {
		t.Fatalf("Nome renomeado = %q, esperado 'Estudos'", renamed.Name)
	}

	// Profile membership
	if err := r.AddProfileMember(ctx, p2.ID, "playlist", "pl_test"); err != nil {
		t.Fatalf("AddProfileMember: %v", err)
	}
	has, err := r.ProfileHasMember(ctx, p2.ID, "playlist", "pl_test")
	if err != nil || !has {
		t.Fatalf("ProfileHasMember = %v, err=%v", has, err)
	}
	members, err := r.ProfileMembers(ctx, p2.ID, "playlist")
	if err != nil || len(members) != 1 || members[0] != "pl_test" {
		t.Fatalf("ProfileMembers: %+v, err=%v", members, err)
	}
	if err := r.RemoveProfileMember(ctx, p2.ID, "playlist", "pl_test"); err != nil {
		t.Fatalf("RemoveProfileMember: %v", err)
	}
	hasAfter, _ := r.ProfileHasMember(ctx, p2.ID, "playlist", "pl_test")
	if hasAfter {
		t.Fatal("membro ainda presente após remoção")
	}

	// Delete profile
	if err := r.DeleteProfile(ctx, p2.ID); err != nil {
		t.Fatalf("DeleteProfile: %v", err)
	}
	afterDelete, _ := r.Profiles(ctx)
	if len(afterDelete) != 1 {
		t.Fatalf("len após delete = %d, esperado 1", len(afterDelete))
	}
}

func TestGuestProfileIsDiscardedAfterCrashCleanup(t *testing.T) {
	r := openTestRepo(t)
	ctx := context.Background()
	guest, err := r.CreateGuestProfile(ctx, "Sessão privada")
	if err != nil {
		t.Fatal(err)
	}
	if !guest.IsGuest() {
		t.Fatalf("perfil guest = %+v", guest)
	}
	if err := r.SetActiveProfile(ctx, guest.ID); err != nil {
		t.Fatal(err)
	}
	if err := r.AddProfileMember(ctx, guest.ID, "playlist", "guest-only"); err != nil {
		t.Fatal(err)
	}

	// Simula o próximo bootstrap após encerramento inesperado.
	if err := r.CleanupEphemeralProfiles(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := r.Profile(ctx, guest.ID); err == nil {
		t.Fatal("guest abandonado permaneceu após cleanup")
	}
	activeID, err := r.ActiveProfileID(ctx)
	if err != nil || activeID != DefaultProfileID {
		t.Fatalf("perfil ativo após cleanup = %q err=%v", activeID, err)
	}
	if has, err := r.ProfileHasMember(ctx, guest.ID, "playlist", "guest-only"); err != nil || has {
		t.Fatalf("membro do guest após cascade: has=%v err=%v", has, err)
	}
}

func TestProfileSelectionRejectsUnknownAndProtectsDefault(t *testing.T) {
	r := openTestRepo(t)
	ctx := context.Background()
	if err := r.SetActiveProfile(ctx, "missing"); err == nil {
		t.Fatal("perfil inexistente foi selecionado")
	}
	if err := r.DeleteProfile(ctx, DefaultProfileID); err == nil {
		t.Fatal("perfil default foi removido")
	}
}
