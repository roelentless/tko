#!/bin/sh
# tko installer / upgrader
# Usage: curl -fsSL https://raw.githubusercontent.com/roelentless/tko/main/install.sh | sh

REPO="roelentless/tko"
REPO_URL="https://github.com/${REPO}"
INSTALL_DIR="$HOME/.local/bin"

# --- Output helpers (only when terminal) ---

if [ -t 1 ]; then
  BOLD='\033[1m'
  DIM='\033[2m'
  RED='\033[31m'
  GREEN='\033[32m'
  YELLOW='\033[33m'
  CYAN='\033[36m'
  RESET='\033[0m'
else
  BOLD='' DIM='' RED='' GREEN='' YELLOW='' CYAN='' RESET=''
fi

ok()   { printf "  ${GREEN}ok${RESET}  %s\n" "$1"; }
info() { printf "  ${CYAN}->${RESET} %s\n" "$1"; }
warn() { printf "  ${YELLOW}!${RESET}  %s\n" "$1"; }
err()  { printf "  ${RED}error${RESET} %s\n" "$1" >&2; }
has()  { command -v "$1" >/dev/null 2>&1; }

if ! has curl; then
  printf "curl is required but not installed.\n" >&2
  exit 1
fi

prompt_yn() {
  if [ "${AUTO_YES}" = true ]; then return 0; fi
  if [ -t 0 ]; then
    printf "%s" "$1"
    read -r answer
  elif [ -e /dev/tty ]; then
    printf "%s" "$1"
    read -r answer < /dev/tty
  else
    return 0
  fi
  case "$answer" in
    n|N|no|No) return 1 ;;
    *) return 0 ;;
  esac
}

# ============================================================================
# Platform detection
# ============================================================================

detect_platform() {
  OS=$(uname -s)
  ARCH=$(uname -m)

  case "$OS" in
    Darwin) PLATFORM="macos" ;;
    Linux)  PLATFORM="linux" ;;
    *)
      err "Unsupported OS: $OS"
      err "See: ${REPO_URL}#install"
      exit 1
      ;;
  esac

  case "$ARCH" in
    x86_64|amd64)  ARCH_LABEL="amd64" ;;
    arm64|aarch64) ARCH_LABEL="arm64" ;;
    *)
      err "Unsupported architecture: $ARCH"
      err "See: ${REPO_URL}#install"
      exit 1
      ;;
  esac
}

# ============================================================================
# Version helpers
# ============================================================================

get_latest_version() {
  curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" 2>/dev/null \
    | grep '"tag_name"' | head -1 | sed 's/.*"tag_name": *"\([^"]*\)".*/\1/'
}

get_installed_version() {
  if has tko; then
    tko --version 2>/dev/null | head -1
  fi
}

# ============================================================================
# Install
# ============================================================================

install_tko() {
  DEST="${INSTALL_DIR}/tko"

  LATEST=$(get_latest_version)
  if [ -z "$LATEST" ]; then
    err "Could not determine latest version from GitHub."
    err "Check your network connection or visit: ${REPO_URL}/releases"
    exit 1
  fi

  if has tko; then
    CURRENT=$(get_installed_version)
    if [ "$CURRENT" = "$LATEST" ]; then
      ok "tko ${CURRENT} is already up to date"
      printf "\n"
      exit 0
    fi
    info "Upgrading tko ${CURRENT} -> ${LATEST}"
  else
    info "Installing tko ${LATEST}"
  fi

  TARBALL="tko-${PLATFORM}-${ARCH_LABEL}.tar.gz"
  URL="${REPO_URL}/releases/download/${LATEST}/${TARBALL}"

  info "Downloading ${URL}"
  TMP_DIR=$(mktemp -d)
  TMP_TAR="$TMP_DIR/$TARBALL"

  if ! curl -fsSL -o "$TMP_TAR" "$URL"; then
    rm -rf "$TMP_DIR"
    err "Download failed: ${URL}"
    err "Check releases: ${REPO_URL}/releases"
    exit 1
  fi

  tar xzf "$TMP_TAR" -C "$TMP_DIR"
  mkdir -p "$INSTALL_DIR"

  if [ -w "$INSTALL_DIR" ] || [ -w "$DEST" ] 2>/dev/null; then
    mv "$TMP_DIR/tko" "$DEST"
    chmod +x "$DEST"
  elif has sudo; then
    sudo mv "$TMP_DIR/tko" "$DEST"
    sudo chmod +x "$DEST"
  else
    err "Cannot write to ${INSTALL_DIR}."
    rm -rf "$TMP_DIR"
    exit 1
  fi

  rm -rf "$TMP_DIR"
  ok "tko ${LATEST} installed to ${DEST}"
}

# ============================================================================
# PATH verification
# ============================================================================

verify_path() {
  case ":$PATH:" in
    *":$INSTALL_DIR:"*) return ;;
  esac

  PATH_LINE="export PATH=\"${INSTALL_DIR}:\$PATH\""
  add_to_profile "$HOME/.profile" "$PATH_LINE"

  CURRENT_SHELL=$(basename "${SHELL:-/bin/sh}")
  if [ "$CURRENT_SHELL" = "zsh" ]; then
    add_to_profile "$HOME/.zprofile" "$PATH_LINE"
  fi

  printf "\n"
  warn "Open a new terminal or run: source ~/.profile"
}

add_to_profile() {
  PROFILE_FILE="$1"
  LINE="$2"
  if [ -f "$PROFILE_FILE" ] && grep -qF "$LINE" "$PROFILE_FILE"; then
    return
  fi
  printf '\n%s\n' "$LINE" >> "$PROFILE_FILE"
  ok "Added PATH entry to ${PROFILE_FILE}"
}

# ============================================================================
# Main
# ============================================================================

main() {
  AUTO_YES=false
  for arg in "$@"; do
    case "$arg" in
      -y|--yes) AUTO_YES=true ;;
      -h|--help)
        printf "tko installer\n\n"
        printf "Usage: curl -fsSL https://raw.githubusercontent.com/roelentless/tko/main/install.sh | sh\n\n"
        printf "Options:\n"
        printf "  -y, --yes   Skip confirmation prompt\n"
        printf "  -h, --help  Show this help\n"
        exit 0
        ;;
    esac
  done

  printf "\n  ${BOLD}tko installer${RESET}\n\n"

  detect_platform
  printf "  platform: %s/%s\n" "$PLATFORM" "$ARCH_LABEL"
  printf "  install:  %s/tko\n\n" "$INSTALL_DIR"

  if ! prompt_yn "  Proceed? [Y/n] "; then
    printf "\n"
    info "Aborted. Manual download: ${REPO_URL}/releases"
    printf "\n"
    exit 0
  fi

  printf "\n"
  install_tko
  verify_path

  printf "\n"
  ok "Done!"
  printf "\n"
  printf "  Next steps:\n"
  printf "    tko hook install   register Claude Code hook\n"
  printf "    tko --help         show all commands\n"
  printf "\n"
  printf "  Docs: ${REPO_URL}\n"
  printf "\n"
}

main "$@"
