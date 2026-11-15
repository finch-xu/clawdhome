#!/bin/bash
set -euo pipefail

root=$(cd "$(dirname "$0")/.." && pwd)
cache="$root/.mvp-cache"
export MACOSX_DEPLOYMENT_TARGET=13.0

tar -xzf "$cache/libevent-2.1.12-stable.tar.gz" -C "$cache"
tar -xzf "$cache/tmux-3.6b.tar.gz" -C "$cache"
cd "$cache/libevent-2.1.12-stable"
# The macOS 27 SDK detects pipe2, which is absent in the macOS 26 guest.
# Use libevent's portable pipe implementation in the guest binary.
ac_cv_func_pipe2=no ./configure --prefix="$cache/event-install" --disable-openssl --disable-shared --enable-static --disable-samples > "$cache/libevent-build.log" 2>&1
make -j 6 >> "$cache/libevent-build.log" 2>&1
make install >> "$cache/libevent-build.log" 2>&1

cd "$cache/tmux-3.6b"
LIBEVENT_CFLAGS="-I$cache/event-install/include" LIBEVENT_LIBS="$cache/event-install/lib/libevent.a" ./configure --disable-utf8proc > "$cache/tmux-build.log" 2>&1
rm -f tmux
make -j 6 >> "$cache/tmux-build.log" 2>&1
./tmux -V
