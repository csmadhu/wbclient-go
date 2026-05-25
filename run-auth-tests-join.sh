#!/bin/bash
set -e

CONTAINER_NAME="wbclient-test"
IMAGE_NAME="wbclient-test-image"
COMPOSE_FILE="./docker/test/docker-compose-join.yml"
MOUNT_PATH="/usr/src"
USERNAME=""
PASSWORD=""
DOMAIN=""
NO_CLEANUP=false

AD_USERNAME=""
AD_PASSWORD=""
DC_IP=""
DC_HOSTNAME=""
WORKGROUP=""
DOMAIN_NAME=""

print_section() {
    echo ""
    echo "========================================================================"
    echo "  $1"
    echo "========================================================================"
    echo ""
}

print_info() {
    echo "[INFO] $1"
}

print_error() {
    echo "[ERROR] $1" >&2
}

usage() {
    cat <<EOF
Usage: $0 -u USERNAME -p PASSWORD -d DOMAIN [SETUP OPTIONS] [OPTIONS]

Pins the container's DNS to --dc-ip at docker-compose time and runs
scripts/domain-join.sh inside the container. See scripts/README.md.

Required flags for testing:
    -u USERNAME         Test username
    -p PASSWORD         Test password
    -d DOMAIN           Domain name for tests (NetBIOS)

Required flags for domain setup:
    --ad-username       AD username for domain join
    --ad-password       AD password for domain join
    --dc-ip             Domain Controller IP — wired into the container's
                        /etc/resolv.conf via compose's dns: directive
    --dc-hostname       Domain Controller FQDN (e.g., dc.testdomain.local).
                        DNS domain and Kerberos realm are derived from this.
    --workgroup         NetBIOS workgroup name (e.g., TESTDOMAIN)

Optional flags:
    -n, --no-cleanup    Keep container running after tests
    -h, --help          Show this help message

Example:
    $0 -u testuser -p "Pass123!" -d TESTDOMAIN \\
       --ad-username admin --ad-password "AdminPass" \\
       --dc-ip 1.1.1.1 --dc-hostname dc.testdomain.local \\
       --workgroup TESTDOMAIN

EOF
    exit 1
}

while [[ $# -gt 0 ]]; do
    case $1 in
        -u) USERNAME="$2"; shift 2 ;;
        -p) PASSWORD="$2"; shift 2 ;;
        -d) DOMAIN="$2"; shift 2 ;;
        --ad-username) AD_USERNAME="$2"; shift 2 ;;
        --ad-password) AD_PASSWORD="$2"; shift 2 ;;
        --dc-ip) DC_IP="$2"; shift 2 ;;
        --dc-hostname) DC_HOSTNAME="$2"; shift 2 ;;
        --workgroup) WORKGROUP="$2"; shift 2 ;;
        -n|--no-cleanup) NO_CLEANUP=true; shift ;;
        -h|--help) usage ;;
        *) print_error "Unknown option: $1"; usage ;;
    esac
done

if [[ -z "$USERNAME" || -z "$PASSWORD" || -z "$DOMAIN" ]]; then
    print_error "Missing required test arguments: -u, -p, -d"
    usage
fi

if [[ -z "$AD_USERNAME" || -z "$AD_PASSWORD" || -z "$DC_IP" || -z "$DC_HOSTNAME" || -z "$WORKGROUP" ]]; then
    print_error "Missing required setup arguments: --ad-username, --ad-password, --dc-ip, --dc-hostname, --workgroup"
    usage
fi

if [[ -z "$DOMAIN_NAME" ]]; then
    DOMAIN_NAME="${DC_HOSTNAME#*.}"
fi

export DC_IP
export DOMAIN_NAME

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

cleanup() {
    if [ "$NO_CLEANUP" = false ]; then
        print_section "CLEANUP"
        print_info "Stopping container (named volumes preserved for restart testing)..."
        docker-compose -f "$COMPOSE_FILE" down >/dev/null 2>&1 || true
        docker stop "$CONTAINER_NAME" >/dev/null 2>&1 || true
        docker rm "$CONTAINER_NAME" >/dev/null 2>&1 || true
        print_info "Removing image: $IMAGE_NAME"
        docker rmi "$IMAGE_NAME" >/dev/null 2>&1 || true
        print_info "Cleanup complete (named volumes preserved)"
        echo ""
    else
        print_section "CONTAINER KEPT RUNNING"
        print_info "Container $CONTAINER_NAME is still running (--no-cleanup)"
        print_info "Try: docker restart $CONTAINER_NAME"
        print_info "     → entrypoint should auto-start winbindd from persisted secrets.tdb"
        print_info "To inspect: docker exec $CONTAINER_NAME wbinfo -t"
        print_info "To stop:    docker-compose -f $COMPOSE_FILE down"
        print_info "To wipe state: docker-compose -f $COMPOSE_FILE down -v"
        echo ""
    fi
}

trap cleanup EXIT

print_section "PRE-RUN CLEANUP"
if docker ps -a --format '{{.Names}}' | grep -q "^${CONTAINER_NAME}$"; then
    print_info "Container $CONTAINER_NAME already exists, removing it..."
    docker stop "$CONTAINER_NAME" >/dev/null 2>&1 || true
    docker rm "$CONTAINER_NAME" >/dev/null 2>&1 || true
    print_info "Existing container removed"
else
    print_info "No existing container found"
fi

print_section "STARTING AUTHENTICATION TESTS (DNS pre-pinned via compose)"
print_info "Image:        $IMAGE_NAME"
print_info "Container:    $CONTAINER_NAME"
print_info "Compose file: $COMPOSE_FILE"
print_info "Mount path:   $MOUNT_PATH"
print_info "Username:     $USERNAME"
print_info "Domain:       $DOMAIN"
print_info "DC IP (DNS):  $DC_IP"
print_info "DNS search:   $DOMAIN_NAME"

print_section "STEP 1: BUILDING DOCKER IMAGE"
print_info "Building image: $IMAGE_NAME"
print_info "Dockerfile: docker/test/Dockerfile"
docker build -t "$IMAGE_NAME" -f docker/test/Dockerfile .
print_info "Image built successfully"

print_section "STEP 2: STARTING DOCKER CONTAINER WITH DC AS DNS"

if ! command -v docker-compose &>/dev/null; then
    print_info "docker-compose not found, installing via apt-get..."
    apt-get update -qq && apt-get install -y -qq docker-compose
    print_info "docker-compose installed: $(docker-compose version)"
else
    print_info "docker-compose found: $(docker-compose version)"
fi

print_info "Running: DC_IP=$DC_IP DOMAIN_NAME=$DOMAIN_NAME docker-compose -f $COMPOSE_FILE up -d"
docker-compose -f "$COMPOSE_FILE" up -d
print_info "Container started; /etc/resolv.conf should already point at $DC_IP"

print_section "STEP 2b: VERIFYING CONTAINER DNS"
sleep 2
print_info "Container /etc/resolv.conf:"
docker exec "$CONTAINER_NAME" cat /etc/resolv.conf || true
print_info "Resolving DC hostname ($DC_HOSTNAME) from inside container..."
if ! docker exec "$CONTAINER_NAME" getent hosts "$DC_HOSTNAME" >/dev/null 2>&1; then
    print_error "DC hostname $DC_HOSTNAME did not resolve via the container's DNS."
    print_error "Check that --dc-ip ($DC_IP) is the AD DNS server and reachable from the container."
    exit 1
fi
print_info "DC hostname resolves correctly"

print_section "STEP 3: JOINING DOMAIN (via scripts/domain-join.sh)"
if docker exec "$CONTAINER_NAME" wbinfo -t >/dev/null 2>&1; then
    print_info "Container already joined and trust verified — skipping net ads join."
else
    print_info "No valid join detected. Running /usr/src/scripts/domain-join.sh..."
    docker exec \
        -e AD_USERNAME="$AD_USERNAME" \
        -e AD_PASSWORD="$AD_PASSWORD" \
        -e DC_FQDN="$DC_HOSTNAME" \
        -e NETBIOS_NAME="$WORKGROUP" \
        "$CONTAINER_NAME" \
        bash -c "/usr/src/scripts/domain-join.sh"
    print_info "Domain join completed"
fi

print_section "STEP 4: VERIFYING CODE MOUNT"
print_info "Checking if code is mounted at $MOUNT_PATH..."
docker exec "$CONTAINER_NAME" ls -la "$MOUNT_PATH" >/dev/null 2>&1
print_info "Code mounted successfully"

print_section "STEP 5: RUNNING AUTHENTICATION TESTS"
print_info "Running: docker exec with TEST_USERNAME=$USERNAME TEST_PASSWORD=*** TEST_DOMAIN=$DOMAIN"
print_info "Command: go test -v -run ."
echo ""

docker exec \
    -e TEST_USERNAME="$USERNAME" \
    -e TEST_PASSWORD="$PASSWORD" \
    -e TEST_DOMAIN="$DOMAIN" \
    "$CONTAINER_NAME" \
    sh -c "cd $MOUNT_PATH && CGO_ENABLED=1 go test -v -run ."

TEST_EXIT_CODE=$?

print_section "TEST RESULTS"
if [ $TEST_EXIT_CODE -eq 0 ]; then
    print_info "✓ All tests PASSED"
    echo ""
    exit 0
else
    print_error "✗ Tests FAILED with exit code $TEST_EXIT_CODE"
    echo ""
    exit $TEST_EXIT_CODE
fi
