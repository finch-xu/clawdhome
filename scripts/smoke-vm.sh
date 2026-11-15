#!/bin/bash
set -euo pipefail

root=$(cd "$(dirname "$0")/.." && pwd)
export LIMA_HOME="${LIMA_HOME:-$(cat "$root/.mvp-cache/instance-home")}"
limactl="$root/.mvp-cache/lima/bin/limactl-mvp"
shares="${1:-$HOME/macOS-CLI-Launcher/cli-mvp}"
fixture=$(mktemp -d "$shares/workspace/mvp-smoke.XXXXXX")
config_fixture=$(mktemp -d "$shares/claude/mvp-smoke.XXXXXX")
trap 'rm -f "$fixture"/host.txt "$fixture"/guest-a.txt "$fixture"/guest-b.txt "$fixture"/done.txt "$config_fixture"/host.txt "$config_fixture"/guest.txt; rmdir "$fixture" "$config_fixture"' EXIT
printf 'from-host\n' > "$fixture/host.txt"
guest_home=$("$limactl" shell --shell=/bin/sh cli-mvp /bin/sh -c 'printf "%s" "$HOME"')
guest_fixture="$guest_home/workspace/${fixture##*/}"

"$limactl" shell --workdir "$guest_fixture" cli-mvp sh -c '
    test "$(cat host.txt)" = from-host || exit 1
    printf "from-guest-a\n" > guest-a.txt
    for i in $(seq 1 100); do
      if test -f done.txt; then exit 0; fi
      sleep 0.1
    done
    exit 1
' &
first=$!
"$limactl" shell --workdir "$guest_fixture" cli-mvp sh -c '
    for i in $(seq 1 100); do
      test -f guest-a.txt && break
      sleep 0.1
    done
    test "$(cat guest-a.txt)" = from-guest-a || exit 1
    printf "from-guest-b\n" > guest-b.txt
'
test "$(cat "$fixture/guest-b.txt")" = from-guest-b
kill -0 "$first"
touch "$fixture/done.txt"
wait "$first"
printf 'from-host\n' > "$config_fixture/host.txt"
"$limactl" shell --workdir "$guest_home/.claude/${config_fixture##*/}" cli-mvp sh -c '
    test "$(cat host.txt)" = from-host || exit 1
    printf "from-guest\n" > guest.txt
'
test "$(cat "$config_fixture/guest.txt")" = from-guest
"$limactl" shell cli-mvp sw_vers -productVersion
"$limactl" shell cli-mvp readlink /etc/localtime
"$limactl" shell cli-mvp defaults read -g AppleLocale
echo 'Concurrent guest SSH and bidirectional workspace and Claude configuration shares passed'
