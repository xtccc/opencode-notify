#!/usr/bin/env bash
set -euo pipefail

BINARY="opencode-notify"
PREFIX="${HOME}/.local/bin"
REMOTE_HOST="${REMOTE_HOST:-work}"
REMOTE_PREFIX="${REMOTE_PREFIX:-.local/bin}"
REMOTE_USER="${REMOTE_USER:-xtcc}"

LOCAL_PLUGIN="${HOME}/.config/opencode/plugins/opencode-notify.js"
STATE_DIR="${XDG_STATE_HOME:-${HOME}/.local/state}/opencode-notify-deploy"

log()   { printf '\033[1;32m[deploy]\033[0m %s\n' "$*"; }
step()  { log "==> $*"; }

fail() {
    printf '\033[1;31m[deploy]\033[0m 错误: %s\n' "$*" >&2
    exit 1
}

verify_binary() {
    local desc="$1" actual="$2"
    [[ "${actual}" == "${SOURCE_SHA}" ]] || fail "${desc} 二进制 sha256 不匹配"
    echo "  ✓ ${desc} sha256 一致 (${actual:0:16}…)"
}

verify_plugin() {
    local host_name="$1" sha="$2"
    local state="${STATE_DIR}/plugin.${host_name}.sha256"
    [[ "${sha}" =~ ^[0-9a-f]{64}$ ]] || fail "${host_name} 插件 sha256 获取失败"
    if [[ -f "${state}" ]]; then
        local prev
        prev=$(cat "${state}")
        [[ "${prev}" == "${sha}" ]] || fail "${host_name} 插件 sha256 与上次部署不一致 (${prev} → ${sha})"
        echo "  ✓ ${host_name} 插件 sha256 一致"
    else
        mkdir -p "$(dirname "${state}")"
        printf '%s\n' "${sha}" > "${state}"
        echo "  ✓ ${host_name} 插件 sha256 已记录"
    fi
}

./build.sh build

SOURCE_SHA=$(sha256sum -- "${BINARY}" | awk '{print $1}')
echo "  build sha256: ${SOURCE_SHA}"

step "部署到本地 ${HOSTNAME}"
install -m 0755 "${BINARY}" "${PREFIX}/${BINARY}"
verify_binary "本地二进制" "$(sha256sum -- "${PREFIX}/${BINARY}" | awk '{print $1}')"
"${PREFIX}/${BINARY}" install
verify_plugin "${HOSTNAME}" "$(sha256sum -- "${LOCAL_PLUGIN}" | awk '{print $1}')"
echo "  local: ${PREFIX}/${BINARY} ($("${PREFIX}/${BINARY}" version))"

step "部署到 ${REMOTE_USER}@${REMOTE_HOST}"
remote_home=$(ssh "${REMOTE_HOST}" 'printf %s "$HOME"')
echo "  remote home: ${remote_home}"
scp "${BINARY}" "${REMOTE_HOST}:/tmp/${BINARY}"
ssh "${REMOTE_HOST}" "install -m 0755 /tmp/${BINARY} '${remote_home}/${REMOTE_PREFIX}/${BINARY}' && rm -f /tmp/${BINARY}"
verify_binary "远端二进制" "$(ssh "${REMOTE_HOST}" "sha256sum -- '${remote_home}/${REMOTE_PREFIX}/${BINARY}' | awk '{print \$1}'")"
ssh "${REMOTE_HOST}" "'${remote_home}/${REMOTE_PREFIX}/${BINARY}' install"
remote_plugin="${remote_home}/.config/opencode/plugins/opencode-notify.js"
verify_plugin "远端" "$(ssh "${REMOTE_HOST}" "sha256sum -- '${remote_plugin}' | awk '{print \$1}'")"

log "完成"