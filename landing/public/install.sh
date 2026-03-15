#!/bin/sh
set -eu

REPO="vishruthb/seek"
INSTALL_DIR="${SEEK_INSTALL_DIR:-$HOME/.local/bin}"
CONFIG_DIR="${HOME}/.config/seek"
TMPDIR=""

info() {
	printf '\033[0;32m▸\033[0m %s\n' "$1"
}

warn() {
	printf '\033[0;33m▸\033[0m %s\n' "$1"
}

error() {
	printf '\033[0;31m✗\033[0m %s\n' "$1" >&2
	exit 1
}

cleanup() {
	if [ -n "${TMPDIR}" ] && [ -d "${TMPDIR}" ]; then
		rm -rf "${TMPDIR}"
	fi
}

trap cleanup EXIT INT HUP TERM

have_cmd() {
	command -v "$1" >/dev/null 2>&1
}

fetch_url() {
	if have_cmd curl; then
		curl -fsSL "$1"
		return 0
	fi
	if have_cmd wget; then
		wget -qO- "$1"
		return 0
	fi
	error "Missing downloader. Install curl or wget and retry."
}

download_to() {
	if have_cmd curl; then
		curl -fsSL "$1" -o "$2"
		return 0
	fi
	if have_cmd wget; then
		wget -qO "$2" "$1"
		return 0
	fi
	error "Missing downloader. Install curl or wget and retry."
}

OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
ARCH="$(uname -m)"

case "$OS" in
darwin) OS="darwin" ;;
linux) OS="linux" ;;
*) error "Unsupported OS: $OS" ;;
esac

case "$ARCH" in
x86_64|amd64) ARCH="amd64" ;;
aarch64|arm64) ARCH="arm64" ;;
*) error "Unsupported architecture: $ARCH" ;;
esac

if ! have_cmd tar; then
	error "Missing tar. Install tar and retry."
fi

info "Detected platform: ${OS}/${ARCH}"

if [ -n "${SEEK_VERSION:-}" ]; then
	LATEST="$SEEK_VERSION"
else
	info "Fetching latest release..."
	LATEST="$(fetch_url "https://api.github.com/repos/${REPO}/releases/latest" | sed -n 's/.*"tag_name"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' | head -n 1)"
fi

[ -n "$LATEST" ] || error "Could not determine the latest release tag."
info "Latest version: ${LATEST}"

TARBALL="seek_${LATEST#v}_${OS}_${ARCH}.tar.gz"
URL="https://github.com/${REPO}/releases/download/${LATEST}/${TARBALL}"
TMPDIR="$(mktemp -d 2>/dev/null || mktemp -d -t seek)"

info "Downloading ${URL}"
download_to "$URL" "${TMPDIR}/${TARBALL}" || error "Download failed. Check https://github.com/${REPO}/releases."

info "Extracting archive"
tar -xzf "${TMPDIR}/${TARBALL}" -C "${TMPDIR}" || error "Failed to extract archive."

mkdir -p "$INSTALL_DIR"
mv "${TMPDIR}/seek" "${INSTALL_DIR}/seek"
chmod +x "${INSTALL_DIR}/seek"
info "Installed seek to ${INSTALL_DIR}/seek"

if "${INSTALL_DIR}/seek" --version >/dev/null 2>&1; then
	VERSION_OUT="$("${INSTALL_DIR}/seek" --version 2>&1 || printf '%s' 'unknown')"
	info "Verified: ${VERSION_OUT}"
else
	warn "Binary installed but version verification failed. Check permissions or your shell PATH."
fi

mkdir -p "$CONFIG_DIR"

case ":${PATH}:" in
*":${INSTALL_DIR}:"*) ;;
*)
	warn "${INSTALL_DIR} is not in your PATH."
	case "${SHELL:-}" in
	*/zsh) SHELL_RC="$HOME/.zshrc" ;;
	*/bash) SHELL_RC="$HOME/.bashrc" ;;
	*) SHELL_RC="your shell config" ;;
	esac
	printf '\n  Add this to %s:\n\n' "$SHELL_RC"
	printf '    export PATH="%s:$PATH"\n\n' "$INSTALL_DIR"
	;;
esac

if "${INSTALL_DIR}/seek" --setup >/dev/null 2>&1; then
	printf '\n'
	"${INSTALL_DIR}/seek" --setup || true
	exit 0
fi

printf '\n'
printf '  ┌─────────────────────────────────────────────┐\n'
printf '  │  seek installed successfully!               │\n'
printf '  │                                             │\n'
printf '  │  Next steps:                                │\n'
printf '  │                                             │\n'
printf '  │  1. Get a Tavily API key:                   │\n'
printf '  │     → https://tavily.com                    │\n'
printf '  │                                             │\n'
printf '  │  2. Get a Groq API key:                     │\n'
printf '  │     → https://console.groq.com              │\n'
printf '  │     or use Ollama locally:                  │\n'
printf '  │     → https://ollama.com                    │\n'
printf '  │                                             │\n'
printf '  │  3. Export your credentials:                │\n'
printf '  │     export TAVILY_API_KEY="tvly-..."        │\n'
printf '  │     export OPENAI_API_KEY="gsk_..."         │\n'
printf '  │                                             │\n'
printf '  │  4. Try it:                                 │\n'
printf '  │     seek "hello world"                      │\n'
printf '  │                                             │\n'
printf '  └─────────────────────────────────────────────┘\n\n'
