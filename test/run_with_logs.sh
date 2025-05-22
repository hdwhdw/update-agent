#!/bin/bash

# Run the test_upgrade.sh script and monitor logs in a split terminal
# This requires tmux to be installed

# Check if tmux is available
if ! command -v tmux &> /dev/null; then
  echo "tmux is required for this script but not installed."
  echo "You can install it with: sudo apt-get install tmux"
  echo "Alternatively, run test_upgrade.sh and monitor_logs.sh in separate terminals."
  exit 1
fi

# Clean up any existing signal files
SIGNAL_DIR="/tmp/upgrade-agent-signals"
mkdir -p "${SIGNAL_DIR}"
rm -f "${SIGNAL_DIR}/container_ready" "${SIGNAL_DIR}/test_complete"

# Kill any existing tmux session with the same name
tmux kill-session -t upgrade-test 2>/dev/null || true

# Start a new tmux session
tmux new-session -d -s upgrade-test

# Split the window horizontally
tmux split-window -h -t upgrade-test

# Make sure both panes start in the right directory
tmux send-keys -t upgrade-test:0.0 "cd $(dirname $0)/.." C-m
tmux send-keys -t upgrade-test:0.1 "cd $(dirname $0)/.." C-m

# Start the monitor first, waiting for the container
tmux send-keys -t upgrade-test:0.1 "echo 'Waiting for test to start...'; ./test/monitor_logs.sh --wait" C-m

# Small delay to make sure monitor is fully initialized
sleep 1

# Run the test script in the left pane
tmux send-keys -t upgrade-test:0.0 "./test/test_upgrade.sh" C-m

# Attach to the tmux session
tmux attach-session -t upgrade-test
