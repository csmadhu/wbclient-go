#!/bin/bash
set -euo pipefail

die() {
    echo "$1: $2" >&2
    exit 1
}

sanitize() {
    local text="$1"
    if [[ -n "${AD_PASSWORD:-}" ]]; then
        text="${text//${AD_PASSWORD}/[REDACTED]}"
    fi
    echo "${text}"
}

validate_env() {
    echo "==> Validating environment"
    if [[ -z "${AD_USERNAME:-}" ]]; then
        die "DOMAIN_LEAVE_VALIDATION_FAILED" "AD_USERNAME is required"
    fi
    if [[ -z "${AD_PASSWORD:-}" ]]; then
        die "DOMAIN_LEAVE_VALIDATION_FAILED" "AD_PASSWORD is required"
    fi
}

leave_domain() {
    echo "==> Leaving domain (user: ${AD_USERNAME})"
    local output
    if ! output=$(net ads leave -U "${AD_USERNAME}"%"${AD_PASSWORD}" 2>&1); then
        die "DOMAIN_LEAVE_FAILED" "$(sanitize "${output}")"
    fi
}

cleanup() {
    echo "==> Stopping domain service and cleaning up"
    pkill winbindd 2>/dev/null || true
    sleep 1
    rm -f /var/lib/samba/private/secrets.tdb || true
    rm -f /etc/samba/smb.conf || true
    rm -f /etc/krb5.conf || true
}

main() {
    validate_env

    echo "=========================================="
    echo "Leaving domain (member mode)"
    echo "  Leave user:   ${AD_USERNAME}"
    echo "=========================================="

    leave_domain
    cleanup

    echo "=========================================="
    echo "Domain leave succeeded."
    echo "=========================================="
}

main "$@"
