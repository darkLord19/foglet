#!/usr/bin/env bash

set -euo pipefail

usage() {
  cat <<'EOF'
Usage:
  scripts/release/build-fogapp-appimage.sh <version-tag> [output-dir]

Examples:
  scripts/release/build-fogapp-appimage.sh v0.2.0 dist
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

if [[ "$(uname -s)" != "Linux" ]]; then
  echo "fogapp AppImage build currently supports Linux only" >&2
  exit 1
fi

if ! command -v appimagetool >/dev/null 2>&1; then
  echo "appimagetool is required" >&2
  exit 1
fi

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
mkdir -p "${OUTPUT_DIR}"
OUTPUT_DIR="$(cd "${OUTPUT_DIR}" && pwd)"
WORK_DIR="$(mktemp -d)"
trap 'rm -rf "${WORK_DIR}"' EXIT

APPDIR="${WORK_DIR}/FogApp.AppDir"
mkdir -p "${APPDIR}/usr/bin" "${APPDIR}/usr/share/applications" "${APPDIR}/usr/share/icons/hicolor/256x256/apps"

echo "building fogapp desktop binary (linux/amd64)..."
(
  cd "${ROOT_DIR}"
  GOOS=linux GOARCH=amd64 CGO_ENABLED=1 \
    go build -tags "desktop,production,webkit2_41" -ldflags "-s -w -X main.version=${VERSION_TAG}" \
      -o "${APPDIR}/usr/bin/fogapp" ./cmd/fogapp
)

cat > "${APPDIR}/AppRun" <<'EOF'
#!/usr/bin/env sh
HERE="$(dirname "$(readlink -f "$0")")"
exec "${HERE}/usr/bin/fogapp" "$@"
EOF
chmod +x "${APPDIR}/AppRun"

cat > "${APPDIR}/fogapp.desktop" <<'EOF'
[Desktop Entry]
Type=Application
Name=Fog
Comment=Turn your local machine into cloud agents
Exec=fogapp
Icon=fogapp
Terminal=false
Categories=Development;
EOF
cp "${APPDIR}/fogapp.desktop" "${APPDIR}/usr/share/applications/fogapp.desktop"

cp "${ROOT_DIR}/cmd/fogapp/build/appicon.png" "${APPDIR}/usr/share/icons/hicolor/256x256/apps/fogapp.png"
cp "${APPDIR}/usr/share/icons/hicolor/256x256/apps/fogapp.png" "${APPDIR}/fogapp.png"

APPIMAGE_NAME="fogapp_${VERSION}_linux_amd64.AppImage"
APPIMAGE_PATH="${OUTPUT_DIR}/${APPIMAGE_NAME}"

echo "packaging ${APPIMAGE_NAME}..."
ARCH=x86_64 appimagetool --appimage-extract-and-run "${APPDIR}" "${APPIMAGE_PATH}"

if command -v sha256sum >/dev/null 2>&1; then
  sha256sum "${APPIMAGE_PATH}" | awk '{print $1 "  '"${APPIMAGE_NAME}"'"}' > "${APPIMAGE_PATH}.sha256"
elif command -v shasum >/dev/null 2>&1; then
  shasum -a 256 "${APPIMAGE_PATH}" | awk '{print $1 "  '"${APPIMAGE_NAME}"'"}' > "${APPIMAGE_PATH}.sha256"
fi

echo "fogapp AppImage written to ${APPIMAGE_PATH}"
