package vm

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestFixedMountsAllowProjectsInsideWorkspace(t *testing.T) {
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	l := Launcher{ShareRoot: root}
	project := filepath.Join(root, "workspace", "my project")
	if err := os.MkdirAll(project, 0700); err != nil {
		t.Fatal(err)
	}
	want := []VMMount{{Location: filepath.Join(root, "workspace"), Writable: true}, {Location: filepath.Join(root, "claude"), Writable: true}}
	if got := l.sharedMounts(); !reflect.DeepEqual(got, want) {
		t.Fatalf("mounts = %+v, want %+v", got, want)
	}
	if rel, err := l.ProjectInWorkspace(project); err != nil || rel != "my project" {
		t.Fatalf("project path = %q, error = %v", rel, err)
	}
	escape := filepath.Join(root, "workspace", "outside")
	if err := os.Symlink(root, escape); err != nil {
		t.Fatal(err)
	}
	if _, err := l.ProjectInWorkspace(escape); err == nil {
		t.Fatal("accepted a symlink leaving the workspace")
	}
}

func TestEnsureShareDirsWrapsMkdirError(t *testing.T) {
	// 在一个普通文件下面建目录必定失败：ShareRoot 的父路径是文件，不是目录。
	blocker := filepath.Join(t.TempDir(), "not-a-dir")
	if err := os.WriteFile(blocker, []byte("x"), 0600); err != nil {
		t.Fatal(err)
	}
	l := &Launcher{ShareRoot: filepath.Join(blocker, "share")}

	err := l.EnsureShareDirs()
	if err == nil {
		t.Fatal("expected EnsureShareDirs to fail when the parent path is a file")
	}
	if !strings.Contains(err.Error(), "共享目录") {
		t.Fatalf("error %q lacks context about what failed", err.Error())
	}
}

func TestSaveSettingsWrapsWriteError(t *testing.T) {
	// Home 指向不存在的目录，CreateTemp 必定失败。
	l := Launcher{Home: filepath.Join(t.TempDir(), "no-such-home")}

	err := l.SaveSettings(Settings{Port: "7890"})
	if err == nil {
		t.Fatal("expected SaveSettings to fail with a missing home directory")
	}
	if !strings.Contains(err.Error(), "设置") {
		t.Fatalf("error %q lacks context about what failed", err.Error())
	}
}
