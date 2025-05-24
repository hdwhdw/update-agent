#!/bin/bash

# Test script for upgrade-agent using Docker container
set -e

# Parse command line arguments
INTERACTIVE=false
IGNORE_UNIMPLEMENTED_RPC=false
SSH_AUTH_TYPE="key"  # Default to key-based authentication
SETUP_SSH_KEY=false  # Whether to run ssh-copy-id
while [[ $# -gt 0 ]]; do
  case $1 in
    -i|--interactive)
      INTERACTIVE=true
      shift
      ;;
    --ignore-unimplemented)
      IGNORE_UNIMPLEMENTED_RPC=true
      shift
      ;;
    --setup-ssh)
      SETUP_SSH_KEY=true
      shift
      ;;
    --ssh-password)
      SSH_AUTH_TYPE="password"
      # Check if next argument looks like a password (not starting with --)
      if [[ $# -gt 1 && ! $2 == --* ]]; then
        SSH_PASSWORD="$2"
        shift 2
      else
        # Just set the auth type, password will be prompted later
        shift
      fi
      ;;
    --ssh-user)
      SSH_USER="$2"
      shift 2
      ;;
    --ssh-key)
      SSH_KEY="$2"
      shift 2
      ;;
    --ssh-host)
      SERVER_IP="$2"
      shift 2
      ;;
    *)
      echo "Unknown option: $1"
      echo "Usage: $0 [-i|--interactive] [--ignore-unimplemented] [--setup-ssh] [--ssh-password [password]] [--ssh-user username] [--ssh-key path/to/key] [--ssh-host hostname]"
      echo "Options:"
      echo "  -i, --interactive         Run in interactive mode with logs in terminal"
      echo "  --ignore-unimplemented    Treat unimplemented gRPC errors as success (for testing)"
      echo "  --setup-ssh               Setup SSH keys using ssh-copy-id for passwordless login"
      echo "                            (one-time setup, recommended for frequent testing)"
      echo "  --ssh-password [password] Use password authentication for SSH (optional password,"
      echo "                            will prompt if not provided). Not needed after --setup-ssh."
      echo "  --ssh-user username       SSH username for remote server (default: current user)"
      echo "  --ssh-key path/to/key     Path to SSH private key (default: ~/.ssh/id_rsa)"
      echo "  --ssh-host hostname       Remote server hostname or IP (default: from GRPC_TARGET)"
      exit 1
      ;;
  esac
done

# Configuration
CONFIG_PATH="/tmp/config.yaml"
CONFIG_MOUNT_PATH="/etc/upgrade-agent/config.yaml"
GRPC_TARGET="10.250.0.101:50052"
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
SSH_KEY="${SSH_KEY:-$HOME/.ssh/id_rsa}"  # Default SSH key location
SSH_OPTIONS="-o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null"

# Helper function for SSH commands
run_ssh() {
  local cmd="$1"
  if [ "$SSH_AUTH_TYPE" = "password" ]; then
    # Check if sshpass is installed
    if ! command -v sshpass >/dev/null 2>&1; then
      echo "Error: sshpass is not installed. Please install it for password authentication."
      echo "   On Ubuntu/Debian: sudo apt-get install sshpass"
      echo "   On RHEL/CentOS: sudo yum install sshpass"
      echo "   On macOS: brew install hudochenkov/sshpass/sshpass"
      exit 1
    fi

    # Prompt for password only once during the script execution
    if [ -z "${SSH_PASSWORD}" ]; then
      read -sp "Enter SSH password for ${SSH_USER}@${SERVER_IP} (will be stored for this session): " SSH_PASSWORD
      echo ""
      # Export password so it's available to all functions in the script
      export SSH_PASSWORD
    fi

    # Password authentication with stored password
    sshpass -p "$SSH_PASSWORD" ssh ${SSH_OPTIONS} ${SSH_USER}@${SERVER_IP} "$cmd"
  else
    # Key-based authentication (default)
    ssh ${SSH_OPTIONS} -i "${SSH_KEY}" ${SSH_USER}@${SERVER_IP} "$cmd"
  fi
}

# Helper function for SCP
run_scp() {
  local src="$1"
  local dest="$2"
  if [ "$SSH_AUTH_TYPE" = "password" ]; then
    # Password already captured by run_ssh if it was called first
    # If not, prompt for it now (only happens if run_scp is called before run_ssh)
    if [ -z "${SSH_PASSWORD}" ]; then
      read -sp "Enter SSH password for ${SSH_USER}@${SERVER_IP} (will be stored for this session): " SSH_PASSWORD
      echo ""
      export SSH_PASSWORD
    fi

    # Password authentication with stored password
    sshpass -p "$SSH_PASSWORD" scp ${SSH_OPTIONS} "$src" "${SSH_USER}@${SERVER_IP}:$dest"
  else
    # Key-based authentication
    scp ${SSH_OPTIONS} -i "${SSH_KEY}" "$src" "${SSH_USER}@${SERVER_IP}:$dest"
  fi
}

# Ensure the firmware directory exists
mkdir -p "$(dirname "${FIRMWARE_SOURCE}")"
touch ${FIRMWARE_SOURCE}

# Handle SSH key setup if requested
if [ "$SETUP_SSH_KEY" = true ]; then
  echo "Setting up SSH key authentication with ${SERVER_IP}..."

  # Check if key exists, generate if not
  if [ ! -f "${SSH_KEY}" ] || [ ! -f "${SSH_KEY}.pub" ]; then
    echo "SSH key ${SSH_KEY} does not exist. Generating new key..."
    ssh-keygen -t rsa -b 4096 -f "${SSH_KEY}" -N ""
  fi

  # Use ssh-copy-id to copy the key
  if [ "$SSH_AUTH_TYPE" = "password" ]; then
    # If user provided password via command line
    if [ -n "${SSH_PASSWORD}" ]; then
      # Check if sshpass is installed
      if ! command -v sshpass >/dev/null 2>&1; then
        echo "Error: sshpass is not installed. Please install it for password-based ssh-copy-id."
        echo "   On Ubuntu/Debian: sudo apt-get install sshpass"
        echo "   On RHEL/CentOS: sudo yum install sshpass"
        exit 1
      fi
      sshpass -p "${SSH_PASSWORD}" ssh-copy-id -i "${SSH_KEY}.pub" ${SSH_OPTIONS} "${SSH_USER}@${SERVER_IP}"
    else
      # Interactive password prompt
      echo "Please enter the password for ${SSH_USER}@${SERVER_IP} when prompted."
      ssh-copy-id -i "${SSH_KEY}.pub" ${SSH_OPTIONS} "${SSH_USER}@${SERVER_IP}"
    fi
  else
    # Just run ssh-copy-id (will prompt for password)
    echo "Please enter the password for ${SSH_USER}@${SERVER_IP} when prompted."
    ssh-copy-id -i "${SSH_KEY}.pub" ${SSH_OPTIONS} "${SSH_USER}@${SERVER_IP}"
  fi

  # Switch to key-based auth now that we've set it up
  SSH_AUTH_TYPE="key"
  echo "SSH key setup complete. Now using key-based authentication."
fi

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
  -v /:/host:ro \\
  --detach \\
  upgrade-server:latest --port ${SERVER_PORT}

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
ignoreUnimplementedRPC: ${IGNORE_UNIMPLEMENTED_RPC}
EOF

echo "Waiting for update process to complete (max 300 seconds)..."
echo "You can also run './test/monitor_logs.sh --wait' in another terminal to follow logs."

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
if [ "$INTERACTIVE" = false ]; then
  # Non-interactive mode: poll logs and check for completion
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
else
  # Interactive mode: monitor logs and check for completion
  echo "Interactive mode: checking for upgrade completion (max $MAX_TIMEOUT seconds)..."
  while true; do
    CURRENT_TIME=$(date +%s)
    ELAPSED_TIME=$((CURRENT_TIME - START_TIME))

    # Exit if we've exceeded the maximum timeout
    if [ $ELAPSED_TIME -ge $MAX_TIMEOUT ]; then
      echo "Maximum wait time reached ($MAX_TIMEOUT seconds). Proceeding..."
      break
    fi

    # Check if upgrade is complete
    if check_upgrade_complete; then
      echo "Upgrade completion detected! Proceeding..."
      break
    fi

    sleep 5
  done
fi

# Signal that the test is complete
echo "${CONTAINER_NAME}" > "${SIGNAL_DIR}/test_complete"

echo "Stopping local container..."
docker stop ${CONTAINER_NAME}
docker rm ${CONTAINER_NAME}

echo "Stopping and removing server container on remote host ${SERVER_IP}..."
run_ssh "
docker stop ${SERVER_CONTAINER_NAME}
docker rm ${SERVER_CONTAINER_NAME}
rm -f /tmp/upgrade-server-image.tar
"

# Clean up local temporary files
rm -f /tmp/upgrade-server-image.tar

if [ "$INTERACTIVE" = false ]; then
  echo "Full logs saved to /tmp/agent-output.log"
fi
echo "Done."
