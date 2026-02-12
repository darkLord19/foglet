#!/usr/bin/env bash

set -euo pipefail

usage() {
  cat <<'EOF'
Usage:
  scripts/release/build-artifacts.sh <version-tag> [output-dir]

Examples:
  scripts/release/build-artifacts.sh v0.2.0 dist
  scripts/release/build-artifacts.sh 0.2.0 dist
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

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
mkdir -p "${OUTPUT_DIR}"
OUTPUT_DIR="$(cd "${OUTPUT_DIR}" && pwd)"
WORK_DIR="$(mktemp -d)"
trap 'rm -rf "${WORK_DIR}"' EXIT

LDFLAGS="-s -w -X main.version=${VERSION_TAG}"
GOFLAGS_COMMON=(-buildvcs=false -trimpath -ldflags "${LDFLAGS}")

: "${GOCACHE:=/tmp/go-build}"
mkdir -p "${GOCACHE}"
export GOCACHE
TARGETS=(
  "darwin amd64"
  "darwin arm64"
  "linux amd64"
  "linux arm64"
)

checksum_file() {
  local file="$1"
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum "${file}" | awk '{print $1}'
    return
  fi
  if command -v shasum >/dev/null 2>&1; then
    shasum -a 256 "${file}" | awk '{print $1}'
    return
  fi
  echo "missing checksum tool (sha256sum or shasum)" >&2
  return 1
}

echo "building release artifacts for ${VERSION_TAG}"
for target in "${TARGETS[@]}"; do
  OS="$(awk '{print $1}' <<<"${target}")"
  ARCH="$(awk '{print $2}' <<<"${target}")"
  STAGE_DIR="${WORK_DIR}/wtx_${VERSION}_${OS}_${ARCH}"
  mkdir -p "${STAGE_DIR}/completions"

  echo "  -> ${OS}/${ARCH}"
  (
    cd "${ROOT_DIR}"
    GOOS="${OS}" GOARCH="${ARCH}" CGO_ENABLED=0 \
      go build "${GOFLAGS_COMMON[@]}" -o "${STAGE_DIR}/wtx" ./cmd/wtx
    GOOS="${OS}" GOARCH="${ARCH}" CGO_ENABLED=0 \
      go build "${GOFLAGS_COMMON[@]}" -o "${STAGE_DIR}/fog" ./cmd/fog
    GOOS="${OS}" GOARCH="${ARCH}" CGO_ENABLED=0 \
      go build "${GOFLAGS_COMMON[@]}" -o "${STAGE_DIR}/fogd" ./cmd/fogd
    GOOS="${OS}" GOARCH="${ARCH}" CGO_ENABLED=0 \
      go build "${GOFLAGS_COMMON[@]}" -o "${STAGE_DIR}/fogcloud" ./cmd/fogcloud
  )

  cp "${ROOT_DIR}/LICENSE" "${STAGE_DIR}/LICENSE"
  cp "${ROOT_DIR}/README.md" "${STAGE_DIR}/README.md"
  cp "${ROOT_DIR}/scripts/completions/wtx.bash" "${STAGE_DIR}/completions/wtx.bash"
  cp "${ROOT_DIR}/scripts/completions/wtx.zsh" "${STAGE_DIR}/completions/wtx.zsh"

  ARCHIVE_NAME="wtx_${VERSION}_${OS}_${ARCH}.tar.gz"
  tar -C "${WORK_DIR}" -czf "${OUTPUT_DIR}/${ARCHIVE_NAME}" "wtx_${VERSION}_${OS}_${ARCH}"
done

if [[ "${BUILD_FOGAPP_APPIMAGE:-false}" == "true" ]]; then
  echo "building fogapp AppImage artifact"
  chmod +x "${ROOT_DIR}/scripts/release/build-fogapp-appimage.sh"
  "${ROOT_DIR}/scripts/release/build-fogapp-appimage.sh" "${VERSION_TAG}" "${OUTPUT_DIR}"
fi

CHECKSUM_FILE="${OUTPUT_DIR}/wtx_${VERSION}_checksums.txt"
: > "${CHECKSUM_FILE}"
for archive in "${OUTPUT_DIR}"/wtx_"${VERSION}"_*.tar.gz; do
  base="$(basename "${archive}")"
  sum="$(checksum_file "${archive}")"
  echo "${sum}  ${base}" >> "${CHECKSUM_FILE}"
done

for appimage in "${OUTPUT_DIR}"/fogapp_"${VERSION}"_*.AppImage; do
  if [[ ! -f "${appimage}" ]]; then
    continue
  fi
  base="$(basename "${appimage}")"
  sum="$(checksum_file "${appimage}")"
  echo "${sum}  ${base}" >> "${CHECKSUM_FILE}"
done

echo "artifacts written to ${OUTPUT_DIR}"
echo "checksums: ${CHECKSUM_FILE}"
