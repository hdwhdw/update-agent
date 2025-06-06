#!/bin/bash
# Helper script to check or manipulate the post_upgrade_done file on a remote host
# Usage: ./remote_post_upgrade.sh [host] [action]

set -e

SSH_HOST=${1:-"localhost"}
ACTION=${2:-"status"}
FILE_PATH="/etc/sonic/post_upgrade_done"

SSH_USER="${SSH_USER:-admin}"
SSH_OPTIONS="-o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null"

run_ssh() {
  ssh ${SSH_OPTIONS} "${SSH_USER}@${SSH_HOST}" "$@"
}

case "$ACTION" in
  create)
    echo "Creating post_upgrade_done file on ${SSH_HOST}..."
    run_ssh "sudo mkdir -p \$(dirname $FILE_PATH) && sudo touch $FILE_PATH && sudo chmod 644 $FILE_PATH"
    echo "Created: $FILE_PATH on ${SSH_HOST}"
    ;;
  remove)
    echo "Removing post_upgrade_done file on ${SSH_HOST}..."
    run_ssh "sudo rm -f $FILE_PATH"
    echo "Removed: $FILE_PATH from ${SSH_HOST}"
    ;;
  status)
    echo "Checking status of post_upgrade_done file on ${SSH_HOST}..."
    run_ssh "if [ -f $FILE_PATH ]; then echo 'File exists:'; sudo ls -la $FILE_PATH; else echo 'File does not exist: $FILE_PATH'; fi"
    ;;
  *)
    echo "Unknown action: $ACTION"
    echo "Usage: $0 [host] [create|remove|status]"
    exit 1
    ;;
esac
