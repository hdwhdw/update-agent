#!/bin/bash

# Test script for upgrade-agent using Docker container
set -e

# Parse command line arguments
INTERACTIVE=false
while [[ $# -gt 0 ]]; do
  case $1 in
    -i|--interactive)
      INTERACTIVE=true
      shift
      ;;
    *)
      echo "Unknown option: $1"
      echo "Usage: $0 [-i|--interactive]"
      exit 1
      ;;
  esac
done

# Configuration
CONFIG_PATH="/tmp/config.yaml"
CONFIG_MOUNT_PATH="/etc/upgrade-agent/config.yaml"
GRPC_TARGET="10.250.0.101:8080"
FIRMWARE_SOURCE="/tmp/sonic.bin"
FIRMWARE_MOUNT_PATH="/firmware/sonic.bin"
UPDATE_MLNX_CPLD="true"
INITIAL_VERSION="1.0.0"
NEW_VERSION="1.1.0"
CONTAINER_NAME="upgrade-agent-test"

# Ensure the firmware directory exists
mkdir -p "$(dirname "${FIRMWARE_SOURCE}")"
touch ${FIRMWARE_SOURCE}

echo "Creating initial config file..."
cat > ${CONFIG_PATH} << EOF
grpcTarget: "${GRPC_TARGET}"
firmwareSource: "${FIRMWARE_MOUNT_PATH}"
updateMlnxCpldFw: ${UPDATE_MLNX_CPLD}
targetVersion: "${INITIAL_VERSION}"  # When this field is updated, it will trigger an update
EOF

echo "Building the Docker container..."
docker build -t upgrade-agent:latest .

echo "Starting upgrade agent container with initial version ${INITIAL_VERSION}..."

# Create a signal file directory if it doesn't exist
SIGNAL_DIR="/tmp/upgrade-agent-signals"
mkdir -p "${SIGNAL_DIR}"

# Remove any old signal files
rm -f "${SIGNAL_DIR}/container_ready" "${SIGNAL_DIR}/test_complete"

if [ "$INTERACTIVE" = true ]; then
  # Run in interactive mode with logs displayed directly in the terminal
  echo "Running in interactive mode. Press Ctrl+C to stop after testing is complete."
  docker run --name ${CONTAINER_NAME} \
    --network=host \
    -v ${CONFIG_PATH}:${CONFIG_MOUNT_PATH} \
    -v ${FIRMWARE_SOURCE}:${FIRMWARE_MOUNT_PATH} \
    upgrade-agent:latest &
  AGENT_CONTAINER_PID=$!
else
  # Run in background mode with logs redirected to file
  docker run --name ${CONTAINER_NAME} \
    --network=host \
    -v ${CONFIG_PATH}:${CONFIG_MOUNT_PATH} \
    -v ${FIRMWARE_SOURCE}:${FIRMWARE_MOUNT_PATH} \
    upgrade-agent:latest > /tmp/agent-output.log 2>&1 &
  AGENT_CONTAINER_PID=$!
  echo "Agent container started with PID: ${AGENT_CONTAINER_PID}"
  echo "You can view logs in real-time with:"
  echo "  - tail -f /tmp/agent-output.log"
  echo "  - ./test/monitor_logs.sh ${CONTAINER_NAME}"
fi

# Create a signal file that the monitor script can check for
# and write the container name to it
echo "${CONTAINER_NAME}" > "${SIGNAL_DIR}/container_ready"

echo "Waiting for agent to initialize (5 seconds)..."
sleep 5

# Display initial logs
echo "=== Initial agent logs ==="
docker logs ${CONTAINER_NAME}
echo "=========================="
echo "Note: You can also monitor logs in real-time using:"
echo "  ./test/monitor_logs.sh --wait"

echo "Updating config to trigger firmware update to version ${NEW_VERSION}..."
cat > ${CONFIG_PATH} << EOF
grpcTarget: "${GRPC_TARGET}"
firmwareSource: "${FIRMWARE_MOUNT_PATH}"
updateMlnxCpldFw: ${UPDATE_MLNX_CPLD}
targetVersion: "${NEW_VERSION}"  # When this field is updated, it will trigger an update
EOF

echo "Waiting for update process (180 seconds, including reboot and stabilization)..."
echo "You can also run './test/monitor_logs.sh --wait' in another terminal to follow logs."

# Show logs periodically during update if not in interactive mode
if [ "$INTERACTIVE" = false ]; then
  for _ in {1..36}; do
    echo "=== Agent logs at $(date) ==="
    docker logs --since=5s ${CONTAINER_NAME}
    echo "==========================="
    sleep 5
  done

  echo "Test complete. Displaying final logs..."
  echo "=== Final agent logs ==="
  docker logs --tail 20 ${CONTAINER_NAME}
  echo "========================"
else
  # In interactive mode, just wait for the update to complete
  sleep 180
fi

# Signal that the test is complete
echo "${CONTAINER_NAME}" > "${SIGNAL_DIR}/test_complete"

echo "Stopping container..."
docker stop ${CONTAINER_NAME}
docker rm ${CONTAINER_NAME}

if [ "$INTERACTIVE" = false ]; then
  echo "Full logs saved to /tmp/agent-output.log"
fi
echo "Done."
