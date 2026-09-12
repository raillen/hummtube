//go:build linux && !android && !server

package desktop

import (
	"fmt"

	"github.com/godbus/dbus/v5"
	"github.com/wailsapp/wails/v3/pkg/application"
)

const (
	statusNotifierPath         = dbus.ObjectPath("/StatusNotifierItem")
	statusNotifierStatusSignal = "org.kde.StatusNotifierItem.NewStatus"
)

type linuxTrayVisibility struct {
	tray       *application.SystemTray
	emitStatus func(string) error
}

func newTrayVisibility(tray *application.SystemTray) trayVisibility {
	return newLinuxTrayVisibility(tray, emitLinuxTrayStatus)
}

func newLinuxTrayVisibility(tray *application.SystemTray, emitStatus func(string) error) trayVisibility {
	return &linuxTrayVisibility{tray: tray, emitStatus: emitStatus}
}

func (visibility *linuxTrayVisibility) SetEnabled(enabled bool) error {
	status := "Passive"
	if enabled {
		status = "Active"
	}
	// Mantém o comportamento nativo quando o Wails implementar Show/Hide no
	// Linux; hoje esses métodos são no-op e o host SNI reage ao NewStatus.
	if visibility.tray != nil {
		if enabled {
			visibility.tray.Show()
		} else {
			visibility.tray.Hide()
		}
	}
	if visibility.emitStatus == nil {
		return fmt.Errorf("emissor de status da bandeja indisponível")
	}
	return visibility.emitStatus(status)
}

func emitLinuxTrayStatus(status string) error {
	connection, err := dbus.SessionBus()
	if err != nil {
		return fmt.Errorf("conectar ao D-Bus da sessão: %w", err)
	}
	if err := connection.Emit(statusNotifierPath, statusNotifierStatusSignal, status); err != nil {
		return fmt.Errorf("emitir status %s da bandeja: %w", status, err)
	}
	return nil
}
