FROM ubuntu:24.04

# Prevent interactive prompts
ENV DEBIAN_FRONTEND=noninteractive

# Install Samba, libwbclient, Go, and AD tools
RUN apt-get update && apt-get install -y \
    samba-common-bin \
    winbind \
    libwbclient-dev \
    libwbclient0 \
    libtalloc2 \
    libtalloc-dev \
    krb5-user \
    crudini \
    golang-go \
    build-essential \
    pkg-config \
    git \
    dnsutils \
    iputils-ping \
    && apt-get clean

RUN mkdir -p /var/lib/samba/private && chmod 700 /var/lib/samba/private

ENV PATH="/usr/local/go/bin:$PATH"
ENV CGO_ENABLED=1

RUN net --version && go version

WORKDIR /usr/src/wbclient

COPY . .

RUN go mod download \
 && chmod +x scripts/*.sh docker/service/entrypoint.sh \
 && go build -buildvcs=false -o /usr/local/bin/wbclient ./cmd/wbclient \
 && find . -mindepth 1 -not -path './scripts/*' -not -path './scripts' \
      -not -path './docker/service/entrypoint.sh' -not -path './docker/service' -not -path './docker' \
      -delete

CMD ["/usr/src/wbclient/docker/service/entrypoint.sh"]
