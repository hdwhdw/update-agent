// Package sonicservice implements the gNOI Sonic Upgrade service
package sonicservice

import (
	"log"

	gnoisonic "upgrade-agent/gnoi_sonic"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Service implements the SonicUpgradeService
type Service struct {
	gnoisonic.UnimplementedSonicUpgradeServiceServer
}

// NewService creates a new SonicUpgradeService instance
func NewService() *Service {
	return &Service{}
}

// UpdateFirmware implements the gRPC firmware update service
func (s *Service) UpdateFirmware(stream gnoisonic.SonicUpgradeService_UpdateFirmwareServer) error {
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
