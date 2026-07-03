#!/bin/bash
set -e

die() {
    echo "$1: $2" >&2
    exit 1
}

main() {
    local output
    if ! output=$(wbinfo -t 2>&1); then
        die "TRUST_CHECK_FAILED" "${output}"
    fi
}

main "$@"
