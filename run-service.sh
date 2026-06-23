#!/bin/bash

# Builds the service image and starts it via docker run with DNS pointed
# at the Domain Controller for AD resolution.
#
# Usage: ./run-service.sh --dc-ip <ip> --domain-name <domain> [--api-token <token>] [--port <port>]

set -e

CONTAINER_NAME="wbclient-service"
IMAGE_NAME="wbclient-service-image"
DOCKERFILE="docker/service/Dockerfile"
API_TOKEN=""
SERVICE_PORT="8080"
SAMBA_LOG_LEVEL="10"
DC_IP=""
DOMAIN_NAME=""

print_section() {
    echo ""
    echo "========================================================================"
    echo "  $1"
    echo "========================================================================"
    echo ""
}

print_info()  { echo "[INFO] $1"; }
print_error() { echo "[ERROR] $1" >&2; }

usage() {
    cat <<EOF
Usage: $0 [OPTIONS]

Required options:
    --dc-ip             IP address of the Domain Controller (used for container DNS)
    --domain-name       DNS domain name for search suffix (e.g. corp.example.com)

Service options:
    --api-token         API token for WBCLIENT_API_TOKEN (default: secret)
    --port              Service port for WBCLIENT_PORT (default: 8080)
    --samba-log-level   Samba log verbosity 0-10 (default: 10)

Other options:
    -h, --help          Show this help message

Notes:
    - Container uses --dns and --dns-search pointed at the DC for AD resolution.
    - Persistence of samba/krb5 state is not handled here; mount volumes at
      the deployment layer (PVC in k8s) if you need join state to survive restarts.

Example:
    $0 --dc-ip 10.0.0.5 --domain-name corp.example.com --api-token mysecrettoken --port 8080
EOF
    exit 1
}

while [[ $# -gt 0 ]]; do
    case $1 in
        --dc-ip)      DC_IP="$2"; shift 2 ;;
        --domain-name) DOMAIN_NAME="$2"; shift 2 ;;
        --api-token)  API_TOKEN="$2"; shift 2 ;;
        --port)       SERVICE_PORT="$2"; shift 2 ;;
        --samba-log-level) SAMBA_LOG_LEVEL="$2"; shift 2 ;;
        -h|--help)    usage ;;
        *)            print_error "Unknown option: $1"; usage ;;
    esac
done

if [[ -z "$DC_IP" || -z "$DOMAIN_NAME" ]]; then
    print_error "--dc-ip and --domain-name are required"
    usage
fi

API_TOKEN="${API_TOKEN:-secret}"

print_section "PRE-RUN CLEANUP"
if docker ps -a --format '{{.Names}}' | grep -q "^${CONTAINER_NAME}$"; then
    print_info "Container $CONTAINER_NAME already exists, removing it..."
    docker stop "$CONTAINER_NAME" >/dev/null 2>&1 || true
    docker rm "$CONTAINER_NAME" >/dev/null 2>&1 || true
    print_info "Existing container removed"
else
    print_info "No existing container found"
fi

print_section "STEP 1: BUILDING DOCKER IMAGE"
print_info "Building image: $IMAGE_NAME"
print_info "Dockerfile:     $DOCKERFILE"
docker build -t "$IMAGE_NAME" -f "$DOCKERFILE" .
print_info "Image built successfully"

print_section "STEP 2: STARTING SERVICE CONTAINER"
print_info "DC IP:        $DC_IP"
print_info "Domain:       $DOMAIN_NAME"

docker run -d \
    --name "$CONTAINER_NAME" \
    -p "$SERVICE_PORT:$SERVICE_PORT" \
    --dns "$DC_IP" \
    --dns-search "$DOMAIN_NAME" \
    -v "$(pwd):/usr/src/wbclient" \
    -v "go-modules-svc:/root/go/pkg/mod" \
    -v "go-cache-svc:/root/.cache" \
    -v "/sys/fs/cgroup:/sys/fs/cgroup:rw" \
    -v "/etc/localtime:/etc/localtime:ro" \
    -v "samba-config:/etc/samba" \
    -v "samba-private:/var/lib/samba/private" \
    -v "samba-logs:/var/log/samba" \
    --privileged \
    -e "WBCLIENT_API_TOKEN=$API_TOKEN" \
    -e "WBCLIENT_PORT=$SERVICE_PORT" \
    -e "SAMBA_LOG_LEVEL=$SAMBA_LOG_LEVEL" \
    -t \
    --stop-signal SIGRTMIN+3 \
    --tmpfs /run \
    --tmpfs /run/lock \
    "$IMAGE_NAME"

sleep 3
if ! docker ps --format '{{.Names}}' | grep -q "^${CONTAINER_NAME}$"; then
    print_error "Container exited immediately. Last logs:"
    docker logs "$CONTAINER_NAME" 2>&1 || true
    exit 1
fi
print_info "Container is running"

print_section "SERVICE RUNNING"
print_info "Container:    $CONTAINER_NAME"
print_info "API token:    $API_TOKEN"
print_info "Port:         $SERVICE_PORT"
print_info "DNS:          $DC_IP (search: $DOMAIN_NAME)"
print_info "Stream logs:  docker logs -f $CONTAINER_NAME"
print_info "Stop service: docker stop $CONTAINER_NAME && docker rm $CONTAINER_NAME"
echo ""
print_info "Sanity check:"
print_info "  curl -H 'Authorization: $API_TOKEN' -X POST http://localhost:$SERVICE_PORT/samba.domain.join.status"
echo ""
