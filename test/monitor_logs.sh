#!/bin/bash

# Script to monitor logs from the upgrade-agent container
# This can be run in parallel with test_upgrade.sh

WAIT_MODE=false
CONTAINER_NAME="upgrade-agent-test"
WAIT_TIMEOUT=120  # Default timeout 120 seconds
SIGNAL_DIR="/tmp/upgrade-agent-signals"

# Parse command line arguments
while [[ $# -gt 0 ]]; do
  case $1 in
    --wait)
      WAIT_MODE=true
      shift
      ;;
    *)
      # Assume it's the container name
      CONTAINER_NAME="$1"
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

  # First, wait for the signal file that indicates container is ready
  if ! wait_for_signal "${SIGNAL_DIR}/container_ready" "Waiting for container to be created" "${WAIT_TIMEOUT}"; then
    echo "Failed to detect container startup. Exiting."
    exit 1
  fi

  # Verify that the container actually exists
  if ! docker ps | grep -q "${CONTAINER_NAME}"; then
    echo "Signal file found but container '${CONTAINER_NAME}' is not running."
    echo "This may indicate a problem with the test script."
    exit 1
  fi

  echo "Container '${CONTAINER_NAME}' found and running!"
elif ! docker ps | grep -q "${CONTAINER_NAME}"; then
  echo "Container '${CONTAINER_NAME}' is not running."
  echo "Usage: $0 [--wait] [container-name]"
  exit 1
fi

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
