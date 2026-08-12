#!/bin/bash
set -euo pipefail

SAMBA_CONF="/etc/samba/smb.conf"

die() {
    echo "$1: $2" >&2
    exit 1
}

migrate_param() {
    local param="$1"
    local value="$2"

    echo "==> Setting parameter: ${param} = ${value}"
    local output
    if ! output=$(crudini --set "${SAMBA_CONF}" global "${param}" "${value}" 2>&1); then
        die "MIGRATION_FAILED" "failed to set '${param}': ${output}"
    fi
}

main() {
    if [[ ! -f "${SAMBA_CONF}" ]]; then
        echo "==> No existing config found; skipping migration"
        exit 0
    fi

    echo "=========================================="
    echo "Running config migration"
    echo "=========================================="

    migrate_param "winbind max domain connections" "8"
    migrate_param "winbind request timeout" "10"
    migrate_param "winbind max clients" "500"

    echo "=========================================="
    echo "Config migration complete."
    echo "=========================================="
}

main "$@"
