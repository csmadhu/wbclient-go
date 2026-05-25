# wbclient-go

A Go library and HTTP service for Samba/Winbind authentication using libwbclient. This project provides user authentication via plaintext and MSCHAPv2 challenge-response methods against Active Directory domains.

## Table of Contents

- [Overview](#overview)
- [Quick Start](#quick-start)
- [Project Structure](#project-structure)
- [Prerequisites](#prerequisites)
- [Installation](#installation)
- [Configuration](#configuration)
- [Running the Service](#running-the-service)
- [API Endpoints](#api-endpoints)
- [Testing](#testing)
- [Logging](#logging)
- [Scripts](#scripts)
- [Docker Setup](#docker-setup)
- [Development](#development)

## Overview

wbclient-go is a Go wrapper around libwbclient (Samba Winbind client library) that provides:
- HTTP API for user authentication
- Support for plaintext and MSCHAPv2 authentication
- Integration with Active Directory via Samba Winbind
- Flexible logging with text and JSON formats

## Quick Start

### Run the Service (dev / test)

`run-service.sh` and `scripts/run-service.sh.dev` both build the image and
start the container via `docker/service/docker-compose.yml`. They are dev
and test conveniences — for production, build the image from
`docker/service/Dockerfile` and run it on your orchestrator (see
[scripts/README.md](scripts/README.md) for pod requirements).

Domain join is now an API call (`POST /samba.domain.join`), not a script
flag.

```bash
# Start the service
./run-service.sh --api-token "your-secure-token" --port 8080

# Service available at http://localhost:8080
# View logs:      docker logs -f wbclient-service
```

### Test API with curl

```bash
curl -X POST http://localhost:8080/samba.user.validate \
  -H "Authorization: your-secure-token" \
  -H "Content-Type: application/json" \
  -d '{
    "username": "testuser",
    "password": "TestPass123",
    "domain": "CORP",
    "isPlainTextAuth": true
  }'
```

### Run Tests

```bash
./run-auth-tests.sh \
  -u testuser -p "TestPass123" -d CORP \
  --ad-username admin --ad-password "YourAdminPass" \
  --domain-name corp.example.com --dc-ip 10.0.1.100 \
  --dc-hostname dc.corp.example.com --workgroup CORP \
  --realm CORP.EXAMPLE.COM
```

## Project Structure

```
wbclient-go/
├── cmd/
│   └── wbclient/          # Main application entry point
│       └── main.go
├── docker/                # Docker configuration
│   ├── service/           # Service Docker files
│   │   ├── Dockerfile     # Builds the service image (production-ready)
│   │   ├── docker-compose.yml  # Dev/test orchestration for the image
│   │   └── entrypoint.sh  # Container startup (runs the binary)
│   ├── test/              # Test environment Docker files
│   │   ├── Dockerfile     # Test container image
│   │   └── docker-compose.yml  # Test orchestration
│   └── setup-samba-member.sh  # AD domain join script
├── log/                   # Logging package
│   ├── log.go            # Core logging interface
│   ├── text.go           # Text format logger
│   ├── json.go           # JSON format logger
│   ├── gcp.go            # GCP-compatible JSON logger
│   └── context.go        # Context-based logging
├── api.go                # HTTP API handlers
├── auth.go               # CGO bindings to libwbclient
├── auth_test.go          # Authentication tests
├── crypto.go             # Cryptographic helpers
├── service.go            # HTTP server setup
├── wbclient.go           # Request/response types
├── go.mod                # Go module dependencies
├── run-auth-tests.sh     # Automated test runner script
└── run-service.sh        # Dev/test service runner (builds + brings up the image via docker-compose)
```

## Prerequisites

- **Go**: Version 1.26.1 or higher
- **Samba**: Version 4.19+ with libwbclient-dev
- **Active Directory**: Domain controller for authentication
- **Docker** (for testing): Docker and docker-compose

### System Dependencies (Linux)

```bash
apt-get install -y samba winbind libwbclient-dev build-essential pkg-config
```

### System Dependencies (macOS)

```bash
brew install samba
```

## Installation

1. Clone the repository:
```bash
git clone https://github.com/yourusername/wbclient-go.git
cd wbclient-go
```

2. Install Go dependencies:
```bash
go mod download
```

3. Build the application:
```bash
CGO_ENABLED=1 go build -o wbclient ./cmd/wbclient
```

## Configuration

The service is configured via environment variables.

### Required Environment Variables

| Variable | Description | Example |
|----------|-------------|---------|
| `WBCLIENT_API_TOKEN` | API authentication token | `your-secure-token-here` |
| `WBCLIENT_PORT` | HTTP server port | `8080` |

### Optional Environment Variables (Debugging/Development)

| Variable | Description | Default | Example |
|----------|-------------|---------|---------|
| `LOG_ENCODING` | Log format: `text` or `json` | `text` | `json` |
| `ENABLE_DEBUG_LOG` | Enable debug logging | `false` | `true` |
| `PROJECT_ID` | GCP project ID (for JSON logging) | - | `my-gcp-project` |

## Running the Service

### As HTTP Server

Set required environment variables and run:

```bash
export WBCLIENT_API_TOKEN="dummy-token-abc123xyz"
export WBCLIENT_PORT="8080"
export LOG_ENCODING="json"           # Optional: use JSON logging
export ENABLE_DEBUG_LOG="false"      # Optional: disable debug logs

./wbclient
```

Or run directly with Go:

```bash
CGO_ENABLED=1 go run ./cmd/wbclient
```

### With Debug Logging

Enable detailed debug output for troubleshooting:

```bash
export WBCLIENT_API_TOKEN="dummy-token-abc123xyz"
export WBCLIENT_PORT="8080"
export ENABLE_DEBUG_LOG="true"        # Enable debug logs
export LOG_ENCODING="text"            # Use human-readable text format

./wbclient
```

The service will start and listen on the configured port. You should see:
```
2024/01/15 10:30:00 INFO wbclient - started at 8080
```

## API Endpoints

All endpoints require the `Authorization` header with the value matching `WBCLIENT_API_TOKEN`.

### 1. User Validation (Password Authentication)

**Endpoint:** `POST /samba.user.validate`

Authenticates a user with plaintext or MSCHAPv2 challenge-response.

**Request Headers:**
```
Authorization: dummy-token-abc123xyz
Content-Type: application/json
X_WBCLIENT_REQ_META: {"request_id":"req-123","correlation_id":"corr-456"}
```

**Request Body:**
```json
{
  "username": "testuser",
  "password": "TestPass123",
  "domain": "TESTDOMAIN",
  "isPlainTextAuth": true
}
```

**Response:**
```json
{
  "success": true,
  "errorCode": 0,
  "errorMessage": "",
  "ntHashHash": [0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0]
}
```

### 2. MSCHAPv2 Authentication

**Endpoint:** `POST /samba.user.auth`

Authenticates using MSCHAPv2 challenge and response.

**Request Body:**
```json
{
  "username": "testuser",
  "domain": "TESTDOMAIN",
  "challenge": [1,2,3,4,5,6,7,8],
  "response": [/* 24-byte response array */]
}
```

**Response:**
```json
{
  "success": true,
  "errorCode": 0,
  "errorMessage": "",
  "ntHashHash": [/* 16-byte hash array */]
}
```

### 3. Domain Join

**Endpoint:** `POST /samba.domain.join`

Joins the running pod/container to an Active Directory domain by executing
`scripts/domain-join.sh`. The handler is idempotent — if `net ads testjoin`
reports a healthy trust, it short-circuits with `{"success": true,
"errorMessage": "already joined"}` and does not re-run the script.

After a successful join, `winbindd` is running and the
`/samba.user.validate` / `/samba.user.auth` endpoints can authenticate
users immediately.

**Request Body:**
```json
{
  "dcfqdn":          "dc.corp.example.com",
  "netbiosName":     "CORP",
  "adUsername":      "joiner@corp.example.com",
  "adPassword":      "AdminPass123!",
  "passwordTimeout": 30
}
```

| Field | Required | Description |
|---|---|---|
| `dcfqdn` | Yes | Fully-qualified hostname of the Domain Controller (e.g. `dc.corp.example.com`). The realm is discovered authoritatively via `net ads info -S <dcfqdn>` — pass the AD DNS zone here and the join fails with a clear error. |
| `netbiosName` | Yes | Short NetBIOS domain name (e.g. `CORP`). |
| `adUsername` | Yes | AD account allowed to add machines to the domain. Used only at join time; not stored. |
| `adPassword` | Yes | Password for `adUsername`. |
| `passwordTimeout` | No | Machine-account password rotation interval, in **days**. Defaults to **30** if omitted or non-positive. The handler converts this to seconds and passes it to the script, which writes it to `smb.conf` as `machine password timeout`. |

**Response:**
```json
{
  "success": true,
  "errorMessage": ""
}
```

`errorMessage` is `"already joined"` on the idempotent short-circuit path,
or contains the script's failure reason on `HTTP 500`.

### 4. Domain Leave

**Endpoint:** `POST /samba.domain.leave`

Leaves the AD domain by executing `scripts/domain-leave.sh`. Stops
`winbindd`, runs `net ads leave` to remove the computer account from AD,
and wipes the persisted `secrets.tdb` / `smb.conf` / `krb5.conf` so the
container ends in a freshly-provisioned state. Idempotent — if not
currently joined, returns `{"success": true, "errorMessage": "not joined"}`
without touching anything.

**Request Body:**
```json
{
  "domain":     "dc.corp.example.com",
  "adUsername": "joiner@corp.example.com",
  "adPassword": "AdminPass123!"
}
```

| Field | Required | Description |
|---|---|---|
| `domain` | No | Informational; preserved in logs for audit clarity. The script reads realm/workgroup from the existing `smb.conf`. |
| `adUsername` | Yes | AD admin account with permission to delete the computer account. The machine account itself usually can't delete itself, so admin credentials are required. |
| `adPassword` | Yes | Password for `adUsername`. |

**Response:**
```json
{
  "success": true,
  "errorMessage": ""
}
```

### 5. Domain Join Status

**Endpoint:** `POST /samba.domain.join.status`

Reports whether the container currently has a healthy AD trust by running
`wbinfo -t` (via `scripts/domain-join-status.sh`) and surfacing the exit
code. No request body required.

A `success: true` response means winbind handshaked the DC successfully
and the auth endpoints are ready to serve traffic. `success: false` means
either the pod has not been joined yet, `winbindd` is not running, or the
machine credential is invalid — the `errorMessage` contains the
underlying `wbinfo -t` output for diagnosis.

Always returns HTTP 200 — the health verdict is in the body, not the
status line. Use it as a readiness probe.

**Request:**
```bash
curl -X POST http://localhost:8080/samba.domain.join.status \
  -H "Authorization: your-secure-token"
```

**Response (healthy):**
```json
{
  "success": true,
  "errorMessage": ""
}
```

**Response (not joined / winbind down):**
```json
{
  "success": false,
  "errorMessage": "wbinfo -t failed: exit status 1: ..."
}
```

> For the deeper details (script flow, persistence requirements, pod
> startup behaviour, `smb.conf` / `krb5.conf` flag reference), see
> [scripts/README.md](scripts/README.md).

## POST /samba.user.auth — Client Guide

This section is a complete reference for calling the MSCHAPv2 auth endpoint. The caller is responsible for computing `challenge` and `response` using an MSCHAPv2 library before calling this API.

### Request

```
POST /samba.user.auth
Content-Type: application/json
Authorization: <WBCLIENT_API_TOKEN>
```

**Body:**

```json
{
  "username":  "alice",
  "domain":    "CORP",
  "challenge": [1, 2, 3, 4, 5, 6, 7, 8],
  "response":  [68, 175, 224, 51, 150, 31, 12, 244, 107, 63, 185, 22, 9, 133, 74, 201, 30, 88, 176, 250, 39, 112, 5, 99]
}
```

| Field | Type | Size | Description |
|-------|------|------|-------------|
| `username` | string | — | AD username (without domain prefix) |
| `domain` | string | — | NetBIOS domain name (e.g. `CORP`) |
| `challenge` | integer array | 8 bytes | MSCHAPv2 challenge bytes, each 0–255 |
| `response` | integer array | 24 bytes | MSCHAPv2 NT-Response bytes, each 0–255 |

> **Encoding note:** All binary fields are JSON arrays of integers (0–255), **not** base64 strings.

**Optional metadata header** (values appear in server logs):

```
X_WBCLIENT_REQ_META: {"request_id":"r1","correlation_id":"c1","session_id":"s1","client_version":"1.0","user_agent":"myclient"}
```

### Response

**HTTP 200** — request was processed (check `success` field for auth outcome):

```json
{
  "success":      true,
  "errorCode":    0,
  "errorMessage": "",
  "ntHashHash":   [142, 220, 68, 31, 0, 177, 204, 88, 19, 55, 241, 99, 7, 130, 50, 12]
}
```

| Field | Type | Description |
|-------|------|-------------|
| `success` | bool | `true` if authentication succeeded |
| `errorCode` | int | `0` = success; see table below |
| `errorMessage` | string | Human-readable Winbind error, e.g. `"Password expired [0xC0000071]"` |
| `ntHashHash` | integer array (16 bytes) | MD4(NTHash) — user session key from Winbind; non-zero only on success |

**Error codes:**

| `errorCode` | Meaning |
|-------------|---------|
| `0` | Success |
| `-1` | Authentication failure or domain not found |
| `-2` | Winbind service not available |
| `-648` | Password expired or must change |

**Other HTTP status codes:**

| Status | Cause |
|--------|-------|
| `400` | Malformed JSON body or missing required fields |
| `401` | Missing or incorrect `Authorization` header |
| `500` | Internal encoding error |

### Go HTTP Client Example

```go
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
)

type UserAuthReq struct {
	Username  string  `json:"username"`
	Domain    string  `json:"domain"`
	Challenge [8]byte `json:"challenge"`
	Response  []byte  `json:"response"`
}

type UserAuthResp struct {
	Success      bool     `json:"success"`
	ErrorCode    int      `json:"errorCode"`
	ErrorMessage string   `json:"errorMessage"`
	NTHashHash   [16]byte `json:"ntHashHash"`
}

func callSambaUserAuth(baseURL, token string, req UserAuthReq) (*UserAuthResp, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	httpReq, err := http.NewRequest(http.MethodPost, baseURL+"/samba.user.auth", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", token)

	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status: %s", resp.Status)
	}

	var result UserAuthResp
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return &result, nil
}

func main() {
	// challenge and response are computed by your MSCHAPv2 library
	req := UserAuthReq{
		Username:  "alice",
		Domain:    "CORP",
		Challenge: [8]byte{1, 2, 3, 4, 5, 6, 7, 8},
		Response:  []byte{ /* 24 bytes from MSCHAPv2 */ },
	}

	resp, err := callSambaUserAuth("http://localhost:8080", "your-api-token", req)
	if err != nil {
		fmt.Println("error:", err)
		return
	}
	fmt.Printf("success=%v errorCode=%d errorMessage=%q\n", resp.Success, resp.ErrorCode, resp.ErrorMessage)
}
```

## Testing

### Running Tests in Docker (Recommended)

The easiest way to run tests is using the automated `run-auth-tests.sh` script. See the [Scripts](#scripts) section for detailed usage.

**Quick example:**

```bash
./run-auth-tests.sh \
  -u testuser -p "TestPass123" -d TESTDOMAIN \
  --ad-username admin --ad-password "AdminPass" \
  --domain-name testdomain.local --dc-ip 10.0.1.100 \
  --dc-hostname dc.testdomain.local --workgroup TESTDOMAIN \
  --realm TESTDOMAIN.LOCAL
```

This script automatically sets up the entire test environment including domain join.

### Running Tests Locally

If you have Samba/Winbind already configured on your system, you can run tests directly:

```bash
TEST_USERNAME=testuser TEST_PASSWORD="TestPass123" TEST_DOMAIN=TESTDOMAIN \
  CGO_ENABLED=1 go test -v ./...
```

**Optional:** Enable debug logging during tests:

```bash
ENABLE_DEBUG_LOG=true TEST_USERNAME=testuser TEST_PASSWORD="TestPass123" \
  TEST_DOMAIN=TESTDOMAIN CGO_ENABLED=1 go test -v ./...
```

## Logging

### Log Formats

#### Text Format (Default)

Human-readable format suitable for development:

```bash
export LOG_ENCODING="text"
```

Example output:
```
2024/01/15 10:30:00 INFO wbclient - started at 8080
2024/01/15 10:30:05 INFO [request_id=req-123] wbclient - process url[/samba.user.validate]
2024/01/15 10:30:05 DEBUG [request_id=req-123] wbclient - authentication attempt for user testuser
2024/01/15 10:30:05 INFO [request_id=req-123] wbclient - url[/samba.user.validate] completed
```

#### JSON Format

Structured logging compatible with GCP Cloud Logging:

```bash
export LOG_ENCODING="json"
export PROJECT_ID="my-gcp-project"  # Optional: for GCP trace integration
```

Example output:
```json
{"timestamp":"2024-01-15T10:30:00Z","severity":"INFO","message":"wbclient - started at 8080"}
{"timestamp":"2024-01-15T10:30:05Z","severity":"INFO","message":"wbclient - process url[/samba.user.validate]","logging.googleapis.com/labels":{"request_id":"req-123"}}
```

### Debug Logging

Enable debug logs for detailed troubleshooting:

```bash
export ENABLE_DEBUG_LOG="true"
```

Debug logs include:
- Detailed authentication flow
- Winbind communication details
- Request/response processing steps

### Request Metadata

Add custom metadata to logs via the `X_WBCLIENT_REQ_META` header:

```bash
curl -X POST http://localhost:8080/samba.user.validate \
  -H "Authorization: dummy-token-abc123xyz" \
  -H "Content-Type: application/json" \
  -H 'X_WBCLIENT_REQ_META: {"request_id":"req-001","correlation_id":"corr-001","session_id":"sess-123"}' \
  -d '{
    "username": "testuser",
    "password": "TestPass123",
    "domain": "TESTDOMAIN",
    "isPlainTextAuth": true
  }'
```

## Scripts

The repository includes two main automation scripts for different use cases:

### 1. `run-service.sh` — Dev/Test Service Runner

Builds the service image (`docker/service/Dockerfile`) and starts the
container via `docker/service/docker-compose.yml`. It is a dev/test
convenience — production deployments build the same Dockerfile and run
the resulting image on their own orchestrator (k8s, etc.).

Domain join is performed at runtime by calling `POST /samba.domain.join`
against the running service; the script does not take AD credentials.

**What it does:**
1. Removes any pre-existing `wbclient-service` container
2. `docker build -t wbclient-service-image -f docker/service/Dockerfile .`
3. `docker-compose -f docker/service/docker-compose.yml up -d`
4. Reports the container status and a sanity-check curl

**Usage:**

```bash
./run-service.sh --api-token "your-secure-token" --port 8080
```

**Options:**

| Flag | Description | Required | Default |
|------|-------------|----------|---------|
| `--api-token` | API token for WBCLIENT_API_TOKEN | No | `secret` |
| `--port` | Service port | No | `8080` |
| `-h, --help` | Show help message | No | - |

`scripts/run-service.sh.dev` is the same script intended for iterating on
the bind-mounted source tree via the same compose file.

**After running:**
- Service is available at `http://localhost:8080` (or your configured port)
- View logs: `docker logs -f wbclient-service`
- Stop service: `docker-compose -f docker/service/docker-compose.yml down`

### 2. `run-auth-tests.sh` - Automated Test Runner

This script runs the full authentication test suite in a Docker container with automatic domain join.

**Purpose:** Run Go tests against a real Active Directory domain to verify authentication functionality.

**What it does:**
1. Builds the test Docker image (`docker/test/Dockerfile`)
2. Starts a test container with docker-compose
3. Joins the Active Directory domain using provided credentials
4. Runs the Go test suite (`go test -v`)
5. Cleans up the container (optional)

**Usage:**

```bash
./run-auth-tests.sh \
  -u testuser \
  -p "TestPass123" \
  -d TESTDOMAIN \
  --ad-username adminuser \
  --ad-password "AdminPass456" \
  --domain-name testdomain.local \
  --dc-ip 10.0.1.100 \
  --dc-hostname dc.testdomain.local \
  --workgroup TESTDOMAIN \
  --realm TESTDOMAIN.LOCAL
```

**All Options:**

| Flag | Description | Required |
|------|-------------|----------|
| `-u USERNAME` | Test username (to authenticate in tests) | Yes |
| `-p PASSWORD` | Test password | Yes |
| `-d DOMAIN` | Domain name for tests | Yes |
| `--ad-username` | AD admin username for domain join | Yes |
| `--ad-password` | AD admin password | Yes |
| `--domain-name` | Full domain name (e.g., example.local) | Yes |
| `--dc-ip` | Domain Controller IP address | Yes |
| `--dc-hostname` | Domain Controller FQDN | Yes |
| `--workgroup` | NetBIOS workgroup name | Yes |
| `--realm` | Kerberos realm (uppercase) | Yes |
| `-n, --no-cleanup` | Keep container running after tests | No |
| `-h, --help` | Show help message | No |

**With no cleanup (for debugging):**

```bash
./run-auth-tests.sh \
  -u testuser -p "TestPass123" -d TESTDOMAIN \
  --ad-username admin --ad-password "AdminPass" \
  --domain-name testdomain.local --dc-ip 10.0.1.50 \
  --dc-hostname dc.testdomain.local --workgroup TESTDOMAIN \
  --realm TESTDOMAIN.LOCAL \
  --no-cleanup
```

This keeps the container running so you can debug issues or run additional tests manually.

## Docker Setup

The project includes two Docker setups: one for the service and one for the test suite.

### Building the Production Image

The production-ready image is built from `docker/service/Dockerfile`. **The
build context must be the project root** (the `.` at the end):

```bash
docker build -t wbclient-service-image -f docker/service/Dockerfile .
```

The Dockerfile bakes the source and the compiled binary, and its `CMD`
runs `docker/service/entrypoint.sh`. Run it on your orchestrator with
the required env vars (`WBCLIENT_API_TOKEN`, `WBCLIENT_PORT`) and an
appropriate persistence story for `/var/lib/samba/private/` — see
[scripts/README.md](scripts/README.md) for pod requirements.

### Running Locally for Dev/Test

`docker/service/docker-compose.yml` is the dev/test orchestration for the
same image. Use `./run-service.sh` (or `scripts/run-service.sh.dev`) to
build the image and bring the container up:

```bash
./run-service.sh --api-token "your-secure-token" --port 8080
```

**Script Options:**

| Flag | Description | Required | Default |
|------|-------------|----------|---------|
| `--api-token` | API token for WBCLIENT_API_TOKEN | No | `secret` |
| `--port` | Service port for WBCLIENT_PORT | No | `8080` |

Domain join is performed at runtime via `POST /samba.domain.join`.

**Managing the Service:**

```bash
# View live logs
docker logs -f wbclient-service

# View persisted logs
tail -f ./docs/logs/wbclient.log

# Stop the service
docker stop wbclient-service

# Remove the service container
docker rm wbclient-service
```

**Advanced Configuration:**

You can configure logging and debugging by modifying the `docker/service/docker-compose.yml` file before running the script. Add these environment variables:

```yaml
environment:
  - LOG_ENCODING=json          # or "text" for human-readable logs
  - ENABLE_DEBUG_LOG=true      # Enable debug logging
  - PROJECT_ID=my-gcp-project  # For GCP Cloud Logging integration
```

Or set them when running docker-compose directly:

```bash
LOG_ENCODING=json ENABLE_DEBUG_LOG=true docker-compose -f docker/service/docker-compose.yml up -d
```

### Manual Docker Setup (Test Environment)

For testing or development, you can manually build and run the test container:

**Building the Test Docker Image:**

```bash
docker build -t wbclient-test-image -f docker/test/Dockerfile .
```

**Running with Docker Compose:**

```bash
docker-compose -f docker/test/docker-compose.yml up -d
```

**Manual Domain Join (Inside Container):**

```bash
# Set environment variables
export AD_USERNAME="adminuser"
export AD_PASSWORD="AdminPass456"
export DOMAIN_NAME="testdomain.local"
export DC_IP="10.0.1.100"
export DC_HOSTNAME="dc.testdomain.local"
export WORKGROUP="TESTDOMAIN"
export REALM="TESTDOMAIN.LOCAL"

# Run setup script
docker exec -it wbclient-test /setup-samba-member.sh
```

## Development

### Building from Source

```bash
CGO_ENABLED=1 go build -o wbclient ./cmd/wbclient
```

### Running Go Tests

```bash
CGO_ENABLED=1 go test -v ./...
```

### Code Organization

- **auth.go**: CGO bindings to libwbclient for Winbind authentication
- **api.go**: HTTP handlers and routing
- **service.go**: HTTP server initialization and configuration
- **crypto.go**: Cryptographic utilities for MSCHAPv2
- **wbclient.go**: Request/response type definitions
- **log/**: Logging package with text and JSON formatters
- **cmd/wbclient/**: Main application entry point
- **docker/setup-samba-member.sh**: Shell script for joining Samba to AD domain
- **docker/service/entrypoint.sh**: Service container startup script
- **run-auth-tests.sh**: Automated test runner
- **run-service.sh**: Automated service deployment

### Adding New Endpoints

1. Define request/response types in `wbclient.go`
2. Implement handler function in `api.go`
3. Register route in `initRoutes()` function in `api.go`

### Understanding the Docker Scripts

**setup-samba-member.sh**: This script is used inside Docker containers to:
- Configure networking and DNS for the domain
- Set up Kerberos authentication
- Configure SSSD (System Security Services Daemon)
- Configure Samba for AD member mode
- Join the domain using `net ads join`
- Start and verify Winbind

**docker/service/entrypoint.sh**: The service container entrypoint that:
- Checks for AD credentials and runs domain join if provided
- Builds the wbclient binary inside the container
- Starts the service with log file persistence to `./docs/logs/wbclient.log`

## License

See [LICENSE](LICENSE) file for details.

## Contributing

Contributions are welcome! Please ensure:
1. All tests pass: `CGO_ENABLED=1 go test -v ./...`
2. Code is properly formatted: `go fmt ./...`
3. CGO bindings are properly documented

## Troubleshooting

### Common Issues

**Issue: `libwbclient not found`**
```
Solution: Install libwbclient-dev
- Ubuntu/Debian: apt-get install libwbclient-dev
- macOS: brew install samba
```

**Issue: `Authentication fails with error code`**
```
Solution: Check domain join status
- Verify winbind is running: systemctl status winbind
- Test domain trust: wbinfo -t
- Verify user exists: wbinfo -u | grep username
```

**Issue: `Port already in use`**
```
Solution: Change WBCLIENT_PORT or kill existing process
- Check process: lsof -i :8080
- Use different port: export WBCLIENT_PORT="9090"
```

**Issue: `Unauthorized 401 response`**
```
Solution: Verify Authorization header matches WBCLIENT_API_TOKEN
```

## Support

For issues and questions, please open an issue on the GitHub repository.

