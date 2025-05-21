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

## Environment Variables

You can also configure the application using environment variables:

```bash
docker run --network=host \
  -e GRPC_TARGET=192.168.1.100:8080 \
  -e FIRMWARE_SOURCE=/path/to/firmware.bin \
  -e UPDATE_MLNX_CPLD_FW=true \
  upgrade-agent:latest
```

## Volumes

To mount firmware files from the host to the container:

```bash
docker run --network=host \
  -v /host/path/to/firmware:/firmware \
  upgrade-agent:latest 192.168.1.100:8080 /firmware/firmware.bin true
```
