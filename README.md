# Upgrade Agent

A gRPC client for firmware updates via the SonicUpgradeService.

## Building the Docker Image

To build the Docker image:

```bash
docker build -t upgrade-agent:latest .
```

## Running the Container

The upgrade-agent requires three arguments:
1. `grpc_target`: The gRPC server address (host:port)
2. `firmware_source`: Path or URL to the firmware source
3. `update_mlnx_cpld_fw`: Whether to update MLNX CPLD firmware ("true" or "false")

Example:

```bash
docker run --network=host upgrade-agent:latest 192.168.1.100:8080 /path/to/firmware.bin true
```

## Configuration

The agent can be configured using a YAML file. By default, it looks for `/etc/upgrade-agent/config.yaml`.

Example configuration:

```yaml
grpcTarget: "192.168.1.100:8080"        # gRPC server address (host:port)
firmwareSource: "/firmware/sonic.bin"   # Path to firmware file
updateMlnxCpldFw: true                  # Whether to update MLNX CPLD firmware
targetVersion: "1.0.0"                  # Target firmware version
ignoreUnimplementedRPC: false           # Whether to treat unimplemented gRPC errors as success (for testing)
```

## Environment Variables

You can also configure the application using environment variables:

```bash
docker run --network=host \
  -e GRPC_TARGET=192.168.1.100:8080 \
  -e FIRMWARE_SOURCE=/path/to/firmware.bin \
  -e UPDATE_MLNX_CPLD_FW=true \
  upgrade-agent:latest
```

## Upgrade Server

The upgrade-server provides gRPC services that implement:
1. gNOI System service
2. gNOI OS service
3. SonicUpgradeService for firmware updates

### Building the Server

```bash
go build -o upgrade-server ./cmd/upgrade-server
```

### Running the Server

```bash
./upgrade-server --port 8080
```

### Docker for Server

You can also run the server using Docker:

```bash
docker build -f Dockerfile.server -t upgrade-server:latest .
docker run -p 8080:8080 upgrade-server:latest
```

### Running the OS.Verify Service

The OS.Verify service extracts the SONiC OS version from the boot image path in `/proc/cmdline`. It looks for patterns like `/image-master.858213-545f73f0a/` and formats the version as `SONiC.master.858213-545f73f0a`.

When running the server in a container on a SONiC device, the simplest solution is to mount the host's filesystem:

```bash
docker run -p 8080:8080 \
  -v /:/host:ro \
  upgrade-server:latest
```

This approach gives the container read-only access to everything on the host, including:
- `/host/proc/cmdline` for extracting the SONiC OS version from boot image path
- Any other path that might contain version information

The server will automatically look for version information in both regular and host-mounted paths, with a preference for host-mounted paths.

If you're experiencing issues with the OS.Verify service, enable verbose logging:

```bash
docker run -p 8080:8080 \
  -v /:/host:ro \
  -e LOG_LEVEL=debug \
  upgrade-server:latest
```

## Project Structure

See [Architecture Documentation](docs/architecture.md) for details on the project structure and component design.

## Volumes

To mount firmware files from the host to the container:

```bash
docker run --network=host \
  -v /host/path/to/firmware:/firmware \
  upgrade-agent:latest 192.168.1.100:8080 /firmware/firmware.bin true
```

## Deployment on SONiC Devices

When deploying the upgrade-server on a SONiC device, ensure proper access to the host filesystem:

```bash
docker run -p 50052:50052 \
  -v /proc:/proc:ro \
  --network=host \
  upgrade-server:latest
```

This ensures the OS.Verify service can correctly extract the SONiC version from the boot image path in `/proc/cmdline`. The version is formatted as `SONiC.master.858213-545f73f0a`.

## Testing

The project includes a simple script for testing the containerized agent:

```bash
# Run the test script
cd /path/to/upgrade-agent
./test/test_upgrade.sh                            # Run test in background mode
./test/test_upgrade.sh -i                         # Run test in interactive mode with logs visible
./test/test_upgrade.sh --ignore-unimplemented     # Run with unimplemented gRPC errors treated as success
```

### Viewing Logs

Several options are available for viewing container logs:

1. **Interactive Mode**:
   ```bash
   ./test/test_upgrade.sh --interactive
   ```

2. **Monitor Logs**:
   ```bash
   ./test/monitor_logs.sh [container-name]        # Monitor existing container
   ./test/monitor_logs.sh --wait                  # Wait for container and monitor until test completes
   ```

3. **Standard Docker Logs**:
   ```bash
   docker logs -f upgrade-agent-test
   ```

4. **Log File**:
   ```bash
   tail -f /tmp/agent-output.log
   ```
