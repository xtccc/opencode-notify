#!/usr/bin/env bash
set -euo pipefail

BINARY="opencode-notify"
PREFIX="${HOME}/.local/bin"
CMD="${1:-build}"

build() {
    CGO_ENABLED=0 go build -o "${BINARY}" ./cmd/opencode-notify
}

test_cmd() {
    go test ./...
}

vet() {
    go vet ./...
}

install_cmd() {
    build
    install -m 0755 "${BINARY}" "${PREFIX}/${BINARY}"
}

uninstall() {
    rm -f "${PREFIX}/${BINARY}"
}

plugin() {
    build
    "${PREFIX}/${BINARY}" install || "./${BINARY}" install
}

clean() {
    rm -f "${BINARY}"
}

case "${CMD}" in
    build)     command ;;
    test|vet)  "${CMD}" ;;
    install)   install_cmd ;;
    uninstall) uninstall ;;
    plugin)    plugin ;;
    clean)     clean ;;
    *)
        echo "用法: ./build.sh {build|test|vet|install|uninstall|plugin|clean}" >&2
        exit 1
        ;;
esac