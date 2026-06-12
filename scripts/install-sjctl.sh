#!/usr/bin/env bash
# Download and install the sjctl binary from GitHub Releases.
#
# Used by the jobs-* skills when sjctl is not already on PATH. Safe to re-run:
# it skips the download if the installed version already matches the target.
#
#   SJCTL_REPO     owner/repo to pull releases from (default below)
#   SJCTL_VERSION  release tag to install, e.g. v0.1.0 (default: latest)
#   SJCTL_BIN_DIR  install directory (default: ~/.solid-jobs-skills/bin)
set -euo pipefail

REPO="${SJCTL_REPO:-solid-company/solid-jobs-skills}"
BIN_DIR="${SJCTL_BIN_DIR:-$HOME/.solid-jobs-skills/bin}"
TARGET_VERSION="${SJCTL_VERSION:-latest}"

err() { echo "install-sjctl: $*" >&2; exit 1; }

# --- detect platform ---------------------------------------------------------
os="$(uname -s)"
case "$os" in
  Linux)  os="linux" ;;
  Darwin) os="darwin" ;;
  *) err "unsupported OS: $os" ;;
esac

arch="$(uname -m)"
case "$arch" in
  x86_64|amd64) arch="amd64" ;;
  arm64|aarch64) arch="arm64" ;;
  *) err "unsupported arch: $arch" ;;
esac

# --- resolve target tag ------------------------------------------------------
api="https://api.github.com/repos/${REPO}/releases"
if [ "$TARGET_VERSION" = "latest" ]; then
  tag="$(curl -fsSL "${api}/latest" | grep -m1 '"tag_name"' | cut -d '"' -f4)"
  [ -n "$tag" ] || err "could not resolve latest release for $REPO"
else
  tag="$TARGET_VERSION"
fi

bin_path="${BIN_DIR}/sjctl"

# --- skip if already installed at the target version -------------------------
if [ -x "$bin_path" ]; then
  current="$("$bin_path" version 2>/dev/null || true)"
  if [ "$current" = "$tag" ]; then
    echo "$bin_path"
    exit 0
  fi
fi

# --- download, verify, extract ----------------------------------------------
asset="sjctl_${os}_${arch}.tar.gz"
base="https://github.com/${REPO}/releases/download/${tag}"
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

curl -fsSL "${base}/${asset}" -o "${tmp}/${asset}" \
  || err "download failed: ${base}/${asset}"
curl -fsSL "${base}/checksums.txt" -o "${tmp}/checksums.txt" \
  || err "download failed: ${base}/checksums.txt"

( cd "$tmp" && grep " ${asset}\$" checksums.txt | sha256sum -c - >/dev/null ) \
  || err "checksum verification failed for ${asset}"

tar -xzf "${tmp}/${asset}" -C "$tmp" sjctl || err "extract failed"

mkdir -p "$BIN_DIR"
install -m 0755 "${tmp}/sjctl" "$bin_path" 2>/dev/null \
  || { cp "${tmp}/sjctl" "$bin_path" && chmod 0755 "$bin_path"; }

echo "$bin_path"
