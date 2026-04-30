FROM ubuntu:24.04

# Prevent interactive prompts
ENV DEBIAN_FRONTEND=noninteractive

# 1. Install Samba 4.19+, libwbclient, and AD DC tools
RUN apt-get update && apt-get install -y \
    samba \
    smbclient \
    winbind \
    libwbclient-dev \
    samba-dsdb-modules \
    krb5-user \
    # 2. Install Go and development tools
    golang-go \
    build-essential \
    pkg-config \
    git \
    libtalloc2 \
    libwbclient0 \
    libtalloc-dev \
    systemd \
    dnsutils \
    iputils-ping \
    libpam-winbind \
    libnss-winbind \
    vim \
    realmd \
    samba-common-bin \
    dbus \
    sssd \
    sssd-tools \
    libnss-sss \
    libpam-sss \
    crudini \
    adcli \
    && apt-get clean



RUN mkdir -p /var/lib/samba/private && chmod 700 /var/lib/samba/private

# Set the path
ENV PATH="/usr/local/go/bin:$PATH"

# 3. Verify native version is 4.19+
RUN samba -V && go version

# Copy scripts
COPY docker/setup-samba-member.sh /setup-samba-member.sh
RUN chmod +x /setup-samba-member.sh

# Keep container running in background
CMD ["tail", "-f", "/dev/null"]
