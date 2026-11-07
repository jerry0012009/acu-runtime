#!/bin/bash
set -euo pipefail
cd "$(dirname "$0")/.."
[[ "$(uname -s)" == Darwin ]] || { echo 'Build on macOS with Xcode command line tools.' >&2; exit 1; }
arch="$(go env GOARCH)"
[[ "$arch" == arm64 || "$arch" == amd64 ]]
version="$(sed -n 's/^const appVersion = "\([^"]*\)"/\1/p' update.go)"
out="$PWD/dist/darwin-$arch"
app="$out/ACU Runtime.app"
mkdir -p "$app/Contents/MacOS/kernel" "$app/Contents/Resources"
export CGO_ENABLED=1 MACOSX_DEPLOYMENT_TARGET=12.0
go build -tags desktop,production -trimpath -o "$app/Contents/MacOS/ACU Runtime" .
"$app/Contents/MacOS/ACU Runtime" --search-runtime-release-json > "$out/search-release.json"
python3 - "$out/search-release.json" <<'PY'
import hashlib, json, pathlib, sys, urllib.request
manifest = json.loads(pathlib.Path(sys.argv[1]).read_text())
url = f"https://github.com/astral-sh/uv/releases/download/{manifest['version']}/{manifest['name']}"
with urllib.request.urlopen(url, timeout=180) as response:
    data = response.read(128 * 1024 * 1024 + 1)
assert len(data) <= 128 * 1024 * 1024
assert hashlib.sha256(data).hexdigest() == manifest['sha256'], 'Search runtime SHA-256 mismatch'
pathlib.Path('web/search-runtime.archive').write_bytes(data)
PY
go build -tags desktop,production -trimpath -o "$app/Contents/MacOS/ACU Runtime" .
"$app/Contents/MacOS/ACU Runtime" --kernel-release-json > "$out/kernel-release.json"
python3 - "$out/kernel-release.json" "$app" "$version" <<'PY'
import hashlib, json, pathlib, plistlib, sys, urllib.request
manifest = json.loads(pathlib.Path(sys.argv[1]).read_text())
app = pathlib.Path(sys.argv[2])
kernel = app / 'Contents/MacOS/kernel/acu-kernel'
url = f"https://github.com/yetone/magpie/releases/download/{manifest['version']}/{manifest['name']}"
with urllib.request.urlopen(url, timeout=180) as response:
    data = response.read(128 * 1024 * 1024 + 1)
assert len(data) <= 128 * 1024 * 1024
assert hashlib.sha256(data).hexdigest() == manifest['sha256'], 'Kernel SHA-256 mismatch'
kernel.write_bytes(data)
kernel.chmod(0o755)
plist = dict(CFBundleName='ACU Runtime', CFBundleDisplayName='ACU Runtime',
             CFBundleIdentifier='com.acucompute.runtime', CFBundleExecutable='ACU Runtime',
             CFBundlePackageType='APPL', CFBundleShortVersionString=sys.argv[3],
             CFBundleVersion=sys.argv[3], CFBundleIconFile='AppIcon',
             LSMinimumSystemVersion='12.0', NSHighResolutionCapable=True,
             NSAppTransportSecurity={'NSAllowsLocalNetworking': True})
with (app / 'Contents/Info.plist').open('wb') as output:
    plistlib.dump(plist, output)
PY
cp web/THIRD-PARTY-NOTICES.txt "$app/Contents/Resources/"
icons="$out/AppIcon.iconset"
mkdir -p "$icons"
for size in 16 32 128 256 512; do
    sips -z "$size" "$size" acu-runtime-icon.png --out "$icons/icon_${size}x${size}.png" >/dev/null
    double=$((size * 2))
    sips -z "$double" "$double" acu-runtime-icon.png --out "$icons/icon_${size}x${size}@2x.png" >/dev/null
done
iconutil -c icns "$icons" -o "$app/Contents/Resources/AppIcon.icns"
# Ad-hoc signatures allow Apple Silicon execution; these are not Developer ID notarization.
codesign --force --sign - "$app/Contents/MacOS/kernel/acu-kernel"
codesign --force --sign - "$app"
codesign --verify --deep --strict --verbose=2 "$app"
plutil -lint "$app/Contents/Info.plist"
bash scripts/smoke-macos.sh "$app" "$version"
mkdir -p dist/release
ditto -c -k --sequesterRsrc --keepParent "$app" "dist/release/ACU-Runtime-$version-macos-$arch.app.zip"
stage="$out/dmg"
mkdir -p "$stage"
ditto "$app" "$stage/ACU Runtime.app"
ln -sfn /Applications "$stage/Applications"
hdiutil create -volname 'ACU Runtime' -srcfolder "$stage" -ov -format UDZO "dist/release/ACU-Runtime-$version-macos-$arch.dmg"
