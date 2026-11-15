package vm

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

func TestSessionListOnlyIncludesLauncherSessions(t *testing.T) {
	input := "cli-56::python3\nmanual::zsh\ncli-90::vim\ncli-91::@vm-exited\n"
	want := []Session{
		{ID: "cli-56", Command: "python3"},
		{ID: "cli-90", Command: "vim"},
		// 已退出的会话保留原始标记，由界面按自己的语言显示。
		{ID: "cli-91", Command: ExitedCommand, Exited: true},
	}
	if got := ParseSessions(input); !reflect.DeepEqual(got, want) {
		t.Fatalf("sessions = %#v, want %#v", got, want)
	}
}

func TestShowDesktopUsesInstanceHostAgentSocket(t *testing.T) {
	home, err := os.MkdirTemp("/tmp", "mvp-desktop-")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(home)
	dir := filepath.Join(home, instance)
	if err := os.Mkdir(dir, 0700); err != nil {
		t.Fatal(err)
	}
	socket, err := net.Listen("unix", filepath.Join(dir, "ha.sock"))
	if err != nil {
		t.Fatal(err)
	}
	requests := make(chan string, 1)
	server := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests <- r.Method + " " + r.URL.Path
		w.WriteHeader(http.StatusAccepted)
	})}
	defer server.Close()
	go server.Serve(socket)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := (Launcher{Home: home}).ShowDesktop(ctx); err != nil {
		t.Fatal(err)
	}
	if got := <-requests; got != "POST /v1/desktop" {
		t.Fatalf("request = %q", got)
	}
}

func TestVMStatusIgnoresOtherInstances(t *testing.T) {
	lines := "{\"name\":\"other\",\"status\":\"Running\"}\n{\"name\":\"cli-mvp\",\"status\":\"Stopped\"}\n"
	if got, err := parseVMInfo(lines); err != nil || got.Status != "stopped" {
		t.Fatalf("status = %q, err = %v", got.Status, err)
	}
	if got, err := parseVMInfo(""); err != nil || got.Status != "" {
		t.Fatalf("status for no instances = %q, err = %v", got.Status, err)
	}
}

func TestGuestProjectPathUsesLimaVirtioFSVolume(t *testing.T) {
	host := "/tmp/project"
	want := "/Volumes/My Shared Files/lima-80c57bbd61e73c94"
	if got := guestProjectPath(host); got != want {
		t.Fatalf("guest path = %q, want %q", got, want)
	}
}

func TestPrepareUpdatesCallerShareRootBeforeFailingOnMissingVM(t *testing.T) {
	root := t.TempDir()
	home := t.TempDir()
	realShare := filepath.Join(root, "real-share")
	if err := os.MkdirAll(filepath.Join(realShare, "workspace"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(realShare, "claude"), 0700); err != nil {
		t.Fatal(err)
	}
	linkShare := filepath.Join(root, "link-share")
	if err := os.Symlink(realShare, linkShare); err != nil {
		t.Fatal(err)
	}

	l := Launcher{Root: root, Home: home, ShareRoot: linkShare, LimactlBin: filepath.Join(root, "no-such-limactl")}
	cfg := Settings{Project: filepath.Join(realShare, "workspace")}

	if err := l.Prepare(context.Background(), cfg); err == nil {
		t.Fatal("expected Prepare to fail at InspectVM with a missing limactl binary")
	}

	// Prepare 的第一步 EnsureShareDirs 会把 ShareRoot 解析成符号链接指向的真实路径。
	// 即使 Prepare 随后因为 limactl 缺失而失败，调用者持有的 l 也应该看到这次更新。
	resolvedRealShare, err := filepath.EvalSymlinks(realShare)
	if err != nil {
		t.Fatal(err)
	}
	if l.ShareRoot != resolvedRealShare {
		t.Fatalf("ShareRoot = %q, want %q (Prepare must propagate the resolved path to the caller)", l.ShareRoot, resolvedRealShare)
	}
}

func TestSaveSettingsRoundTripsThemeAndLocalePrefs(t *testing.T) {
	home := t.TempDir()
	l := Launcher{Home: home}
	want := Settings{Port: "7890", LocalePref: "zh-Hans", ThemePref: "dark"}
	if err := l.SaveSettings(want); err != nil {
		t.Fatal(err)
	}
	got := l.Settings()
	if got.LocalePref != "zh-Hans" || got.ThemePref != "dark" {
		t.Fatalf("prefs not persisted: LocalePref = %q, ThemePref = %q", got.LocalePref, got.ThemePref)
	}
}

func TestSettingsDefaultsBothPrefsToFollowSystem(t *testing.T) {
	home := t.TempDir()
	legacy := `{"Project":"/some/path","Port":"7890"}`
	if err := os.WriteFile(filepath.Join(home, "launcher.json"), []byte(legacy), 0600); err != nil {
		t.Fatal(err)
	}
	cfg := Launcher{Home: home}.Settings()
	if cfg.ThemePref != "" || cfg.LocalePref != "" {
		t.Fatalf("a legacy config must leave both prefs empty (follow system), got theme = %q, locale = %q", cfg.ThemePref, cfg.LocalePref)
	}
	if cfg.Port != "7890" {
		t.Fatalf("legacy Port lost: %q", cfg.Port)
	}
}

func TestParseSessionsIncludesStartedAt(t *testing.T) {
	createdAt := time.Date(2026, 11, 15, 11, 18, 0, 0, time.UTC)
	lines := fmt.Sprintf("cli-abc1::claude::%d\n", createdAt.Unix())
	sessions := ParseSessions(lines)
	if len(sessions) != 1 {
		t.Fatalf("got %d sessions, want 1", len(sessions))
	}
	if !sessions[0].StartedAt.Equal(createdAt) {
		t.Fatalf("StartedAt = %v, want %v", sessions[0].StartedAt, createdAt)
	}
}

func TestParseSessionsHandlesMissingTimestamp(t *testing.T) {
	sessions := ParseSessions("cli-abc1::claude\n") // 旧格式，没有第三段
	if len(sessions) != 1 {
		t.Fatalf("got %d sessions, want 1 (a malformed line must not be dropped)", len(sessions))
	}
	if !sessions[0].StartedAt.IsZero() {
		t.Fatalf("StartedAt = %v, want the zero value when the timestamp is missing", sessions[0].StartedAt)
	}
	if sessions[0].Command != "claude" {
		t.Fatalf("Command = %q, want %q (the other fields must still parse)", sessions[0].Command, "claude")
	}
}

func TestParseSessionsHandlesUnparsableTimestamp(t *testing.T) {
	sessions := ParseSessions("cli-abc1::claude::not-a-number\n")
	if len(sessions) != 1 {
		t.Fatalf("got %d sessions, want 1", len(sessions))
	}
	if !sessions[0].StartedAt.IsZero() {
		t.Fatalf("StartedAt = %v, want the zero value for an unparsable timestamp", sessions[0].StartedAt)
	}
}

func TestSessionUptimeReportsElapsedTime(t *testing.T) {
	now := time.Date(2026, 11, 15, 12, 0, 0, 0, time.UTC)
	got, ok := SessionUptime(Session{StartedAt: now.Add(-42 * time.Minute)}, now)
	if !ok || got != 42*time.Minute {
		t.Fatalf("SessionUptime() = (%v, %v), want (42m, true)", got, ok)
	}
}

func TestSessionUptimeUnavailableWhenStartedAtMissing(t *testing.T) {
	got, ok := SessionUptime(Session{}, time.Now())
	if ok || got != 0 {
		t.Fatalf("SessionUptime() = (%v, %v), want (0, false) for a zero StartedAt", got, ok)
	}
}

func TestSessionUptimeClampsNegativeToZero(t *testing.T) {
	now := time.Date(2026, 11, 15, 12, 0, 0, 0, time.UTC)
	got, ok := SessionUptime(Session{StartedAt: now.Add(time.Hour)}, now) // 时钟漂移
	if !ok || got != 0 {
		t.Fatalf("SessionUptime() = (%v, %v), want (0, true) clamped, never negative", got, ok)
	}
}

func TestParseSessionsCapturesExitCode(t *testing.T) {
	createdAt := time.Date(2026, 11, 15, 11, 0, 0, 0, time.UTC)
	lines := fmt.Sprintf("cli-abc1::%s::%d::1\n", ExitedCommand, createdAt.Unix())
	sessions := ParseSessions(lines)
	if len(sessions) != 1 {
		t.Fatalf("got %d sessions, want 1", len(sessions))
	}
	if !sessions[0].Exited {
		t.Fatal("Exited = false, want true for a dead pane")
	}
	if sessions[0].ExitCode != 1 {
		t.Fatalf("ExitCode = %d, want 1", sessions[0].ExitCode)
	}
}

func TestParseSessionsLeavesLiveSessionsUnexited(t *testing.T) {
	lines := fmt.Sprintf("cli-abc1::claude::%d::\n", time.Now().Unix())
	sessions := ParseSessions(lines)
	if sessions[0].Exited {
		t.Fatal("Exited = true, want false while the pane is alive")
	}
}

func TestSessionUptimeUnavailableForExitedSession(t *testing.T) {
	now := time.Date(2026, 11, 15, 12, 0, 0, 0, time.UTC)
	s := Session{StartedAt: now.Add(-42 * time.Minute), Exited: true}
	// 已结束的会话不该再报「运行 42 分钟」——那和「已退出」自相矛盾。
	if got, ok := SessionUptime(s, now); ok || got != 0 {
		t.Fatalf("SessionUptime() = (%v, %v), want (0, false) for an exited session", got, ok)
	}
}
