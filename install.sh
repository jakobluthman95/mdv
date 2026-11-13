#!/bin/sh
# Install mdv from the latest GitHub release:
#   curl -fsSL https://raw.githubusercontent.com/jakobluthman95/mdv/main/install.sh | sh
#
# Options (environment variables):
#   MDV_VERSION      release to install, e.g. v0.1.0 (default: latest)
#   MDV_INSTALL_DIR  where to put the binary (default: ~/.local/bin)
#   MDV_DOWNLOAD_URL where to download the archives from (for testing)
set -eu

REPO="jakobluthman95/mdv"
INSTALL_DIR="${MDV_INSTALL_DIR:-$HOME/.local/bin}"

say() { printf '%s\n' "$*"; }
fail() { printf 'mdv install: %s\n' "$*" >&2; exit 1; }

command -v curl >/dev/null 2>&1 || fail "curl is required"
command -v tar >/dev/null 2>&1 || fail "tar is required"

case "$(uname -s)" in
  Darwin) os=darwin ;;
  Linux) os=linux ;;
  *) fail "unsupported OS $(uname -s); build from source instead (see the README)" ;;
esac
case "$(uname -m)" in
  x86_64 | amd64) arch=amd64 ;;
  arm64 | aarch64) arch=arm64 ;;
  *) fail "unsupported CPU $(uname -m); build from source instead (see the README)" ;;
esac

tag="${MDV_VERSION:-latest}"
if [ "$tag" = latest ]; then
  # /releases/latest redirects to /releases/tag/<tag>
  url=$(curl -fsSLI -o /dev/null -w '%{url_effective}' "https://github.com/$REPO/releases/latest") ||
    fail "could not reach GitHub"
  case "$url" in
    */releases/tag/*) tag="${url##*/}" ;;
    *) fail "no releases published yet; build from source instead (see the README)" ;;
  esac
fi
case "$tag" in v*) ;; *) tag="v$tag" ;; esac

base="${MDV_DOWNLOAD_URL:-https://github.com/$REPO/releases/download/$tag}"
archive="mdv_${tag#v}_${os}_${arch}.tar.gz"

tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT

say "Downloading mdv $tag ($os/$arch)..."
curl -fsSL -o "$tmp/$archive" "$base/$archive" || fail "download failed: $base/$archive"
curl -fsSL -o "$tmp/checksums.txt" "$base/checksums.txt" || fail "download failed: $base/checksums.txt"

expected=$(grep " $archive\$" "$tmp/checksums.txt" | cut -d' ' -f1)
[ -n "$expected" ] || fail "$archive is not listed in checksums.txt"
if command -v sha256sum >/dev/null 2>&1; then
  actual=$(sha256sum "$tmp/$archive" | cut -d' ' -f1)
else
  actual=$(shasum -a 256 "$tmp/$archive" | cut -d' ' -f1)
fi
[ "$expected" = "$actual" ] || fail "checksum mismatch for $archive"

tar -xzf "$tmp/$archive" -C "$tmp" mdv
mkdir -p "$INSTALL_DIR"
mv "$tmp/mdv" "$INSTALL_DIR/mdv"
chmod 755 "$INSTALL_DIR/mdv"
say "Installed $("$INSTALL_DIR/mdv" --version) to $INSTALL_DIR/mdv"

case ":$PATH:" in
  *":$INSTALL_DIR:"*) ;;
  *)
    say ""
    say "$INSTALL_DIR is not on your PATH. Add it, for example:"
    say "  echo 'export PATH=\"$INSTALL_DIR:\$PATH\"' >> ~/.zshrc && source ~/.zshrc"
    ;;
esac

say ""
say "Next:"
say "  mdv init      set up the docs folder and tokens"
say "  mdv doctor    check everything"
