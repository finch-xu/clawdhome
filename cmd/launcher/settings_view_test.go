package main

import (
	"strings"
	"testing"

	"github.com/finch-xu/clawdhome/vm"
)

func TestSaveSettingsTrimsPortBeforePersisting(t *testing.T) {
	l := vm.Launcher{Home: t.TempDir()}
	a := &app{window: syncUpdater{}, launcher: l, draft: vm.Settings{Port: "  7890  "}}

	a.saveSettings()

	if a.cfg.Port != "7890" {
		t.Fatalf("persisted port = %q, want the trimmed %q", a.cfg.Port, "7890")
	}
	if a.settingsMessage != "设置已保存" {
		t.Fatalf("settingsMessage = %q, want the saved confirmation", a.settingsMessage)
	}
	if got := l.Settings().Port; got != "7890" {
		t.Fatalf("port read back from disk = %q, want %q", got, "7890")
	}
}

func TestSaveSettingsReportsRejectedPort(t *testing.T) {
	l := vm.Launcher{Home: t.TempDir()}
	a := &app{window: syncUpdater{}, launcher: l, draft: vm.Settings{Port: "65536"}}

	a.saveSettings()

	if a.settingsMessage == "" || a.settingsMessage == "设置已保存" {
		t.Fatalf("settingsMessage = %q, want an error for an out-of-range port", a.settingsMessage)
	}
	if a.cfg.Port != "" {
		t.Fatalf("cfg.Port = %q, want it left untouched after a rejected save", a.cfg.Port)
	}
}

func TestTestPortRejectsInvalidPortWithoutHandshake(t *testing.T) {
	for _, port := range []string{"abc", "0", "65536", ""} {
		a := &app{window: syncUpdater{}, draft: vm.Settings{Port: port}}

		a.testPort()

		if !strings.Contains(a.portTestMessage, "1–65535") {
			t.Fatalf("port %q: message = %q, want the valid-range hint", port, a.portTestMessage)
		}
		if a.portTesting {
			t.Fatalf("port %q: portTesting = true, want no handshake for an invalid port", port)
		}
	}
}
