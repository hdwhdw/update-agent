# Upgrade Agent Architecture

## Overview

The Upgrade Agent is a gRPC-based system for managing firmware updates on SONiC devices. It implements the gNOI (gRPC Network Operations Interface) System service and a custom SonicUpgradeService.

## Project Structure

```
upgrade-agent/
├── cmd/                       # Command-line applications
│   ├── test-agent/            # Test client for the upgrade agent
│   ├── upgrade-agent/         # The upgrade agent client
│   └── upgrade-server/        # The upgrade server
├── gnoi_sonic/                # Generated gRPC code for the Sonic service
├── internal/                  # Internal packages
│   ├── config/                # Configuration handling
│   ├── grpcclient/            # gRPC client implementation
│   ├── server/                # Server implementation
│   ├── service/               # Common service functionality
│   ├── sonicservice/          # SonicUpgradeService implementation
│   └── systemservice/         # gNOI System service implementation
├── proto/                     # Protocol buffer definitions
│   └── sonic_upgrade.proto    # SonicUpgradeService definition
└── test/                      # Testing scripts and utilities
```

## Components

### Server

The server package (`internal/server`) provides a generic gRPC server that hosts both the gNOI System service and the SonicUpgradeService. It handles:

- Server initialization
- Service registration
- Graceful shutdown
- Signal handling

### System Service

The systemservice package (`internal/systemservice`) implements the gNOI System service, which provides basic system functionality including:

- Time retrieval (System.Time RPC)

### Sonic Upgrade Service

The sonicservice package (`internal/sonicservice`) implements the SonicUpgradeService, which provides:

- Firmware update functionality (UpdateFirmware RPC)

## Communication Flow

1. A client (such as the upgrade-agent) connects to the upgrade-server using gRPC
2. The client sends requests to the appropriate service (System or SonicUpgrade)
3. The server processes these requests and returns responses
4. For streaming RPCs like UpdateFirmware, status updates are sent continuously until completion

## Building and Running

To build the upgrade server:

```sh
go build -o upgrade-server ./cmd/upgrade-server
```

To run the upgrade server:

```sh
./upgrade-server --port 8080
```

## Extending the Server

To add new services:

1. Define the service in a Protocol Buffer file
2. Generate the Go code using the `protoc` compiler
3. Create a new package under `internal/` to implement the service
4. Update the `internal/server/server.go` file to register the new service

## Docker Support

The project includes Docker support for easy deployment:

- `Dockerfile` - For building the upgrade agent
- `Dockerfile.server` - For building the upgrade server
- `docker-compose.yml` - For running both services together
