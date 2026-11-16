package main

import (
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/finch-xu/clawdhome/vm"
)

func TestLauncherNavigationKeepsSessionAndDiscardsSettingsPreview(t *testing.T) {
	look := &stubAppearance{system: "en", theme: "dark"}
	a := &app{locale: localeEN, lastPoll: time.Now(), state: vm.VMStopped, selected: "cli-example", appearance: look}
	screen := ui.NewTester(a.view, 660, 460)
	screen.Frame()
	click := func(label string) {
		t.Helper()
		if err := screen.Click(label); err != nil {
			t.Fatal(err)
		}
		screen.Frame()
	}
	click("Settings")
	if a.page != pageSettings {
		t.Fatal("settings navigation did not open settings")
	}
	a.draft.ThemePref = "dark"
	click("Network")
	if a.page != pageNetwork || a.draft != a.cfg || look.theme != "" {
		t.Fatalf("leaving settings: page=%v, theme=%q, draft=%+v", a.page, look.theme, a.draft)
	}
	click("Status Details")
	if !a.detailsOpen {
		t.Fatal("the status details button did not open the panel")
	}
	click("Sessions")
	if a.page != pageSessions || a.selected != "cli-example" || !a.detailsOpen {
		t.Fatal("switching pages lost the selected session or bottom bar state")
	}
	if rect, ok := screen.Find("Hide Details"); !ok || rect.Y+rect.H > 460 {
		t.Fatalf("status action clipped at narrow size: %v, found=%v", rect, ok)
	}
}

func TestThemeShortcutPersistsAllThreeChoices(t *testing.T) {
	look := &stubAppearance{}
	a := &app{launcher: vm.Launcher{Home: t.TempDir()}, appearance: look}
	for _, want := range []string{"light", "dark", ""} {
		a.cycleTheme(a.cfg.ThemePref)
		if got := a.launcher.Settings().ThemePref; got != want || look.theme != want {
			t.Fatalf("saved theme %q, applied theme %q, want %q", got, look.theme, want)
		}
	}
}

func TestSessionListKeepsNameAndExitStatusAtNarrowSize(t *testing.T) {
	a := &app{locale: localeZhHans, lastPoll: time.Now(), state: vm.VMStopped,
		sessions: []vm.Session{{ID: "cli-abcd", Command: "printf 'a::b'; exit 1", Exited: true, ExitCodeKnown: true, ExitCode: 1}}}
	screen := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(ui.DarkTheme())
		a.view(c)
	}, 640, 420)
	screen.Frame()
	if !screen.HasText("printf 'a::b'; exit 1") || !screen.HasText("退出码 1") {
		t.Fatalf("session name and state absent: %v", screen.Texts())
	}
	r, ok := screen.Find("新建会话")
	if !ok || r.Y+r.H > 420 || r.W <= 0 {
		t.Fatalf("new session action clipped: %v, found=%v", r, ok)
	}
}
