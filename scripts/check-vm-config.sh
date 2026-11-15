#!/bin/bash
set -euo pipefail
root=$(cd "$(dirname "$0")/.." && pwd)
export LIMA_HOME="$root/.mvp-cache/config-check"
if ! "$root/scripts/create-vm.sh" "$LIMA_HOME/test-shares" --validate; then
  echo 'Expected the MVP instance configuration to pass Lima validation' >&2
  exit 1
fi
echo 'MVP instance configuration is valid'
