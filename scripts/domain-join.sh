#!/bin/bash
set -e

: "${DC_FQDN:?DC_FQDN is required (e.g. dc01.corp.example.com)}"
: "${NETBIOS_NAME:?NETBIOS_NAME is required (e.g. CORP)}"
: "${AD_USERNAME:?AD_USERNAME is required}"
: "${AD_PASSWORD:?AD_PASSWORD is required}"

if [ -z "${REALM}" ]; then
    echo "Discovering realm via 'net ads info -S ${DC_FQDN}'..."
    mkdir -p /etc/samba && touch /etc/samba/smb.conf
    DISCOVERED_REALM=$(net ads info -S "${DC_FQDN}" 2>/dev/null \
        | awk -F': ' '/^Realm:/ {print $2}' \
        | tr -d ' \r')
    if [ -z "${DISCOVERED_REALM}" ]; then
        echo "ERROR: 'net ads info -S ${DC_FQDN}' returned no Realm field." >&2
        echo "       Cannot proceed without an authoritative realm from the DC." >&2
        echo "       Verify DC_FQDN points at a reachable Domain Controller (not the AD zone)" >&2
        echo "       and that CLDAP (UDP/389) to it is not blocked." >&2
        exit 1
    fi
    REALM="${DISCOVERED_REALM}"
    echo "  Realm reported by DC: ${REALM}"
fi

if [ -z "${DOMAIN_NAME}" ]; then
    DOMAIN_NAME="$(echo "${REALM}" | tr '[:upper:]' '[:lower:]')"
fi

echo "=========================================="
echo "Joining domain (member mode)"
echo "  DC FQDN:      ${DC_FQDN}"
echo "  DNS domain:   ${DOMAIN_NAME}"
echo "  Realm:        ${REALM}"
echo "  NetBIOS:      ${NETBIOS_NAME}"
echo "  Join user:    ${AD_USERNAME}"
echo "=========================================="

mkdir -p /var/log/samba

[[ -f /etc/samba/smb.conf && ! -f /etc/samba/smb.conf.original ]] && \
    mv /etc/samba/smb.conf /etc/samba/smb.conf.original
[[ -f /etc/krb5.conf && ! -f /etc/krb5.conf.original ]] && \
    mv /etc/krb5.conf /etc/krb5.conf.original

cat > /etc/krb5.conf << EOF
[libdefaults]
    default_realm = ${REALM}
    dns_lookup_realm = false
    dns_lookup_kdc = false
    udp_preference_limit = 0

[realms]
    ${REALM} = {
        kdc = ${DC_FQDN}
        admin_server = ${DC_FQDN}
        default_domain = ${REALM}
    }

[domain_realm]
    .${DOMAIN_NAME} = ${REALM}
    ${DOMAIN_NAME} = ${REALM}
EOF

touch /etc/samba/smb.conf
SAMBA_CONF=/etc/samba/smb.conf
crudini --set ${SAMBA_CONF} global "security" "ads"
crudini --set ${SAMBA_CONF} global "realm" "${REALM}"
crudini --set ${SAMBA_CONF} global "workgroup" "${NETBIOS_NAME}"
crudini --set ${SAMBA_CONF} global "client ntlmv2 auth" "yes"
crudini --set ${SAMBA_CONF} global "ntlm auth" "yes"
crudini --set ${SAMBA_CONF} global "winbind use default domain" "yes"
crudini --set ${SAMBA_CONF} global "machine password timeout" "${MACHINE_PASSWORD_TIMEOUT:-2592000}"

echo ""
echo "Generated smb.conf:"
cat ${SAMBA_CONF}
echo ""

net ads join -S "${DC_FQDN}" -U"${AD_USERNAME}"%"${AD_PASSWORD}"
net ads testjoin

pkill winbindd 2>/dev/null || true
/usr/sbin/winbindd -D
sleep 3

echo ""
echo "Verifying Kerberos trust with DC..."
wbinfo -t
echo ""

echo "=========================================="
echo "Domain join complete."
echo "  - Machine credentials: /var/lib/samba/private/secrets.tdb"
echo "  - winbindd running; libwbclient ready."
echo "=========================================="
