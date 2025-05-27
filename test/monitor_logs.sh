#!/bin/bash

# Script to monitor logs from the upgrade-agent container
# This can be run in parallel with test_upgrade.sh

WAIT_MODE=false
CONTAINER_NAME="upgrade-agent-test"
SERVER_MODE=false
SERVER_CONTAINER_NAME="upgrade-server-test"
WAIT_TIMEOUT=240  # Extended timeout to 240 seconds to accommodate longer reboot process
SIGNAL_DIR="/tmp/upgrade-agent-signals"

# Parse command line arguments
while [[ $# -gt 0 ]]; do
  case $1 in
    --wait)
      WAIT_MODE=true
      shift
      ;;
    --server)
      SERVER_MODE=true
      shift
      ;;
    *)
      # Assume it's the container name
      if [ "$SERVER_MODE" = true ]; then
        SERVER_CONTAINER_NAME="$1"
      else
        CONTAINER_NAME="$1"
      fi
      shift
      ;;
  esac
done

# Create signal directory if it doesn't exist
mkdir -p "${SIGNAL_DIR}"

# Function to wait for signal file
wait_for_signal() {
  local signal_file="$1"
  local message="$2"
  local timeout="$3"

  echo "${message} (timeout: ${timeout}s)..."

  start_time=$(date +%s)
  while [[ ! -f "${signal_file}" ]]; do
    current_time=$(date +%s)
    elapsed=$((current_time - start_time))

    # Check if we've reached timeout
    if [[ $elapsed -gt $timeout ]]; then
      echo "Timeout waiting for ${signal_file}."
      return 1
    fi

    echo -n "."
    sleep 1
  done

  echo ""
  # If the signal file exists, read the container name from it
  if [[ -f "${signal_file}" ]]; then
    CONTAINER_NAME=$(cat "${signal_file}")
    echo "Signal received! Using container: ${CONTAINER_NAME}"
    return 0
  fi

  return 1
}

# Wait for the container if wait mode is enabled
if [ "$WAIT_MODE" = true ]; then
  echo "Waiting for test to start container..."

  # Check which signal file to wait for based on mode
  SIGNAL_FILE="${SIGNAL_DIR}/container_ready"
  if [ "$SERVER_MODE" = true ]; then
    SIGNAL_FILE="${SIGNAL_DIR}/server_ready"
  fi

  # First, wait for the signal file that indicates container is ready
  if ! wait_for_signal "${SIGNAL_FILE}" "Waiting for container to be created" "${WAIT_TIMEOUT}"; then
    echo "Failed to detect container startup. Exiting."
    exit 1
  fi

  # Read the container name from the signal file
  if [ "$SERVER_MODE" = true ]; then
    SERVER_CONTAINER_NAME=$(cat "${SIGNAL_FILE}")
    echo "Signal received! Using server container: ${SERVER_CONTAINER_NAME}"
  else
    CONTAINER_NAME=$(cat "${SIGNAL_FILE}")
    echo "Signal received! Using agent container: ${CONTAINER_NAME}"
  fi

  # For server mode, we don't verify that the container exists locally
  if [ "$SERVER_MODE" = false ] && ! docker ps | grep -q "${CONTAINER_NAME}"; then
    echo "Signal file found but container '${CONTAINER_NAME}' is not running."
    echo "This may indicate a problem with the test script."
    exit 1
  fi

  if [ "$SERVER_MODE" = true ]; then
    echo "Server container '${SERVER_CONTAINER_NAME}' should be running remotely!"
  else
    echo "Container '${CONTAINER_NAME}' found and running!"
  fi
elif [ "$SERVER_MODE" = true ]; then
  # In server mode without wait, we assume the server container is running remotely
  echo "Assuming server container '${SERVER_CONTAINER_NAME}' is running remotely."
elif ! docker ps | grep -q "${CONTAINER_NAME}"; then
  echo "Container '${CONTAINER_NAME}' is not running."
  echo "Usage: $0 [--wait] [--server] [container-name]"
  echo "Options:"
  echo "  --wait      Wait for the container to be started by test_upgrade.sh"
  echo "  --server    Monitor the server logs instead of agent logs"
  exit 1
fi

if [ "$SERVER_MODE" = true ]; then
  echo "Monitoring logs for server container '${SERVER_CONTAINER_NAME}'..."
  echo "Press Ctrl+C to stop monitoring."
  echo "==========================================="

  # For server logs, we need to use SSH to fetch logs
  if [ "$WAIT_MODE" = true ]; then
    # Function to monitor server logs via SSH
    monitor_server_logs() {
      while true; do
        # Check if test is complete
        if [ -f "${SIGNAL_DIR}/test_complete" ]; then
          echo "Test complete signal received. Exiting server log monitor."
          break
        fi

        # Use SSH to get the logs
        ssh -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null -i "$HOME/.ssh/id_rsa" "$(whoami)@$(echo $GRPC_TARGET | cut -d':' -f1)" "docker logs --since=5s ${SERVER_CONTAINER_NAME} 2>&1" || echo "Error retrieving server logs"
        sleep 5
      done
    }

    monitor_server_logs
  else
    # In non-wait mode, just tail the server logs file
    if [ -f "/tmp/server-output.log" ]; then
      tail -f /tmp/server-output.log
    else
      echo "Server log file not found at /tmp/server-output.log"
      echo "Make sure test_upgrade.sh is running or has been run."
      exit 1
    fi
  fi
else
  echo "Monitoring logs for container '${CONTAINER_NAME}'..."
  echo "Press Ctrl+C to stop monitoring."
  echo "==========================================="

  # Start monitoring logs in the background
  docker logs -f "${CONTAINER_NAME}" &
  LOGS_PID=$!

  # Wait for test to complete
  if [ "$WAIT_MODE" = true ]; then
    # Wait for signal that test is complete
    wait_for_signal "${SIGNAL_DIR}/test_complete" "Monitoring logs until test completes" "${WAIT_TIMEOUT}" || true

    # Give a little time to show final logs
    sleep 2

    # Clean up
    kill $LOGS_PID 2>/dev/null || true
    echo ""
    echo "Test complete. Exiting log monitor."
    exit 0
  else
    # Just monitor logs until user hits Ctrl+C
    wait $LOGS_PID
  fi
fi
