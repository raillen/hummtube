//go:build linux && !android && !server

package desktop

import "github.com/godbus/dbus/v5"

// systemTrayAvailable verifica o watcher SNI antes de pedir ao Wails para
// registrar o StatusNotifierItem. Sem um watcher, a bandeja não é funcional e
// o Wails apenas registra um erro de D-Bus no stderr.
func systemTrayAvailable() bool {
	connection, err := dbus.SessionBus()
	if err != nil {
		return false
	}
	defer connection.Close()

	var hasOwner bool
	call := connection.BusObject().Call("org.freedesktop.DBus.NameHasOwner", 0, "org.kde.StatusNotifierWatcher")
	if call.Err != nil {
		return false
	}
	return call.Store(&hasOwner) == nil && hasOwner
}
