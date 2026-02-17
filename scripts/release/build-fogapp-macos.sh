#!/usr/bin/env bash

set -euo pipefail

usage() {
  cat <<'EOF'
Usage:
  scripts/release/build-fogapp-macos.sh <version-tag> [output-dir]

Examples:
  scripts/release/build-fogapp-macos.sh v0.2.0 dist
EOF
}

if [[ "${1:-}" == "" ]]; then
  usage
  exit 1
fi

VERSION_TAG="$1"
OUTPUT_DIR="${2:-dist}"
VERSION="${VERSION_TAG#v}"

if [[ "${VERSION}" == "" ]]; then
  echo "invalid version tag: ${VERSION_TAG}" >&2
  exit 1
fi

if [[ "$(uname -s)" != "Darwin" ]]; then
  echo "fogapp macOS DMG build requires macOS" >&2
  exit 1
fi

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
mkdir -p "${OUTPUT_DIR}"
OUTPUT_DIR="$(cd "${OUTPUT_DIR}" && pwd)"
WORK_DIR="$(mktemp -d)"
trap 'rm -rf "${WORK_DIR}"' EXIT

ARCH="$(uname -m)"
case "${ARCH}" in
  x86_64) GOARCH="amd64" ;;
  arm64)  GOARCH="arm64" ;;
  *)
    echo "unsupported architecture: ${ARCH}" >&2
    exit 1
    ;;
esac

APP_NAME="Fog.app"
APP_DIR="${WORK_DIR}/${APP_NAME}"
mkdir -p "${APP_DIR}/Contents/MacOS" "${APP_DIR}/Contents/Resources"

echo "building fogapp desktop binary (darwin/${GOARCH})..."
(
  cd "${ROOT_DIR}"
  CGO_ENABLED=1 GOOS=darwin GOARCH="${GOARCH}" \
  CGO_LDFLAGS="-framework UniformTypeIdentifiers" \
    go build -tags "desktop,production" -ldflags "-s -w -X main.version=${VERSION_TAG}" \
      -o "${APP_DIR}/Contents/MacOS/fogapp" ./cmd/fogapp
)

# Generate Info.plist with release version
cat > "${APP_DIR}/Contents/Info.plist" <<PLIST
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
    <dict>
        <key>CFBundlePackageType</key>
        <string>APPL</string>
        <key>CFBundleName</key>
        <string>Fog</string>
        <key>CFBundleExecutable</key>
        <string>fogapp</string>
        <key>CFBundleIdentifier</key>
        <string>com.wails.fogapp</string>
        <key>CFBundleVersion</key>
        <string>${VERSION}</string>
        <key>CFBundleGetInfoString</key>
        <string>Built using Wails (https://wails.io)</string>
        <key>CFBundleShortVersionString</key>
        <string>${VERSION}</string>
        <key>CFBundleIconFile</key>
        <string>iconfile</string>
        <key>LSMinimumSystemVersion</key>
        <string>10.13.0</string>
        <key>NSHighResolutionCapable</key>
        <string>true</string>
        <key>NSHumanReadableCopyright</key>
        <string>Copyright © $(date +%Y) Fog</string>
        <key>NSAppTransportSecurity</key>
        <dict>
            <key>NSAllowsLocalNetworking</key>
            <true/>
        </dict>
    </dict>
</plist>
PLIST

# Convert appicon.png to icns using sips + iconutil (available on macOS)
APP_ICON="${ROOT_DIR}/cmd/fogapp/build/appicon.png"
if [[ -f "${APP_ICON}" ]]; then
  ICONSET_DIR="${WORK_DIR}/fogapp.iconset"
  mkdir -p "${ICONSET_DIR}"
  # Ensure source is proper PNG format
  ICON_SRC="${WORK_DIR}/appicon_src.png"
  sips -s format png "${APP_ICON}" --out "${ICON_SRC}" >/dev/null 2>&1
  sips -z 16 16     "${ICON_SRC}" --out "${ICONSET_DIR}/icon_16x16.png"      >/dev/null 2>&1
  sips -z 32 32     "${ICON_SRC}" --out "${ICONSET_DIR}/icon_16x16@2x.png"   >/dev/null 2>&1
  sips -z 32 32     "${ICON_SRC}" --out "${ICONSET_DIR}/icon_32x32.png"      >/dev/null 2>&1
  sips -z 64 64     "${ICON_SRC}" --out "${ICONSET_DIR}/icon_32x32@2x.png"   >/dev/null 2>&1
  sips -z 128 128   "${ICON_SRC}" --out "${ICONSET_DIR}/icon_128x128.png"    >/dev/null 2>&1
  sips -z 256 256   "${ICON_SRC}" --out "${ICONSET_DIR}/icon_128x128@2x.png" >/dev/null 2>&1
  sips -z 256 256   "${ICON_SRC}" --out "${ICONSET_DIR}/icon_256x256.png"    >/dev/null 2>&1
  sips -z 512 512   "${ICON_SRC}" --out "${ICONSET_DIR}/icon_256x256@2x.png" >/dev/null 2>&1
  sips -z 512 512   "${ICON_SRC}" --out "${ICONSET_DIR}/icon_512x512.png"    >/dev/null 2>&1
  sips -z 1024 1024 "${ICON_SRC}" --out "${ICONSET_DIR}/icon_512x512@2x.png" >/dev/null 2>&1
  iconutil -c icns "${ICONSET_DIR}" -o "${APP_DIR}/Contents/Resources/iconfile.icns"
  echo "icon converted from appicon.png"
else
  echo "warning: appicon.png not found, building without icon" >&2
fi

# --- Ad-hoc code sign ---
echo "ad-hoc signing ${APP_NAME}..."
codesign --force --deep --sign - "${APP_DIR}"

# --- Build DMG ---
DMG_NAME="fogapp_${VERSION}_darwin_${GOARCH}.dmg"
DMG_PATH="${OUTPUT_DIR}/${DMG_NAME}"
DMG_STAGING="${WORK_DIR}/dmg_staging"
mkdir -p "${DMG_STAGING}"

cp -R "${APP_DIR}" "${DMG_STAGING}/"

# Create a symlink to /Applications for drag-to-install
ln -s /Applications "${DMG_STAGING}/Applications"

echo "packaging ${DMG_NAME}..."
hdiutil create -volname "Fog" \
  -srcfolder "${DMG_STAGING}" \
  -ov -format UDZO \
  "${DMG_PATH}"

# Checksum
if command -v shasum >/dev/null 2>&1; then
  shasum -a 256 "${DMG_PATH}" | awk '{print $1 "  '"${DMG_NAME}"'"}' > "${DMG_PATH}.sha256"
elif command -v sha256sum >/dev/null 2>&1; then
  sha256sum "${DMG_PATH}" | awk '{print $1 "  '"${DMG_NAME}"'"}' > "${DMG_PATH}.sha256"
fi

echo "fogapp macOS DMG written to ${DMG_PATH}"
