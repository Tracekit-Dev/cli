#!/bin/sh
# TraceKit CLI Installer
# Usage: curl -fsSL https://cli.tracekit.dev/install.sh | sh

set -e

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

# Configuration
REPO="Tracekit-Dev/cli"
BINARY_NAME="tracekit"
INSTALL_DIR="/usr/local/bin"
VERSION="${TRACEKIT_VERSION:-latest}"

# Print with color
print_info() {
    printf "${CYAN}ℹ${NC} %s\n" "$1"
}

print_success() {
    printf "${GREEN}✓${NC} %s\n" "$1"
}

print_error() {
    printf "${RED}✗${NC} %s\n" "$1" >&2
}

print_warning() {
    printf "${YELLOW}⚠${NC} %s\n" "$1"
}

# Detect OS and architecture
detect_platform() {
    OS=$(uname -s | tr '[:upper:]' '[:lower:]')
    ARCH=$(uname -m)

    case $OS in
        linux)
            OS="linux"
            ;;
        darwin)
            OS="darwin"
            ;;
        mingw*|msys*|cygwin*)
            OS="windows"
            ;;
        *)
            print_error "Unsupported operating system: $OS"
            exit 1
            ;;
    esac

    case $ARCH in
        x86_64|amd64)
            ARCH="amd64"
            ;;
        arm64|aarch64)
            ARCH="arm64"
            ;;
        i386|i686)
            ARCH="386"
            ;;
        armv7l|armv6l)
            ARCH="arm"
            ;;
        *)
            print_error "Unsupported architecture: $ARCH"
            exit 1
            ;;
    esac

    PLATFORM="${OS}-${ARCH}"
    if [ "$OS" = "windows" ]; then
        BINARY_NAME="${BINARY_NAME}.exe"
    fi
}

# Get latest version from GitHub
get_latest_version() {
    if [ "$VERSION" = "latest" ]; then
        print_info "Fetching latest version..."
        VERSION=$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" | grep '"tag_name"' | sed -E 's/.*"([^"]+)".*/\1/')
        if [ -z "$VERSION" ]; then
            print_error "Failed to fetch latest version"
            exit 1
        fi
        print_success "Latest version: $VERSION"
    fi
}

# Download binary
download_binary() {
    DOWNLOAD_URL="https://github.com/${REPO}/releases/download/${VERSION}/${BINARY_NAME}-${PLATFORM}"
    TMP_FILE="/tmp/${BINARY_NAME}"

    print_info "Downloading TraceKit CLI ${VERSION} for ${PLATFORM}..."

    if ! curl -fsSL "$DOWNLOAD_URL" -o "$TMP_FILE"; then
        print_error "Failed to download from $DOWNLOAD_URL"
        print_error "Please check if the release exists for your platform"
        exit 1
    fi

    print_success "Downloaded successfully"
}

# Install binary
install_binary() {
    print_info "Installing to ${INSTALL_DIR}/${BINARY_NAME}..."

    # Check if we need sudo
    if [ -w "$INSTALL_DIR" ]; then
        mv "$TMP_FILE" "${INSTALL_DIR}/${BINARY_NAME}"
        chmod +x "${INSTALL_DIR}/${BINARY_NAME}"
    else
        print_info "Requesting administrator privileges..."
        sudo mv "$TMP_FILE" "${INSTALL_DIR}/${BINARY_NAME}"
        sudo chmod +x "${INSTALL_DIR}/${BINARY_NAME}"
    fi

    print_success "Installed to ${INSTALL_DIR}/${BINARY_NAME}"
}

# Verify installation
verify_installation() {
    if ! command -v $BINARY_NAME >/dev/null 2>&1; then
        print_warning "Installation complete, but '${BINARY_NAME}' is not in PATH"
        print_info "You may need to add ${INSTALL_DIR} to your PATH"
        return
    fi

    INSTALLED_VERSION=$(${BINARY_NAME} --version 2>/dev/null || echo "unknown")
    print_success "TraceKit CLI installed successfully!"
    print_info "Version: ${INSTALLED_VERSION}"
}

# Print next steps
print_next_steps() {
    echo ""
    echo "────────────────────────────────────────────────────────"
    echo ""
    printf "${GREEN}🎉 Installation Complete!${NC}\n"
    echo ""
    echo "Next steps:"
    echo "  1. Run 'tracekit init' in your project directory"
    echo "  2. Follow the prompts to create your account"
    echo "  3. Start monitoring your application"
    echo ""
    echo "Documentation: https://docs.tracekit.dev"
    echo "Get help:      tracekit --help"
    echo ""
    echo "────────────────────────────────────────────────────────"
    echo ""
}

# Main installation flow
main() {
    echo ""
    printf "${CYAN}╔════════════════════════════════════════╗${NC}\n"
    printf "${CYAN}║                                        ║${NC}\n"
    printf "${CYAN}║      ${NC}TraceKit CLI Installer${CYAN}          ║${NC}\n"
    printf "${CYAN}║                                        ║${NC}\n"
    printf "${CYAN}╚════════════════════════════════════════╝${NC}\n"
    echo ""

    detect_platform
    print_info "Detected platform: ${PLATFORM}"

    get_latest_version
    download_binary
    install_binary
    verify_installation
    print_next_steps
}

# Run installer
main
