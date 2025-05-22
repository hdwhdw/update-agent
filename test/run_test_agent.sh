#!/bin/bash
# Run the test agent with specified parameters

CONFIG_PATH="/tmp/config.yaml"
GRPC_TARGET="10.250.0.101:8080"
FIRMWARE_SOURCE="/tmp/sonic.bin"
UPDATE_MLNX_CPLD="true"
INITIAL_VERSION="1.0.0"
NEW_VERSION="1.1.0"

# Create or update the config file
cat > ${CONFIG_PATH} << EOF
grpcTarget: ${GRPC_TARGET}
firmwareSource: ${FIRMWARE_SOURCE}
updateMlnxCpldFw: "${UPDATE_MLNX_CPLD}"
targetVersion: "${INITIAL_VERSION}"  # When this field is updated, it will trigger an update
EOF

# Run the test agent
echo "Running test agent to update version from ${INITIAL_VERSION} to ${NEW_VERSION}..."
CONFIG_PATH=${CONFIG_PATH} go run ./cmd/test-agent/main.go -test -version="${NEW_VERSION}" -delay=5
