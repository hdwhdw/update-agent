#!/bin/bash

# Test script for upgrade-agent using Docker container
set -e

# Parse command line arguments
IGNORE_UNIMPLEMENTED_RPC=false
FAKE_REBOOT=true
while [[ $# -gt 0 ]]; do
  case $1 in
    --ignore-unimplemented)
      IGNORE_UNIMPLEMENTED_RPC=true
      shift
      ;;
    --no-fake-reboot)
      FAKE_REBOOT=false
      shift
      ;;
    --ssh-user)
      SSH_USER="$2"
      shift 2
      ;;
    --ssh-host)
      SERVER_IP="$2"
      shift 2
      ;;
    *)
      echo "Unknown option: $1"
      echo "Usage: $0 [--ignore-unimplemented] [--no-fake-reboot] [--ssh-user username] [--ssh-host hostname]"
      echo "Options:"
      echo "  --ignore-unimplemented    Treat unimplemented gRPC errors as success (for testing)"
      echo "  --no-fake-reboot          Disable fake reboot mode (actually reboot the system)"
      echo "  --ssh-user username       SSH username for remote server (default: current user)"
      echo "  --ssh-host hostname       Remote server hostname or IP (default: from GRPC_TARGET)"
      exit 1
      ;;
  esac
done

# Configuration
CONFIG_PATH="/tmp/config.yaml"
CONFIG_MOUNT_PATH="/etc/upgrade-agent/config.yaml"
GRPC_TARGET="10.250.0.101:50060"
FIRMWARE_SOURCE="/tmp/sonic.bin"
FIRMWARE_MOUNT_PATH="/firmware/sonic.bin"
UPDATE_MLNX_CPLD="true"
INITIAL_VERSION="1.0.0"
NEW_VERSION="1.1.0"
CONTAINER_NAME="upgrade-agent-test"
SERVER_CONTAINER_NAME="upgrade-server-test"
# Extract port from GRPC_TARGET to ensure consistency
SERVER_PORT="$(echo $GRPC_TARGET | cut -d':' -f2)"
SERVER_IP="$(echo $GRPC_TARGET | cut -d':' -f1)"
SSH_USER="${SSH_USER:-$(whoami)}"  # Default to current user if not set
SSH_KEY="$HOME/.ssh/id_rsa"  # Default SSH key location
SSH_OPTIONS="-o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null"

# Helper function for SSH commands
run_ssh() {
  local cmd="$1"
  # Key-based authentication
  ssh ${SSH_OPTIONS} -i "${SSH_KEY}" ${SSH_USER}@${SERVER_IP} "$cmd"
}

# Helper function for SCP
run_scp() {
  local src="$1"
  local dest="$2"
  # Key-based authentication
  scp ${SSH_OPTIONS} -i "${SSH_KEY}" "$src" "${SSH_USER}@${SERVER_IP}:$dest"
}

# Ensure the firmware directory exists
mkdir -p "$(dirname "${FIRMWARE_SOURCE}")"
touch ${FIRMWARE_SOURCE}

# Setup SSH key authentication
echo "Setting up SSH key authentication with ${SERVER_IP}..."

# Check if key exists, generate if not
if [ ! -f "${SSH_KEY}" ] || [ ! -f "${SSH_KEY}.pub" ]; then
  echo "SSH key ${SSH_KEY} does not exist. Generating new key..."
  ssh-keygen -t rsa -b 4096 -f "${SSH_KEY}" -N ""
fi

# Use ssh-copy-id to copy the key
echo "Please enter the password for ${SSH_USER}@${SERVER_IP} when prompted."
ssh-copy-id -i "${SSH_KEY}.pub" ${SSH_OPTIONS} "${SSH_USER}@${SERVER_IP}"

echo "SSH key setup complete. Using key-based authentication."

echo "Creating initial config file..."
cat > ${CONFIG_PATH} << EOF
grpcTarget: "${GRPC_TARGET}"
firmwareSource: "${FIRMWARE_MOUNT_PATH}"
updateMlnxCpldFw: ${UPDATE_MLNX_CPLD}
targetVersion: "${INITIAL_VERSION}"  # When this field is updated, it will trigger an update
ignoreUnimplementedRPC: ${IGNORE_UNIMPLEMENTED_RPC}
EOF

echo "Building the Docker containers..."
docker build -t upgrade-agent:latest .
docker build -t upgrade-server:latest -f Dockerfile.server .

echo "Copying the server image to remote host ${SERVER_IP}..."
# Save the server Docker image to a file
docker save upgrade-server:latest > /tmp/upgrade-server-image.tar

# Copy the Docker image to the remote server
run_scp "/tmp/upgrade-server-image.tar" "/tmp/"

# SSH to remote server, load the image, and start the container
echo "Starting upgrade server container on remote host ${SERVER_IP}..."
run_ssh "
# Load the Docker image
docker load < /tmp/upgrade-server-image.tar

# Remove any existing container with the same name
docker rm -f upgrade-server-test >/dev/null 2>&1 || true

# Start the server container
# Mount the entire host filesystem for simplicity and full access to all OS information
docker run --name upgrade-server-test \\
  --network=host \\
  --privileged \\
  --restart=always \\
  --cap-add=SYS_BOOT \\
  -v /:/host \\
  --detach \\
  upgrade-server:latest --port ${SERVER_PORT} $([ "$FAKE_REBOOT" = true ] && echo "--fake-reboot")

# Display initial logs
echo \"Server container started on \$(hostname)\"
docker logs upgrade-server-test
"

echo "Waiting for server to initialize (5 seconds)..."
sleep 5

echo "=== Initial server logs on ${SERVER_IP} ==="
run_ssh "docker logs ${SERVER_CONTAINER_NAME}"
echo "==========================="

echo "Starting upgrade agent container with initial version ${INITIAL_VERSION}..."

# Create a signal file directory if it doesn't exist
SIGNAL_DIR="/tmp/upgrade-agent-signals"
mkdir -p "${SIGNAL_DIR}"

# Remove any old signal files
rm -f "${SIGNAL_DIR}/container_ready" "${SIGNAL_DIR}/test_complete" "${SIGNAL_DIR}/server_ready"

# Set up server log monitoring in background
echo "${SERVER_CONTAINER_NAME}" > "${SIGNAL_DIR}/server_ready"
echo "Starting server log monitoring..."
{
  # Loop to continuously fetch server logs
  while true; do
    if [ -f "${SIGNAL_DIR}/test_complete" ]; then
      break
    fi
    run_ssh "docker logs --since=5s ${SERVER_CONTAINER_NAME} 2>&1" >> /tmp/server-output.log
    sleep 5
  done
} &
SERVER_LOGS_PID=$!
echo "Server logs will be collected to /tmp/server-output.log"

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
echo "  - tail -f /tmp/server-output.log"
echo "  - ./test/monitor_logs.sh ${CONTAINER_NAME}"
echo "  - ./test/monitor_logs.sh --server (for server logs)"

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
ignoreUnimplementedRPC: ${IGNORE_UNIMPLEMENTED_RPC}
EOF

echo "Waiting for update process to complete (max 300 seconds)..."
echo "You can also run './test/monitor_logs.sh --wait' in another terminal to follow agent logs."
echo "You can also run 'tail -f /tmp/server-output.log' in another terminal to follow server logs."

# Set a maximum timeout (in seconds)
MAX_TIMEOUT=300
START_TIME=$(date +%s)

# Success indicator phrases to look for in logs
COMPLETE_INDICATORS=(
  "Firmware update to version ${NEW_VERSION} completed successfully"
  "System reboot completed successfully"
  "System stabilization period complete"
  "OS version after update"
)

# Function to check if upgrade is complete
check_upgrade_complete() {
  # Get the latest logs
  local logs
  logs=$(docker logs ${CONTAINER_NAME} 2>&1)

  # Check for any of the success indicators
  for indicator in "${COMPLETE_INDICATORS[@]}"; do
    if echo "$logs" | grep -q "$indicator"; then
      return 0  # Found a completion indicator
    fi
  done

  return 1  # No completion indicator found
}

# Monitor logs and check for upgrade completion
while true; do
  CURRENT_TIME=$(date +%s)
  ELAPSED_TIME=$((CURRENT_TIME - START_TIME))

  # Exit if we've exceeded the maximum timeout
  if [ $ELAPSED_TIME -ge $MAX_TIMEOUT ]; then
    echo "Maximum wait time reached ($MAX_TIMEOUT seconds). Proceeding..."
    break
  fi

  # Display recent logs
  echo "=== Agent logs at $(date) [${ELAPSED_TIME}s elapsed] ==="
  docker logs --since=5s ${CONTAINER_NAME}
  echo "==========================="

  # Display recent server logs
  echo "=== Server logs at $(date) [${ELAPSED_TIME}s elapsed] ==="
  run_ssh "docker logs --since=5s ${SERVER_CONTAINER_NAME} 2>&1" || echo "Could not retrieve server logs"
  echo "==========================="

  # Check if upgrade is complete
  if check_upgrade_complete; then
    echo "Upgrade completion detected! Proceeding..."
    break
  fi

  sleep 5
done

echo "Test complete. Displaying final logs..."
echo "=== Final agent logs ==="
docker logs --tail 20 ${CONTAINER_NAME}
echo "========================"

echo "=== Final server logs ==="
run_ssh "docker logs --tail 20 ${SERVER_CONTAINER_NAME} 2>&1" || echo "Could not retrieve final server logs"
echo "========================"

# Signal that the test is complete
echo "${CONTAINER_NAME}" > "${SIGNAL_DIR}/test_complete"

echo "Stopping local container..."
docker stop ${CONTAINER_NAME}
docker rm ${CONTAINER_NAME}

# Kill the server log monitoring process
if [ -n "${SERVER_LOGS_PID}" ]; then
  echo "Stopping server log monitoring (PID: ${SERVER_LOGS_PID})..."
  kill ${SERVER_LOGS_PID} 2>/dev/null || true
fi

echo "Stopping and removing server container on remote host ${SERVER_IP}..."
run_ssh "
docker stop ${SERVER_CONTAINER_NAME}
docker rm ${SERVER_CONTAINER_NAME}
rm -f /tmp/upgrade-server-image.tar
"

# Clean up local temporary files
rm -f /tmp/upgrade-server-image.tar

echo "Full logs saved to:"
echo "- Agent logs: /tmp/agent-output.log"
echo "- Server logs: /tmp/server-output.log"
echo "Done."
