package vm

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/plugins/terminal"
	"github.com/egoist/mygo/ui"
	egress "github.com/finch-xu/clawdhome/lima-egress"
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
	shareRoot, err := DefaultShareRoot()
	if err != nil {
		t.Fatal(err)
	}
	l := Launcher{Root: root, Home: strings.TrimSpace(string(home)), ShareRoot: shareRoot, LimactlBin: filepath.Join(root, ".mvp-cache/lima/bin/limactl-mvp")}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	vm, err := l.InspectVM(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if status := l.NetworkStatus(vm); status == nil || status.Online || status.Route != egress.RouteBlocked {
		t.Fatalf("expected a live gateway with disconnected internet, got %+v", status)
	}
	if snapshot, err := l.NetworkSnapshot(ctx, false); err != nil ||
		!strings.Contains(snapshot, "=== IPv4 sockets ===") || !strings.Contains(snapshot, "=== DNS resolvers ===") {
		t.Fatalf("guest network snapshot incomplete: %v", err)
	}
	create := func(project, command string) string {
		t.Helper()
		id, err := l.Create(ctx, project, command)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			cleanupCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			_ = l.KillSession(cleanupCtx, id)
		})
		return id
	}
	if err := l.Prepare(ctx, l.Settings()); err != nil {
		t.Fatal(err)
	}
	project, err := os.MkdirTemp(l.Workspace(), "mvp project ")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(project)
	if err := os.WriteFile(filepath.Join(project, "marker"), []byte("mvp-workspace-ok"), 0600); err != nil {
		t.Fatal(err)
	}
	otherProject, err := os.MkdirTemp(l.Workspace(), "mvp other ")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(otherProject)
	zone, err := commandOutput(l.Cmd(ctx, "", l.LimactlBin, vmShellArgs("/usr/bin/readlink /etc/localtime")...))
	if err != nil || !strings.HasSuffix(strings.TrimSpace(string(zone)), "/zoneinfo/America/Los_Angeles") {
		t.Fatalf("guest timezone = %q, error = %v", zone, err)
	}

	a := create(project, "/bin/sh -c 'test \"$(cat marker)\" = mvp-workspace-ok || exit 1; echo mvp-ready; exec /bin/zsh -f'")
	otherCfg := l.Settings()
	otherCfg.Project = otherProject
	if err := l.Prepare(ctx, otherCfg); err != nil {
		t.Fatal(err)
	}
	if otherVM, err := l.InspectVM(ctx); err != nil || otherVM.HostAgentPID != vm.HostAgentPID {
		t.Fatalf("switching projects changed the VM host agent: %+v, %v", otherVM, err)
	}
	b := create(otherProject, "/bin/sh")
	tmux := func(args ...string) string {
		t.Helper()
		out, err := commandOutput(l.Cmd(ctx, "", l.LimactlBin, l.TmuxArgs(args...)...))
		if err != nil {
			t.Fatal(err)
		}
		return strings.TrimSpace(string(out))
	}
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
	sessions, err := l.List(ctx)
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

	finished := create(project, "/usr/bin/printf 'mvp-finished::0'")
	failed := create(project, "/bin/sh -c 'exit 1'")
	signaled := create(project, "/bin/sh -c 'kill -TERM $$'")
	for _, tc := range []struct {
		id      string
		known   bool
		code    int
		signal  string
		command string
	}{
		{finished, true, 0, "", "mvp-finished::0"},
		{failed, true, 1, "", "exit 1"},
		{signaled, false, 0, "TERM", "kill -TERM"},
	} {
		deadline := time.Now().Add(5 * time.Second)
		for tmux("display-message", "-p", "-t", tc.id+":0", "#{pane_dead}") != "1" {
			if time.Now().After(deadline) {
				t.Fatal("short CLI did not exit")
			}
			time.Sleep(25 * time.Millisecond)
		}
		sessions, err := l.List(ctx)
		if err != nil {
			t.Fatal(err)
		}
		var got Session
		for _, s := range sessions {
			if s.ID == tc.id {
				got = s
				break
			}
		}
		if !got.Exited || got.ExitCodeKnown != tc.known || got.ExitCode != tc.code ||
			got.ExitSignal != tc.signal || !strings.Contains(got.Command, tc.command) || got.StartedAt.IsZero() {
			t.Fatalf("unexpected tmux exit metadata: %+v", got)
		}
	}
	if output := tmux("capture-pane", "-p", "-S", "-", "-t", finished+":0"); !strings.Contains(output, "mvp-finished::0") {
		t.Fatalf("short CLI output was lost: %q", output)
	}
}

type testTerminal struct {
	*ui.Tester
	terminal *terminal.Terminal
}

func attachTestTerminal(t *testing.T, l Launcher, id string) testTerminal {
	t.Helper()
	term, err := terminal.New(terminal.Options{
		Command: append([]string{l.LimactlBin}, l.TmuxArgs("attach-session", "-t", "="+id)...),
		Env:     []string{"LIMA_HOME=" + l.Home},
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
