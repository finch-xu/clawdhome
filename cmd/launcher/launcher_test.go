package main

import (
	"context"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestSessionListOnlyIncludesLauncherSessions(t *testing.T) {
	input := "cli-56::python3\nmanual::zsh\ncli-90::vim\ncli-91::@vm-exited\n"
	want := []session{{"cli-56", "python3"}, {"cli-90", "vim"}, {"cli-91", "已退出"}}
	if got := parseSessions(input); !reflect.DeepEqual(got, want) {
		t.Fatalf("sessions = %#v, want %#v", got, want)
	}
}

func TestClaudeShortcutUsesNativeInstallerPath(t *testing.T) {
	home := t.TempDir()
	cmd := exec.Command("/bin/sh", "-c", claudeGuestScript)
	cmd.Env = []string{"HOME=" + home, "PATH=" + home}
	if out, err := cmd.CombinedOutput(); err == nil || !strings.Contains(string(out), "Claude Code is not installed in this VM") {
		t.Fatalf("missing Claude: %q, %v", out, err)
	}
	bin := filepath.Join(home, ".local", "bin")
	if err := os.MkdirAll(bin, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(bin, "claude"), []byte("#!/bin/sh\nprintf '%s' 'local Claude stub'\n"), 0700); err != nil {
		t.Fatal(err)
	}
	cmd = exec.Command("/bin/sh", "-c", claudeGuestScript)
	cmd.Env = []string{"HOME=" + home, "PATH=" + home}
	if out, err := cmd.CombinedOutput(); err != nil || string(out) != "local Claude stub" {
		t.Fatalf("local Claude: %q, %v", out, err)
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
	if err := (launcher{home: home}).showDesktop(ctx); err != nil {
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

func TestFixedMountsAllowProjectsInsideWorkspace(t *testing.T) {
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	l := launcher{shareRoot: root}
	project := filepath.Join(root, "workspace", "my project")
	if err := os.MkdirAll(project, 0700); err != nil {
		t.Fatal(err)
	}
	want := []vmMount{{Location: filepath.Join(root, "workspace"), Writable: true}, {Location: filepath.Join(root, "claude"), Writable: true}}
	if got := l.sharedMounts(); !reflect.DeepEqual(got, want) {
		t.Fatalf("mounts = %+v, want %+v", got, want)
	}
	if rel, err := l.projectInWorkspace(project); err != nil || rel != "my project" {
		t.Fatalf("project path = %q, error = %v", rel, err)
	}
	escape := filepath.Join(root, "workspace", "outside")
	if err := os.Symlink(root, escape); err != nil {
		t.Fatal(err)
	}
	if _, err := l.projectInWorkspace(escape); err == nil {
		t.Fatal("accepted a symlink leaving the workspace")
	}
}
