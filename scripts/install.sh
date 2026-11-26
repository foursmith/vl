#!/bin/sh
set -eu

fail() {
    printf 'vl: %s\n' "$*" >&2
    exit 1
}

main() {
    case $(uname -s) in
        Darwin) os=darwin ;;
        Linux) os=linux ;;
        *) fail 'This installer supports macOS and Linux.' ;;
    esac
    case $(uname -m) in
        arm64|aarch64) arch=arm64 ;;
        x86_64|amd64) arch=x86_64 ;;
        *) fail 'This installer supports arm64 and x86_64.' ;;
    esac

    if command -v sha256sum >/dev/null 2>&1; then
        checksum=sha256sum
    elif command -v shasum >/dev/null 2>&1; then
        checksum=shasum
    else
        fail 'Install sha256sum or shasum to verify the download.'
    fi

    releases=https://github.com/foursmith/vl/releases
    tag='@RELEASE_TAG@'
    printf '%s\n' "$tag" | grep -Eq '^v[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z]+([.-][0-9A-Za-z]+)*)?$' ||
        fail 'Download the installer from a vl release.'
    archive="vl_${tag#v}_${os}_${arch}.tar.gz"
    download="$releases/download/$tag"

    install_dir=${INSTALL_DIR:-"$HOME/.local/bin"}
    mkdir -p -- "$install_dir"
    install_dir=$(cd -- "$install_dir" && pwd)
    [ ! -d "$install_dir/vl" ] || fail "$install_dir/vl is a directory."
    stage=$(mktemp -d "$install_dir/.vl-install.XXXXXX")
    trap 'rm -rf -- "$stage"' EXIT
    trap 'exit 1' HUP INT TERM

    cd "$stage"
    curl -fsSL "$download/$archive" -o "$archive"
    curl -fsSL "$download/checksums.txt" -o checksums.txt
    awk -v archive="$archive" '$2 == archive { print }' checksums.txt > checksum.txt
    [ -s checksum.txt ] || fail "No checksum found for $archive."
    if [ "$checksum" = sha256sum ]; then
        sha256sum -c checksum.txt
    else
        shasum -a 256 -c checksum.txt
    fi
    tar -xzf "$archive" vl
    chmod 755 vl
    mv -f -- vl "$install_dir/vl"

    printf 'Installed vl %s to %s/vl\n' "${tag#v}" "$install_dir"
    case ":${PATH:-}:" in
        *":$install_dir:"*) ;;
        *) printf 'Add %s to your PATH to run vl.\n' "$install_dir" ;;
    esac
}

main "$@"
