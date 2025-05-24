# Upgrade Agent Architecture

## Overview

The Upgrade Agent is a gRPC-based system for managing firmware updates on SONiC devices. It implements the gNOI (gRPC Network Operations Interface) System service and a custom SonicUpgradeService.

## Project Structure

```
upgrade-agent/
├── cmd/                       # Command-line applications
│   ├── test-agent/            # Test client for the upgrade agent
│   │   └── main.go            # Entry point for the test agent
│   ├── upgrade-agent/         # The upgrade agent client
│   │   └── main.go            # Entry point for the upgrade agent
│   └── upgrade-server/        # The upgrade server
│       └── main.go            # Entry point for the upgrade server
├── gnoi_sonic/                # Generated gRPC code for the Sonic service
│   ├── sonic_upgrade_grpc.pb.go # Generated gRPC bindings
│   └── sonic_upgrade.pb.go    # Generated protocol buffer code
├── internal/                  # Internal packages
│   ├── agent/                 # The core upgrade agent
│   │   └── agent.go           # Agent implementation
│   ├── config/                # Configuration handling
│   │   └── config.go          # Configuration manager
│   ├── grpcclient/            # gRPC client implementation
│   │   └── client.go          # Client implementation
│   ├── grpcserver/            # gRPC server implementation
│   │   └── server.go          # Server implementation
│   ├── osservice/             # gNOI OS service implementation
│   │   └── os.go              # OSService implementation
│   ├── sonicservice/          # SonicUpgradeService implementation
│   │   └── sonic.go           # SonicUpgradeService implementation
│   └── systemservice/         # gNOI System service implementation
│       └── system.go          # SystemService implementation
├── proto/                     # Protocol buffer definitions
│   └── sonic_upgrade.proto    # SonicUpgradeService definition
└── test/                      # Testing scripts and utilities
    ├── monitor_logs.sh        # Script for monitoring logs
    └── test_upgrade.sh        # Test script for the upgrade process
```

## Components

### gRPC Server

The grpcserver package (`internal/grpcserver/server.go`) provides a generic gRPC server that hosts both the gNOI System service and the SonicUpgradeService. It handles:

- Server initialization
- Service registration
- Graceful shutdown
- Signal handling

### Agent

The agent package (`internal/agent/agent.go`) implements the core logic of the upgrade agent, which includes:

- Managing firmware updates
- Communicating with the gRPC server
- Handling configuration updates
- Processing reboot and verification workflows

### System Service

The systemservice package (`internal/systemservice/system.go`) implements the gNOI System service, which provides basic system functionality including:

- Time retrieval (System.Time RPC)

### OS Service

The osservice package (`internal/osservice/os.go`) implements the gNOI OS service, which provides:

- OS version information (OS.Verify RPC)
- Extracts SONiC OS version from boot image path in `/proc/cmdline`

### Sonic Upgrade Service

The sonicservice package (`internal/sonicservice/sonic.go`) implements the SonicUpgradeService, which provides:

- Firmware update functionality (UpdateFirmware RPC)

### gRPC Client

The grpcclient package (`internal/grpcclient/client.go`) provides a client for interacting with the gRPC server. It includes:

- Establishing connections to the gRPC server
- Methods for invoking RPCs on the SonicUpgradeService and gNOI services
- Handling of streaming responses for the firmware update process

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
4. Update the `internal/grpcserver/server.go` file to register the new service

## Docker Support

The project includes Docker support for easy deployment:

- `Dockerfile` - For building the upgrade agent
- `Dockerfile.server` - For building the upgrade server
- `docker-compose.yml` - For running both services together
