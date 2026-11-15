package vm

import (
	"context"
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
	want := []Session{{"cli-56", "python3"}, {"cli-90", "vim"}, {"cli-91", "已退出"}}
	if got := parseSessions(input); !reflect.DeepEqual(got, want) {
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
