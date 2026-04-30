#!/bin/bash

# Script to run authentication tests inside a Docker container
# Usage: ./run-auth-tests.sh -i <image> -u <username> -p <password> -d <domain>

set -e  # Exit on error

# Variables
CONTAINER_NAME="wbclient-test"
IMAGE_NAME="wbclient-test-image"
MOUNT_PATH="/usr/src"
USERNAME=""
PASSWORD=""
DOMAIN=""
NO_CLEANUP=false

# Setup script environment variables
AD_USERNAME=""
AD_PASSWORD=""
DOMAIN_NAME=""
DC_IP=""
DC_HOSTNAME=""
WORKGROUP=""
REALM=""

# Print section separator
print_section() {
    echo ""
    echo "========================================================================"
    echo "  $1"
    echo "========================================================================"
    echo ""
}

# Print info message
print_info() {
    echo "[INFO] $1"
}

# Print error message
print_error() {
    echo "[ERROR] $1" >&2
}

# Show usage
usage() {
    cat <<EOF
Usage: $0 -u USERNAME -p PASSWORD -d DOMAIN [SETUP OPTIONS] [OPTIONS]

Required flags for testing:
    -u USERNAME         Test username
    -p PASSWORD         Test password
    -d DOMAIN           Domain name for tests

Required flags for domain setup:
    --ad-username       AD username for domain join
    --ad-password       AD password for domain join
    --domain-name       Domain name (e.g., testdomain.local)
    --dc-ip             Domain Controller IP address
    --dc-hostname       Domain Controller hostname
    --workgroup         NetBIOS workgroup name
    --realm             Kerberos realm

Optional flags:
    -n, --no-cleanup    Keep container running after tests
    -h, --help          Show this help message

Example:
    $0 -u testuser -p "Pass123!" -d TESTDOMAIN \\
       --ad-username admin --ad-password "AdminPass" \\
       --domain-name testdomain.local --dc-ip 1.1.1.1 \\
       --dc-hostname dc.testdomain.local --workgroup TESTDOMAIN \\
       --realm TESTDOMAIN.LOCAL

EOF
    exit 1
}

# Parse arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        -u)
            USERNAME="$2"
            shift 2
            ;;
        -p)
            PASSWORD="$2"
            shift 2
            ;;
        -d)
            DOMAIN="$2"
            shift 2
            ;;
        --ad-username)
            AD_USERNAME="$2"
            shift 2
            ;;
        --ad-password)
            AD_PASSWORD="$2"
            shift 2
            ;;
        --domain-name)
            DOMAIN_NAME="$2"
            shift 2
            ;;
        --dc-ip)
            DC_IP="$2"
            shift 2
            ;;
        --dc-hostname)
            DC_HOSTNAME="$2"
            shift 2
            ;;
        --workgroup)
            WORKGROUP="$2"
            shift 2
            ;;
        --realm)
            REALM="$2"
            shift 2
            ;;
        -n|--no-cleanup)
            NO_CLEANUP=true
            shift
            ;;
        -h|--help)
            usage
            ;;
        *)
            print_error "Unknown option: $1"
            usage
            ;;
    esac
done

# Validate required arguments
if [[ -z "$USERNAME" || -z "$PASSWORD" || -z "$DOMAIN" ]]; then
    print_error "Missing required test arguments: -u, -p, -d"
    usage
fi

if [[ -z "$AD_USERNAME" || -z "$AD_PASSWORD" || -z "$DOMAIN_NAME" || -z "$DC_IP" || -z "$DC_HOSTNAME" || -z "$WORKGROUP" || -z "$REALM" ]]; then
    print_error "Missing required setup arguments: --ad-username, --ad-password, --domain-name, --dc-ip, --dc-hostname, --workgroup, --realm"
    usage
fi

# Get absolute path of current directory
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Cleanup function
cleanup() {
    if [ "$NO_CLEANUP" = false ]; then
        print_section "CLEANUP"
        print_info "Stopping and removing container: $CONTAINER_NAME"
        docker stop "$CONTAINER_NAME" >/dev/null 2>&1 || true
        docker rm "$CONTAINER_NAME" >/dev/null 2>&1 || true
        print_info "Removing image: $IMAGE_NAME"
        docker rmi "$IMAGE_NAME" >/dev/null 2>&1 || true
        print_info "Cleanup completed"
        echo ""
    else
        print_section "CONTAINER KEPT RUNNING"
        print_info "Container $CONTAINER_NAME is still running (--no-cleanup flag used)"
        print_info "Image $IMAGE_NAME is still present (--no-cleanup flag used)"
        print_info "To view logs: docker logs $CONTAINER_NAME"
        print_info "To stop: docker stop $CONTAINER_NAME"
        print_info "To remove container: docker rm $CONTAINER_NAME"
        print_info "To remove image: docker rmi $IMAGE_NAME"
        echo ""
    fi
}

# Trap to ensure cleanup on exit
trap cleanup EXIT

# Check if container already exists and remove it
print_section "PRE-RUN CLEANUP"
if docker ps -a --format '{{.Names}}' | grep -q "^${CONTAINER_NAME}$"; then
    print_info "Container $CONTAINER_NAME already exists, removing it..."
    docker stop "$CONTAINER_NAME" >/dev/null 2>&1 || true
    docker rm "$CONTAINER_NAME" >/dev/null 2>&1 || true
    print_info "Existing container removed"
else
    print_info "No existing container found"
fi

# Start
print_section "STARTING AUTHENTICATION TESTS"
print_info "Image:      $IMAGE_NAME"
print_info "Container:  $CONTAINER_NAME"
print_info "Mount path: $MOUNT_PATH"
print_info "Username:   $USERNAME"
print_info "Domain:     $DOMAIN"

# Step 1: Build Docker image
print_section "STEP 1: BUILDING DOCKER IMAGE"
print_info "Building image: $IMAGE_NAME"
print_info "Dockerfile: docker/test/Dockerfile"
docker build -t "$IMAGE_NAME" -f docker/test/Dockerfile .

print_info "Image built successfully"

# Step 2: Run container with docker-compose
print_section "STEP 2: STARTING DOCKER CONTAINER"

# Check for docker-compose, install if missing
if ! command -v docker-compose &>/dev/null; then
    print_info "docker-compose not found, installing via apt-get..."
    apt-get update -qq && apt-get install -y -qq docker-compose
    print_info "docker-compose installed: $(docker-compose version)"
else
    print_info "docker-compose found: $(docker-compose version)"
fi

print_info "Running: docker-compose up -d"
print_info "Compose file: ./docker/test/docker-compose.yml"
docker-compose -f ./docker/test/docker-compose.yml up -d

print_info "Container started successfully"

# Step 3: Wait for container setup
print_section "STEP 3: WAITING FOR CONTAINER SETUP"
sleep 5
print_info "Running setup script with domain configuration..."
docker exec \
    -e AD_USERNAME="$AD_USERNAME" \
    -e AD_PASSWORD="$AD_PASSWORD" \
    -e DOMAIN_NAME="$DOMAIN_NAME" \
    -e DC_IP="$DC_IP" \
    -e DC_HOSTNAME="$DC_HOSTNAME" \
    -e WORKGROUP="$WORKGROUP" \
    -e REALM="$REALM" \
    "$CONTAINER_NAME" \
    sh -c "/setup-samba-member.sh" || true
print_info "Container should be ready now"

# Step 4: Verify mount
print_section "STEP 4: VERIFYING CODE MOUNT"
print_info "Checking if code is mounted at $MOUNT_PATH..."
docker exec "$CONTAINER_NAME" ls -la "$MOUNT_PATH" >/dev/null 2>&1
print_info "Code mounted successfully"

# Step 5: Run tests
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

# Step 6: Results
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
