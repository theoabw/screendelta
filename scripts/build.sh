#!/usr/bin/env bash
# Cross-compile ScreenDelta for the platforms the engine claims to support.
#
# NFR-008 requires Linux and Windows, and the users of a screen-reading component
# are on Windows desktops, so both binaries are produced by every release build.
#
# Usage:
#   scripts/build.sh              # build both targets into bin/
#   scripts/build.sh --host-only  # build only the host platform
#   scripts/build.sh --help
#
# Output: bin/screendelta-linux-amd64 and bin/screendelta-windows-amd64.exe
# Rollback: remove bin/. Nothing outside the repository is written.

set -euo pipefail

readonly REPO_ROOT="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
readonly OUT_DIR="${REPO_ROOT}/bin"
readonly PKG="./cmd/screendelta"

host_only=false
for arg in "$@"; do
    case "${arg}" in
        --host-only) host_only=true ;;
        --help|-h)
            sed -n '2,16p' "${BASH_SOURCE[0]}" | sed 's/^# \{0,1\}//'
            exit 0
            ;;
        *)
            echo "build.sh: unknown option ${arg}" >&2
            exit 1
            ;;
    esac
done

command -v go >/dev/null 2>&1 || {
    echo "build.sh: go is not installed" >&2
    exit 1
}

mkdir -p "${OUT_DIR}"

echo "building linux/amd64"
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -o "${OUT_DIR}/screendelta-linux-amd64" "${PKG}"

if [[ "${host_only}" == false ]]; then
    echo "building windows/amd64"
    CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -trimpath -o "${OUT_DIR}/screendelta-windows-amd64.exe" "${PKG}"
fi

ls -l "${OUT_DIR}"
