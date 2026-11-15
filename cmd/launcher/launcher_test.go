package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

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
