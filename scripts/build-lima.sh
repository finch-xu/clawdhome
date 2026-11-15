#!/bin/bash
set -euo pipefail

root=$(cd "$(dirname "$0")/.." && pwd)
cache="$root/.mvp-cache"
src="$cache/lima-2.3.0"
export GOMODCACHE="$cache/go-mod-cache" GOCACHE="$cache/go-build-cache"
export GOTOOLCHAIN=go1.26.8

if [[ ! -d "$src/vendor" ]]; then
  tar -xzf "$cache/lima-src-v2.3.0.tar.gz" -C "$cache"
  tar -xzf "$cache/lima-vendor-v2.3.0.tar.gz" -C "$src" vendor
fi

cd "$src"
if [[ ! -e .mvp-egress-patched ]]; then
  patch -p1 --forward --batch --dry-run < "$root/patches/lima-2.3.0-egress.patch"
  patch -p1 --forward --batch < "$root/patches/lima-2.3.0-egress.patch"
  touch .mvp-egress-patched
fi
if [[ ! -e .mvp-desktop-patched ]]; then
  patch -p1 --forward --batch --dry-run < "$root/patches/lima-2.3.0-desktop.patch"
  patch -p1 --forward --batch < "$root/patches/lima-2.3.0-desktop.patch"
  touch .mvp-desktop-patched
fi
xnet="$(go mod download -json golang.org/x/net@v0.62.0 | python3 -c 'import json,sys; print(json.load(sys.stdin)["Dir"])')"
if [[ ! -d vendor/golang.org/x/net/proxy ]]; then
  mkdir -p vendor/golang.org/x/net/internal
  cp -R "$xnet/proxy" vendor/golang.org/x/net/
  cp -R "$xnet/internal/socks" vendor/golang.org/x/net/internal/
  sed -i '' '/^golang.org\/x\/net\/html$/a\
golang.org/x/net/internal/socks\
golang.org/x/net/proxy
' vendor/modules.txt
fi
mkdir -p pkg/networks/usernet/egress
cp "$root"/lima-egress/*.go pkg/networks/usernet/egress/
go test -mod=vendor ./pkg/networks/usernet/egress
go build -mod=vendor -ldflags '-X github.com/lima-vm/lima/v2/pkg/version.Version=2.3.0' -o "$cache/lima/bin/limactl-mvp" ./cmd/limactl
codesign -f -s - --entitlements vz.entitlements "$cache/lima/bin/limactl-mvp"
"$cache/lima/bin/limactl-mvp" --version
