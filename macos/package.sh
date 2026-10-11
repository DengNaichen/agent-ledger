#!/bin/bash
set -euo pipefail

root="$(cd "$(dirname "$0")" && pwd)"
output="$root/dist"
app="$output/InfTask Preview.app"

swift build --package-path "$root" -c release
bin="$(swift build --package-path "$root" -c release --show-bin-path)"
mkdir -p "$output"
staging="$(mktemp -d "$output/.package.XXXXXX")"
trap 'rm -rf "$staging"' EXIT
bundle="$staging/InfTask Preview.app"
mkdir -p "$bundle/Contents/MacOS" "$bundle/Contents/Resources" "$staging/AppIcon.iconset"
cp "$root/Info.plist" "$bundle/Contents/Info.plist"
cp "$bin/InfTaskPreview" "$bundle/Contents/MacOS/InfTaskPreview"
# SwiftPM resolves Bundle.module inside the application resource directory.
cp -R "$bin/InfTaskPreview_InfTaskPreview.bundle" "$bundle/Contents/Resources/"
cp "$root/../LICENSE" "$bundle/Contents/Resources/InfTask-LICENSE.txt"
cp "$root/README.md" "$bundle/Contents/Resources/Preview-README.md"
xcrun swift "$root/make-icon.swift" "$staging/AppIcon.iconset"
iconutil -c icns "$staging/AppIcon.iconset" -o "$bundle/Contents/Resources/AppIcon.icns"
plutil -lint "$bundle/Contents/Info.plist"
codesign --force --sign - "$bundle"
codesign --verify --deep --strict "$bundle"
# Replace only this script's generated app, never an installed application.
rm -rf "$app"
mv "$bundle" "$app"
ditto -c -k --sequesterRsrc --keepParent "$app" "$output/InfTask-Preview-macOS-$(uname -m).zip"
printf '\nApp: %s\nZip: %s\n' "$app" "$output/InfTask-Preview-macOS-$(uname -m).zip"
