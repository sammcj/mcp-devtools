#!/usr/bin/env bash
#
# MCP DevTools Installer
#
# Quick install: curl -fsSL https://raw.githubusercontent.com/sammcj/mcp-devtools/main/install.sh | bash
# Test locally: ./install.sh --dry-run
#
# Environment variables:
#   INSTALL_DIR   - Custom installation directory
#   VERSION       - Specific version to install (default: latest)
#   NO_CONFIG     - Skip config generation (set to any value)
#   FORCE         - Skip all confirmation prompts (set to any value)
#   NO_COLOR      - Disable coloured output (set to any value)
#   DRY_RUN       - Show what would happen without making changes (set to any value)
#
# Command line arguments:
#   --dry-run     - Show what would happen without making changes

set -euo pipefail

# Parse command line arguments
while [ $# -gt 0 ]; do
    case $1 in
        --dry-run)
            DRY_RUN=1
            shift
            ;;
        *)
            shift
            ;;
    esac
done

# Constants
GITHUB_REPO="sammcj/mcp-devtools"
BINARY_NAME="mcp-devtools"
CONFIG_DIR="${HOME}/.mcp-devtools"
EXAMPLES_DIR="${CONFIG_DIR}/examples"

# Colours (disabled if NO_COLOR is set or not a TTY)
if [ -t 1 ] && [ -z "${NO_COLOR:-}" ]; then
    RED='\033[0;31m'
    GREEN='\033[0;32m'
    YELLOW='\033[1;33m'
    BLUE='\033[0;34m'
    BOLD='\033[1m'
    RESET='\033[0m'
else
    RED=''
    GREEN=''
    YELLOW=''
    BLUE=''
    BOLD=''
    RESET=''
fi

# Helper functions
info() {
    echo -e "${BLUE}ℹ${RESET} $*"
}

success() {
    echo -e "${GREEN}✓${RESET} $*"
}

warn() {
    echo -e "${YELLOW}⚠${RESET} $*"
}

error() {
    echo -e "${RED}✗${RESET} $*" >&2
}

bold() {
    echo -e "${BOLD}$*${RESET}"
}

dry_run() {
    if [ -n "${DRY_RUN:-}" ]; then
        echo -e "${YELLOW}[DRY RUN]${RESET} $*"
        return 0
    fi
    return 1
}

# Cleanup on exit
cleanup() {
    if [ -n "${TEMP_DIR:-}" ] && [ -d "$TEMP_DIR" ]; then
        rm -rf "$TEMP_DIR"
    fi
}
trap cleanup EXIT INT TERM

# Check for required commands
check_dependencies() {
    local missing=()

    if ! command -v curl >/dev/null 2>&1 && ! command -v wget >/dev/null 2>&1; then
        missing+=("curl or wget")
    fi

    if ! command -v tar >/dev/null 2>&1; then
        missing+=("tar")
    fi

    if [ ${#missing[@]} -gt 0 ]; then
        error "Missing required dependencies: ${missing[*]}"
        error "Please install them and try again"
        exit 1
    fi
}

# Download a file using curl or wget
download() {
    local url="$1"
    local output="$2"

    if command -v curl >/dev/null 2>&1; then
        curl -fsSL "$url" -o "$output"
    elif command -v wget >/dev/null 2>&1; then
        wget -q "$url" -O "$output"
    else
        error "Neither curl nor wget found"
        exit 1
    fi
}

# Detect OS
detect_os() {
    local os
    os="$(uname -s)"
    case "$os" in
        Darwin*)
            echo "darwin"
            ;;
        Linux*)
            echo "linux"
            ;;
        MINGW*|MSYS*|CYGWIN*)
            echo "windows"
            ;;
        *)
            error "Unsupported operating system: $os"
            exit 1
            ;;
    esac
}

# Detect architecture
detect_arch() {
    local arch
    arch="$(uname -m)"
    case "$arch" in
        x86_64|amd64)
            echo "amd64"
            ;;
        aarch64|arm64)
            echo "arm64"
            ;;
        *)
            error "Unsupported architecture: $arch"
            exit 1
            ;;
    esac
}

# Get latest release version from GitHub
get_latest_version() {
    local api_url="https://api.github.com/repos/${GITHUB_REPO}/releases/latest"
    local version

    if command -v curl >/dev/null 2>&1; then
        version=$(curl -fsSL "$api_url" | grep '"tag_name"' | sed -E 's/.*"([^"]+)".*/\1/' | sed 's/^v//')
    elif command -v wget >/dev/null 2>&1; then
        version=$(wget -qO- "$api_url" | grep '"tag_name"' | sed -E 's/.*"([^"]+)".*/\1/' | sed 's/^v//')
    fi

    if [ -z "$version" ]; then
        error "Failed to fetch latest version from GitHub"
        exit 1
    fi

    echo "$version"
}

# Determine installation directory
determine_install_dir() {
    # If INSTALL_DIR is set, use that
    if [ -n "${INSTALL_DIR:-}" ]; then
        echo "$INSTALL_DIR"
        return
    fi

    # If GOBIN is set, use that
    if [ -n "${GOBIN:-}" ]; then
        echo "$GOBIN"
        return
    fi

    # Try /usr/local/bin if it exists, is writable, and is in PATH
    if [ -d "/usr/local/bin" ] && [ -w "/usr/local/bin" ] && is_in_path "/usr/local/bin"; then
        echo "/usr/local/bin"
        return
    fi

    # If GOPATH is set and in PATH, use that
    if [ -n "${GOPATH:-}" ]; then
        local gopath_bin="${GOPATH}/bin"
        if echo "$PATH" | grep -q "$gopath_bin"; then
            echo "$gopath_bin"
            return
        fi
    fi

    # If ~/.local/bin exists and is in PATH, use that
    local local_bin="${HOME}/.local/bin"
    if [ -d "$local_bin" ] && echo "$PATH" | grep -q "$local_bin"; then
        echo "$local_bin"
        return
    fi

    # Default to ~/.local/bin (will be created if needed)
    echo "$local_bin"
}

# Check if directory is in PATH
is_in_path() {
    local dir="$1"
    # Normalise dir (remove trailing slash)
    dir="${dir%/}"

    # Split PATH by colon and check for exact match
    local path_entry
    local old_ifs="$IFS"
    IFS=':'
    for path_entry in $PATH; do
        # Normalise path_entry (remove trailing slash)
        path_entry="${path_entry%/}"
        if [ "$path_entry" = "$dir" ]; then
            IFS="$old_ifs"
            return 0
        fi
    done
    IFS="$old_ifs"
    return 1
}

# Check if directory is writable (creates it if needed)
check_install_dir() {
    local dir="$1"

    # Try to create directory if it doesn't exist
    if [ ! -d "$dir" ]; then
        if ! mkdir -p "$dir" 2>/dev/null; then
            error "Cannot create installation directory: ${dir}"
            error "You may need to run with sudo or choose a different directory"
            error "Use: INSTALL_DIR=/path/you/can/write ./install.sh"
            exit 1
        fi
    fi

    # Check if directory is writable
    if [ ! -w "$dir" ]; then
        error "Installation directory is not writable: ${dir}"
        error "You may need to run with sudo or choose a different directory"
        error "Use: INSTALL_DIR=/path/you/can/write ./install.sh"
        exit 1
    fi

    return 0
}

# Find existing installation
find_existing_install() {
    command -v "$BINARY_NAME" 2>/dev/null || true
}

# Download and install binary
install_binary() {
    local version="$1"
    local os="$2"
    local arch="$3"
    local install_dir="$4"

    # Construct filename based on OS
    # Format: mcp-devtools-{os}-{arch} or mcp-devtools-windows-{arch}.exe
    local filename
    if [ "$os" = "windows" ]; then
        filename="${BINARY_NAME}-${os}-${arch}.exe"
    else
        filename="${BINARY_NAME}-${os}-${arch}"
    fi

    local download_url="https://github.com/${GITHUB_REPO}/releases/download/v${version}/${filename}"
    local binary_path="${install_dir}/${BINARY_NAME}"

    if dry_run "Would check install directory is writable:"; then
        dry_run "  ${install_dir}"
        dry_run ""
        dry_run "Would download ${BINARY_NAME} v${version} for ${os}/${arch}:"
        dry_run "  ${download_url}"
        dry_run ""

        dry_run "Would install binary to:"
        dry_run "  ${binary_path}"
        dry_run ""

        if [ "$os" = "darwin" ]; then
            dry_run "Would remove macOS quarantine attribute"
            dry_run ""
        fi

        dry_run "Would verify installation by running:"
        dry_run "  ${binary_path} --version"
        return 0
    fi

    # Check install directory is writable before downloading
    check_install_dir "$install_dir"

    echo "Downloading..."

    # Create temporary directory
    TEMP_DIR=$(mktemp -d)
    local temp_binary="${TEMP_DIR}/${BINARY_NAME}"

    # Download binary directly
    if ! download "$download_url" "$temp_binary" 2>/dev/null; then
        error "Failed to download from $download_url"
        exit 1
    fi

    # Install new binary
    mv "$temp_binary" "$binary_path"
    chmod +x "$binary_path"

    # Remove macOS quarantine attribute if on macOS
    if [ "$os" = "darwin" ] && command -v xattr >/dev/null 2>&1; then
        xattr -d com.apple.quarantine "$binary_path" 2>/dev/null || true
    fi

    # Verify installation
    if ! "$binary_path" --version >/dev/null 2>&1; then
        error "Installation failed - binary verification failed"
        exit 1
    fi
}

# Generate example configurations
generate_configs() {
    local install_path="$1"
    local defaultEnv='"env": {
        "ENABLE_ADDITIONAL_TOOLS": "security,sequential_thinking,code_skim,code_rename",
        "DISABLED_TOOLS": "",
        "NOTE_FOR_HUMANS": "A minimal set of tools are enabled by default, MCP DevTools provides many additional useful tools including efficient Context7 documentation search, AWS documentation, Frontend UI Framework templates, Code search and optimisation and many others, visit https://github.com/sammcj/mcp-devtools for more information on available tools and configuration"
      }'

    if dry_run "Would generate example configurations in:"; then
        dry_run "  ${EXAMPLES_DIR}/"
        dry_run ""
        dry_run "Files that would be created:"
        dry_run "  • claude-desktop.json"
        dry_run "  • vscode-cline.json"
        dry_run "  • lm-studio.json"
        dry_run "  • generic-stdio.json"
        dry_run "  • http-server.json"
        dry_run "  • mcp-devtools.service"
        dry_run ""

        # Detect which file manager would be used
        if [ -f /proc/sys/fs/binfmt_misc/WSLInterop ] || grep -qi microsoft /proc/version 2>/dev/null; then
            dry_run "Would open Windows Explorer (WSL detected)"
        elif command -v open >/dev/null 2>&1; then
            dry_run "Would open Finder (macOS detected)"
        elif command -v xdg-open >/dev/null 2>&1; then
            dry_run "Would open file manager (Linux detected)"
        elif command -v explorer >/dev/null 2>&1; then
            dry_run "Would open Explorer (Windows detected)"
        fi
        return 0
    fi

    mkdir -p "$EXAMPLES_DIR"

    # Claude Desktop config
    cat > "${EXAMPLES_DIR}/claude-desktop.json" <<EOF
{
  "mcpServers": {
    "dev-tools": {
      "type": "stdio",
      "command": "${install_path}",
      $defaultEnv
    }
  }
}
EOF

    # VS Code / Cline config
    cat > "${EXAMPLES_DIR}/vscode-cline.json" <<EOF
{
  "mcpServers": {
    "dev-tools": {
      "type": "stdio",
      "command": "${install_path}",
      $defaultEnv
    }
  }
}
EOF

    # LM Studio config
    cat > "${EXAMPLES_DIR}/lm-studio.json" <<EOF
{
  "mcpServers": {
    "dev-tools": {
      "command": "${install_path}",
      $defaultEnv
    }
  }
}
EOF

    # Generic stdio config
    cat > "${EXAMPLES_DIR}/generic-stdio.json" <<EOF
{
  "mcpServers": {
    "dev-tools": {
      "type": "stdio",
      "command": "${install_path}",
      $defaultEnv
    }
  }
}
EOF

    # HTTP server example
    cat > "${EXAMPLES_DIR}/http-server.json" <<EOF
{
  "mcpServers": {
    "dev-tools": {
      "type": "streamableHttp",
      "url": "http://localhost:18080/http"
    }
  }
}
EOF

    # Systemd service example
    cat > "${EXAMPLES_DIR}/mcp-devtools.service" <<EOF
[Unit]
Description=MCP DevTools Server
After=network.target

[Service]
Type=simple
ExecStart=${install_path} --transport http --port 18080
Restart=always
RestartSec=10
Environment="ENABLE_ADDITIONAL_TOOLS=security,sequential_thinking,code_skim,code_rename"

[Install]
WantedBy=multi-user.target
EOF

    # Open file manager to show the configs (platform-specific)
    if [ -f /proc/sys/fs/binfmt_misc/WSLInterop ] || grep -qi microsoft /proc/version 2>/dev/null; then
        # WSL2 - use Windows Explorer
        if command -v explorer.exe >/dev/null 2>&1; then
            local win_path
            win_path=$(wslpath -w "$EXAMPLES_DIR" 2>/dev/null || echo "$EXAMPLES_DIR")
            explorer.exe "$win_path" >/dev/null 2>&1 || true
        fi
    elif command -v open >/dev/null 2>&1; then
        # macOS
        open "$EXAMPLES_DIR" >/dev/null 2>&1 || true
    elif command -v xdg-open >/dev/null 2>&1; then
        # Linux with desktop environment
        xdg-open "$EXAMPLES_DIR" >/dev/null 2>&1 || true
    elif command -v explorer >/dev/null 2>&1; then
        # Windows (Git Bash, MSYS2, Cygwin)
        explorer "$(cygpath -w "$EXAMPLES_DIR" 2>/dev/null || echo "$EXAMPLES_DIR")" >/dev/null 2>&1 || true
    fi
}

# Automatically add install dir to PATH
add_to_path() {
    local install_dir="$1"

    # Don't modify in dry run mode
    if [ -n "${DRY_RUN:-}" ]; then
        dry_run "Would add ${install_dir} to PATH in shell RC file"
        return 0
    fi

    # Skip if already in PATH
    if is_in_path "$install_dir"; then
        return 0
    fi

    # Detect shell RC file
    local rc_file=""

    if [ -n "${BASH_VERSION:-}" ] && [ -f "${HOME}/.bashrc" ]; then
        rc_file="${HOME}/.bashrc"
    elif [ -n "${ZSH_VERSION:-}" ] && [ -f "${HOME}/.zshrc" ]; then
        rc_file="${HOME}/.zshrc"
    elif [ -f "${HOME}/.bashrc" ]; then
        rc_file="${HOME}/.bashrc"
    elif [ -f "${HOME}/.zshrc" ]; then
        rc_file="${HOME}/.zshrc"
    fi

    # If no RC file found, skip
    if [ -z "$rc_file" ]; then
        warn "Could not find shell RC file to update PATH"
        return 1
    fi

    # Check if already in RC file
    if grep -q "${install_dir}" "$rc_file" 2>/dev/null; then
        return 0
    fi

    # Add to PATH silently
    {
        echo ""
        echo "# Added by MCP DevTools installer on $(date +%Y-%m-%d)"
        echo "export PATH=\"${install_dir}:\$PATH\""
    } >> "$rc_file" 2>/dev/null
    return 0
}

# Main installation flow
main() {
    # Check dependencies silently
    check_dependencies

    # Detect platform
    local os arch
    os=$(detect_os)
    arch=$(detect_arch)

    # Get version to install
    local version="${VERSION:-}"
    if [ -z "$version" ]; then
        version=$(get_latest_version)
    fi

    # Check for existing installation
    local existing
    existing=$(find_existing_install)

    # Determine installation directory
    local install_dir
    install_dir=$(determine_install_dir)

    # Simple summary
    echo
    if [ -n "$existing" ]; then
        echo "Updating mcp-devtools v${version} (${os}/${arch}) at ${install_dir}"
    else
        echo "Installing mcp-devtools v${version} (${os}/${arch}) to ${install_dir}"
    fi

    # Single confirmation (unless FORCE, DRY_RUN, or stdin is not a terminal)
    if [ -z "${FORCE:-}" ] && [ -z "${DRY_RUN:-}" ] && [ -t 0 ]; then
        read -p "Continue? [y/N] " -n 1 -r
        echo
        if [[ ! $REPLY =~ ^[Yy]$ ]]; then
            exit 0
        fi
    fi

    # Install binary
    install_binary "$version" "$os" "$arch" "$install_dir"

    # Add to PATH if not already there
    local added_to_path=false
    local rc_file=""
    if ! is_in_path "$install_dir"; then
        if add_to_path "$install_dir"; then
            added_to_path=true
            if [ -f "${HOME}/.bashrc" ] && grep -q "${install_dir}" "${HOME}/.bashrc" 2>/dev/null; then
                rc_file="${HOME}/.bashrc"
            elif [ -f "${HOME}/.zshrc" ] && grep -q "${install_dir}" "${HOME}/.zshrc" 2>/dev/null; then
                rc_file="${HOME}/.zshrc"
            fi
        fi
    fi

    # Generate configs unless NO_CONFIG is set
    if [ -z "${NO_CONFIG:-}" ]; then
        generate_configs "${install_dir}/${BINARY_NAME}" >/dev/null 2>&1
    fi

    # Simple completion message
    echo
    success "Installed to ${install_dir}/${BINARY_NAME}"
    ls -la "${install_dir}/${BINARY_NAME}"

    if [ "$added_to_path" = true ] && [ -n "$rc_file" ]; then
        success "Added to ${rc_file}"
        echo
        echo "To use now: source ${rc_file}"
        echo "Or restart your shell"
    elif ! is_in_path "$install_dir"; then
        echo
        warn "Not in PATH - add this to your shell profile:"
        echo "  export PATH=\"${install_dir}:\$PATH\""
    fi

    if [ -z "${NO_CONFIG:-}" ]; then
        echo
        echo "Example configs: ${EXAMPLES_DIR}/"
    fi
    echo
}

main "$@"
