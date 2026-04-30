#!/bin/bash
set -e

MOUNT_PATH="/usr/src"
LOG_DIR="$MOUNT_PATH/docs/logs"
LOG_FILE="$LOG_DIR/wbclient.log"

mkdir -p "$LOG_DIR"

# Join domain if AD credentials are provided
if [[ -n "$AD_USERNAME" ]]; then
    echo "[entrypoint] Running domain join..."
    /setup-samba-member.sh
    echo "[entrypoint] Domain join complete"
fi

echo "[entrypoint] Building wbclient binary..."
cd "$MOUNT_PATH"
go build -buildvcs=false -o /usr/local/bin/wbclient ./cmd/wbclient
echo "[entrypoint] Build complete"

echo "[entrypoint] Starting wbclient service (logs -> $LOG_FILE)..."
exec /usr/local/bin/wbclient 2>&1 | tee "$LOG_FILE"
