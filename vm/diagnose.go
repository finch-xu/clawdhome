package vm

import (
	"context"
	"fmt"
	"strings"
)

// CheckSSH 在 guest 里跑一条空命令，用来区分「SSH 通道不可用」和「tmux 没起来」。
func (l Launcher) CheckSSH(ctx context.Context) error {
	if _, err := commandOutput(l.Cmd(ctx, "", l.LimactlBin, vmShellArgs("/usr/bin/true")...)); err != nil {
		return fmt.Errorf("无法通过 SSH 连接 VM: %w", err)
	}
	return nil
}

const networkSnapshotScript = `
printf '=== IPv4 sockets ===\n'; /usr/sbin/netstat -anv -f inet
printf '\n=== IPv6 sockets ===\n'; /usr/sbin/netstat -anv -f inet6
printf '\n=== Interfaces ===\n'; /sbin/ifconfig -a
printf '\n=== Routes ===\n'; /usr/sbin/netstat -rn
printf '\n=== DNS resolvers ===\n'; /usr/sbin/scutil --dns
`

const anthropicProbeScript = `
printf '=== Anthropic DNS A ===\n'
/usr/bin/dig +time=2 +tries=1 api.anthropic.com A
printf '\n=== Anthropic HTTPS ===\n'
/usr/bin/curl --noproxy '*' --max-time 8 -sS -o /dev/null -w 'HTTP %{http_code}\n' https://api.anthropic.com/ 2>&1
printf '\n'
`

// NetworkSnapshot 读取 guest 的套接字、网卡、路由和 DNS 配置快照。
// withAnthropic 为 true 时，额外检查 api.anthropic.com 的 DNS 和 HTTPS 可达性。
func (l Launcher) NetworkSnapshot(ctx context.Context, withAnthropic bool) (string, error) {
	script := networkSnapshotScript
	if withAnthropic {
		script = anthropicProbeScript + networkSnapshotScript
	}
	out, err := commandOutput(l.Cmd(ctx, "", l.LimactlBin, vmShellArgs(script)...))
	if err != nil {
		return "", withCode(ErrSnapshot, fmt.Errorf("无法读取 VM 网络信息: %w", err))
	}
	return strings.TrimSpace(string(out)), nil
}
