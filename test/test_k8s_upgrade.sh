#!/bin/bash

# Test script for upgrade-agent using Kubernetes deployment
# This script will:
# 1. Build the server and agent images
# 2. Transfer the images to the target node and load them
# 3. Label the node to enable agent and server deployment
# 4. Update the target version in the config to trigger an update
# 5. Monitor logs to verify the update is working
# 6. Clean up resources after testing

set -e

# Default configuration
NODE_NAME="vlab-01"
# Use env command for shell compatibility (works in both bash and zsh)
MINIKUBE_PREFIX="env NO_PROXY=192.168.49.2 minikube kubectl --"
SSH_USER="${SSH_USER:-admin}"
SSH_KEY="$HOME/.ssh/id_rsa"
SSH_OPTIONS="-o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null"

# Parse command line arguments
while [[ $# -gt 0 ]]; do
  case $1 in
    --node)
      NODE_NAME="$2"
      shift 2
      ;;
    --ssh-user)
      SSH_USER="$2"
      shift 2
      ;;
    *)
      echo "Unknown option: $1"
      echo "Usage: $0 [--node nodename] [--ssh-user username]"
      echo "Options:"
      echo "  --node nodename        Kubernetes node name (default: vlab-01)"
      echo "  --ssh-user username    SSH username for node (default: admin)"
      exit 1
      ;;
  esac
done

# Helper function for SSH commands
run_ssh() {
  local cmd="$1"
  ssh ${SSH_OPTIONS} -i "${SSH_KEY}" ${SSH_USER}@${NODE_NAME} "$cmd"
}

# Helper function for SCP
run_scp() {
  local src="$1"
  local dest="$2"
  scp ${SSH_OPTIONS} -i "${SSH_KEY}" "$src" "${SSH_USER}@${NODE_NAME}:$dest"
}

# Build docker images
echo "Building upgrade-server and upgrade-agent images..."
cd "$(dirname "$0")/.."
docker build -f Dockerfile.server -t upgrade-server:latest .
docker build -f Dockerfile -t upgrade-agent:latest .

# Save images to tar files
echo "Saving images to tar files..."
docker save -o /tmp/upgrade-server.tar upgrade-server:latest
docker save -o /tmp/upgrade-agent.tar upgrade-agent:latest

# Transfer images to the node
echo "Transferring images to ${NODE_NAME}..."
run_scp /tmp/upgrade-server.tar /tmp/
run_scp /tmp/upgrade-agent.tar /tmp/

# Load images on the node
echo "Loading images on ${NODE_NAME}..."
run_ssh "docker load -i /tmp/upgrade-server.tar"
run_ssh "docker load -i /tmp/upgrade-agent.tar"

# Label the node to enable agent and server
echo "Labeling node ${NODE_NAME} for agent and server deployment..."
# Check if labels already exist and use --overwrite if they do
SERVER_LABEL=$(${MINIKUBE_PREFIX} get node ${NODE_NAME} -o jsonpath='{.metadata.labels.upgrade_server_enabled}' 2>/dev/null || echo "")
AGENT_LABEL=$(${MINIKUBE_PREFIX} get node ${NODE_NAME} -o jsonpath='{.metadata.labels.upgrade_agent_enabled}' 2>/dev/null || echo "")

if [ -z "$SERVER_LABEL" ]; then
  echo "Adding upgrade_server_enabled label..."
  ${MINIKUBE_PREFIX} label node ${NODE_NAME} upgrade_server_enabled=true
else
  echo "upgrade_server_enabled label already exists with value: $SERVER_LABEL"
  ${MINIKUBE_PREFIX} label node ${NODE_NAME} upgrade_server_enabled=true --overwrite
fi

if [ -z "$AGENT_LABEL" ]; then
  echo "Adding upgrade_agent_enabled label..."
  ${MINIKUBE_PREFIX} label node ${NODE_NAME} upgrade_agent_enabled=true
else
  echo "upgrade_agent_enabled label already exists with value: $AGENT_LABEL"
  ${MINIKUBE_PREFIX} label node ${NODE_NAME} upgrade_agent_enabled=true --overwrite
fi

# Apply Kubernetes configurations
echo "Applying Kubernetes configurations..."
${MINIKUBE_PREFIX} apply -f kubernetes/upgrade-agent-config.yaml
${MINIKUBE_PREFIX} apply -f kubernetes/upgrade-server-service.yaml
${MINIKUBE_PREFIX} apply -f kubernetes/upgrade-server-daemonset.yaml
${MINIKUBE_PREFIX} apply -f kubernetes/upgrade-agent-daemonset.yaml

# Wait for pods to be ready
echo "Waiting for pods to be ready..."
sleep 10

# Get pod names
AGENT_POD=$(${MINIKUBE_PREFIX} get pods -l app=upgrade-agent -o jsonpath='{.items[0].metadata.name}')
SERVER_POD=$(${MINIKUBE_PREFIX} get pods -l app=upgrade-server -o jsonpath='{.items[0].metadata.name}')

echo "Agent pod: ${AGENT_POD}"
echo "Server pod: ${SERVER_POD}"

# Get container IDs using SSH and docker instead of kubectl
echo "Getting container IDs via SSH..."
AGENT_CONTAINER=$(run_ssh "docker ps | grep upgrade-agent | awk '{print \$1}' | head -1")
SERVER_CONTAINER=$(run_ssh "docker ps | grep upgrade-server | awk '{print \$1}' | head -1")

echo "Agent container: ${AGENT_CONTAINER}"
echo "Server container: ${SERVER_CONTAINER}"

# Verify config is mounted correctly using SSH and docker exec
echo "Verifying config is mounted correctly..."
run_ssh "docker exec ${AGENT_CONTAINER} cat /etc/upgrade-agent/config.yaml"

# Now update the config to trigger an upgrade
echo "Updating target version to trigger an upgrade..."
TEMP_CONFIG=$(mktemp)
${MINIKUBE_PREFIX} get configmap upgrade-agent-config -o yaml > ${TEMP_CONFIG}
sed -i 's/targetVersion: "1.2.1"/targetVersion: "1.2.2"/' ${TEMP_CONFIG}
${MINIKUBE_PREFIX} apply -f ${TEMP_CONFIG}
rm ${TEMP_CONFIG}

# Verify the updated config is picked up by the container
echo "Waiting for config to update..."
sleep 5
echo "Verifying updated config in container..."
run_ssh "docker exec ${AGENT_CONTAINER} cat /etc/upgrade-agent/config.yaml"

# Stream logs from both containers using SSH and docker logs
echo "Streaming logs from agent and server containers..."
echo "=== AGENT LOGS ==="
run_ssh "docker logs -f ${AGENT_CONTAINER} | grep -E 'post-reboot|upgrade in progress|post_upgrade_done|version after'" &
AGENT_LOG_PID=$!

echo "=== SERVER LOGS ==="
run_ssh "docker logs -f ${SERVER_CONTAINER}" &
SERVER_LOG_PID=$!

# Wait for logs to show for a while
echo "Monitoring logs for 60 seconds..."
sleep 60

# Check for post_upgrade_done file
echo "Checking for post_upgrade_done file..."
run_ssh "ls -la /etc/sonic/post_upgrade_done 2>/dev/null || echo 'Post upgrade done file not found'"

# Cleanup
echo "Cleaning up..."
kill ${AGENT_LOG_PID} ${SERVER_LOG_PID} 2>/dev/null || true

# Uncomment to cleanup resources when testing is complete
# ${MINIKUBE_PREFIX} delete -f kubernetes/upgrade-agent-daemonset.yaml
# ${MINIKUBE_PREFIX} delete -f kubernetes/upgrade-server-daemonset.yaml
# ${MINIKUBE_PREFIX} delete -f kubernetes/upgrade-server-service.yaml
# ${MINIKUBE_PREFIX} delete -f kubernetes/upgrade-agent-config.yaml
# echo "Removing labels from node ${NODE_NAME}..."
# ${MINIKUBE_PREFIX} label node ${NODE_NAME} upgrade_agent_enabled- || true
# ${MINIKUBE_PREFIX} label node ${NODE_NAME} upgrade_server_enabled- || true

echo "Test complete!"
