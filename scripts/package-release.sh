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

cp LICENSE THIRD_PARTY_NOTICES.md "$stage/"
cp -R third_party "$stage/"
goroot=$(go env GOROOT)
go_license="$goroot/LICENSE"
if [[ ! -f $go_license ]]; then
    # Homebrew installs the license alongside libexec.
    go_license="$goroot/../LICENSE"
fi
mkdir -p "$stage/licenses/go" "$stage/archives"
cp "$go_license" "$goroot/PATENTS" "$stage/licenses/go/"

archives=()
for platform in darwin-amd64 darwin-arm64 linux-amd64 linux-arm64 windows-amd64 windows-arm64; do
    export GOOS=${platform%-*} GOARCH=${platform#*-}
    binary=vl
    if [[ $GOOS == windows ]]; then
        binary=vl.exe
    fi
    make build VERSION="$version" OUTPUT="$stage/$binary"

    go list -deps -f '{{with .Module}}{{if not .Main}}{{.Path}}{{"\t"}}{{.Dir}}{{end}}{{end}}' ./cmd/vl |
        sort -u > "$stage/modules"
    while IFS=$'\t' read -r module directory; do
        [[ -n $module ]] || continue
        destination="$stage/licenses/$module"
        mkdir -p "$destination"
        find "$directory" -maxdepth 1 -type f \( -iname 'license*' -o -iname 'copying*' -o -iname '*notice*' -o -name PATENTS \) \
            -exec cp {} "$destination/" \;
    done < "$stage/modules"

    contents=("$binary" LICENSE THIRD_PARTY_NOTICES.md third_party licenses)
    archive="vl_${version}_${platform}"
    if [[ $GOOS == windows ]]; then
        archive+=.zip
        (cd "$stage" && zip -qr "archives/$archive" "${contents[@]}")
    else
        archive+=.tar.gz
        COPYFILE_DISABLE=1 tar -czf "$stage/archives/$archive" -C "$stage" "${contents[@]}"
    fi
    archives+=("$archive")
done

(cd "$stage/archives" && shasum -a 256 "${archives[@]}" > SHA256SUMS)
mkdir -p dist
mv "$stage/archives/"* dist/
