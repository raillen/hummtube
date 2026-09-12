//go:build !linux || android || server

package desktop

import "github.com/wailsapp/wails/v3/pkg/application"

type nativeTrayVisibility struct {
	tray *application.SystemTray
}

func newTrayVisibility(tray *application.SystemTray) trayVisibility {
	return &nativeTrayVisibility{tray: tray}
}

func (visibility *nativeTrayVisibility) SetEnabled(enabled bool) error {
	if enabled {
		visibility.tray.Show()
	} else {
		visibility.tray.Hide()
	}
	return nil
}
