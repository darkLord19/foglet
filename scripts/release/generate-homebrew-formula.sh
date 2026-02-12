#!/usr/bin/env bash

set -euo pipefail

usage() {
  cat <<'EOF'
Usage:
  scripts/release/generate-homebrew-formula.sh <version-tag> <checksums-file> [github-repo]

Examples:
  scripts/release/generate-homebrew-formula.sh v0.2.0 dist/wtx_0.2.0_checksums.txt
  scripts/release/generate-homebrew-formula.sh v0.2.0 dist/wtx_0.2.0_checksums.txt darkLord19/wtx
EOF
}

if [[ "${1:-}" == "" || "${2:-}" == "" ]]; then
  usage
  exit 1
fi

VERSION_TAG="$1"
CHECKSUMS_FILE="$2"
GITHUB_REPO="${3:-darkLord19/wtx}"
VERSION="${VERSION_TAG#v}"

if [[ ! -f "${CHECKSUMS_FILE}" ]]; then
  echo "checksums file not found: ${CHECKSUMS_FILE}" >&2
  exit 1
fi

checksum_for() {
  local artifact="$1"
  local sum
  sum="$(awk -v f="${artifact}" '$2 == f {print $1}' "${CHECKSUMS_FILE}")"
  if [[ "${sum}" == "" ]]; then
    echo "missing checksum for ${artifact}" >&2
    exit 1
  fi
  echo "${sum}"
}

DARWIN_AMD64="wtx_${VERSION}_darwin_amd64.tar.gz"
DARWIN_ARM64="wtx_${VERSION}_darwin_arm64.tar.gz"
LINUX_AMD64="wtx_${VERSION}_linux_amd64.tar.gz"
LINUX_ARM64="wtx_${VERSION}_linux_arm64.tar.gz"

DARWIN_AMD64_SHA="$(checksum_for "${DARWIN_AMD64}")"
DARWIN_ARM64_SHA="$(checksum_for "${DARWIN_ARM64}")"
LINUX_AMD64_SHA="$(checksum_for "${LINUX_AMD64}")"
LINUX_ARM64_SHA="$(checksum_for "${LINUX_ARM64}")"

BASE_URL="https://github.com/${GITHUB_REPO}/releases/download/${VERSION_TAG}"

cat <<EOF
class Wtx < Formula
  desc "Turn your local machine into cloud agents"
  homepage "https://github.com/${GITHUB_REPO}"
  version "${VERSION}"
  license "MIT"

  on_macos do
    if Hardware::CPU.arm?
      url "${BASE_URL}/${DARWIN_ARM64}"
      sha256 "${DARWIN_ARM64_SHA}"
    else
      url "${BASE_URL}/${DARWIN_AMD64}"
      sha256 "${DARWIN_AMD64_SHA}"
    end
  end

  on_linux do
    if Hardware::CPU.arm?
      url "${BASE_URL}/${LINUX_ARM64}"
      sha256 "${LINUX_ARM64_SHA}"
    else
      url "${BASE_URL}/${LINUX_AMD64}"
      sha256 "${LINUX_AMD64_SHA}"
    end
  end

  def install
    stage = Dir["wtx_*"].find { |d| File.directory?(d) }
    odie "release archive layout changed" if stage.nil?

    bin.install "#{stage}/wtx"
    bin.install "#{stage}/fog"
    bin.install "#{stage}/fogd"
    bash_completion.install "#{stage}/completions/wtx.bash" => "wtx"
    zsh_completion.install "#{stage}/completions/wtx.zsh" => "_wtx"
  end

  test do
    assert_match "version", shell_output("#{bin}/wtx version")
    assert_match "version", shell_output("#{bin}/fog version")
    assert_match "version", shell_output("#{bin}/fogd version")
  end
end
EOF
