package desktop

import (
	"sync"

	"github.com/nanotube/nanotube-web/internal/storage"
)

type trayVisibility interface {
	SetEnabled(enabled bool) error
}

// traySettingsController serializa alterações que podem chegar de goroutines
// RPC distintas. Isso mantém visibilidade e reconstrução do menu ordenadas.
type traySettingsController struct {
	mu          sync.Mutex
	visibility  trayVisibility
	rebuildMenu func()
}

func newTraySettingsController(visibility trayVisibility, rebuildMenu func()) *traySettingsController {
	return &traySettingsController{visibility: visibility, rebuildMenu: rebuildMenu}
}

func (controller *traySettingsController) SetEnabled(enabled bool) error {
	controller.mu.Lock()
	defer controller.mu.Unlock()
	return controller.visibility.SetEnabled(enabled)
}

func (controller *traySettingsController) ApplySetting(key, value string) error {
	controller.mu.Lock()
	defer controller.mu.Unlock()
	switch key {
	case storage.SettingTrayEnabled:
		return controller.visibility.SetEnabled(value == "1")
	case traySeekSetting:
		if controller.rebuildMenu != nil {
			controller.rebuildMenu()
		}
	}
	return nil
}
