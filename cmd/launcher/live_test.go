package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/plugins/terminal"
	"github.com/egoist/mygo/ui"
)

func TestLiveVMCanReattachEmbeddedTerminalOffline(t *testing.T) {
	root := os.Getenv("MVP_TEST_WORKSPACE")
	if root == "" {
		t.Skip("set MVP_TEST_WORKSPACE for the local macOS guest test")
	}
	home, err := os.ReadFile(filepath.Join(root, ".mvp-cache", "instance-home"))
	if err != nil {
		t.Fatal(err)
	}
	shareRoot, err := defaultShareRoot()
	if err != nil {
		t.Fatal(err)
	}
	l := launcher{root: root, home: strings.TrimSpace(string(home)), shareRoot: shareRoot, lima: filepath.Join(root, ".mvp-cache/lima/bin/limactl-mvp")}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	vm, err := l.inspectVM(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if status := l.networkStatus(vm); status == nil || status.Online {
		t.Fatalf("expected a live gateway with disconnected internet, got %+v", status)
	}
	if err := l.prepare(ctx, l.settings()); err != nil {
		t.Fatal(err)
	}
	project, err := os.MkdirTemp(l.workspace(), "mvp project ")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(project)
	if err := os.WriteFile(filepath.Join(project, "marker"), []byte("mvp-workspace-ok"), 0600); err != nil {
		t.Fatal(err)
	}
	otherProject, err := os.MkdirTemp(l.workspace(), "mvp other ")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(otherProject)
	zone, err := commandOutput(l.cmd(ctx, "", l.lima, vmShellArgs("/usr/bin/readlink /etc/localtime")...))
	if err != nil || !strings.HasSuffix(strings.TrimSpace(string(zone)), "/zoneinfo/America/Los_Angeles") {
		t.Fatalf("guest timezone = %q, error = %v", zone, err)
	}

	a, err := l.create(ctx, project, "/bin/sh -c 'test \"$(cat marker)\" = mvp-workspace-ok || exit 1; echo mvp-ready; exec /bin/zsh -f'")
	if err != nil {
		t.Fatal(err)
	}
	otherCfg := l.settings()
	otherCfg.Project = otherProject
	if err := l.prepare(ctx, otherCfg); err != nil {
		t.Fatal(err)
	}
	if otherVM, err := l.inspectVM(ctx); err != nil || otherVM.HostAgentPID != vm.HostAgentPID {
		t.Fatalf("switching projects changed the VM host agent: %+v, %v", otherVM, err)
	}
	b, err := l.create(ctx, otherProject, "/bin/sh")
	if err != nil {
		t.Fatal(err)
	}
	tmux := func(args ...string) string {
		t.Helper()
		out, err := commandOutput(l.cmd(ctx, "", l.lima, l.tmuxArgs(args...)...))
		if err != nil {
			t.Fatal(err)
		}
		return strings.TrimSpace(string(out))
	}
	defer func() {
		for _, id := range []string{a, b} {
			_ = l.cmd(ctx, "", l.lima, l.tmuxArgs("kill-session", "-t", "="+id)...).Run()
		}
	}()
	// Pane names are session IDs, so this PID distinguishes reattaching
	// from accidentally starting the CLI again.
	target := a + ":0"
	pid := tmux("display-message", "-p", "-t", target, "#{pane_pid}")
	if pid == "" {
		t.Fatal("guest CLI has no PID")
	}
	for id, path := range map[string]string{a: project, b: otherProject} {
		if got := tmux("display-message", "-p", "-t", id+":0", "#{pane_current_path}"); !strings.HasSuffix(got, "/"+filepath.Base(path)) {
			t.Fatalf("session %s started in %q, expected project %q", id, got, filepath.Base(path))
		}
	}

	term := attachTestTerminal(t, l, a)
	term.Type("echo mvp-first-attach")
	term.Key(0, ui.KeyEnter)
	waitForText(t, term, "mvp-first-attach", time.Second*5)
	term.close()

	// A fresh UI terminal connects to the detached guest session.
	again := attachTestTerminal(t, l, a)
	defer again.close()
	again.Type("echo mvp-second-attach")
	again.Key(0, ui.KeyEnter)
	waitForText(t, again, "mvp-second-attach", time.Second*5)
	if got := tmux("display-message", "-p", "-t", target, "#{pane_pid}"); got != pid {
		t.Fatalf("attach changed the guest process PID from %s to %s", pid, got)
	}
	history := tmux("capture-pane", "-p", "-S", "-", "-t", target)
	if !strings.Contains(history, "\nmvp-first-attach\n") || !strings.Contains(history, "\nmvp-second-attach\n") {
		t.Fatalf("the two terminal commands did not both run in the original guest session")
	}
	tmux("has-session", "-t", "="+b)
	sessions, err := l.list(ctx)
	if err != nil {
		t.Fatal(err)
	}
	listed := make(map[string]bool)
	for _, s := range sessions {
		listed[s.ID] = true
	}
	if !listed[a] || !listed[b] {
		t.Fatalf("session discovery did not find %s and %s: %v", a, b, sessions)
	}

	finished, err := l.create(ctx, project, "/usr/bin/printf mvp-finished")
	if err != nil {
		t.Fatal(err)
	}
	defer l.cmd(ctx, "", l.lima, l.tmuxArgs("kill-session", "-t", "="+finished)...).Run()
	deadline := time.Now().Add(5 * time.Second)
	for tmux("display-message", "-p", "-t", finished+":0", "#{pane_dead}") != "1" {
		if time.Now().After(deadline) {
			t.Fatal("short CLI did not exit")
		}
		time.Sleep(25 * time.Millisecond)
	}
	if output := tmux("capture-pane", "-p", "-S", "-", "-t", finished+":0"); !strings.Contains(output, "mvp-finished") {
		t.Fatalf("short CLI output was lost: %q", output)
	}
}

type testTerminal struct {
	*ui.Tester
	terminal *terminal.Terminal
}

func attachTestTerminal(t *testing.T, l launcher, id string) testTerminal {
	t.Helper()
	term, err := terminal.New(terminal.Options{
		Command: append([]string{l.lima}, l.tmuxArgs("attach-session", "-t", "="+id)...),
		Env:     []string{"LIMA_HOME=" + l.home},
	})
	if err != nil {
		t.Fatal(err)
	}
	view := ui.NewTester(func(c *ui.Context) {
		terminal.View(c, term).Fill().AutoFocus()
	}, 800, 480)
	view.Frame()
	waitForText(t, testTerminal{view, term}, "mvp-ready", 5*time.Second)
	return testTerminal{view, term}
}

func (t testTerminal) close() { _ = t.terminal.Close() }

func waitForText(t *testing.T, term testTerminal, text string, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		term.Frame()
		if strings.Contains(term.terminal.Text(), text) {
			return
		}
		time.Sleep(25 * time.Millisecond)
	}
	t.Fatalf("terminal did not show %q: %s", text, term.terminal.Text())
}
