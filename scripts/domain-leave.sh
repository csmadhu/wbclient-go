#!/bin/bash
set -e

: "${AD_USERNAME:?AD_USERNAME is required}"
: "${AD_PASSWORD:?AD_PASSWORD is required}"

echo "=========================================="
echo "Leaving domain (member mode)"
echo "  Leave user:   ${AD_USERNAME}"
echo "=========================================="

pkill winbindd 2>/dev/null || true
sleep 1

net ads leave -U "${AD_USERNAME}"%"${AD_PASSWORD}"

rm -f /var/lib/samba/private/secrets.tdb
rm -f /etc/samba/smb.conf
rm -f /etc/krb5.conf

echo "=========================================="
echo "Domain leave complete."
echo "  - AD computer account removed."
echo "  - secrets.tdb, smb.conf, krb5.conf cleared."
echo "  - winbindd stopped."
echo "=========================================="
