FROM ubuntu:24.04

# Prevent interactive prompts
ENV DEBIAN_FRONTEND=noninteractive

# Install Samba, libwbclient, Go, and AD tools
RUN apt-get update && apt-get install -y \
    samba \
    smbclient \
    winbind \
    libwbclient-dev \
    samba-dsdb-modules \
    krb5-user \
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

ENV PATH="/usr/local/go/bin:$PATH"
ENV CGO_ENABLED=1

RUN samba -V && go version

COPY docker/setup-samba-member.sh /setup-samba-member.sh
COPY docker/service/entrypoint.sh /entrypoint.sh
RUN chmod +x /setup-samba-member.sh /entrypoint.sh

CMD ["/entrypoint.sh"]
