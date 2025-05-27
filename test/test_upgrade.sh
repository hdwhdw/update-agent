#!/bin/bash

# Test script for upgrade-agent using Docker container
# Both the upgrade-server and upgrade-agent run on the same remote host.
# This script will:
# 1. Build and push both container images to the remote host
# 2. Configure and start the server container on the remote host
# 3. Configure and start the agent container on the remote host
# 4. Monitor both containers remotely and stream logs back
# 5. Trigger a firmware update by changing the config file
# 6. Clean up resources after testing
#
# IMPORTANT: You MUST specify the --ssh-host parameter
set -e

# Parse command line arguments
IGNORE_UNIMPLEMENTED_RPC=true  # Default is true as requested
FAKE_REBOOT=true
while [[ $# -gt 0 ]]; do
  case $1 in
    --no-ignore-unimplemented)
      IGNORE_UNIMPLEMENTED_RPC=false
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
      echo "Usage: $0 [--no-ignore-unimplemented] [--no-fake-reboot] [--ssh-user username] [--ssh-host hostname]"
      echo "Options:"
      echo "  --no-ignore-unimplemented Disable treating unimplemented gRPC errors as success"
      echo "  --no-fake-reboot          Disable fake reboot mode (actually reboot the system)"
      echo "  --ssh-user username       SSH username for remote server (default: admin)"
      echo "  --ssh-host hostname       Remote server hostname or IP (REQUIRED for this test)"
      exit 1
      ;;
  esac
done

# Configuration
CONFIG_PATH="/tmp/config.yaml"
CONFIG_MOUNT_PATH="/etc/upgrade-agent/config.yaml"
# For agent running on the same host as the server, use localhost
GRPC_TARGET="localhost:50060"
FIRMWARE_SOURCE="/tmp/sonic.bin"
FIRMWARE_MOUNT_PATH="/firmware/sonic.bin"
UPDATE_MLNX_CPLD="true"
INITIAL_VERSION="1.0.0"
NEW_VERSION="1.1.0"
CONTAINER_NAME="upgrade-agent-test"
SERVER_CONTAINER_NAME="upgrade-server-test"
# Extract port from GRPC_TARGET to ensure consistency
SERVER_PORT="$(echo $GRPC_TARGET | cut -d':' -f2)"
# If SERVER_IP is not explicitly set by --ssh-host, use the IP from GRPC_TARGET
# but only if it's not localhost (in which case we need a real remote IP)
if [ -z "${SERVER_IP}" ]; then
  echo "Error: The --ssh-host parameter is required."
  echo "Please specify the hostname or IP address of the remote server."
  echo "Usage example: $0 --ssh-host 192.168.1.100"
  exit 1
fi
SSH_USER="${SSH_USER:-admin}"  # Default to admin user if not set
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

# Copy the config file to the remote server
run_scp "${CONFIG_PATH}" "/tmp/config.yaml"

echo "Building the Docker containers..."
docker build -t upgrade-agent:latest .
docker build -t upgrade-server:latest -f Dockerfile.server .

echo "Saving and copying both Docker images to remote host ${SERVER_IP}..."
# Save both Docker images to files
docker save upgrade-server:latest > /tmp/upgrade-server-image.tar
docker save upgrade-agent:latest > /tmp/upgrade-agent-image.tar

# Copy both Docker images to the remote server
run_scp "/tmp/upgrade-server-image.tar" "/tmp/"
run_scp "/tmp/upgrade-agent-image.tar" "/tmp/"

# SSH to remote server, load the images, and start the containers
echo "Starting containers on remote host ${SERVER_IP}..."
run_ssh "
# Load the Docker images
docker load < /tmp/upgrade-server-image.tar
docker load < /tmp/upgrade-agent-image.tar

# Remove any existing containers with the same names
docker rm -f upgrade-server-test >/dev/null 2>&1 || true
docker rm -f ${CONTAINER_NAME} >/dev/null 2>&1 || true

# Create remote config and firmware directories
mkdir -p /tmp/upgrade-agent
mkdir -p $(dirname /tmp${FIRMWARE_SOURCE})
touch /tmp${FIRMWARE_SOURCE}

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

echo "Starting upgrade agent container with initial version ${INITIAL_VERSION} on remote host ${SERVER_IP}..."

# Create a signal file directory if it doesn't exist
SIGNAL_DIR="/tmp/upgrade-agent-signals"
mkdir -p "${SIGNAL_DIR}"

# Remove any old signal files
rm -f "${SIGNAL_DIR}/container_ready" "${SIGNAL_DIR}/test_complete" "${SIGNAL_DIR}/server_ready"

# Set up server log monitoring in background
echo "${SERVER_CONTAINER_NAME}" > "${SIGNAL_DIR}/server_ready"
echo "Starting server and agent log monitoring..."
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

{
  # Loop to continuously fetch agent logs
  while true; do
    if [ -f "${SIGNAL_DIR}/test_complete" ]; then
      break
    fi
    run_ssh "docker logs --since=5s ${CONTAINER_NAME} 2>&1" >> /tmp/agent-output.log
    sleep 5
  done
} &
AGENT_LOGS_PID=$!

echo "Logs will be collected to:"
echo "  - Server logs: /tmp/server-output.log"
echo "  - Agent logs: /tmp/agent-output.log"

# Start the agent container on the remote host
run_ssh "
# Start the agent container
docker run --name ${CONTAINER_NAME} \\
  --network=host \\
  -v /tmp/config.yaml:${CONFIG_MOUNT_PATH} \\
  -v /tmp${FIRMWARE_SOURCE}:${FIRMWARE_MOUNT_PATH} \\
  --detach \\
  upgrade-agent:latest
echo \"Agent container started on \$(hostname)\"
"

# Create a signal file that the monitor script can check for
# and write the container name to it
echo "${CONTAINER_NAME}" > "${SIGNAL_DIR}/container_ready"

echo "Waiting for agent to initialize (5 seconds)..."
sleep 5

# Display initial logs
echo "=== Initial agent logs from ${SERVER_IP} ==="
run_ssh "docker logs ${CONTAINER_NAME}"
echo "=========================="
echo "Note: Both containers are running on the remote host ${SERVER_IP}."
echo "Logs are being collected in the background to:"
echo "  - /tmp/agent-output.log"
echo "  - /tmp/server-output.log"

echo "Updating config to trigger firmware update to version ${NEW_VERSION}..."
cat > ${CONFIG_PATH} << EOF
grpcTarget: "${GRPC_TARGET}"
firmwareSource: "${FIRMWARE_MOUNT_PATH}"
updateMlnxCpldFw: ${UPDATE_MLNX_CPLD}
targetVersion: "${NEW_VERSION}"  # When this field is updated, it will trigger an update
ignoreUnimplementedRPC: ${IGNORE_UNIMPLEMENTED_RPC}
EOF

# Copy the updated config file to the remote server
run_scp "${CONFIG_PATH}" "/tmp/config.yaml"

echo "Waiting for update process to complete (max 300 seconds)..."
echo "Both the upgrade-agent and upgrade-server are running on remote host ${SERVER_IP}."
echo "You can monitor logs in real-time with:"
echo "  - tail -f /tmp/agent-output.log (for agent logs)"
echo "  - tail -f /tmp/server-output.log (for server logs)"

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
  # Get the latest logs from the remote agent
  local logs
  logs=$(run_ssh "docker logs ${CONTAINER_NAME} 2>&1")

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
  run_ssh "docker logs --since=5s ${CONTAINER_NAME} 2>&1" || echo "Could not retrieve agent logs"
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
run_ssh "docker logs --tail 20 ${CONTAINER_NAME} 2>&1" || echo "Could not retrieve final agent logs"
echo "========================"

echo "=== Final server logs ==="
run_ssh "docker logs --tail 20 ${SERVER_CONTAINER_NAME} 2>&1" || echo "Could not retrieve final server logs"
echo "========================"

# Signal that the test is complete
echo "${CONTAINER_NAME}" > "${SIGNAL_DIR}/test_complete"

# Kill the log monitoring processes
if [ -n "${SERVER_LOGS_PID}" ]; then
  echo "Stopping server log monitoring (PID: ${SERVER_LOGS_PID})..."
  kill ${SERVER_LOGS_PID} 2>/dev/null || true
fi

if [ -n "${AGENT_LOGS_PID}" ]; then
  echo "Stopping agent log monitoring (PID: ${AGENT_LOGS_PID})..."
  kill ${AGENT_LOGS_PID} 2>/dev/null || true
fi

echo "Stopping and removing containers on remote host ${SERVER_IP}..."
run_ssh "
docker stop ${CONTAINER_NAME} ${SERVER_CONTAINER_NAME}
docker rm ${CONTAINER_NAME} ${SERVER_CONTAINER_NAME}
rm -f /tmp/upgrade-server-image.tar /tmp/upgrade-agent-image.tar /tmp/config.yaml
"

# Clean up local temporary files
rm -f /tmp/upgrade-server-image.tar /tmp/upgrade-agent-image.tar

echo "Full logs saved to:"
echo "- Agent logs: /tmp/agent-output.log"
echo "- Server logs: /tmp/server-output.log"
echo "Done."
