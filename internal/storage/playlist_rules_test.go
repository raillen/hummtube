package storage

import (
	"context"
	"testing"
	"time"

	"github.com/nanotube/nanotube-web/internal/domain"
)

func TestSmartRuleCRUD(t *testing.T) {
	r := openTestRepo(t)
	ctx := context.Background()
	pl, _ := r.CreatePlaylist(ctx, "Smart", "", "")
	rule := domain.SmartRule{Version: domain.CurrentSmartRuleVersion, Filter: domain.PlaylistFilter{Watched: domain.SearchWatchUnwatched}, Sort: domain.PlaylistSortPublished, Term: "golang"}
	if err := r.SaveSmartRule(ctx, pl.ID, rule); err != nil {
		t.Fatalf("SaveSmartRule: %v", err)
	}
	got, err := r.GetSmartRule(ctx, pl.ID)
	if err != nil {
		t.Fatalf("GetSmartRule: %v", err)
	}
	if got.Term != "golang" || got.Sort != domain.PlaylistSortPublished {
		t.Fatalf("got %+v", got)
	}
	if ok, _ := r.IsSmartPlaylist(ctx, pl.ID); !ok {
		t.Fatal("IsSmart false")
	}
}

func TestSmartPlaylistDeterminism(t *testing.T) {
	r := openTestRepo(t)
	ctx := context.Background()
	now := time.Now()
	for i := 0; i < 10; i++ {
		seedPlaylistRich(t, r, string(rune('a'+i)), "c1", string(rune('a'+i)), "", time.Duration(i)*time.Minute, now.Add(-time.Duration(i)*time.Hour), "")
	}
	rule := domain.SmartRule{Version: domain.CurrentSmartRuleVersion, Sort: domain.PlaylistSortPublished}
	a, _ := r.SmartPlaylistVideos(ctx, rule)
	b, _ := r.SmartPlaylistVideos(ctx, rule)
	if len(a) != len(b) {
		t.Fatalf("len %d vs %d", len(a), len(b))
	}
	for i := range a {
		if a[i].ID != b[i].ID {
			t.Fatalf("ordem difere em %d: %s vs %s", i, a[i].ID, b[i].ID)
		}
	}
}

func TestPresetCRUD(t *testing.T) {
	r := openTestRepo(t)
	ctx := context.Background()
	rule := domain.SmartRule{Version: domain.CurrentSmartRuleVersion, Filter: domain.PlaylistFilter{Favorite: true}}
	id, err := r.SavePreset(ctx, "", "Favoritos", rule)
	if err != nil {
		t.Fatalf("SavePreset: %v", err)
	}
	if err := r.RenamePreset(ctx, id, "Meus favoritos"); err != nil {
		t.Fatalf("Rename: %v", err)
	}
	list, err := r.ListPresets(ctx)
	if err != nil || len(list) != 1 || list[0].Name != "Meus favoritos" {
		t.Fatalf("ListPresets: %+v err %v", list, err)
	}
	if err := r.DeletePreset(ctx, id); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	list, _ = r.ListPresets(ctx)
	if len(list) != 0 {
		t.Fatalf("delete falhou: %v", list)
	}
}

func TestSmartPlaylistMissingData(t *testing.T) {
	r := openTestRepo(t)
	ctx := context.Background()
	seedPlaylistRich(t, r, "v1", "", "Sem data", "", 0, time.Time{}, "")
	rule := domain.SmartRule{Version: domain.CurrentSmartRuleVersion, Filter: domain.PlaylistFilter{Age: domain.FeedAgeWeek}}
	vids, err := r.SmartPlaylistVideos(ctx, rule)
	if err != nil {
		t.Fatalf("Smart: %v", err)
	}
	_ = vids
}

func TestGetSmartRuleRejectsFutureColumnVersion(t *testing.T) {
	r := openTestRepo(t)
	ctx := context.Background()
	pl, _ := r.CreatePlaylist(ctx, "Futura", "", "")
	raw, _ := domain.MarshalSmartRule(domain.SmartRule{Sort: domain.PlaylistSortAdded})
	if _, err := r.db.ExecContext(ctx,
		`INSERT INTO playlist_rules (playlist_id, version, rule_json, updated_at) VALUES (?, ?, ?, ?)`,
		pl.ID, domain.CurrentSmartRuleVersion+1, string(raw), fmtTime(time.Now())); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if _, err := r.GetSmartRule(ctx, pl.ID); err == nil {
		t.Fatal("regra de versão futura deveria ser rejeitada")
	}
}

func TestGetSmartRuleRejectsVersionMismatch(t *testing.T) {
	r := openTestRepo(t)
	ctx := context.Background()
	pl, _ := r.CreatePlaylist(ctx, "Inconsistente", "", "")
	raw, _ := domain.MarshalSmartRule(domain.SmartRule{Sort: domain.PlaylistSortAdded})
	if _, err := r.db.ExecContext(ctx,
		`INSERT INTO playlist_rules (playlist_id, version, rule_json, updated_at) VALUES (?, ?, ?, ?)`,
		pl.ID, 0, string(raw), fmtTime(time.Now())); err != nil {
		t.Fatalf("seed: %v", err)
	}
	// Coluna 0 é legado aceitável (JSON sem versão assume a atual).
	if _, err := r.GetSmartRule(ctx, pl.ID); err != nil {
		t.Fatalf("legado coluna 0 deveria carregar: %v", err)
	}

	// Coluna e JSON discordando de verdade é corrupção.
	bad, _ := domain.MarshalSmartRule(domain.SmartRule{Sort: domain.PlaylistSortTitle})
	if _, err := r.db.ExecContext(ctx,
		`UPDATE playlist_rules SET rule_json = ? WHERE playlist_id = ?`, string(bad), pl.ID); err != nil {
		t.Fatalf("update: %v", err)
	}
	if _, err := r.GetSmartRule(ctx, pl.ID); err != nil {
		t.Fatalf("mesma versão em coluna e JSON deveria carregar: %v", err)
	}
}

func TestUpgradeSmartRuleContract(t *testing.T) {
	raw, _ := domain.MarshalSmartRule(domain.SmartRule{Term: "x"})
	// Versão atual passa.
	if got, err := domain.UpgradeSmartRule(raw, domain.CurrentSmartRuleVersion); err != nil || got.Term != "x" {
		t.Fatalf("UpgradeSmartRule atual: %+v, %v", got, err)
	}
	// Legado sem versão assume a atual.
	if got, err := domain.UpgradeSmartRule([]byte(`{"term":"y"}`), 0); err != nil || got.Term != "y" || got.Version != domain.CurrentSmartRuleVersion {
		t.Fatalf("UpgradeSmartRule legado: %+v, %v", got, err)
	}
	// Versão futura é rejeitada.
	if _, err := domain.UpgradeSmartRule(raw, domain.CurrentSmartRuleVersion+1); err == nil {
		t.Fatal("versão futura deveria falhar")
	}
	// JSON com versão diferente da coluna é rejeitado.
	if _, err := domain.UpgradeSmartRule([]byte(`{"version":99}`), 1); err == nil {
		t.Fatal("JSON futuro deveria falhar")
	}
}
