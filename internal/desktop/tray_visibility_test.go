package desktop

import (
	"errors"
	"sync"
	"testing"
)

type recordingTrayVisibility struct {
	mu     sync.Mutex
	states []bool
	err    error
}

func (visibility *recordingTrayVisibility) SetEnabled(enabled bool) error {
	visibility.mu.Lock()
	defer visibility.mu.Unlock()
	visibility.states = append(visibility.states, enabled)
	return visibility.err
}

func TestTraySettingsControllerAppliesVisibilityAndMenuChanges(t *testing.T) {
	visibility := &recordingTrayVisibility{}
	menuRebuilds := 0
	controller := newTraySettingsController(visibility, func() { menuRebuilds++ })

	if err := controller.ApplySetting("ui.tray_enabled", "0"); err != nil {
		t.Fatal(err)
	}
	if err := controller.ApplySetting("ui.tray_enabled", "1"); err != nil {
		t.Fatal(err)
	}
	if err := controller.ApplySetting(traySeekSetting, "30"); err != nil {
		t.Fatal(err)
	}
	if err := controller.ApplySetting("unrelated", "value"); err != nil {
		t.Fatal(err)
	}

	if len(visibility.states) != 2 || visibility.states[0] || !visibility.states[1] {
		t.Fatalf("estados de visibilidade = %v, esperado [false true]", visibility.states)
	}
	if menuRebuilds != 1 {
		t.Fatalf("reconstruções do menu = %d, esperado 1", menuRebuilds)
	}
}

func TestTraySettingsControllerPropagatesVisibilityFailure(t *testing.T) {
	want := errors.New("falha nativa")
	controller := newTraySettingsController(&recordingTrayVisibility{err: want}, nil)
	if err := controller.SetEnabled(false); !errors.Is(err, want) {
		t.Fatalf("erro = %v, esperado %v", err, want)
	}
}
