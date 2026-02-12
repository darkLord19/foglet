#!/usr/bin/env bash

set -euo pipefail

REPO="${FOG_GITHUB_REPO:-darkLord19/wtx}"
VERSION_TAG=""
INSTALL_DIR=""
DRY_RUN="false"
SKIP_VERIFY="false"
NO_SUDO="false"

usage() {
  cat <<'EOF'
Install wtx/fog/fogd/fogcloud from GitHub release artifacts (Linux only).

Usage:
  scripts/install-linux.sh [options]

Options:
  --version <tag>      Install a specific version tag (e.g. v0.2.0 or 0.2.0)
  --install-dir <dir>  Target bin directory (default: /usr/local/bin if writable, else ~/.local/bin)
  --repo <owner/repo>  GitHub repo (default: darkLord19/wtx)
  --no-sudo            Never attempt sudo install
  --skip-verify        Skip SHA-256 verification (not recommended)
  --dry-run            Print resolved actions without downloading/installing
  -h, --help           Show this help
EOF
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --version)
      VERSION_TAG="${2:-}"
      shift 2
      ;;
    --install-dir)
      INSTALL_DIR="${2:-}"
      shift 2
      ;;
    --repo)
      REPO="${2:-}"
      shift 2
      ;;
    --no-sudo)
      NO_SUDO="true"
      shift
      ;;
    --skip-verify)
      SKIP_VERIFY="true"
      shift
      ;;
    --dry-run)
      DRY_RUN="true"
      shift
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      echo "unknown argument: $1" >&2
      usage
      exit 1
      ;;
  esac
done

if [[ "$(uname -s)" != "Linux" ]]; then
  echo "this installer supports Linux only" >&2
  exit 1
fi

require_cmd() {
  local cmd="$1"
  if ! command -v "${cmd}" >/dev/null 2>&1; then
    echo "missing required command: ${cmd}" >&2
    exit 1
  fi
}

if command -v curl >/dev/null 2>&1; then
  HTTP_GET_TOOL="curl"
elif command -v wget >/dev/null 2>&1; then
  HTTP_GET_TOOL="wget"
else
  echo "missing download tool: curl or wget is required" >&2
  exit 1
fi

require_cmd tar
if [[ "${SKIP_VERIFY}" != "true" ]]; then
  if ! command -v sha256sum >/dev/null 2>&1 && ! command -v shasum >/dev/null 2>&1; then
    echo "missing checksum tool: sha256sum or shasum is required" >&2
    exit 1
  fi
fi

http_get() {
  local url="$1"
  local output="$2"
  if [[ "${HTTP_GET_TOOL}" == "curl" ]]; then
    curl -fsSL "${url}" -o "${output}"
  else
    wget -qO "${output}" "${url}"
  fi
}

http_get_stdout() {
  local url="$1"
  if [[ "${HTTP_GET_TOOL}" == "curl" ]]; then
    curl -fsSL "${url}"
  else
    wget -qO- "${url}"
  fi
}

detect_arch() {
  case "$(uname -m)" in
    x86_64|amd64) echo "amd64" ;;
    aarch64|arm64) echo "arm64" ;;
    *)
      echo "unsupported CPU architecture: $(uname -m)" >&2
      exit 1
      ;;
  esac
}

resolve_latest_tag() {
  local api_url="https://api.github.com/repos/${REPO}/releases/latest"
  local body
  body="$(http_get_stdout "${api_url}")"
  local tag
  tag="$(printf '%s' "${body}" | sed -n 's/.*"tag_name"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' | head -n1)"
  if [[ "${tag}" == "" ]]; then
    echo "failed to resolve latest release tag from ${api_url}" >&2
    exit 1
  fi
  echo "${tag}"
}

normalize_tag() {
  local raw="$1"
  if [[ "${raw}" == "" ]]; then
    echo "$(resolve_latest_tag)"
    return
  fi
  if [[ "${raw}" == v* ]]; then
    echo "${raw}"
    return
  fi
  echo "v${raw}"
}

resolve_install_dir() {
  if [[ "${INSTALL_DIR}" != "" ]]; then
    echo "${INSTALL_DIR}"
    return
  fi

  if [[ -w "/usr/local/bin" || ! -e "/usr/local/bin" ]]; then
    echo "/usr/local/bin"
    return
  fi
  echo "${HOME}/.local/bin"
}

checksum_of_file() {
  local file="$1"
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum "${file}" | awk '{print $1}'
    return
  fi
  shasum -a 256 "${file}" | awk '{print $1}'
}

install_binary() {
  local src="$1"
  local dst="$2"

  if [[ "${NO_SUDO}" == "true" || -w "${dst}" ]]; then
    install -m 0755 "${src}" "${dst}/"
    return
  fi

  if ! command -v sudo >/dev/null 2>&1; then
    echo "cannot write to ${dst} and sudo is unavailable" >&2
    exit 1
  fi
  sudo install -m 0755 "${src}" "${dst}/"
}

ARCH="$(detect_arch)"
TAG="$(normalize_tag "${VERSION_TAG}")"
VERSION="${TAG#v}"
OS="linux"
INSTALL_TO="$(resolve_install_dir)"

ARCHIVE="wtx_${VERSION}_${OS}_${ARCH}.tar.gz"
CHECKSUMS="wtx_${VERSION}_checksums.txt"
BASE_URL="https://github.com/${REPO}/releases/download/${TAG}"
ARCHIVE_URL="${BASE_URL}/${ARCHIVE}"
CHECKSUMS_URL="${BASE_URL}/${CHECKSUMS}"

echo "repo:        ${REPO}"
echo "version:     ${TAG}"
echo "platform:    ${OS}/${ARCH}"
echo "install dir: ${INSTALL_TO}"
echo "archive:     ${ARCHIVE_URL}"

if [[ "${DRY_RUN}" == "true" ]]; then
  echo "dry-run mode: no download or install performed"
  exit 0
fi

mkdir -p "${INSTALL_TO}"

WORK_DIR="$(mktemp -d)"
trap 'rm -rf "${WORK_DIR}"' EXIT

ARCHIVE_PATH="${WORK_DIR}/${ARCHIVE}"
CHECKSUMS_PATH="${WORK_DIR}/${CHECKSUMS}"

echo "downloading archive..."
http_get "${ARCHIVE_URL}" "${ARCHIVE_PATH}"

if [[ "${SKIP_VERIFY}" != "true" ]]; then
  echo "downloading checksums..."
  http_get "${CHECKSUMS_URL}" "${CHECKSUMS_PATH}"
  EXPECTED="$(awk -v f="${ARCHIVE}" '$2 == f {print $1}' "${CHECKSUMS_PATH}")"
  if [[ "${EXPECTED}" == "" ]]; then
    echo "checksum entry missing for ${ARCHIVE}" >&2
    exit 1
  fi
  ACTUAL="$(checksum_of_file "${ARCHIVE_PATH}")"
  if [[ "${ACTUAL}" != "${EXPECTED}" ]]; then
    echo "checksum mismatch for ${ARCHIVE}" >&2
    exit 1
  fi
fi

echo "extracting archive..."
tar -xzf "${ARCHIVE_PATH}" -C "${WORK_DIR}"
STAGE_DIR="${WORK_DIR}/wtx_${VERSION}_${OS}_${ARCH}"
if [[ ! -d "${STAGE_DIR}" ]]; then
  echo "unexpected archive layout (missing ${STAGE_DIR})" >&2
  exit 1
fi

install_binary "${STAGE_DIR}/wtx" "${INSTALL_TO}"
install_binary "${STAGE_DIR}/fog" "${INSTALL_TO}"
install_binary "${STAGE_DIR}/fogd" "${INSTALL_TO}"
install_binary "${STAGE_DIR}/fogcloud" "${INSTALL_TO}"

echo "installed successfully:"
echo "  ${INSTALL_TO}/wtx"
echo "  ${INSTALL_TO}/fog"
echo "  ${INSTALL_TO}/fogd"
echo "  ${INSTALL_TO}/fogcloud"

if [[ "${INSTALL_TO}" != */bin ]]; then
  echo "note: add ${INSTALL_TO} to PATH"
fi
