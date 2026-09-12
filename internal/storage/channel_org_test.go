package storage

import (
	"context"
	"testing"
)

func TestChannelFavoritesCRUD(t *testing.T) {
	r := openTestRepo(t)
	ctx := context.Background()

	if ids, err := r.FavoriteChannelIDs(ctx); err != nil || len(ids) != 0 {
		t.Fatalf("inicial: ids=%v err=%v", ids, err)
	}

	if err := r.AddChannelFavorite(ctx, "ch1"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if err := r.AddChannelFavorite(ctx, "ch1"); err != nil { // repetir não duplica
		t.Fatalf("add repetido: %v", err)
	}
	if err := r.AddChannelFavorite(ctx, "ch2"); err != nil {
		t.Fatalf("add: %v", err)
	}

	ids, err := r.FavoriteChannelIDs(ctx)
	if err != nil {
		t.Fatalf("ids: %v", err)
	}
	if !ids["ch1"] || !ids["ch2"] || len(ids) != 2 {
		t.Fatalf("ids = %v", ids)
	}

	if err := r.RemoveChannelFavorite(ctx, "ch1"); err != nil {
		t.Fatalf("remove: %v", err)
	}
	// Remover o que não existe não é erro.
	if err := r.RemoveChannelFavorite(ctx, "ch1"); err != nil {
		t.Fatalf("remove repetido: %v", err)
	}
	ids, _ = r.FavoriteChannelIDs(ctx)
	if ids["ch1"] || !ids["ch2"] {
		t.Fatalf("após remoção: ids = %v", ids)
	}

	if err := r.AddChannelFavorite(ctx, ""); err == nil {
		t.Fatal("canal vazio deveria falhar")
	}
}

func TestChannelFolderCRUD(t *testing.T) {
	r := openTestRepo(t)
	ctx := context.Background()

	if _, err := r.CreateFolder(ctx, "  "); err == nil {
		t.Fatal("nome vazio deveria falhar")
	}

	f1, err := r.CreateFolder(ctx, "Música")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if f1.ID == "" || f1.Name != "Música" {
		t.Fatalf("folder = %+v", f1)
	}
	f2, err := r.CreateFolder(ctx, "Tecnologia")
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	folders, err := r.Folders(ctx)
	if err != nil {
		t.Fatalf("folders: %v", err)
	}
	if len(folders) != 2 {
		t.Fatalf("folders = %d, esperado 2", len(folders))
	}

	if err := r.RenameFolder(ctx, f1.ID, "Música & Podcasts"); err != nil {
		t.Fatalf("rename: %v", err)
	}
	folders, _ = r.Folders(ctx)
	byID := map[string]string{}
	for _, f := range folders {
		byID[f.ID] = f.Name
	}
	if byID[f1.ID] != "Música & Podcasts" {
		t.Fatalf("rename não aplicado: %v", byID)
	}

	if err := r.DeleteFolder(ctx, f2.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	folders, _ = r.Folders(ctx)
	if len(folders) != 1 {
		t.Fatalf("após delete: folders = %d", len(folders))
	}
}

func TestChannelFolderMembership(t *testing.T) {
	r := openTestRepo(t)
	ctx := context.Background()

	f1, _ := r.CreateFolder(ctx, "Música")
	f2, _ := r.CreateFolder(ctx, "Tecnologia")

	if err := r.AddChannelToFolder(ctx, f1.ID, "ch1"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if err := r.AddChannelToFolder(ctx, f1.ID, "ch1"); err != nil { // não duplica
		t.Fatalf("add repetido: %v", err)
	}
	if err := r.AddChannelToFolder(ctx, f1.ID, "ch2"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if err := r.AddChannelToFolder(ctx, f2.ID, "ch2"); err != nil { // mesmo canal em 2 pastas
		t.Fatalf("add: %v", err)
	}

	members, err := r.FolderMembership(ctx)
	if err != nil {
		t.Fatalf("membership: %v", err)
	}
	if len(members[f1.ID]) != 2 || len(members[f2.ID]) != 1 {
		t.Fatalf("members = %v", members)
	}

	if err := r.RemoveChannelFromFolder(ctx, f1.ID, "ch2"); err != nil {
		t.Fatalf("remove: %v", err)
	}
	if err := r.RemoveChannelFromFolder(ctx, f1.ID, "ch2"); err != nil { // não é erro
		t.Fatalf("remove repetido: %v", err)
	}
	members, _ = r.FolderMembership(ctx)
	if len(members[f1.ID]) != 1 || len(members[f2.ID]) != 1 {
		t.Fatalf("após remoção: members = %v", members)
	}

	// Excluir a pasta limpa a associação (FK ON DELETE CASCADE).
	if err := r.DeleteFolder(ctx, f1.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	members, _ = r.FolderMembership(ctx)
	if _, ok := members[f1.ID]; ok {
		t.Fatalf("pasta excluída ainda tem membros: %v", members)
	}
}
