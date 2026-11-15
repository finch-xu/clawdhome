package vm

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
	"time"

	egress "github.com/finch-xu/clawdhome/lima-egress"
)

const (
	instance      = "cli-mvp"
	GuestTimeZone = "America/Los_Angeles"
)

type Session struct {
	ID, Command string
	// StartedAt 是 tmux 记录的会话创建时间；tmux 没给出可解析的时间戳时为零值。
	StartedAt time.Time
}

const sessionSeparator = "::"
const exitedSession = "@vm-exited"

// SessionUptime 返回会话已运行的时长，以及这个时长是否可用。StartedAt 为零值
// 时返回不可用；创建时间晚于 now（宿主与 guest 的时钟漂移）时钳制为 0，不返回
// 负数时长。
func SessionUptime(s Session, now time.Time) (time.Duration, bool) {
	if s.StartedAt.IsZero() {
		return 0, false
	}
	if d := now.Sub(s.StartedAt); d > 0 {
		return d, true
	}
	return 0, true
}

type VMInfo struct {
	Name, Status string
	HostAgentPID int
	Config       struct {
		TimeZone string
		Mounts   []VMMount
	}
}

// ExitedCommand 是已退出会话的命令占位值；界面按自己的语言显示它。
const ExitedCommand = exitedSession

// ParseSessions 解析 tmux list-sessions 的输出。每行形如
// `id::command::created_unix_timestamp`；第三段缺失或无法解析时 StartedAt 留
// 零值，该行照常保留。
func ParseSessions(lines string) []Session {
	var result []Session
	for _, line := range strings.Split(lines, "\n") {
		id, rest, ok := strings.Cut(line, sessionSeparator)
		if !ok || !strings.HasPrefix(id, "cli-") {
			continue
		}
		command, created, _ := strings.Cut(rest, sessionSeparator)
		s := Session{ID: id, Command: command}
		if sec, err := strconv.ParseInt(strings.TrimSpace(created), 10, 64); err == nil {
			s.StartedAt = time.Unix(sec, 0).UTC()
		}
		result = append(result, s)
	}
	return result
}

type Launcher struct {
	Root, Home, LimactlBin, ShareRoot string
}

type Settings struct {
	Project string
	Port    string
	// LocalePref 和 ThemePref 为空表示跟随系统。
	LocalePref string `json:"localePref,omitempty"`
	ThemePref  string `json:"themePref,omitempty"`
}

func (l Launcher) Settings() Settings {
	var cfg Settings
	raw, err := os.ReadFile(filepath.Join(l.Home, "launcher.json"))
	if err == nil {
		_ = json.Unmarshal(raw, &cfg)
	}
	if l.ShareRoot != "" {
		if _, err := l.ProjectInWorkspace(cfg.Project); err != nil {
			cfg.Project = l.Workspace()
		}
	}
	return cfg
}

func (l Launcher) SaveSettings(cfg Settings) error {
	if cfg.Port != "" && !ValidPort(cfg.Port) {
		return fmt.Errorf("Clash 端口须为 1–65535")
	}
	if cfg.Project != "" {
		if _, err := l.ProjectInWorkspace(cfg.Project); err != nil {
			return err
		}
	}
	raw, err := json.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("无法序列化设置: %w", err)
	}
	file, err := os.CreateTemp(l.Home, ".launcher-*.json")
	if err != nil {
		return fmt.Errorf("无法创建设置临时文件: %w", err)
	}
	defer os.Remove(file.Name())
	_, err = file.Write(raw)
	closeErr := file.Close()
	if err != nil {
		return fmt.Errorf("无法写入设置: %w", err)
	}
	if closeErr != nil {
		return fmt.Errorf("无法关闭设置文件: %w", closeErr)
	}
	if err := os.Rename(file.Name(), filepath.Join(l.Home, "launcher.json")); err != nil {
		return fmt.Errorf("无法保存设置: %w", err)
	}
	return nil
}

func ValidPort(port string) bool {
	n, err := strconv.ParseUint(port, 10, 16)
	return err == nil && n > 0
}

func (l Launcher) Cmd(ctx context.Context, port, path string, args ...string) *exec.Cmd {
	command := exec.CommandContext(ctx, path, args...)
	command.Env = append(os.Environ(), "LIMA_HOME="+l.Home, "MVP_LIMACTL="+l.LimactlBin,
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

func (l Launcher) InspectVM(ctx context.Context) (VMInfo, error) {
	out, err := commandOutput(l.Cmd(ctx, "", l.LimactlBin, "list", "--json"))
	if err != nil {
		return VMInfo{}, fmt.Errorf("无法读取 VM 状态: %w", err)
	}
	return parseVMInfo(string(out))
}

func (l Launcher) ShowDesktop(ctx context.Context) error {
	dialer := net.Dialer{}
	transport := &http.Transport{
		DisableKeepAlives: true,
		DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
			return dialer.DialContext(ctx, "unix", filepath.Join(l.Home, instance, "ha.sock"))
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

func parseVMInfo(lines string) (VMInfo, error) {
	for _, line := range strings.Split(strings.TrimSpace(lines), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		var vm VMInfo
		if err := json.Unmarshal([]byte(line), &vm); err != nil {
			return VMInfo{}, err
		}
		if vm.Name == instance {
			vm.Status = strings.ToLower(vm.Status)
			return vm, nil
		}
	}
	return VMInfo{}, nil
}

// prepare 用指针接收者：第一步的 ensureShareDirs 会把 shareRoot 解析成符号链接
// 指向的真实路径，这次更新必须对调用者可见，因为 guestProjectPath 用 shareRoot
// 算 VirtioFS 卷名的哈希。
func (l *Launcher) Prepare(ctx context.Context, cfg Settings) error {
	if err := l.EnsureShareDirs(); err != nil {
		return err
	}
	if _, err := l.ProjectInWorkspace(cfg.Project); err != nil {
		return err
	}
	mounts := l.sharedMounts()
	vm, err := l.InspectVM(ctx)
	if err != nil {
		return err
	}
	if vm.Status == "running" {
		if !vm.hasMounts(mounts) {
			return fmt.Errorf("请先停止 VM，再更新共享目录")
		}
		return l.ensureGuestSetup(ctx)
	}
	if egress.ReadRoute(l.Home) == egress.RouteClash && !ValidPort(cfg.Port) {
		return fmt.Errorf("请在设置中填写有效的 Clash 端口")
	}
	if vm.Status == "" {
		if _, err := commandOutput(l.Cmd(ctx, cfg.Port, filepath.Join(l.Root, "scripts", "create-vm.sh"), l.ShareRoot)); err != nil {
			return fmt.Errorf("创建 VM 失败: %w", err)
		}
	} else if !vm.hasMounts(mounts) || vm.Config.TimeZone != GuestTimeZone {
		encoded, err := json.Marshal(mounts)
		if err != nil {
			return fmt.Errorf("序列化共享目录配置失败: %w", err)
		}
		expression := ".mounts = " + string(encoded) + " | .timezone = " + strconv.Quote(GuestTimeZone)
		if _, err := commandOutput(l.Cmd(ctx, cfg.Port, l.LimactlBin, "edit", "--tty=false", "--set", expression, instance)); err != nil {
			return fmt.Errorf("更新 VM 共享目录或时区失败: %w", err)
		}
	}
	if err := l.SaveSettings(cfg); err != nil {
		return err
	}
	if _, err := commandOutput(l.Cmd(ctx, cfg.Port, l.LimactlBin, "start", "--tty=false", "--timeout=25m", instance)); err != nil {
		return fmt.Errorf("启动 VM 失败: %w", err)
	}
	return l.ensureGuestSetup(ctx)
}

func (l Launcher) ensureGuestSetup(ctx context.Context) error {
	if err := l.linkGuestShares(ctx); err != nil {
		return err
	}
	if _, err := commandOutput(l.Cmd(ctx, "", l.LimactlBin,
		vmShellArgs(`test -r "$HOME/.local/bin/cli-mvp.tmux.conf" &&
			case "$(/usr/bin/readlink /etc/localtime)" in */zoneinfo/`+GuestTimeZone+`) ;; *) exit 1 ;; esac && `+execTmux, "-V")...)); err == nil {
		return nil
	}
	_, err := commandOutput(l.Cmd(ctx, "", filepath.Join(l.Root, "scripts", "setup-guest.sh")))
	if err != nil {
		return fmt.Errorf("VM 初始化失败，请检查本地 tmux 构建与 guest 设置: %w", err)
	}
	return nil
}

const execTmux = `exec "$HOME/.local/bin/tmux" -L cli-mvp -f "$HOME/.local/bin/cli-mvp.tmux.conf" "$@"`

func vmShellArgs(script string, args ...string) []string {
	return append([]string{"shell", "--shell=/bin/sh", instance, "/bin/sh", "-c", script, "launcher"}, args...)
}

func (l Launcher) TmuxArgs(args ...string) []string {
	return vmShellArgs(execTmux, args...)
}

func (l Launcher) List(ctx context.Context) ([]Session, error) {
	result, err := commandOutput(l.Cmd(ctx, "", l.LimactlBin,
		l.TmuxArgs("list-sessions", "-F", "#{session_name}"+sessionSeparator+"#{?pane_dead,"+exitedSession+",#{pane_current_command}}"+sessionSeparator+"#{session_created}")...))
	if err != nil {
		if strings.Contains(err.Error(), "no server running") ||
			(strings.Contains(err.Error(), "error connecting to") && strings.Contains(err.Error(), "No such file")) {
			return nil, nil
		}
		return nil, fmt.Errorf("无法列出 VM 中的会话: %w", err)
	}
	return ParseSessions(string(result)), nil
}

// Lima's Darwin VZ guest mounts each VirtioFS share under its generated tag.
func guestProjectPath(hostPath string) string {
	hash := sha256.Sum256([]byte(hostPath + ":" + hostPath))
	return "/Volumes/My Shared Files/lima-" + hex.EncodeToString(hash[:8])
}

func (l Launcher) Create(ctx context.Context, project, command string) (string, error) {
	if strings.TrimSpace(command) == "" {
		return "", fmt.Errorf("请输入 CLI 命令")
	}
	rel, err := l.ProjectInWorkspace(project)
	if err != nil {
		return "", err
	}
	var token [4]byte
	if _, err := rand.Read(token[:]); err != nil {
		return "", fmt.Errorf("无法生成会话 ID: %w", err)
	}
	id := "cli-" + hex.EncodeToString(token[:])
	// Validate the share before tmux can fall back to the guest home directory.
	args := vmShellArgs(`cd "$HOME/workspace/$1" || { echo "项目目录未挂载" >&2; exit 1; }; shift; `+execTmux,
		rel, "new-session", "-d", "-s", id, command)
	if _, err := commandOutput(l.Cmd(ctx, "", l.LimactlBin, args...)); err != nil {
		return id, fmt.Errorf("无法在 VM 中启动会话: %w", err)
	}
	return id, nil
}

func (l Launcher) NetworkStatus(vm VMInfo) *egress.GatewayStatus {
	if vm.Status != "running" || vm.HostAgentPID == 0 {
		return nil
	}
	raw, err := os.ReadFile(filepath.Join(l.Home, instance, "mvp-network.json"))
	if err != nil {
		return nil
	}
	var state egress.GatewayStatus
	if json.Unmarshal(raw, &state) != nil || state.PID != vm.HostAgentPID || !state.Running {
		return nil
	}
	return &state
}

// Stop 停止虚拟机，guest 内运行的进程随之结束。
func (l Launcher) Stop(ctx context.Context) error {
	if _, err := commandOutput(l.Cmd(ctx, "", l.LimactlBin, "stop", instance)); err != nil {
		return fmt.Errorf("停止 VM 失败: %w", err)
	}
	return nil
}

// KillSession 结束 guest 内指定的 tmux 会话。
func (l Launcher) KillSession(ctx context.Context, id string) error {
	if _, err := commandOutput(l.Cmd(ctx, "", l.LimactlBin, l.TmuxArgs("kill-session", "-t", "="+id)...)); err != nil {
		return fmt.Errorf("无法结束会话 %s: %w", id, err)
	}
	return nil
}
