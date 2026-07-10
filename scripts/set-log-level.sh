#!/bin/bash
set -euo pipefail

SAMBA_CONF="/etc/samba/smb.conf"

die() {
    echo "$1: $2" >&2
    exit 1
}

validate_env() {
    echo "==> Validating environment"
    if [[ -z "${SAMBA_LOG_LEVEL:-}" ]]; then
        die "VALIDATION_FAILED" "SAMBA_LOG_LEVEL is required (0-10)"
    fi
    if [[ "$SAMBA_LOG_LEVEL" -lt 0 || "$SAMBA_LOG_LEVEL" -gt 10 ]]; then
        die "VALIDATION_FAILED" "SAMBA_LOG_LEVEL must be between 0 and 10, got: ${SAMBA_LOG_LEVEL}"
    fi
}

stop_winbind() {
    echo "==> Stopping winbind"
    pkill winbindd 2>/dev/null || true
    sleep 1
}

update_log_level() {
    echo "==> Updating log level to ${SAMBA_LOG_LEVEL}"
    local output
    if ! output=$(crudini --set ${SAMBA_CONF} global "log level" "${SAMBA_LOG_LEVEL}" 2>&1); then
        die "CONFIG_FAILED" "${output}"
    fi
}

start_winbind() {
    echo "==> Starting winbind"
    local output
    if ! output=$(/usr/sbin/winbindd -D 2>&1); then
        die "SERVICE_START_FAILED" "${output}"
    fi
    sleep 3
}

main() {
    validate_env

    echo "=========================================="
    echo "Setting Samba log level"
    echo "  Log level:    ${SAMBA_LOG_LEVEL}"
    echo "=========================================="

    stop_winbind
    update_log_level
    start_winbind

    echo "=========================================="
    echo "Log level update succeeded."
    echo "=========================================="
}

main "$@"
