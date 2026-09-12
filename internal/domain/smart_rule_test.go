package domain

import "testing"

func TestSmartRuleMarshalRoundTrip(t *testing.T) {
	r := SmartRule{
		Version: CurrentSmartRuleVersion,
		Filter:  PlaylistFilter{Watched: SearchWatchUnwatched},
		Sort:    PlaylistSortPublished,
		Term:    "golang",
	}
	data, err := MarshalSmartRule(r)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	got, err := UnmarshalSmartRule(data)
	if err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if got.Term != "golang" || got.Sort != PlaylistSortPublished {
		t.Fatalf("roundtrip: %+v", got)
	}
}

func TestSmartRuleFutureVersion(t *testing.T) {
	if _, err := UnmarshalSmartRule([]byte(`{"version":99,"filter":{}}`)); err == nil {
		t.Fatal("deveria rejeitar versão futura")
	}
}

func TestSmartRuleInvalidJSON(t *testing.T) {
	if _, err := UnmarshalSmartRule([]byte(`{invalid`)); err == nil {
		t.Fatal("deveria rejeitar JSON inválido")
	}
}

func TestBuiltInPresets(t *testing.T) {
	presets := BuiltInPresets()
	if len(presets) != 4 {
		t.Fatalf("presets = %d, want 4", len(presets))
	}
	for name, r := range presets {
		if err := r.Validate(); err != nil {
			t.Fatalf("preset %q inválido: %v", name, err)
		}
	}
}

func TestSmartRuleSummary(t *testing.T) {
	if (SmartRule{}).Summary() == "" {
		t.Fatal("summary vazio deveria ter fallback")
	}
	r := SmartRule{Version: CurrentSmartRuleVersion, Term: "x", SubscribedOnly: true}
	if r.Summary() == "" {
		t.Fatal("summary com termo")
	}
}
