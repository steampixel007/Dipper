#!/bin/sh
set -eu

usage() {
    printf 'Usage: sh scripts/build.sh [--test] [--render] [--cross-compile]\n'
}

run_tests=0
render=0
cross_compile=0
while [ "$#" -gt 0 ]; do
    case "$1" in
        --test) run_tests=1 ;;
        --render) render=1 ;;
        --cross-compile) cross_compile=1 ;;
        -h|--help) usage; exit 0 ;;
        *) usage >&2; printf 'Unknown option: %s\n' "$1" >&2; exit 2 ;;
    esac
    shift
done

case "$(uname -s)" in
    Darwin|Linux) ;;
    *) printf 'This script builds on macOS or Linux; use scripts/build.ps1 on Windows.\n' >&2; exit 1 ;;
esac

repo_root=$(CDPATH= cd "$(dirname "$0")/.." && pwd -P)
CGO_ENABLED=0
GOTOOLCHAIN=${GOTOOLCHAIN:-auto}
GOTELEMETRY=off
export CGO_ENABLED GOTOOLCHAIN GOTELEMETRY
unset GOOS GOARCH
mkdir -p "$repo_root/bin"
cd "$repo_root"

if [ "$run_tests" -eq 1 ]; then
    go test -count=1 -timeout 5m ./...
    go vet ./...
fi

go build -trimpath -o bin/dipper ./cmd/dipper

if [ "$render" -eq 1 ]; then
    ./bin/dipper --render ./docs/assets/ui
fi

if [ "$cross_compile" -eq 1 ]; then
    for target in linux:amd64 darwin:arm64 darwin:amd64 windows:amd64; do
        target_os=${target%%:*}
        target_arch=${target#*:}
        if [ "$target_os" = windows ]; then
            GOOS=windows GOARCH="$target_arch" go build -trimpath -ldflags '-H windowsgui' -o bin/dipper.exe ./cmd/dipper
        else
            GOOS="$target_os" GOARCH="$target_arch" go build -trimpath -o "bin/dipper-$target_os-$target_arch" ./cmd/dipper
        fi
    done
fi
