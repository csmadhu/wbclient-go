#!/bin/bash
set -e

LOG_DIR="/usr/src/wbclient/docs/logs"
mkdir -p "$LOG_DIR"

exec /usr/local/bin/wbclient 2>&1 | tee "$LOG_DIR/wbclient.log"
