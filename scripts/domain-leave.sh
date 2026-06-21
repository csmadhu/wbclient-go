#!/bin/bash
# Note: intentionally NOT `set -e`. The local-state cleanup at the bottom is
# security-critical and must run even if `net ads leave` fails (DC unreachable,
# already left, stale credentials, etc.). Errors are handled per-step instead.
set -uo pipefail

: "${AD_USERNAME:?AD_USERNAME is required}"
: "${AD_PASSWORD:?AD_PASSWORD is required}"

echo "=========================================="
echo "Leaving domain (member mode)"
echo "  Leave user:   ${AD_USERNAME}"
echo "=========================================="

pkill winbindd 2>/dev/null || true
sleep 1

# Best-effort AD-side cleanup. If this fails (e.g. DC unreachable, machine
# account already removed), we still proceed to wipe local trust state below
# so the machine cannot continue to authenticate users with stale secrets.
if ! net ads leave -U "${AD_USERNAME}"%"${AD_PASSWORD}"; then
    echo "WARN: 'net ads leave' failed; AD computer account may remain."
    echo "      Proceeding with local state cleanup anyway."
fi

rm -f /var/lib/samba/private/secrets.tdb
rm -f /etc/samba/smb.conf
rm -f /etc/krb5.conf

echo "=========================================="
echo "Domain leave complete."
echo "  - AD computer account removal: attempted (see warnings above)."
echo "  - secrets.tdb, smb.conf, krb5.conf cleared."
echo "  - winbindd stopped."
echo "=========================================="
