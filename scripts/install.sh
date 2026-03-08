#!/bin/sh
set -e

REPO="mycli-sh/mycli"
BINARY_NAME="my"

# Override these env vars for testing / mirrors:
#   MYCLI_RELEASE_URL  — URL returning JSON with "tag_name" (default: GitHub API)
#   MYCLI_DOWNLOAD_BASE — base URL for archive downloads (default: GitHub releases)

main() {
    os=$(uname -s | tr '[:upper:]' '[:lower:]')
    arch=$(uname -m)

    case "$os" in
        linux)  os="linux" ;;
        darwin) os="darwin" ;;
        *)
            echo "Error: unsupported OS: $os" >&2
            exit 1
            ;;
    esac

    case "$arch" in
        x86_64|amd64) arch="amd64" ;;
        aarch64|arm64) arch="arm64" ;;
        *)
            echo "Error: unsupported architecture: $arch" >&2
            exit 1
            ;;
    esac

    release_url="${MYCLI_RELEASE_URL:-https://api.github.com/repos/${REPO}/releases/latest}"
    download_base="${MYCLI_DOWNLOAD_BASE:-https://github.com/${REPO}/releases/download}"

    # Fetch latest release tag
    if command -v curl >/dev/null 2>&1; then
        tag=$(curl -fsSL "$release_url" | grep '"tag_name"' | sed 's/.*"tag_name": *"//;s/".*//')
    elif command -v wget >/dev/null 2>&1; then
        tag=$(wget -qO- "$release_url" | grep '"tag_name"' | sed 's/.*"tag_name": *"//;s/".*//')
    else
        echo "Error: curl or wget is required" >&2
        exit 1
    fi

    if [ -z "$tag" ]; then
        echo "Error: could not determine latest release" >&2
        exit 1
    fi

    version="${tag#v}"
    filename="${BINARY_NAME}_${version}_${os}_${arch}.tar.gz"
    url="${download_base}/${tag}/${filename}"

    tmpdir=$(mktemp -d)
    trap 'rm -rf "$tmpdir"' EXIT

    echo "Downloading ${BINARY_NAME} ${tag} for ${os}/${arch}..."
    if command -v curl >/dev/null 2>&1; then
        curl -fsSL "$url" -o "${tmpdir}/${filename}"
    else
        wget -qO "${tmpdir}/${filename}" "$url"
    fi

    tar -xzf "${tmpdir}/${filename}" -C "$tmpdir"

    # Determine install directory
    install_dir="/usr/local/bin"
    if [ -w "$install_dir" ]; then
        mv "${tmpdir}/${BINARY_NAME}" "${install_dir}/${BINARY_NAME}"
    elif command -v sudo >/dev/null 2>&1; then
        echo "Installing to ${install_dir} (requires sudo)..."
        sudo mv "${tmpdir}/${BINARY_NAME}" "${install_dir}/${BINARY_NAME}"
    else
        install_dir="${HOME}/.local/bin"
        mkdir -p "$install_dir"
        mv "${tmpdir}/${BINARY_NAME}" "${install_dir}/${BINARY_NAME}"
        echo "Installed to ${install_dir}/${BINARY_NAME}"
        case ":$PATH:" in
            *":${install_dir}:"*) ;;
            *)
                echo "Warning: ${install_dir} is not in your PATH." >&2
                echo "Add it with: export PATH=\"${install_dir}:\$PATH\"" >&2
                ;;
        esac
    fi

    chmod +x "${install_dir}/${BINARY_NAME}"

    echo "Successfully installed ${BINARY_NAME} ${tag}"
    if command -v "$BINARY_NAME" >/dev/null 2>&1; then
        "$BINARY_NAME" --version
    fi
}

main
