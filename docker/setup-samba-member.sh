#!/bin/bash
set -e

# ============================================
# Samba Domain Member Setup Script
# ============================================
# This script sets up a Samba domain member in a Docker container
# Based on actual working configuration from testing
#
# Usage:
#   export AD_USERNAME="testadmin"
#   export AD_PASSWORD="Pass123!"
#   export DOMAIN_NAME="testdomain.local"
#   export DC_IP="1.1.1.1"
#   export DC_HOSTNAME="testdomain-dc.testdomain.local"
#   export WORKGROUP="TESTDOMAIN"
#   export REALM="TESTDOMAIN.COM"
#   ./setup-samba-member.sh
# ============================================

# Configuration
export AD_USERNAME="${AD_USERNAME}"
export AD_PASSWORD="${AD_PASSWORD}"
export DOMAIN_NAME="${DOMAIN_NAME}"
export DC_IP="${DC_IP}"
export DC_HOSTNAME="${DC_HOSTNAME}"
export WORKGROUP="${WORKGROUP}"
export REALM="${REALM}"

echo "=========================================="
echo "Samba Domain Member Setup"
echo "=========================================="
echo "Domain: ${DOMAIN_NAME}"
echo "DC: ${DC_HOSTNAME} (${DC_IP})"
echo "Workgroup: ${WORKGROUP}"
echo "Realm: ${REALM}"
echo "Username: ${AD_USERNAME}"
echo "=========================================="
echo ""

# ============================================
# Step 1: Create directories
# ============================================
echo "Step 1: Creating directories..."
mkdir -p /var/log/samba

# ============================================
# Step 2: Backup configs
# ============================================
echo "Step 2: Backing up configs..."
[[ -f /etc/samba/smb.conf && ! -f /etc/samba/smb.conf.original ]] && \
    mv /etc/samba/smb.conf /etc/samba/smb.conf.original
[[ -f /etc/krb5.conf && ! -f /etc/krb5.conf.original ]] && \
    mv /etc/krb5.conf /etc/krb5.conf.original

# ============================================
# Step 3: Configure /etc/hosts
# ============================================
echo "Step 3: Configuring /etc/hosts..."
if ! grep -q "${DC_HOSTNAME}" /etc/hosts; then
    echo "${DC_IP} ${DC_HOSTNAME} ${DC_HOSTNAME%%.*} ${WORKGROUP}" >> /etc/hosts
fi

# ============================================
# Step 4: Configure /etc/resolv.conf
# ============================================
echo "Step 4: Configuring /etc/resolv.conf..."
cat > /etc/resolv.conf << EOF
nameserver ${DC_IP}
search ${DOMAIN_NAME}
options ndots:0
EOF

# ============================================
# Step 5: Configure Kerberos
# ============================================
echo "Step 5: Configuring Kerberos..."
cat > /etc/krb5.conf << EOF
[logging]
    default = FILE:/var/log/krb5.log
    kdc = FILE:/var/log/kdc.log
    admin_server = FILE:/var/log/kadmind.log

[libdefaults]
    default_realm = ${REALM}
    dns_lookup_realm = false
    dns_lookup_kdc = false
    udp_preference_limit = 0

[realms]
    ${REALM} = {
        kdc = ${DC_HOSTNAME}
        admin_server = ${DC_HOSTNAME}
        default_domain = ${REALM}
    }
    ${DOMAIN_NAME} = {
        kdc = ${DC_HOSTNAME}
        admin_server = ${DC_HOSTNAME}
        default_domain = ${DOMAIN_NAME}
    }
    ${WORKGROUP} = {
        kdc = ${DC_HOSTNAME}
        admin_server = ${DC_HOSTNAME}
        default_domain = ${REALM}
    }

[domain_realm]
    .${DOMAIN_NAME} = ${REALM}
    ${DOMAIN_NAME} = ${REALM}
EOF

# ============================================
# Step 6: Create empty smb.conf
# ============================================
echo "Step 6: Creating empty smb.conf..."
touch /etc/samba/smb.conf

# ============================================
# Step 7: Start D-Bus
# ============================================
echo "Step 7: Starting D-Bus..."
/etc/init.d/dbus start
sleep 2

# ============================================
# Step 8: Discover domain
# ============================================
echo "Step 8: Discovering domain..."
realm discover -v ${DC_HOSTNAME}

# ============================================
# Step 9: Join domain
# ============================================
echo "Step 9: Joining domain..."
printf "${AD_PASSWORD}" | realm join -v ${DOMAIN_NAME} \
    --user=${AD_USERNAME} \
    --install=/

# ============================================
# Step 10: Configure SSSD
# ============================================
echo "Step 10: Configuring SSSD..."
crudini --set /etc/sssd/sssd.conf "domain/${DOMAIN_NAME}" "ad_server" "${DC_IP}"
chmod 600 /etc/sssd/sssd.conf

# ============================================
# Step 11: Start SSSD
# ============================================
echo "Step 11: Starting SSSD..."
/etc/init.d/sssd start
sleep 3

# ============================================
# Step 12: Configure Samba
# ============================================
echo "Step 12: Configuring Samba..."
SAMBA_CONF=/etc/samba/smb.conf

# Based on ACTUAL working configuration (verified)
crudini --set ${SAMBA_CONF} global "security" "ads"
crudini --set ${SAMBA_CONF} global "realm" "${REALM}"
crudini --set ${SAMBA_CONF} global "workgroup" "${WORKGROUP}"
crudini --set ${SAMBA_CONF} global "winbind use default domain" "yes"
crudini --set ${SAMBA_CONF} global "winbind enum users" "yes"
crudini --set ${SAMBA_CONF} global "winbind enum groups" "yes"
crudini --set ${SAMBA_CONF} global "winbind refresh tickets" "yes"
crudini --set ${SAMBA_CONF} global "template shell" "/bin/bash"
crudini --set ${SAMBA_CONF} global "template homedir" "/home/%U"
crudini --set ${SAMBA_CONF} global "client ntlmv2 auth" "yes"
crudini --set ${SAMBA_CONF} global "ntlm auth" "yes"
crudini --set ${SAMBA_CONF} global "password server" "${DC_IP}"
crudini --set ${SAMBA_CONF} global "name resolve order" "host wins bcast"
crudini --set ${SAMBA_CONF} global "dns proxy" "no"

echo ""
echo "Generated smb.conf:"
cat ${SAMBA_CONF}
echo ""

# ============================================
# Step 13: Update NSSwitch
# ============================================
echo "Step 13: Updating NSSwitch..."
if ! grep -q "sss" /etc/nsswitch.conf; then
    sed -i 's/^\(passwd:.*\)$/\1 sss/' /etc/nsswitch.conf
    sed -i 's/^\(group:.*\)$/\1 sss/' /etc/nsswitch.conf
    sed -i 's/^\(shadow:.*\)$/\1 sss/' /etc/nsswitch.conf
fi

# ============================================
# Step 14: Get Kerberos ticket
# ============================================
echo "Step 14: Getting Kerberos ticket..."
echo "${AD_PASSWORD}" | kinit -V ${AD_USERNAME}@${REALM}

# ============================================
# Step 15: Join with Samba
# ============================================
echo "Step 15: Joining with Samba..."
net ads join -U"${AD_USERNAME}"%"${AD_PASSWORD}"
net ads testjoin

# ============================================
# Step 16: Restart Winbind
# ============================================
echo "Step 16: Restarting Winbind..."
pkill winbindd 2>/dev/null || true
/usr/sbin/winbindd -D
sleep 3

# ============================================
# Step 17: Verify
# ============================================
echo "=========================================="
echo "Verification Tests"
echo "=========================================="

echo ""
echo "1. Testing Winbind trust..."
wbinfo -t

echo ""
echo "2. Listing domain users (first 10)..."
wbinfo -u | head -10

echo ""
echo "3. Testing authentication..."
wbinfo --authenticate=${AD_USERNAME}%${AD_PASSWORD}

echo ""
echo "4. Checking domain info..."
wbinfo --domain-info=${WORKGROUP}

echo ""
echo "=========================================="
echo "Setup Complete!"
echo "=========================================="