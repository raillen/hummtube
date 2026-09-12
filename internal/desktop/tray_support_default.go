//go:build !linux || android || server

package desktop

func systemTrayAvailable() bool { return true }
