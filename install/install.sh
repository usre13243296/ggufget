#!/bin/sh
# Download the ggufget single-file binary, verify its SHA256 and put it on PATH.
#
#   sh install.sh                 # latest known version, best install dir
#   VERSION=1.2.1 sh install.sh   # pin a version
#   PREFIX=$HOME/.local sh install.sh
#
# This script only installs. All model logic lives in the binary.
set -eu

VERSION="${VERSION:-1.2.1}"
REPO="${REPO:-usre13243296/ggufget}"
PREFIX="${PREFIX:-}"

say() { printf '==> %s\n' "$*"; }
ok() { printf '  ok %s\n' "$*"; }
warn() { printf '  !  %s\n' "$*"; }
die() {
    printf '  x  %s\n' "$*" >&2
    exit 1
}

# ---------------------------------------------------------------- platform
os=$(uname -s 2>/dev/null || echo unknown)
case "$os" in
    Linux) os=linux ;;
    Darwin) os=darwin ;;
    *) die "unsupported operating system: $os (use the Go source or a release asset for it)" ;;
esac

arch=$(uname -m 2>/dev/null || echo unknown)
case "$arch" in
    x86_64 | amd64) arch=amd64 ;;
    aarch64 | arm64) arch=arm64 ;;
    *) die "unsupported architecture: $arch" ;;
esac

asset="ggufget_${VERSION}_${os}_${arch}"
url="https://github.com/${REPO}/releases/download/v${VERSION}/${asset}"

say "ggufget ${VERSION} for ${os}/${arch}"

# ---------------------------------------------------------------- downloader
if command -v curl >/dev/null 2>&1; then
    fetch() { curl -fsSL "$1" -o "$2"; }
elif command -v wget >/dev/null 2>&1; then
    fetch() { wget -qO "$2" "$1"; }
else
    die "need curl or wget on PATH"
fi

# ------------------------------------------------------------------- install dir
if [ -n "$PREFIX" ]; then
    bindir="$PREFIX/bin"
elif [ "$(id -u)" -eq 0 ] || [ -w /usr/local/bin ]; then
    bindir="/usr/local/bin"
else
    bindir="$HOME/.local/bin"
fi

tmp=$(mktemp -d 2>/dev/null || mktemp -d -t ggufget)
trap 'rm -rf "$tmp"' EXIT INT TERM

# ------------------------------------------------------------------- download
say "downloading ${asset}"
fetch "$url" "$tmp/ggufget" ||
    die "download failed. If github.com is slow or blocked, fetch the asset with a proxy and copy it to ${bindir}/ggufget"

# ---------------------------------------------------------------- verify
expected=""
if fetch "${url%/*}/SHA256SUMS" "$tmp/SHA256SUMS" 2>/dev/null; then
    expected=$(grep " ${asset}\$" "$tmp/SHA256SUMS" 2>/dev/null | awk '{print $1}' | head -n1 || true)
fi

if [ -n "$expected" ]; then
    if command -v sha256sum >/dev/null 2>&1; then
        actual=$(sha256sum "$tmp/ggufget" | awk '{print $1}')
    elif command -v shasum >/dev/null 2>&1; then
        actual=$(shasum -a 256 "$tmp/ggufget" | awk '{print $1}')
    elif command -v openssl >/dev/null 2>&1; then
        actual=$(openssl dgst -sha256 "$tmp/ggufget" | awk '{print $NF}')
    else
        actual=""
        warn "no sha256 tool found, skipping verification"
    fi
    if [ -n "$actual" ] && [ "$actual" != "$expected" ]; then
        die "checksum mismatch: expected $expected, got $actual"
    fi
    [ -n "$actual" ] && ok "sha256 $(printf '%s' "$actual" | cut -c1-16)..."
else
    warn "SHA256SUMS not published for this release, skipping verification"
fi

# ------------------------------------------------------------------- install
say "installing to ${bindir}"
mkdir -p "$bindir"
install -m 0755 "$tmp/ggufget" "$bindir/ggufget" 2>/dev/null ||
    { cp "$tmp/ggufget" "$bindir/ggufget" && chmod 0755 "$bindir/ggufget"; }
ok "${bindir}/ggufget"

# ------------------------------------------------------------------- PATH
case ":${PATH}:" in
    *":${bindir}:"*) ok "already on PATH" ;;
    *)
        warn "${bindir} is not on PATH; add this to your shell profile:"
        printf '\n    export PATH="%s:$PATH"\n\n' "$bindir"
        ;;
esac

cat <<EOF

Done. Try it:
  ggufget doctor
  ggufget list lmstudio-community/Qwen2.5-7B-Instruct-GGUF
  ggufget get  lmstudio-community/Qwen2.5-7B-Instruct-GGUF -f 2
EOF
