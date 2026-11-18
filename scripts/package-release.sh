#!/usr/bin/env bash
set -euo pipefail

version=${1:?usage: package-release.sh VERSION}
if [[ ! $version =~ ^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(-[0-9A-Za-z]+([.-][0-9A-Za-z]+)*)?$ ]]; then
    printf 'Invalid release version: %s\n' "$version" >&2
    exit 1
fi

export CGO_ENABLED=0
stage=$(mktemp -d)
trap 'rm -rf "$stage"' EXIT

goroot=$(go env GOROOT)
go_license="$goroot/LICENSE"
if [[ ! -f $go_license ]]; then
    # Homebrew installs the license alongside libexec.
    go_license="$goroot/../LICENSE"
fi
mkdir "$stage/archives"

archives=()
for platform in darwin-amd64 darwin-arm64 linux-amd64 linux-arm64 windows-amd64 windows-arm64; do
    export GOOS=${platform%-*} GOARCH=${platform#*-}
    package="$stage/$platform"
    mkdir -p "$package/licenses/go"
    cp LICENSE THIRD_PARTY_NOTICES.md "$package/"
    cp -R third_party "$package/"
    cp "$go_license" "$goroot/PATENTS" "$package/licenses/go/"

    binary=vl
    if [[ $GOOS == windows ]]; then
        binary=vl.exe
    fi
    make build VERSION="$version" OUTPUT="$package/$binary"

    go list -deps -f '{{with .Module}}{{if not .Main}}{{.Path}}{{"\t"}}{{.Dir}}{{end}}{{end}}' ./cmd/vl |
        sort -u > "$stage/modules"
    while IFS=$'\t' read -r module directory; do
        [[ -n $module ]] || continue
        destination="$package/licenses/$module"
        mkdir -p "$destination"
        find "$directory" -maxdepth 1 -type f \( -iname 'license*' -o -iname 'copying*' -o -iname '*notice*' -o -name PATENTS \) \
            -exec sh -c 'destination=$1; shift; cp "$@" "$destination/"' sh "$destination" {} +
    done < "$stage/modules"

    contents=("$binary" LICENSE THIRD_PARTY_NOTICES.md third_party licenses)
    arch=$GOARCH
    if [[ $arch == amd64 ]]; then
        arch=x86_64
    fi
    archive="vl_${version}_${GOOS}_${arch}"
    if [[ $GOOS == windows ]]; then
        archive+=.zip
        (cd "$package" && zip -qr "$stage/archives/$archive" "${contents[@]}")
    else
        archive+=.tar.gz
        COPYFILE_DISABLE=1 tar -czf "$stage/archives/$archive" -C "$package" "${contents[@]}"
    fi
    archives+=("$archive")
done

tag="v$version"
sed "s/@RELEASE_TAG@/$tag/g" install.sh > "$stage/archives/install.sh"
sed "s/@RELEASE_TAG@/$tag/g" install.ps1 > "$stage/archives/install.ps1"
chmod +x "$stage/archives/install.sh"
sh -n "$stage/archives/install.sh"
assets=("${archives[@]}" install.sh install.ps1)
(cd "$stage/archives" && shasum -a 256 "${assets[@]}" > checksums.txt)
mkdir -p dist
for asset in "${assets[@]}"; do
    mv "$stage/archives/$asset" dist/
done
mv "$stage/archives/checksums.txt" dist/
