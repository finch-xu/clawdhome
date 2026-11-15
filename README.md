# clawdhome

在隔离的 macOS 虚拟机里运行 CLI 工具的 macOS 原生启动器。

一个 MyGo 窗口连接同一台 Apple Silicon Lima macOS guest。CLI 在 guest 内的 tmux 会话中运行。VM 外网和宿主机到 SSH 的本地通道独立处理。

## 本机验证

在项目根目录运行 `./scripts/run-app.sh`。当前验证实例是 `cli-mvp`，Lima 状态目录记录在 `.mvp-cache/instance-home`。退出启动器时 guest 中的会话继续运行，重开后点对应会话即可接回。短命令退出后仍可查看输出。停止 VM 会结束其中的进程。

VM 启动时默认隐藏系统桌面。需要操作系统界面时，点「显示 VM 桌面」。关闭桌面窗口后，VM 和 CLI 会话继续运行，按钮可以再次打开同一个桌面。

「设置」里可以选择项目目录、保存本机 Clash 端口和测试 SOCKS5 握手。测试端口时不发送目标地址。运行中保存的端口在下次启动 VM 时生效。「状态详情」分别显示 VM、SSH、Clash、Google 探测和实际出口策略。

点「Claude Code」或按 `⌘⇧C`，在 VM 的项目目录启动 `claude`。点「终端」或按 `⌘⇧T`，在同一目录打开新的 zsh 会话。Claude Code 需要安装在 VM 内。

## 共享目录

每台 VM 有两处可写的 VirtioFS 共享。当前实例使用以下路径。

| 宿主机路径 | VM 路径 | 内容 |
| --- | --- | --- |
| `~/macOS-CLI-Launcher/cli-mvp/claude/` | `~/.claude/` | 这台 VM 的 Claude Code 个人配置与会话历史 |
| `~/macOS-CLI-Launcher/cli-mvp/workspace/` | `~/workspace/` | 项目和代码，双方改动实时可见 |

把项目放在宿主机 `workspace/` 的子目录中，例如 `workspace/my-project/`。在「设置 → 选择目录」选中它，之后新开的会话从该项目启动。切换项目无需重启 VM，原终端会话保留各自的目录。

Claude Code 二进制安装在 VM 本地的 `~/.local/share/claude/`，入口是 `~/.local/bin/claude`。测试实例已离线安装 Claude Code `2.1.325`，可运行 `~/.local/bin/claude --version`。登录和模型请求需要 VM 外网可用；系统钥匙串和 `~/.claude.json` 由 VM 管理。Claude 配置目录可能含历史或备用登录凭据，请放在项目仓库之外。

在测试 VM 运行时执行 `./scripts/smoke-vm.sh`，可用临时测试文件验证两处目录的双向读写。

## 网络

离线验证可以选择「断开公网」。自动化测试用本地 SOCKS5 和 TLS 服务模拟 Google，验证 DoH、TCP 半关闭和已有连接撤销。Clash 模式的端口需指向宿主机 IPv4 回环上的 SOCKS5 或 Mixed 专用入站，规则应绑定无 `DIRECT` 回退的代理策略。网关从关闭状态启动，开放后按选定模式转发 IPv4 TCP 和 DoH。公网 UDP、IPv6、宿主机通用回环服务和局域网被拒绝。

「网络连接」默认使用 **系统路由**，沿用宿主机的网络出口。可以手动切换为 **Clash** 或 **断开公网**，手动选择会保留。切换时撤销旧公网连接，重新通过探测后才打开新出口。宿主机到 VM 的本地 SSH 保持可用。

「刷新连接」显示 VM 的 TCP/UDP 套接字、网卡、路由和 DNS 配置快照。「测试 Anthropic」额外在 VM 内检查 `api.anthropic.com` 的 DNS 和 HTTPS。启用 Clash 后，握手成功只代表本地代理端口可用，仍需等 Google 探测开放防火墙。DoH 默认走 Google，发生传输或 HTTP 失败时通过同一出口尝试 Cloudflare。

## 重建

测试版本为 Lima 2.3.0、macOS guest 26.6.2、MyGo 0.9.0 和 tmux 3.6b。

1. 将 Lima 2.3.0 的 Darwin arm64 官方发布归档解压到 `.mvp-cache/lima/`。将 v2.3.0 源码和同版 vendor 归档分别放为 `.mvp-cache/lima-src-v2.3.0.tar.gz` 和 `.mvp-cache/lima-vendor-v2.3.0.tar.gz`。运行 `scripts/build-lima.sh`，脚本生成带虚拟化签名的 `limactl-mvp`。
2. 将 Apple 26.6.2 恢复镜像 `UniversalMac_26.6.2_25G83_Restore.ipsw` 放入 `.mvp-cache/`。Lima 读取并校验配置固定的 SHA-256。用 `mktemp -d /tmp/cli-mvp.XXXXXX > .mvp-cache/instance-home` 准备短路径。创建实例时设置 `LIMA_HOME` 为文件中的目录，`MVP_LIMACTL` 为上一步的绝对路径，然后执行 `./scripts/create-vm.sh`。启动实例时还需设置 `MVP_CLASH_SOCKS5=127.0.0.1:端口`。
3. 将 tmux 3.6b 与 libevent 2.1.12 的上游发布源码归档放入 `.mvp-cache/`，文件名分别为 `tmux-3.6b.tar.gz` 和 `libevent-2.1.12-stable.tar.gz`。运行 `scripts/build-tmux.sh`。首次从启动器运行 CLI 时自动经 SSH 离线安装 tmux，并设置 `en-US`、`en_US` 和 `America/Los_Angeles`。也可在 VM 启动后运行 `scripts/setup-guest.sh` 手动配置。
4. 用 Go 1.27.3 执行 `go build -o .mvp-cache/launcher ./cmd/launcher`。MyGo 0.9.0 指定的 arm64 `libghostty-vt.dylib` 放在 `.mvp-cache/mygo-native/` 后运行 `scripts/run-app.sh`。

[Lima 2.3.0](https://github.com/lima-vm/lima/releases/tag/v2.3.0) · [MyGo 终端插件](https://mygo.egoist.dev/docs/plugins/terminal)
