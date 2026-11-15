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
	a := &app{window: syncUpdater{}, launcher: l, state: "stopped",
		terms: map[string]*terminal.Terminal{}}

	done := make(chan struct{})
	a.runWithDone("/bin/zsh", done)
	<-done

	if a.state != "stopped" {
		t.Fatalf("state = %q, want rollback to %q after a failed run", a.state, "stopped")
	}
	if a.message == "" {
		t.Fatal("expected an error message after a failed run")
	}
	if a.busy {
		t.Fatal("busy should be cleared once run finishes")
	}
}

func TestRunIsIgnoredWhileBusy(t *testing.T) {
	a := &app{window: syncUpdater{}, busy: true, state: "running", message: "正在启动会话…"}

	done := make(chan struct{})
	a.runWithDone("/bin/zsh", done)
	<-done

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
	a := &app{window: syncUpdater{}, launcher: l, state: "stopped",
		cfg:   vm.Settings{Project: filepath.Join(linkShare, "workspace")},
		terms: map[string]*terminal.Terminal{}}

	done := make(chan struct{})
	a.runWithDone("/bin/zsh", done)
	<-done

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
}
