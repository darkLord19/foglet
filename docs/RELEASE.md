# Release Guide

This project ships release artifacts for:
- `linux/amd64`
- `linux/arm64`
- `darwin/amd64`
- `darwin/arm64`

Each archive includes:
- `wtx`
- `fog`
- `fogd`
- `fogcloud`
- shell completions

Optional desktop artifact:
- `fogapp_<version>_linux_amd64.AppImage` (built when `BUILD_FOGAPP_APPIMAGE=true`)

## Local release build

```bash
scripts/release/build-artifacts.sh v0.2.0 dist

# or via Makefile
make release-artifacts RELEASE_TAG=v0.2.0

# include fogapp AppImage in dist/
BUILD_FOGAPP_APPIMAGE=true scripts/release/build-artifacts.sh v0.2.0 dist

# AppImage-only helper
make release-fogapp-appimage RELEASE_TAG=v0.2.0
```

Outputs:
- `dist/wtx_<version>_<os>_<arch>.tar.gz`
- `dist/wtx_<version>_checksums.txt`
- optional `dist/fogapp_<version>_linux_amd64.AppImage`
- optional `dist/fogapp_<version>_linux_amd64.AppImage.sha256`

## Generate Homebrew formula

```bash
scripts/release/generate-homebrew-formula.sh \
  v0.2.0 \
  dist/wtx_0.2.0_checksums.txt \
  darkLord19/wtx > dist/wtx.rb

# or via Makefile
make release-formula RELEASE_TAG=v0.2.0
```

Formula behavior:
- selects correct asset URL by OS/CPU architecture
- installs `wtx`, `fog`, `fogd`, and `fogcloud`
- installs `wtx` completions

## GitHub release wiring

Workflow:
- `.github/workflows/release.yml`
- Trigger: push tag matching `v*`

Workflow steps:
1. Build release artifacts (`scripts/release/build-artifacts.sh`)
2. Generate Homebrew formula (`scripts/release/generate-homebrew-formula.sh`)
3. Upload release assets:
   - tar.gz archives
   - fogapp AppImage + AppImage sha256
   - checksums file
   - generated `wtx.rb`

Desktop packaging notes:
- CI installs `libgtk-3-dev`, `libwebkit2gtk-4.1-dev`, `libayatana-appindicator3-dev`, and `appimagetool`.
- AppImage build currently targets Linux `amd64`.

## Optional automatic Homebrew tap update

If these repository secrets are configured, the release workflow updates the tap repo automatically:
- `HOMEBREW_TAP_REPO` (example: `your-org/homebrew-tap`)
- `HOMEBREW_TAP_PAT` (token with push access)

The workflow writes formula file to:
- `Formula/wtx.rb` in the tap repository.

## Linux installer

Use release artifacts directly:

```bash
scripts/install-linux.sh
scripts/install-linux.sh --version v0.2.0
scripts/install-linux.sh --dry-run
```

Security defaults:
- SHA-256 verification is enabled by default.
- `--skip-verify` exists for controlled debugging only.
