#!/bin/bash
# Simple script to manually create or remove the post_upgrade_done file
# This is useful for testing the post-upgrade verification

set -e

ACTION=${1:-"status"}
FILE_PATH="/etc/sonic/post_upgrade_done"

case "$ACTION" in
  create)
    echo "Creating post_upgrade_done file..."
    sudo mkdir -p "$(dirname "$FILE_PATH")"
    sudo touch "$FILE_PATH"
    sudo chmod 644 "$FILE_PATH"
    echo "Created: $FILE_PATH"
    ;;
  remove)
    echo "Removing post_upgrade_done file..."
    sudo rm -f "$FILE_PATH"
    echo "Removed: $FILE_PATH"
    ;;
  status)
    if [ -f "$FILE_PATH" ]; then
      echo "Post upgrade done file exists: $FILE_PATH"
      ls -la "$FILE_PATH"
    else
      echo "Post upgrade done file does not exist: $FILE_PATH"
    fi
    ;;
  *)
    echo "Unknown action: $ACTION"
    echo "Usage: $0 [create|remove|status]"
    exit 1
    ;;
esac
