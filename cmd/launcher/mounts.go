package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

type vmMount struct {
	Location   string `json:"location"`
	MountPoint string `json:"mountPoint,omitempty"`
	Writable   bool   `json:"writable"`
}

func defaultShareRoot() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, "macOS-CLI-Launcher", instance), nil
}

func (l launcher) workspace() string { return filepath.Join(l.shareRoot, "workspace") }

func (l launcher) sharedMounts() []vmMount {
	return []vmMount{
		{Location: l.workspace(), Writable: true},
		{Location: filepath.Join(l.shareRoot, "claude"), Writable: true},
	}
}

func (l *launcher) ensureShareDirs() error {
	if err := os.MkdirAll(l.shareRoot, 0700); err != nil {
		return err
	}
	root, err := filepath.EvalSymlinks(l.shareRoot)
	if err != nil {
		return err
	}
	l.shareRoot = root
	for _, mount := range l.sharedMounts() {
		if err := os.MkdirAll(mount.Location, 0700); err != nil {
			return err
		}
		info, err := os.Lstat(mount.Location)
		if err != nil {
			return err
		}
		if !info.IsDir() {
			return fmt.Errorf("共享目录须为本地目录 (%s)", mount.Location)
		}
	}
	return nil
}

func (l launcher) projectInWorkspace(project string) (string, error) {
	invalid := fmt.Errorf("请在工作空间内选择项目目录 (%s)", l.workspace())
	if !filepath.IsAbs(project) {
		return "", invalid
	}
	resolved, err := filepath.EvalSymlinks(project)
	if err != nil {
		return "", invalid
	}
	root, err := filepath.EvalSymlinks(l.workspace())
	if err != nil {
		return "", err
	}
	rel, err := filepath.Rel(root, resolved)
	if err != nil || rel == ".." || strings.HasPrefix(rel, "../") {
		return "", invalid
	}
	info, err := os.Stat(resolved)
	if err != nil || !info.IsDir() {
		return "", invalid
	}
	return rel, nil
}

func (vm vmInfo) hasMounts(want []vmMount) bool {
	actual := slices.Clone(vm.Config.Mounts)
	for i := range actual {
		if actual[i].MountPoint == actual[i].Location {
			actual[i].MountPoint = ""
		}
	}
	return slices.Equal(actual, want)
}

func (l launcher) linkGuestShares(ctx context.Context) error {
	mounts := l.sharedMounts()
	const link = `
set -eu
link_share() {
    target=$1
    path=$2
    test -d "$target"
    marker="$HOME/.local/share/cli-mvp-${path##*/}.share-migration"
    if test -L "$path"; then
        test "$(/usr/bin/readlink "$path")" = "$target" || {
            printf 'Existing symlink points elsewhere: %s\n' "$path" >&2
            exit 1
        }
        if test -f "$marker" && test "$(/bin/cat "$marker")" = "$target"; then /bin/rm "$marker"; fi
        return
    fi
    backup=
    if test -e "$path"; then
        test -d "$path"
        if test -n "$(/bin/ls -A "$path")"; then
            if test -f "$marker"; then
                test "$(/bin/cat "$marker")" = "$target"
            elif test -n "$(/bin/ls -A "$target")"; then
                printf 'Both directories already contain data: %s and %s\n' "$path" "$target" >&2
                exit 1
            else
                /bin/mkdir -p "$HOME/.local/share"
                journal=$(/usr/bin/mktemp "$marker.XXXXXX")
                printf '%s\n' "$target" > "$journal"
                /bin/mv "$journal" "$marker"
            fi
            /usr/bin/rsync -a --ignore-existing "$path/" "$target/"
        fi
        backup=$(/usr/bin/mktemp -d "$HOME/.before-share.XXXXXX")
        /bin/mv "$path" "$backup/original"
    fi
    if ! /bin/ln -s "$target" "$path"; then
        if test -n "$backup"; then /bin/mv "$backup/original" "$path"; fi
        exit 1
    fi
    if test -f "$marker" && test "$(/bin/cat "$marker")" = "$target"; then /bin/rm "$marker"; fi
}
link_share "$1" "$HOME/workspace"
link_share "$2" "$HOME/.claude"
`
	args := vmShellArgs(link, guestProjectPath(mounts[0].Location), guestProjectPath(mounts[1].Location))
	if _, err := commandOutput(l.cmd(ctx, "", l.lima, args...)); err != nil {
		return fmt.Errorf("VM 工作空间或 Claude 配置目录初始化失败 (%w)", err)
	}
	return nil
}
