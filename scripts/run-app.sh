#!/bin/bash
set -euo pipefail
root=$(cd "$(dirname "$0")/.." && pwd)
bundle="$root/.mvp-cache/CLI Launcher.app"
test -x "$root/.mvp-cache/launcher"
mkdir -p "$bundle/Contents/MacOS"
cp "$root/.mvp-cache/launcher" "$bundle/Contents/MacOS/launcher.new"
mv -f "$bundle/Contents/MacOS/launcher.new" "$bundle/Contents/MacOS/launcher"
cat > "$bundle/Contents/Info.plist" <<'PLIST'
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "https://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>CFBundleIdentifier</key><string>local.mac-cli-launcher.mvp</string>
  <key>CFBundleName</key><string>CLI Launcher</string>
  <key>CFBundleExecutable</key><string>launcher</string>
  <key>CFBundlePackageType</key><string>APPL</string>
  <key>CFBundleVersion</key><string>1</string>
</dict>
</plist>
PLIST
exec open -a "$bundle" --args -workspace "$root"
