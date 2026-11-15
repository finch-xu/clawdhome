package vm

import (
	"os"
	"path/filepath"
	"reflect"
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
