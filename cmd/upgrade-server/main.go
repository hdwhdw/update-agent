package main

import (
	"flag"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"

	gnoisonic "upgrade-agent/gnoi_sonic"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// SonicUpgradeServer is a minimal implementation of the SonicUpgradeService
type SonicUpgradeServer struct {
	gnoisonic.UnimplementedSonicUpgradeServiceServer
}

// UpdateFirmware implements the gRPC firmware update service
func (s *SonicUpgradeServer) UpdateFirmware(stream gnoisonic.SonicUpgradeService_UpdateFirmwareServer) error {
	// Placeholder implementation - doesn't actually do anything
	log.Println("Received UpdateFirmware request")

	// Read the request parameters
	req, err := stream.Recv()
	if err != nil {
		log.Printf("Error receiving request: %v", err)
		return status.Errorf(codes.Internal, "failed to receive request: %v", err)
	}

	params := req.GetFirmwareUpdate()
	log.Printf("Firmware update request: source=%s, updateMlnxCpldFw=%v",
		params.GetFirmwareSource(), params.GetUpdateMlnxCpldFw())

	// Send a dummy response sequence
	responses := []struct {
		logLine  string
		state    gnoisonic.UpdateFirmwareStatus_State
		exitCode int32
	}{
		{"Starting firmware update...", gnoisonic.UpdateFirmwareStatus_STARTED, 0},
		{"Checking firmware file...", gnoisonic.UpdateFirmwareStatus_RUNNING, 0},
		{"Validating firmware signature...", gnoisonic.UpdateFirmwareStatus_RUNNING, 0},
		{"Preparing update process...", gnoisonic.UpdateFirmwareStatus_RUNNING, 0},
		{"Applying firmware update...", gnoisonic.UpdateFirmwareStatus_RUNNING, 0},
		{"Firmware update completed successfully", gnoisonic.UpdateFirmwareStatus_SUCCEEDED, 0},
	}

	for _, resp := range responses {
		if err := stream.Send(&gnoisonic.UpdateFirmwareStatus{
			LogLine:  resp.logLine,
			State:    resp.state,
			ExitCode: resp.exitCode,
		}); err != nil {
			log.Printf("Error sending response: %v", err)
			return status.Errorf(codes.Internal, "failed to send response: %v", err)
		}
	}

	log.Println("Firmware update request completed")
	return nil
}

func main() {
	port := flag.String("port", "8080", "The server port")
	flag.Parse()

	log.Printf("Starting upgrade server on port %s", *port)

	lis, err := net.Listen("tcp", "0.0.0.0:"+*port)
	if err != nil {
		log.Fatalf("Failed to listen: %v", err)
	}

	grpcServer := grpc.NewServer()
	gnoisonic.RegisterSonicUpgradeServiceServer(grpcServer, &SonicUpgradeServer{})

	// Start server in a goroutine
	go func() {
		if err := grpcServer.Serve(lis); err != nil {
			log.Fatalf("Failed to serve: %v", err)
		}
	}()

	log.Printf("Server listening at %v", lis.Addr())

	// Wait for termination signal
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	sig := <-sigCh

	log.Printf("Received signal %v, shutting down...", sig)
	grpcServer.GracefulStop()
}
