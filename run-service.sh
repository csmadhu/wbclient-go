#!/bin/bash

# Script to build and run wbclient service inside a Docker container
# Usage: ./run-service.sh --ad-username <user> --ad-password <pass> ... --api-token <token>

set -e  # Exit on error

# Variables
CONTAINER_NAME="wbclient-service"
IMAGE_NAME="wbclient-service-image"
MOUNT_PATH="/usr/src"
API_TOKEN=""
SERVICE_PORT="8080"

# AD/domain setup environment variables
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
Usage: $0 [SETUP OPTIONS] [SERVICE OPTIONS]

Required flags for domain setup:
    --ad-username       AD username for domain join
    --ad-password       AD password for domain join
    --domain-name       Domain name (e.g., testdomain.local)
    --dc-ip             Domain Controller IP address
    --dc-hostname       Domain Controller hostname
    --workgroup         NetBIOS workgroup name
    --realm             Kerberos realm

Service options:
    --api-token         API token for WBCLIENT_API_TOKEN (default: secret)
    --port              Service port for WBCLIENT_PORT (default: 8080)

Other options:
    -h, --help          Show this help message

Example:
    $0 --ad-username admin --ad-password "AdminPass" \\
       --domain-name testdomain.local --dc-ip 1.1.1.1 \\
       --dc-hostname dc.testdomain.local --workgroup TESTDOMAIN \\
       --realm TESTDOMAIN.LOCAL \\
       --api-token mysecrettoken --port 8080

EOF
    exit 1
}

# Parse arguments
while [[ $# -gt 0 ]]; do
    case $1 in
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
        --api-token)
            API_TOKEN="$2"
            shift 2
            ;;
        --port)
            SERVICE_PORT="$2"
            shift 2
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

# Validate required AD arguments
if [[ -z "$AD_USERNAME" || -z "$AD_PASSWORD" || -z "$DOMAIN_NAME" || -z "$DC_IP" || -z "$DC_HOSTNAME" || -z "$WORKGROUP" || -z "$REALM" ]]; then
    print_error "Missing required setup arguments: --ad-username, --ad-password, --domain-name, --dc-ip, --dc-hostname, --workgroup, --realm"
    usage
fi

# Apply defaults for service options
API_TOKEN="${API_TOKEN:-secret}"

# Remove existing container with same name
print_section "PRE-RUN CLEANUP"
if docker ps -a --format '{{.Names}}' | grep -q "^${CONTAINER_NAME}$"; then
    print_info "Container $CONTAINER_NAME already exists, removing it..."
    docker stop "$CONTAINER_NAME" >/dev/null 2>&1 || true
    docker rm "$CONTAINER_NAME" >/dev/null 2>&1 || true
    print_info "Existing container removed"
else
    print_info "No existing container found"
fi

# Step 1: Build Docker image
print_section "STEP 1: BUILDING DOCKER IMAGE"
print_info "Building image: $IMAGE_NAME"
print_info "Dockerfile: docker/service/Dockerfile"
docker build -t "$IMAGE_NAME" -f docker/service/Dockerfile .
print_info "Image built successfully"

# Step 2: Check for docker-compose, install if missing
print_section "STEP 2: CHECKING DOCKER COMPOSE"
if ! command -v docker-compose &>/dev/null; then
    print_info "docker-compose not found, installing via apt-get..."
    apt-get update -qq && apt-get install -y -qq docker-compose
    print_info "docker-compose installed: $(docker-compose version)"
else
    print_info "docker-compose found: $(docker-compose version)"
fi

# Step 3: Start container with docker-compose
print_section "STEP 3: STARTING SERVICE CONTAINER"
print_info "Running: docker-compose up -d"
print_info "Compose file: ./docker/service/docker-compose.yml"

export WBCLIENT_API_TOKEN="$API_TOKEN"
export WBCLIENT_PORT="$SERVICE_PORT"
export AD_USERNAME AD_PASSWORD DOMAIN_NAME DC_IP DC_HOSTNAME WORKGROUP REALM

docker-compose -f ./docker/service/docker-compose.yml up -d

# Give the container a moment to start or fail
sleep 3
if ! docker ps --format '{{.Names}}' | grep -q "^${CONTAINER_NAME}$"; then
    print_error "Container exited immediately. Last logs:"
    docker logs "$CONTAINER_NAME" 2>&1 || true
    exit 1
fi
print_info "Container is running"

# Step 4: Verify mount
print_section "STEP 4: VERIFYING CODE MOUNT"
print_info "Checking if code is mounted at $MOUNT_PATH..."
docker exec "$CONTAINER_NAME" ls -la "$MOUNT_PATH" >/dev/null 2>&1
print_info "Code mounted successfully"

# Step 5: Print service info and exit
print_section "SERVICE RUNNING"
print_info "Container:      $CONTAINER_NAME"
print_info "API token:      $API_TOKEN"
print_info "Port:           $SERVICE_PORT"
print_info "Stream logs:    docker logs -f $CONTAINER_NAME"
print_info "Persisted logs: tail -f ./docs/logs/wbclient.log"
print_info "Stop service:   docker stop $CONTAINER_NAME"
echo ""
