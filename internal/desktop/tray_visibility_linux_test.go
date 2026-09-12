//go:build linux && !android && !server

package desktop

import (
	"errors"
	"testing"
)

func TestLinuxTrayVisibilityEmitsSNIStatus(t *testing.T) {
	var statuses []string
	visibility := newLinuxTrayVisibility(nil, func(status string) error {
		statuses = append(statuses, status)
		return nil
	})

	if err := visibility.SetEnabled(false); err != nil {
		t.Fatal(err)
	}
	if err := visibility.SetEnabled(true); err != nil {
		t.Fatal(err)
	}
	if len(statuses) != 2 || statuses[0] != "Passive" || statuses[1] != "Active" {
		t.Fatalf("status emitidos = %v", statuses)
	}
}

func TestLinuxTrayVisibilityPropagatesDBusFailure(t *testing.T) {
	want := errors.New("dbus indisponível")
	visibility := newLinuxTrayVisibility(nil, func(string) error { return want })
	if err := visibility.SetEnabled(false); !errors.Is(err, want) {
		t.Fatalf("erro = %v, esperado %v", err, want)
	}
}
