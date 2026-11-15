package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/plugins/terminal"
	"github.com/egoist/mygo/ui"

	egress "github.com/finch-xu/clawdhome/lima-egress"
	"github.com/finch-xu/clawdhome/vm"
)

// 各操作的超时预算。启动 VM 要下载/引导 guest，所以给得最宽；停止和结束会话
// 只是发一条命令，给得最紧。
const (
	viewRefreshInterval = time.Second      // view 自身的重绘节奏
	pollInterval        = 2 * time.Second  // 两次状态轮询之间的最小间隔
	pollTimeout         = 5 * time.Second  // 单次状态轮询的超时
	showDesktopTimeout  = 3 * time.Second  // 打开 VM 桌面窗口
	runTimeout          = 26 * time.Minute // 准备并启动 VM，再开一个会话
	stopTimeout         = time.Minute      // 停止 VM
	killTimeout         = 5 * time.Second  // 结束一个 tmux 会话
)

const terminalCommand = "/bin/zsh"

const claudeGuestScript = `
if command -v claude >/dev/null 2>&1; then
    exec claude
fi
if test -x "$HOME/.local/bin/claude"; then
    exec "$HOME/.local/bin/claude"
fi
printf "%s\n" "Claude Code is not installed in this VM. Install claude in the VM, then try again." >&2
exit 127
`

const claudeCommand = terminalCommand + " -lic '" + claudeGuestScript + "'"

// windowUpdater 把状态写回 UI 线程。*mygo.Window 满足这个接口；测试用一个
// 同步执行回调的替身替换它，以便断言后台工作完成后的状态。
type windowUpdater interface {
	Update(func())
}

// appearance 套用外观和语言偏好。这两件事要走 MyGo 的全局 API，而那些调用会
// 把工作派发到主线程并等待完成——测试进程里没有事件循环，直接调会死锁，所以
// 做成接口由测试注入空实现。
type appearance interface {
	SetTheme(source string)
	SetLocale(tag string)
	// SystemLocale 返回系统当前选用的语言标签。
	SystemLocale() string
}

// mygoAppearance 是跑在真实应用里的实现。
type mygoAppearance struct{}

func (mygoAppearance) SetTheme(source string) { mygo.Theme.SetSource(toThemeSource(source)) }
func (mygoAppearance) SetLocale(tag string)   { mygo.App.SetLocale(localePrefTag(tag)) }
func (mygoAppearance) SystemLocale() string   { return mygo.App.Locale() }

// app 的所有字段只在 UI 线程读写：view 渲染回调、按钮点击回调，或
// window.Update(func(){...}) 回调内部。后台 goroutine 只读取启动前拷贝的
// 快照值，结果一律通过 window.Update 写回。
type app struct {
	launcher vm.Launcher
	// window 用于状态回写，测试里可替换；realWindow 持有真实窗口，
	// 供 OnClosed/IsDestroyed/对话框父窗口这些窗口专属能力使用。
	window           windowUpdater
	realWindow       *mygo.Window
	appearance       appearance
	locale           locale
	cfg              vm.Settings
	command          string
	message          string
	state            string
	sessions         []vm.Session
	terms            map[string]*terminal.Terminal
	selected         string
	busy             bool
	ssh              string
	network          *egress.GatewayStatus
	polling          bool
	vmRevision       uint64
	snapshotRevision uint64
	lastPoll         time.Time
	settingsOpen     bool
	detailsOpen      bool
	draft            vm.Settings
	settingsMessage  string
	portTestMessage  string
	portTesting      bool
	networkOpen      bool
	networkBusy      bool
	networkText      string
	networkMessage   string
	preferredRoute   egress.Route
}

func main() {
	workspace := flag.String("workspace", defaultWorkspace(), "MVP workspace")
	flag.Parse()
	root, err := filepath.Abs(*workspace)
	if err != nil {
		log.Fatal(err)
	}
	if os.Getenv("MYGO_GHOSTTY_VT") == "" {
		if p := ghosttyLibraryPath(root); p != "" {
			if err := os.Setenv("MYGO_GHOSTTY_VT", p); err != nil {
				log.Fatal(err)
			}
		}
	}
	home, err := os.ReadFile(filepath.Join(root, ".mvp-cache", "instance-home"))
	if err != nil {
		log.Fatal(err)
	}
	shareRoot, err := vm.DefaultShareRoot()
	if err != nil {
		log.Fatal(err)
	}
	l := vm.Launcher{Root: root, Home: strings.TrimSpace(string(home)), ShareRoot: shareRoot,
		LimactlBin: filepath.Join(root, ".mvp-cache", "lima", "bin", "limactl-mvp")}
	if err := l.EnsureShareDirs(); err != nil {
		log.Fatal(err)
	}
	a := &app{launcher: l, cfg: l.Settings(), appearance: mygoAppearance{}, command: terminalCommand, state: "unknown", ssh: vm.SSHChecking, terms: make(map[string]*terminal.Terminal)}
	a.preferredRoute = egress.ReadRoute(l.Home)

	// 语言和外观都在 App.Run 之前设置：英语是默认语言，没有保存过偏好时由
	// MyGo 按系统语言选择；保存过则覆盖。
	mygo.App.SetLanguages(string(localeEN), string(localeZhHans))
	// 空串让 MyGo 清掉它自己存的覆盖，回到跟随系统；非法值也走这条路。
	a.appearance.SetLocale(a.cfg.LocalePref)
	a.appearance.SetTheme(a.cfg.ThemePref)

	mygo.App.WhenReady(func() {
		a.locale = toLocale(mygo.App.Locale())
		mygo.App.SetMenu(mygo.NewMenu([]*mygo.MenuItem{
			{Role: mygo.RoleAppMenu}, {Role: mygo.RoleEditMenu},
			{Label: tr("menu.sessions", a.locale), Submenu: []*mygo.MenuItem{
				{Label: tr("menu.startClaude", a.locale), Accelerator: "CmdOrCtrl+Shift+C", Click: a.launchFromMenu(claudeCommand)},
				{Label: tr("menu.openTerminal", a.locale), Accelerator: "CmdOrCtrl+Shift+T", Click: a.launchFromMenu(terminalCommand)},
			}},
		}))
		a.realWindow = mygo.NewWindow(mygo.WindowOptions{
			Title: tr("app.title", a.locale), Width: 920, Height: 620, Content: ui.View(a.view),
		})
		a.window = a.realWindow
		// 这两个回调要在窗口建好、a.window 赋值之后才注册：它们的唯一作用是
		// 驱动重绘，而在窗口存在之前触发会对 nil 的 a.window 调 Update。
		mygo.App.OnLocaleChanged(func(tag string) {
			a.window.Update(func() { a.locale = toLocale(tag) })
		})
		mygo.Theme.OnUpdated(func() {
			a.window.Update(func() {}) // 空回调触发重绘，让 view 用新主题取色
		})
		a.realWindow.OnClosed(func() {
			for id, term := range a.terms {
				delete(a.terms, id)
				_ = term.Close()
			}
		})
	})
	if err := mygo.App.Run(); err != nil {
		log.Fatal(err)
	}
}

// workspaceMarker 是判定「这个目录是项目根」的依据：它记录当前验证实例的
// Lima 状态目录，只有项目根下才有。用 marker 而不是数目录层数，是因为 bundle
// 的位置取决于谁构建它——scripts/run-app.sh 放在 .mvp-cache/ 下，mygo dev 放在
// .mygo/dev/<goos>-<goarch>/ 下，深一层。
var workspaceMarker = filepath.Join(".mvp-cache", "instance-home")

// ghosttyLibraryPath 返回仓库里手工放置的 libghostty-vt.dylib，没有则返回空
// 串。调用方只在非空时设 MYGO_GHOSTTY_VT：那是查找链的第一位，而框架不校验
// 文件是否存在（ffi.Library.Path 只看变量非空），所以指向一个不存在的路径会
// 让终端插件直接加载失败，而不是回退到 app resources 或用户缓存。
func ghosttyLibraryPath(root string) string {
	p := filepath.Join(root, ".mvp-cache", "mygo-native", "libghostty-vt.dylib")
	if info, err := os.Stat(p); err == nil && info.Mode().IsRegular() {
		return p
	}
	return ""
}

func defaultWorkspace() string {
	exe, err := os.Executable()
	if err != nil {
		return "."
	}
	return findWorkspace(filepath.Dir(exe))
}

// findWorkspace 从 startDir 逐级向上，返回第一个含 workspaceMarker 的目录；
// 一直到文件系统顶都没有时返回 "."，由调用方按工作目录解析（go run 的场景）。
func findWorkspace(startDir string) string {
	for dir := startDir; ; {
		if info, err := os.Stat(filepath.Join(dir, workspaceMarker)); err == nil && info.Mode().IsRegular() {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "."
		}
		dir = parent
	}
}

func (a *app) view(c *ui.Context) {
	c.After(viewRefreshInterval)
	if time.Since(a.lastPoll) >= pollInterval {
		a.poll()
	}
	ui.Column(c).Fill().Padding(10).Gap(6).Children(func() {
		ui.Row(c.Key("project")).Gap(6).Children(func() {
			project := a.cfg.Project
			if project == "" {
				project = tr("topbar.noProject", a.locale)
			}
			ui.Text(c, project).Grow(1).SingleLine().Ellipsis("…")
			if ui.Button(c, tr("settings.title", a.locale)).Disabled(a.busy).Clicked() {
				a.settingsOpen = !a.settingsOpen
				if a.settingsOpen {
					a.draft, a.settingsMessage, a.portTestMessage = a.cfg, "", ""
				}
			}
		})
		if a.settingsOpen {
			a.settingsView(c.Key("settings"))
		}
		ui.Row(c.Key("controls")).Gap(6).Children(func() {
			ui.TextInput(c, &a.command).Grow(1).
				Placeholder(tr("session.commandHint", a.locale)).
				Label(tr("session.commandLabel", a.locale)).Disabled(a.busy)
			if ui.PrimaryButton(c, tr("session.run", a.locale)).Disabled(a.busy).Clicked() {
				a.run(a.command)
			}
			if ui.Button(c, tr("session.claudeCode", a.locale)).Disabled(a.busy).Clicked() {
				a.run(claudeCommand)
			}
			if ui.Button(c, tr("session.terminal", a.locale)).Disabled(a.busy).Clicked() {
				a.run(terminalCommand)
			}
			if ui.Button(c, tr("topbar.showDesktop", a.locale)).Disabled(a.busy || a.state != "running").Clicked() {
				a.showDesktop()
			}
			if ui.Button(c, tr("topbar.stopVM", a.locale)).Disabled(a.busy || a.state != "running").Clicked() {
				a.stop()
			}
		})
		if a.message != "" {
			ui.Text(c, a.message).SingleLine().Ellipsis("…")
		}
		ui.Row(c.Key("sessions")).Gap(5).Children(func() {
			for _, s := range a.sessions {
				if ui.Button(c.Key(s.ID), a.sessionLabel(s)).Disabled(a.busy).Clicked() {
					a.attach(s.ID)
				}
			}
			if a.selected != "" && ui.Button(c, tr("session.end", a.locale)).Disabled(a.busy).Clicked() {
				a.kill()
			}
		})
		// 这三条分支的主区域都带 Grow(1)，由它吸收剩余高度，才能把下面的状态条
		// 压到窗口底部。新增视图分支时必须同样带 Grow(1)，否则状态条会浮到内容
		// 下方而不是贴着窗口下边缘。
		if a.networkOpen {
			a.networkPanel(c.Key("network-panel"))
		} else if term := a.terms[a.selected]; term != nil {
			ui.Box(c.Key("terminal-shell")).Grow(1).MinHeight(0).Children(func() {
				terminal.View(c.Key(a.selected), term).Fill().AutoFocus()
			})
		} else {
			ui.Box(c.Key("empty")).Grow(1).Center().Children(func() {
				ui.Text(c, tr("session.empty", a.locale))
			})
		}
		// 状态条常驻在窗口底部，详情在它上方展开。
		a.statusBar(c.Key("status"))
	})
}

func (a *app) showDesktop() {
	launcher := a.launcher
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), showDesktopTimeout)
		defer cancel()
		if err := launcher.ShowDesktop(ctx); err != nil {
			a.window.Update(func() { a.message = err.Error() })
		}
	}()
}

func (a *app) chooseDirectory() {
	workspace, message := a.launcher.Workspace(), tr("dialog.chooseProject", a.locale)
	go func() {
		dirs, err := mygo.Dialog.Open(mygo.OpenDialogOptions{Parent: a.realWindow, Directory: true,
			DefaultPath: workspace, Message: message})
		a.window.Update(func() {
			if err != nil {
				a.settingsMessage = err.Error()
			} else if a.settingsOpen && len(dirs) > 0 {
				a.draft.Project = dirs[0]
			}
		})
	}()
}

func (a *app) poll() {
	if a.busy || a.polling {
		return
	}
	a.lastPoll, a.polling = time.Now(), true
	// 后台读 launcher 的快照，不碰 a.launcher，与 app 字段的 UI 线程约定一致。
	launcher, revision := a.launcher, a.vmRevision
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), pollTimeout)
		defer cancel()
		info, vmErr := launcher.InspectVM(ctx)
		network := launcher.NetworkStatus(info)
		var sessions []vm.Session
		var sessionsErr error
		ssh := vm.SSHNotRunning
		if info.Status == "running" {
			sessions, sessionsErr = launcher.List(ctx)
			ssh = vm.SSHAvailable
			if sessionsErr != nil {
				if err := launcher.CheckSSH(ctx); err != nil {
					ssh = vm.SSHUnavailable
				}
			}
		}
		a.window.Update(func() {
			a.polling = false
			if revision != a.vmRevision {
				a.poll()
				return
			}
			if a.busy {
				return
			}
			if vmErr != nil {
				a.state, a.ssh, a.network = "unknown", vm.SSHUnknown, nil
				return
			}
			if a.state == "running" && info.Status != "running" {
				a.clearNetworkSnapshot()
			}
			a.state, a.ssh, a.network = info.Status, ssh, network
			if sessionsErr == nil {
				a.sessions = sessions
			}
		})
	}()
}

func (a *app) launchFromMenu(command string) func(*mygo.MenuItem, *mygo.Window) {
	return func(_ *mygo.MenuItem, _ *mygo.Window) {
		if a.realWindow != nil && !a.realWindow.IsDestroyed() {
			a.window.Update(func() { a.run(command) })
		}
	}
}

func (a *app) run(command string) {
	if a.busy {
		return
	}
	cfg := a.cfg
	a.vmRevision++
	a.busy, a.message = true, tr("session.starting", a.locale)
	previous := a.state
	if a.state != "running" {
		a.state = "starting"
	}
	// 后台在 launcher 的副本上工作，不直接写 a.launcher：Prepare 会把 ShareRoot
	// 解析成符号链接指向的真实路径，这次更新随结果一起经 window.Update 写回。
	launcher := a.launcher
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), runTimeout)
		defer cancel()
		err := launcher.Prepare(ctx, cfg)
		var id string
		var sessions []vm.Session
		if err == nil {
			id, err = launcher.Create(ctx, cfg.Project, command)
		}
		if err == nil {
			sessions, _ = launcher.List(ctx)
		}
		a.window.Update(func() {
			a.launcher = launcher
			a.finish(err)
			a.state = previous
			if err == nil {
				a.state, a.sessions, a.ssh = "running", sessions, vm.SSHAvailable
				a.attach(id)
			}
			a.poll()
		})
	}()
}

func (a *app) attach(id string) {
	if a.terms[id] == nil {
		var term *terminal.Terminal
		var err error
		term, err = terminal.New(terminal.Options{
			Command: append([]string{a.launcher.LimactlBin}, a.launcher.TmuxArgs("attach-session", "-t", "="+id)...),
			Env:     []string{"LIMA_HOME=" + a.launcher.Home},
			OnExit: func(code int) {
				a.window.Update(func() { a.terminalExited(id, term, code) })
			},
		})
		if err != nil {
			a.message = err.Error()
			return
		}
		a.terms[id] = term
	}
	a.selected = id
	a.networkOpen = false
}

func (a *app) terminalExited(id string, term *terminal.Terminal, code int) {
	if term == nil || a.terms[id] != term {
		return
	}
	delete(a.terms, id)
	go term.Close()
	if a.selected == id {
		a.selected = ""
		if code != 0 {
			a.message = tr("session.reattach", a.locale)
		}
	}
	a.poll()
}

func (a *app) stop() {
	a.vmRevision++
	a.busy, a.message = true, tr("vm.stopping", a.locale)
	a.state = "stopping"
	launcher := a.launcher
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), stopTimeout)
		defer cancel()
		err := launcher.Stop(ctx)
		a.window.Update(func() {
			a.finish(err)
			a.state = "running"
			if err == nil {
				a.state, a.selected, a.sessions = "stopped", "", nil
				a.network, a.ssh = nil, vm.SSHNotRunning
				a.clearNetworkSnapshot()
			}
			a.poll()
		})
	}()
}

func (a *app) kill() {
	id := a.selected
	a.vmRevision++
	a.busy = true
	launcher := a.launcher
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), killTimeout)
		defer cancel()
		err := launcher.KillSession(ctx, id)
		a.window.Update(func() { a.finish(err); a.poll() })
	}()
}

func (a *app) finish(err error) {
	a.busy, a.message = false, ""
	if err != nil {
		a.message = err.Error()
	}
}

// sessionLabel 组合会话按钮的文案：命令名、短 ID，以及运行时长或退出码。
func (a *app) sessionLabel(s vm.Session) string {
	command := s.Command
	if s.Exited {
		command = tr("session.exited", a.locale)
	}
	label := command + " " + s.ID[4:]
	if s.Exited {
		return label + " · " + fmt.Sprintf(tr("session.exitCode", a.locale), s.ExitCode)
	}
	if d, ok := vm.SessionUptime(s, time.Now()); ok {
		return label + " · " + formatUptime(d, a.locale)
	}
	return label + " · " + tr("session.uptimeUnknown", a.locale)
}
