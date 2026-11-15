package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	egress "github.com/finch-xu/clawdhome/lima-egress"
)

const (
	instance      = "cli-mvp"
	guestTimeZone = "America/Los_Angeles"
)

type session struct {
	ID, Command string
}

const sessionSeparator = "::"
const exitedSession = "@vm-exited"

type vmInfo struct {
	Name, Status string
	HostAgentPID int
	Config       struct {
		TimeZone string
		Mounts   []vmMount
	}
}

func parseSessions(lines string) []session {
	var result []session
	for _, line := range strings.Split(lines, "\n") {
		id, command, ok := strings.Cut(line, sessionSeparator)
		if ok && strings.HasPrefix(id, "cli-") {
			if command == exitedSession {
				command = "已退出"
			}
			result = append(result, session{id, command})
		}
	}
	return result
}

type launcher struct {
	root, home, lima, shareRoot string
}

type settings struct {
	Project string
	Port    string
}

func (l launcher) settings() settings {
	var cfg settings
	raw, err := os.ReadFile(filepath.Join(l.home, "launcher.json"))
	if err == nil {
		_ = json.Unmarshal(raw, &cfg)
	}
	if l.shareRoot != "" {
		if _, err := l.projectInWorkspace(cfg.Project); err != nil {
			cfg.Project = l.workspace()
		}
	}
	return cfg
}

func (l launcher) saveSettings(cfg settings) error {
	if cfg.Port != "" && !validPort(cfg.Port) {
		return fmt.Errorf("Clash 端口须为 1–65535")
	}
	if cfg.Project != "" {
		if _, err := l.projectInWorkspace(cfg.Project); err != nil {
			return err
		}
	}
	raw, err := json.Marshal(cfg)
	if err != nil {
		return err
	}
	file, err := os.CreateTemp(l.home, ".launcher-*.json")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	_, err = file.Write(raw)
	closeErr := file.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	return os.Rename(file.Name(), filepath.Join(l.home, "launcher.json"))
}

func validPort(port string) bool {
	n, err := strconv.ParseUint(port, 10, 16)
	return err == nil && n > 0
}

func (l launcher) cmd(ctx context.Context, port, path string, args ...string) *exec.Cmd {
	command := exec.CommandContext(ctx, path, args...)
	command.Env = append(os.Environ(), "LIMA_HOME="+l.home, "MVP_LIMACTL="+l.lima,
		"MVP_CLASH_SOCKS5=127.0.0.1:"+port)
	return command
}

func commandOutput(cmd *exec.Cmd) ([]byte, error) {
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("%w: %s", err, strings.TrimSpace(stderr.String()))
	}
	return out, nil
}

func (l launcher) vmStatus(ctx context.Context) (string, error) {
	vm, err := l.inspectVM(ctx)
	return vm.Status, err
}

func (l launcher) inspectVM(ctx context.Context) (vmInfo, error) {
	out, err := commandOutput(l.cmd(ctx, "", l.lima, "list", "--json"))
	if err != nil {
		return vmInfo{}, err
	}
	return parseVMInfo(string(out))
}

func (l launcher) showDesktop(ctx context.Context) error {
	dialer := net.Dialer{}
	transport := &http.Transport{
		DisableKeepAlives: true,
		DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
			return dialer.DialContext(ctx, "unix", filepath.Join(l.home, instance, "ha.sock"))
		},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "http://localhost/v1/desktop", nil)
	if err != nil {
		return err
	}
	client := http.Client{Transport: transport, CheckRedirect: func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	}}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("无法打开 VM 桌面 (%w)", err)
	}
	defer resp.Body.Close()
	switch resp.StatusCode {
	case http.StatusAccepted:
		return nil
	case http.StatusNotFound:
		return fmt.Errorf("请停止并重新启动 VM，以启用桌面窗口按钮")
	default:
		return fmt.Errorf("无法打开 VM 桌面 (%s)", resp.Status)
	}
}

func parseVMInfo(lines string) (vmInfo, error) {
	for _, line := range strings.Split(strings.TrimSpace(lines), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		var vm vmInfo
		if err := json.Unmarshal([]byte(line), &vm); err != nil {
			return vmInfo{}, err
		}
		if vm.Name == instance {
			vm.Status = strings.ToLower(vm.Status)
			return vm, nil
		}
	}
	return vmInfo{}, nil
}

func (l launcher) prepare(ctx context.Context, cfg settings) error {
	if err := l.ensureShareDirs(); err != nil {
		return err
	}
	if _, err := l.projectInWorkspace(cfg.Project); err != nil {
		return err
	}
	mounts := l.sharedMounts()
	vm, err := l.inspectVM(ctx)
	if err != nil {
		return err
	}
	if vm.Status == "running" {
		if !vm.hasMounts(mounts) {
			return fmt.Errorf("请先停止 VM，再更新共享目录")
		}
		return l.ensureGuestSetup(ctx)
	}
	if egress.ReadRoute(l.home) == egress.RouteClash && !validPort(cfg.Port) {
		return fmt.Errorf("请在设置中填写有效的 Clash 端口")
	}
	if vm.Status == "" {
		if _, err := commandOutput(l.cmd(ctx, cfg.Port, filepath.Join(l.root, "scripts", "create-vm.sh"), l.shareRoot)); err != nil {
			return err
		}
	} else if !vm.hasMounts(mounts) || vm.Config.TimeZone != guestTimeZone {
		encoded, _ := json.Marshal(mounts)
		expression := ".mounts = " + string(encoded) + " | .timezone = " + strconv.Quote(guestTimeZone)
		if _, err := commandOutput(l.cmd(ctx, cfg.Port, l.lima, "edit", "--tty=false", "--set", expression, instance)); err != nil {
			return err
		}
	}
	if err := l.saveSettings(cfg); err != nil {
		return err
	}
	if _, err := commandOutput(l.cmd(ctx, cfg.Port, l.lima, "start", "--tty=false", "--timeout=25m", instance)); err != nil {
		return err
	}
	return l.ensureGuestSetup(ctx)
}

func (l launcher) ensureGuestSetup(ctx context.Context) error {
	if err := l.linkGuestShares(ctx); err != nil {
		return err
	}
	if _, err := commandOutput(l.cmd(ctx, "", l.lima,
		vmShellArgs(`test -r "$HOME/.local/bin/cli-mvp.tmux.conf" &&
			case "$(/usr/bin/readlink /etc/localtime)" in */zoneinfo/`+guestTimeZone+`) ;; *) exit 1 ;; esac && `+execTmux, "-V")...)); err == nil {
		return nil
	}
	_, err := commandOutput(l.cmd(ctx, "", filepath.Join(l.root, "scripts", "setup-guest.sh")))
	if err != nil {
		return fmt.Errorf("VM 初始化失败，请检查本地 tmux 构建与 guest 设置: %w", err)
	}
	return nil
}

const execTmux = `exec "$HOME/.local/bin/tmux" -L cli-mvp -f "$HOME/.local/bin/cli-mvp.tmux.conf" "$@"`

func vmShellArgs(script string, args ...string) []string {
	return append([]string{"shell", "--shell=/bin/sh", instance, "/bin/sh", "-c", script, "launcher"}, args...)
}

func (l launcher) tmuxArgs(args ...string) []string {
	return vmShellArgs(execTmux, args...)
}

func (l launcher) list(ctx context.Context) ([]session, error) {
	result, err := commandOutput(l.cmd(ctx, "", l.lima,
		l.tmuxArgs("list-sessions", "-F", "#{session_name}"+sessionSeparator+"#{?pane_dead,"+exitedSession+",#{pane_current_command}}")...))
	if err != nil {
		if strings.Contains(err.Error(), "no server running") ||
			(strings.Contains(err.Error(), "error connecting to") && strings.Contains(err.Error(), "No such file")) {
			return nil, nil
		}
		return nil, err
	}
	return parseSessions(string(result)), nil
}

// Lima's Darwin VZ guest mounts each VirtioFS share under its generated tag.
func guestProjectPath(hostPath string) string {
	hash := sha256.Sum256([]byte(hostPath + ":" + hostPath))
	return "/Volumes/My Shared Files/lima-" + hex.EncodeToString(hash[:8])
}

func (l launcher) create(ctx context.Context, project, command string) (string, error) {
	if strings.TrimSpace(command) == "" {
		return "", fmt.Errorf("请输入 CLI 命令")
	}
	rel, err := l.projectInWorkspace(project)
	if err != nil {
		return "", err
	}
	var token [4]byte
	if _, err := rand.Read(token[:]); err != nil {
		return "", err
	}
	id := "cli-" + hex.EncodeToString(token[:])
	// Validate the share before tmux can fall back to the guest home directory.
	args := vmShellArgs(`cd "$HOME/workspace/$1" || { echo "项目目录未挂载" >&2; exit 1; }; shift; `+execTmux,
		rel, "new-session", "-d", "-s", id, command)
	_, err = commandOutput(l.cmd(ctx, "", l.lima, args...))
	return id, err
}

func (l launcher) networkStatus(vm vmInfo) *egress.GatewayStatus {
	if vm.Status != "running" || vm.HostAgentPID == 0 {
		return nil
	}
	raw, err := os.ReadFile(filepath.Join(l.home, instance, "mvp-network.json"))
	if err != nil {
		return nil
	}
	var state egress.GatewayStatus
	if json.Unmarshal(raw, &state) != nil || state.PID != vm.HostAgentPID || !state.Running {
		return nil
	}
	return &state
}
