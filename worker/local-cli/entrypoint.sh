#!/bin/bash
set -e

# We use the default data dir path to simplify things.
DATA_DIR="/root/.local/share/qiyuan-worker-default"
mkdir -p "$DATA_DIR"

if [ ! -f "$DATA_DIR/config.yaml" ]; then
    echo "Initializing worker config..."
    # Connect to the API container via internal docker network
    qiyuan-worker init --server "${WORKER_SERVER_URL:-http://api:8001}"
fi

if [ ! -f "$DATA_DIR/device.json" ]; then
    echo ""
    echo "=========================================================="
    echo "First time setup: Please pair this worker in the admin UI!"
    echo "Worker will print pairing code below..."
    echo "=========================================================="
    qiyuan-worker pair --display-name "docker-worker-01"
fi

echo "Starting Xvfb..."
Xvfb :99 -screen 0 1280x1024x24 &
export DISPLAY=:99
sleep 1

echo "Starting worker loop..."
exec qiyuan-worker run
