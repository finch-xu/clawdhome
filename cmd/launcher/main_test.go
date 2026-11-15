package main

import (
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
