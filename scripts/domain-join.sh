#!/bin/bash
set -e

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
    if [[ -z "${DC_FQDN:-}" ]]; then
        die "VALIDATION_FAILED" "DC_FQDN is required (e.g. dc01.corp.example.com)"
    fi
    if [[ -z "${NETBIOS_NAME:-}" ]]; then
        die "VALIDATION_FAILED" "NETBIOS_NAME is required (e.g. CORP)"
    fi
    if [[ -z "${AD_USERNAME:-}" ]]; then
        die "VALIDATION_FAILED" "AD_USERNAME is required"
    fi
    if [[ -z "${AD_PASSWORD:-}" ]]; then
        die "VALIDATION_FAILED" "AD_PASSWORD is required"
    fi
}

discover_realm() {
    if [ -n "${REALM}" ]; then
        echo "==> Using provided realm: ${REALM}"
    else
        echo "==> Discovering realm (DC: ${DC_FQDN})"
        mkdir -p /etc/samba && touch /etc/samba/smb.conf
        DISCOVERED_REALM=$(net ads info -S "${DC_FQDN}" 2>/dev/null \
            | awk -F': ' '/^Realm:/ {print $2}' \
            | tr -d ' \r')
        if [ -z "${DISCOVERED_REALM}" ]; then
            die "REALM_DISCOVERY_FAILED" "'net ads info -S ${DC_FQDN}' returned no Realm. Verify DC_FQDN points at a reachable Domain Controller and CLDAP (UDP/389) is not blocked."
        fi
        REALM="${DISCOVERED_REALM}"
        echo "    Realm: ${REALM}"
    fi

    if [ -z "${DOMAIN_NAME}" ]; then
        DOMAIN_NAME="$(echo "${REALM}" | tr '[:upper:]' '[:lower:]')"
    fi
}

backup_configs() {
    echo "==> Backing up existing configs"
    [[ -f /etc/samba/smb.conf && ! -f /etc/samba/smb.conf.original ]] && \
        mv /etc/samba/smb.conf /etc/samba/smb.conf.original
    # cp (not mv) — Kubernetes single-file bind-mount returns EBUSY on rename/unlink.
    [[ -f /etc/krb5.conf && ! -f /etc/krb5.conf.original ]] && \
        cp /etc/krb5.conf /etc/krb5.conf.original
}

configure_kerberos() {
    echo "==> Configuring authentication (realm: ${REALM}, kdc: ${DC_FQDN})"
    if ! cat > /etc/krb5.conf << EOF
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
    then
        die "CONFIG_FAILED" "failed to write authentication config"
    fi
}

configure_samba() {
    echo "==> Configuring domain membership (realm: ${REALM}, workgroup: ${NETBIOS_NAME})"
    touch /etc/samba/smb.conf
    SAMBA_CONF=/etc/samba/smb.conf
    local output
    if ! output=$(
        crudini --set ${SAMBA_CONF} global "security" "ads" &&
        crudini --set ${SAMBA_CONF} global "realm" "${REALM}" &&
        crudini --set ${SAMBA_CONF} global "workgroup" "${NETBIOS_NAME}" &&
        crudini --set ${SAMBA_CONF} global "client ntlmv2 auth" "yes" &&
        crudini --set ${SAMBA_CONF} global "ntlm auth" "yes" &&
        crudini --set ${SAMBA_CONF} global "winbind use default domain" "yes" &&
        crudini --set ${SAMBA_CONF} global "machine password timeout" "${MACHINE_PASSWORD_TIMEOUT:-2592000}" &&
        crudini --set ${SAMBA_CONF} global "log level" "${SAMBA_LOG_LEVEL:-3}"
    2>&1); then
        die "CONFIG_FAILED" "${output}"
    fi
}

join_domain() {
    echo "==> Joining domain (DC: ${DC_FQDN}, user: ${AD_USERNAME})"
    local output
    if ! output=$(net ads join -S "${DC_FQDN}" -U"${AD_USERNAME}"%"${AD_PASSWORD}" 2>&1); then
        die "DOMAIN_JOIN_FAILED" "$(sanitize "${output}")"
    fi

    echo "==> Verifying domain membership"
    if ! output=$(net ads testjoin 2>&1); then
        die "DOMAIN_JOIN_VERIFY_FAILED" "$(sanitize "${output}")"
    fi
}

start_winbind() {
    echo "==> Starting domain service"
    pkill winbindd 2>/dev/null || true
    local output
    if ! output=$(/usr/sbin/winbindd -D 2>&1); then
        die "SERVICE_START_FAILED" "${output}"
    fi
    sleep 3
}

verify_trust() {
    echo "==> Verifying domain trust"
    local output
    if ! output=$(wbinfo -t 2>&1); then
        die "TRUST_VERIFICATION_FAILED" "$(sanitize "${output}")"
    fi
}

main() {
    validate_env
    discover_realm

    echo "=========================================="
    echo "Joining domain (member mode)"
    echo "  DC FQDN:      ${DC_FQDN}"
    echo "  DNS domain:   ${DOMAIN_NAME}"
    echo "  Realm:        ${REALM}"
    echo "  NetBIOS:      ${NETBIOS_NAME}"
    echo "  Join user:    ${AD_USERNAME}"
    echo "=========================================="

    mkdir -p /var/log/samba

    backup_configs
    configure_kerberos
    configure_samba
    join_domain
    start_winbind
    verify_trust

    echo "=========================================="
    echo "Domain join succeeded."
    echo "=========================================="
}

main "$@"
