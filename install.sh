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

# Verify checksum of downloaded file
verify_checksum() {
    local file="$1"
    local expected_checksum="$2"

    if [ -z "$expected_checksum" ]; then
        warn "No checksum provided, skipping verification"
        return 0
    fi

    local actual_checksum
    if command -v sha256sum >/dev/null 2>&1; then
        actual_checksum=$(sha256sum "$file" | awk '{print $1}')
    elif command -v shasum >/dev/null 2>&1; then
        actual_checksum=$(shasum -a 256 "$file" | awk '{print $1}')
    else
        warn "No SHA256 tool found (sha256sum or shasum), skipping checksum verification"
        return 0
    fi

    if [ "$actual_checksum" != "$expected_checksum" ]; then
        error "Checksum verification failed!"
        error "Expected: ${expected_checksum}"
        error "Actual:   ${actual_checksum}"
        error "The downloaded file may be corrupted or compromised"
        return 1
    fi

    success "Checksum verified"
    return 0
}

# Download checksums file and extract checksum for specific file
get_checksum_for_file() {
    local version="$1"
    local filename="$2"
    local checksums_url="https://github.com/${GITHUB_REPO}/releases/download/v${version}/checksums.txt"

    local checksums
    if command -v curl >/dev/null 2>&1; then
        checksums=$(curl -fsSL "$checksums_url" 2>/dev/null || echo "")
    elif command -v wget >/dev/null 2>&1; then
        checksums=$(wget -qO- "$checksums_url" 2>/dev/null || echo "")
    fi

    if [ -z "$checksums" ]; then
        warn "Could not download checksums file, proceeding without verification"
        return 0
    fi

    # Extract checksum for our specific file
    local checksum
    checksum=$(echo "$checksums" | grep "$filename" | awk '{print $1}')

    echo "$checksum"
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
        dry_run "Would check for SHA256 checksum file (optional):"
        dry_run "  https://github.com/${GITHUB_REPO}/releases/download/v${version}/checksums.txt"
        dry_run ""

        if [ -f "$binary_path" ]; then
            local backup
            backup="${binary_path}.backup.$(date +%Y%m%d-%H%M%S)"
            dry_run "Would backup existing binary:"
            dry_run "  ${binary_path} -> ${backup}"
            dry_run ""
        fi

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

    info "Downloading ${BINARY_NAME} v${version} for ${os}/${arch}..."

    # Get expected checksum (optional)
    local expected_checksum
    expected_checksum=$(get_checksum_for_file "$version" "$filename")

    # Create temporary directory
    TEMP_DIR=$(mktemp -d)
    local temp_binary="${TEMP_DIR}/${BINARY_NAME}"

    # Download binary directly
    if ! download "$download_url" "$temp_binary"; then
        error "Failed to download from $download_url"
        error "Please check that the release exists for your platform"
        exit 1
    fi

    success "Downloaded ${filename}"

    # Verify checksum if available
    if [ -n "$expected_checksum" ]; then
        info "Verifying checksum..."
        if ! verify_checksum "$temp_binary" "$expected_checksum"; then
            error "Aborting installation due to checksum mismatch"
            exit 1
        fi
    fi

    # Backup existing binary if it exists
    if [ -f "$binary_path" ]; then
        local backup
        backup="${binary_path}.backup.$(date +%Y%m%d-%H%M%S)"
        info "Backing up existing binary to ${backup}"
        mv "$binary_path" "$backup"
    fi

    # Install new binary
    mv "$temp_binary" "$binary_path"
    chmod +x "$binary_path"

    # Remove macOS quarantine attribute if on macOS
    if [ "$os" = "darwin" ] && command -v xattr >/dev/null 2>&1; then
        xattr -d com.apple.quarantine "$binary_path" 2>/dev/null || true
    fi

    success "Installed to ${binary_path}"

    # Verify installation
    if ! "$binary_path" --version >/dev/null 2>&1; then
        error "Binary verification failed"
        exit 1
    fi

    success "Installation verified"
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

    info "Generating example configurations..."

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

    success "Example configurations created in ${EXAMPLES_DIR}/"

    # Open file manager to show the configs (platform-specific)
    if [ -f /proc/sys/fs/binfmt_misc/WSLInterop ] || grep -qi microsoft /proc/version 2>/dev/null; then
        # WSL2 - use Windows Explorer
        if command -v explorer.exe >/dev/null 2>&1; then
            # Convert WSL path to Windows path
            local win_path
            win_path=$(wslpath -w "$EXAMPLES_DIR" 2>/dev/null || echo "$EXAMPLES_DIR")
            explorer.exe "$win_path" >/dev/null 2>&1 || true
        fi
    elif command -v open >/dev/null 2>&1; then
        # macOS
        open "$EXAMPLES_DIR"
    elif command -v xdg-open >/dev/null 2>&1; then
        # Linux with desktop environment
        xdg-open "$EXAMPLES_DIR" >/dev/null 2>&1 || true
    elif command -v explorer >/dev/null 2>&1; then
        # Windows (Git Bash, MSYS2, Cygwin)
        explorer "$(cygpath -w "$EXAMPLES_DIR" 2>/dev/null || echo "$EXAMPLES_DIR")"
    fi
}

# Show next steps
show_next_steps() {
    local install_dir="$1"
    local os="$2"

    echo
    bold "Installation complete! 🎉"
    echo
    echo "Next steps:"
    echo

    # Check if install dir is in PATH
    if ! is_in_path "$install_dir"; then
        warn "Installation directory is not in your PATH"
        echo
        echo "Add this to your shell profile (~/.bashrc, ~/.zshrc, etc.):"
        echo
        echo "  export PATH=\"${install_dir}:\$PATH\""
        echo
        echo "Then reload your shell:"
        echo
        echo "  source ~/.bashrc  # or ~/.zshrc"
        echo
    fi

    echo "1. Add MCP DevTools to your MCP client configuration"
    echo
    echo "   Example configs are available in:"
    echo "   ${EXAMPLES_DIR}/"
    echo
    echo "   Common locations for MCP client configs:"

    if [ "$os" = "darwin" ]; then
        echo "   • Claude Desktop: ~/Library/Application Support/Claude/claude_desktop_config.json"
        echo "   • VS Code (Cline): ~/Library/Application Support/Code/User/globalStorage/saoudrizwan.claude-dev/settings/cline_mcp_settings.json"
    else
        echo "   • Claude Desktop: ~/.config/Claude/claude_desktop_config.json"
        echo "   • VS Code (Cline): ~/.config/Code/User/globalStorage/saoudrizwan.claude-dev/settings/cline_mcp_settings.json"
    fi

    echo
    echo "2. (Optional) Set up API keys for additional features:"
    echo "   • BRAVE_API_KEY - For Brave Search (https://brave.com/search/api/)"
    echo "   • GOOGLE_SEARCH_API_KEY - For Google Search"
    echo "   • CONTEXT7_API_KEY - For higher rate limits on package docs"
    echo
    echo "3. Restart your MCP client to load the server"
    echo
    echo "Documentation: https://github.com/${GITHUB_REPO}"
    echo "Available tools: https://github.com/${GITHUB_REPO}/blob/main/docs/tools/overview.md"
    echo
}

# Main installation flow
main() {
    echo
    bold "═══════════════════════════════════════════════════════"
    if [ -n "${DRY_RUN:-}" ]; then
        bold "    MCP DevTools Installer (DRY RUN MODE)"
    else
        bold "         MCP DevTools Installer"
    fi
    bold "═══════════════════════════════════════════════════════"
    echo

    if [ -n "${DRY_RUN:-}" ]; then
        warn "Running in DRY RUN mode - no changes will be made"
        echo
    fi

    # Check dependencies
    check_dependencies

    # Detect platform
    local os arch
    os=$(detect_os)
    arch=$(detect_arch)

    info "Detected platform: ${os}/${arch}"

    # Get version to install
    local version="${VERSION:-}"
    if [ -z "$version" ]; then
        version=$(get_latest_version)
    fi

    info "Version to install: v${version}"

    # Check for existing installation
    local existing
    existing=$(find_existing_install)
    if [ -n "$existing" ]; then
        warn "Found existing installation: ${existing}"

        # Get current version
        local current_version
        current_version=$("$existing" --version 2>/dev/null | grep -oE '[0-9]+\.[0-9]+\.[0-9]+' || echo "unknown")
        info "Current version: ${current_version}"

        if [ -z "${FORCE:-}" ] && [ -z "${DRY_RUN:-}" ]; then
            echo
            read -p "Do you want to replace it? [y/N] " -n 1 -r
            echo
            if [[ ! $REPLY =~ ^[Yy]$ ]]; then
                info "Installation cancelled"
                exit 0
            fi
        fi
    fi

    # Determine installation directory
    local install_dir
    install_dir=$(determine_install_dir)

    info "Installation directory: ${install_dir}"
    echo

    # Show summary and ask for confirmation (unless FORCE or DRY_RUN)
    if [ -z "${FORCE:-}" ] && [ -z "${DRY_RUN:-}" ]; then
        bold "Installation Summary:"
        echo "  Platform:     ${os}/${arch}"
        echo "  Version:      v${version}"
        echo "  Install to:   ${install_dir}/${BINARY_NAME}"

        if [ -n "$existing" ]; then
            echo "  Action:       Update existing installation"
        else
            echo "  Action:       Fresh installation"
        fi

        if [ -z "${NO_CONFIG:-}" ]; then
            echo "  Configs:      ${EXAMPLES_DIR}/"
        fi

        if ! is_in_path "$install_dir"; then
            echo "  PATH:         ⚠ Not in PATH (instructions will be shown)"
        else
            echo "  PATH:         ✓ Already in PATH"
        fi

        echo
        read -p "Proceed with installation? [y/N] " -n 1 -r
        echo
        if [[ ! $REPLY =~ ^[Yy]$ ]]; then
            info "Installation cancelled"
            exit 0
        fi
        echo
    fi

    # Install binary
    install_binary "$version" "$os" "$arch" "$install_dir"

    # Generate configs unless NO_CONFIG is set
    if [ -z "${NO_CONFIG:-}" ]; then
        echo
        generate_configs "${install_dir}/${BINARY_NAME}"
    fi

    # Show next steps
    show_next_steps "$install_dir" "$os"
}

main "$@"
