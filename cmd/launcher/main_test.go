package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/egoist/mygo/plugins/terminal"
	"github.com/finch-xu/clawdhome/vm"
)

// syncUpdater 同步执行回调，替代真实窗口的 Update。
type syncUpdater struct{}

func (syncUpdater) Update(fn func()) { fn() }

type queuedUpdater chan func()

func (q queuedUpdater) Update(fn func()) { q <- fn }

func (q queuedUpdater) next(t *testing.T) func() {
	t.Helper()
	select {
	case fn := <-q:
		return fn
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for a UI update")
		return nil
	}
}

// stubAppearance 记录被套用的偏好，不碰 MyGo 的全局 API（那些调用在没有事件
// 循环的测试进程里会死锁）。
type stubAppearance struct {
	theme, locale string
	system        string
}

func (s *stubAppearance) SetTheme(source string) { s.theme = source }
func (s *stubAppearance) SetLocale(tag string)   { s.locale = tag }
func (s *stubAppearance) SystemLocale() string {
	if s.system == "" {
		return "en"
	}
	return s.system
}

func TestPollSkipsWhenBusy(t *testing.T) {
	a := &app{window: syncUpdater{}, busy: true, lastPoll: time.Now().Add(-10 * time.Second)}
	a.poll()
	if a.polling {
		t.Fatal("poll should not start a new poll while busy")
	}
}

func TestPollSkipsWhileAnotherPollIsInFlight(t *testing.T) {
	a := &app{window: syncUpdater{}, polling: true, lastPoll: time.Now().Add(-10 * time.Second)}
	before := a.lastPoll
	a.poll()
	if !a.lastPoll.Equal(before) {
		t.Fatal("poll should not restart its clock while a poll is in flight")
	}
}

func TestRunRollsBackStateWhenPrepareFails(t *testing.T) {
	root := t.TempDir()
	l := vm.Launcher{Root: root, Home: t.TempDir(), ShareRoot: t.TempDir(),
		LimactlBin: filepath.Join(root, "no-such-limactl")}
	q := make(queuedUpdater, 4)
	a := &app{window: q, launcher: l, state: "stopped",
		terms: map[string]*terminal.Terminal{}}

	a.run("/bin/zsh")
	q.next(t)()

	if a.state != "stopped" {
		t.Fatalf("state = %q, want rollback to %q after a failed run", a.state, "stopped")
	}
	if a.message.text(a.locale) == "" {
		t.Fatal("expected an error message after a failed run")
	}
	if a.busy {
		t.Fatal("busy should be cleared once run finishes")
	}
	q.next(t)() // Reconcile with the backend in a separate UI event.
}

func TestRunIsIgnoredWhileBusy(t *testing.T) {
	a := &app{window: syncUpdater{}, busy: true, state: "running", message: notice{key: "session.starting"}}

	a.run("/bin/zsh")

	if a.state != "running" {
		t.Fatalf("state = %q, want it untouched while busy", a.state)
	}
}

// TestRunWorksOnALauncherCopy 证明后台 goroutine 在 launcher 的副本上调用
// Prepare（Prepare 是指针接收者，会改写 ShareRoot），而不是直接写共享的
// a.launcher；解析后的路径随结果一起经 window.Update 写回。
func TestRunWorksOnALauncherCopy(t *testing.T) {
	root := t.TempDir()
	realShare := filepath.Join(root, "real-share")
	for _, dir := range []string{"workspace", "claude"} {
		if err := os.MkdirAll(filepath.Join(realShare, dir), 0700); err != nil {
			t.Fatal(err)
		}
	}
	linkShare := filepath.Join(root, "link-share")
	if err := os.Symlink(realShare, linkShare); err != nil {
		t.Fatal(err)
	}
	l := vm.Launcher{Root: root, Home: t.TempDir(), ShareRoot: linkShare,
		LimactlBin: filepath.Join(root, "no-such-limactl")}
	q := make(queuedUpdater, 4)
	a := &app{window: q, launcher: l, state: "stopped",
		cfg:   vm.Settings{Project: filepath.Join(linkShare, "workspace")},
		terms: map[string]*terminal.Terminal{}}

	a.run("/bin/zsh")
	q.next(t)()

	// run 失败（limactl 缺失）后，解析过的 ShareRoot 仍然要写回 a.launcher，
	// 否则下次 Create 会用过期路径算 VirtioFS 卷名。
	want, err := filepath.EvalSymlinks(realShare)
	if err != nil {
		t.Fatal(err)
	}
	if a.launcher.ShareRoot != want {
		t.Fatalf("a.launcher.ShareRoot = %q, want the resolved %q written back through window.Update",
			a.launcher.ShareRoot, want)
	}
	q.next(t)()
}

// markWorkspace 在 root 下建出 marker 文件，再建出 exeDir 并返回它。marker 是
// 判定「这是项目根」的唯一依据，所以每个用例都要先有它。
func markWorkspace(t *testing.T, root, exeDir string) string {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(root, ".mvp-cache"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".mvp-cache", "instance-home"), []byte("/tmp/x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(root, exeDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	return dir
}

// scripts/run-app.sh 的布局：bundle 直接放在 .mvp-cache 下。
func TestFindWorkspaceFromScriptBundle(t *testing.T) {
	root := t.TempDir()
	exeDir := markWorkspace(t, root, filepath.Join(".mvp-cache", "CLI Launcher.app", "Contents", "MacOS"))

	if got := findWorkspace(exeDir); got != root {
		t.Fatalf("findWorkspace(%q) = %q, want the project root %q", exeDir, got, root)
	}
}

// mygo dev 的布局：bundle 在 .mygo/dev/<goos>-<goarch>/ 下，比脚本深一层。靠
// 数目录层数的做法会在这里返回 <root>/.mygo/dev。
func TestFindWorkspaceFromMygoDevBundle(t *testing.T) {
	root := t.TempDir()
	exeDir := markWorkspace(t, root,
		filepath.Join(".mygo", "dev", "darwin-arm64", "CLI Launcher Dev.app", "Contents", "MacOS"))

	if got := findWorkspace(exeDir); got != root {
		t.Fatalf("findWorkspace(%q) = %q, want the project root %q", exeDir, got, root)
	}
}

// go run ./cmd/launcher：可执行文件在临时目录里，和项目根毫无关系。找不到
// marker 时回退 "."，由调用方按工作目录解析。
func TestFindWorkspaceFallsBackToWorkingDirectory(t *testing.T) {
	exeDir := t.TempDir()

	if got := findWorkspace(exeDir); got != "." {
		t.Fatalf("findWorkspace(%q) = %q, want %q when no marker is above it", exeDir, got, ".")
	}
}

// 手工放过 dylib 的仓库（README「重建」第 4 步）：指向它，省掉一次下载。
func TestGhosttyLibraryPathUsesTheCachedDylib(t *testing.T) {
	root := t.TempDir()
	dylib := filepath.Join(root, ".mvp-cache", "mygo-native", "libghostty-vt.dylib")
	if err := os.MkdirAll(filepath.Dir(dylib), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dylib, []byte("not a real dylib"), 0o644); err != nil {
		t.Fatal(err)
	}

	if got := ghosttyLibraryPath(root); got != dylib {
		t.Fatalf("ghosttyLibraryPath(%q) = %q, want the cached %q", root, got, dylib)
	}
}

// 缓存里没有 dylib 时必须返回空串，让调用方不要设 MYGO_GHOSTTY_VT：那个变量
// 是查找链的第一位，且框架不校验文件存在，设了就会掐死 mygo dev 签进 app
// resources 的那一份和用户缓存里下载的那一份。
func TestGhosttyLibraryPathIsEmptyWithoutTheCachedDylib(t *testing.T) {
	if got := ghosttyLibraryPath(t.TempDir()); got != "" {
		t.Fatalf("ghosttyLibraryPath = %q, want \"\" so the framework keeps looking", got)
	}
}

func fakeLauncher(t *testing.T, script string) vm.Launcher {
	t.Helper()
	root := t.TempDir()
	bin := filepath.Join(root, "limactl")
	if err := os.WriteFile(bin, []byte("#!/bin/sh\n"+script), 0700); err != nil {
		t.Fatal(err)
	}
	return vm.Launcher{Root: root, Home: root, LimactlBin: bin}
}

func TestStopDiscardsOlderPollAndNetworkSnapshot(t *testing.T) {
	q := make(queuedUpdater, 4)
	a := &app{window: q, state: "running", ssh: vm.SSHAvailable,
		launcher: fakeLauncher(t, `
case "$1" in
	list) printf '{"name":"cli-mvp","status":"Running"}\n';;
	shell) echo old-result;;
	stop) touch "$LIMA_HOME/stopped";;
esac
`)}
	a.poll()
	applyPoll := q.next(t)
	a.refreshNetwork(false)
	applySnapshot := q.next(t)
	a.stop()
	q.next(t)()
	applyPoll()
	applySnapshot()
	if a.state != "stopped" || a.networkText != "" || a.networkBusy {
		t.Fatalf("old callbacks after stop: state=%q, snapshot=%q, networkBusy=%v", a.state, a.networkText, a.networkBusy)
	}
	q.next(t)()
}

func TestKillFailureClearsBusyAndKeepsSelection(t *testing.T) {
	q := make(queuedUpdater, 4)
	a := &app{window: q, selected: "cli-a", state: "running", launcher: fakeLauncher(t, "exit 1")}
	a.kill()
	q.next(t)()
	if a.busy || a.selected != "cli-a" || a.message.text(a.locale) == "" {
		t.Fatalf("selection=%q, busy=%v, message=%q", a.selected, a.busy, a.message.text(a.locale))
	}
	q.next(t)()
}

func TestOldTerminalExitDoesNotClearNewAttachment(t *testing.T) {
	old, current := new(terminal.Terminal), new(terminal.Terminal)
	a := &app{selected: "cli-a", terms: map[string]*terminal.Terminal{"cli-a": current}}
	a.terminalExited("cli-a", old, 1)
	if a.selected != "cli-a" || a.terms["cli-a"] != current || a.message.text(a.locale) != "" {
		t.Fatal("an old terminal's exit changed the new attachment")
	}
}
