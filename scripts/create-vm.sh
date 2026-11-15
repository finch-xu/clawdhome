#!/bin/bash
set -euo pipefail

root=$(cd "$(dirname "$0")/.." && pwd)
shares="${1:-$HOME/macOS-CLI-Launcher/cli-mvp}"
export LIMA_HOME="${LIMA_HOME:-$root/.mvp-cache/lima-home}"
limactl="${MVP_LIMACTL:-$root/.mvp-cache/lima/bin/limactl-mvp}"
restore="$root/.mvp-cache/UniversalMac_26.6.2_25G83_Restore.ipsw"
config="$LIMA_HOME/macos-mvp.yaml"

mkdir -p "$LIMA_HOME"
umask 077
mkdir -p "$shares/workspace" "$shares/claude"
test ! -L "$shares/workspace" && test ! -L "$shares/claude"
shares=$(cd "$shares" && pwd -P)
python3 - "$config" "$shares" "$restore" <<'PY'
import json
import pathlib
import sys

config_file, shares, restore = sys.argv[1:]
config = {
    "minimumLimaVersion": "2.3.0",
    "os": "Darwin",
    "arch": "aarch64",
    "vmType": "vz",
    "images": [{"location": restore, "arch": "aarch64", "digest": "sha256:885503b7f4b06609e9a512f2befd40f59730640a3f1233e3892d60affdd51c95"}],
    "cpus": 4,
    "memory": "6GiB",
    "disk": "48GiB",
    "timezone": "America/Los_Angeles",
    "video": {"display": "default"},
    "mountType": "virtiofs",
    "mounts": [{"location": str(pathlib.Path(shares) / name), "writable": True}
               for name in ("workspace", "claude")],
    "networkOpts": {"hostLoopback": False},
    "hostResolver": {"enabled": False},
    "propagateProxyEnv": False,
    "osOpts": {"Darwin": {"suppressFirstLoginSetup": True}},
}
pathlib.Path(config_file).write_text(json.dumps(config, indent=2) + "\n")
PY

if [[ "${2:-}" == "--validate" ]]; then
  exec "$limactl" validate "$config"
fi
exec "$limactl" create --tty=false --name=cli-mvp "$config"
