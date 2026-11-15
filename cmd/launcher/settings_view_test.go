package main

import (
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
	if want := tr("settings.saved", a.locale); a.settingsMessage != want {
		t.Fatalf("settingsMessage = %q, want %q from the dictionary", a.settingsMessage, want)
	}
	if got := l.Settings().Port; got != "7890" {
		t.Fatalf("port read back from disk = %q, want %q", got, "7890")
	}
}

func TestSaveSettingsReportsRejectedPort(t *testing.T) {
	l := vm.Launcher{Home: t.TempDir()}
	a := &app{window: syncUpdater{}, launcher: l, draft: vm.Settings{Port: "65536"}}

	a.saveSettings()

	if a.settingsMessage == "" || a.settingsMessage == tr("settings.saved", a.locale) {
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

		if want := tr("settings.portRange", a.locale); a.portTestMessage != want {
			t.Fatalf("port %q: message = %q, want %q from the dictionary", port, a.portTestMessage, want)
		}
		if a.portTesting {
			t.Fatalf("port %q: portTesting = true, want no handshake for an invalid port", port)
		}
	}
}

func TestCancelRevertsAppliedAppearanceAndLanguage(t *testing.T) {
	// 外观和语言是点一下就生效的，所以取消必须把它们退回已保存的值，
	// 否则界面会和 launcher.json 不一致，下次启动又变回去。
	look := &stubAppearance{}
	a := &app{window: syncUpdater{}, appearance: look, settingsOpen: true,
		cfg:    vm.Settings{ThemePref: "", LocalePref: ""},
		draft:  vm.Settings{ThemePref: "dark", LocalePref: string(localeZhHans)},
		locale: localeZhHans}

	a.cancelSettings()

	if look.theme != "" {
		t.Fatalf("appearance.SetTheme(%q), want it reverted to the saved empty pref", look.theme)
	}
	if look.locale != "" {
		t.Fatalf("appearance.SetLocale(%q), want it cleared back to follow-system", look.locale)
	}

	if a.settingsOpen {
		t.Fatal("cancel should close the settings panel")
	}
	if a.draft != a.cfg {
		t.Fatalf("draft = %+v, want it reverted to cfg %+v", a.draft, a.cfg)
	}
}
