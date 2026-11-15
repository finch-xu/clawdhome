#!/bin/bash
set -euo pipefail

root=$(cd "$(dirname "$0")/.." && pwd)
export LIMA_HOME="${LIMA_HOME:-$(cat "$root/.mvp-cache/instance-home")}"
limactl="$root/.mvp-cache/lima/bin/limactl-mvp"
tmux="$root/.mvp-cache/tmux-3.6b/tmux"
test -x "$tmux"
"$limactl" shell cli-mvp /bin/sh -c '
    set -e
    mkdir -p "$HOME/.local/bin"
    tmp=$(mktemp "$HOME/.local/bin/tmux.XXXXXX")
    trap '\''rm -f "$tmp"'\'' EXIT
    cat > "$tmp"
    chmod 700 "$tmp"
    mv -f "$tmp" "$HOME/.local/bin/tmux"
    "$HOME/.local/bin/tmux" -V
' < "$tmux"
"$limactl" shell cli-mvp /bin/sh -c '
    set -e
    /usr/bin/defaults write -g AppleLanguages -array en-US
    /usr/bin/defaults write -g AppleLocale -string en_US
    /usr/bin/sudo -S -p "" /usr/sbin/systemsetup -settimezone America/Los_Angeles < "$HOME/password"
    /usr/bin/defaults read -g AppleLanguages
    /usr/bin/defaults read -g AppleLocale
    zone=$(/usr/bin/readlink /etc/localtime)
    printf "%s\n" "$zone"
    test "${zone##*/zoneinfo/}" = America/Los_Angeles
    printf "%s\n" "set-option -g remain-on-exit on" > "$HOME/.local/bin/cli-mvp.tmux.conf"
'
