// Package systemservice implements the gNOI System service
package systemservice

import (
	"context"
	"log"
	"os/exec"
	"time"

	"github.com/openconfig/gnoi/system"
)

// Service implements the gNOI System service
type Service struct {
	system.UnimplementedSystemServer
	fakeReboot bool
}

// NewService creates a new System service instance
func NewService(fakeReboot bool) *Service {
	return &Service{
		fakeReboot: fakeReboot,
	}
}

// Time implements the gNOI System.Time RPC
func (s *Service) Time(ctx context.Context, req *system.TimeRequest) (*system.TimeResponse, error) {
	log.Println("Received System.Time request")

	// Get the current system time in nanoseconds since epoch
	now := time.Now()
	nanos := now.UnixNano()

	log.Printf("Responding with current system time: %v", now)
	return &system.TimeResponse{
		Time: uint64(nanos),
	}, nil
}

// Reboot implements the gNOI System.Reboot RPC
func (s *Service) Reboot(ctx context.Context, req *system.RebootRequest) (*system.RebootResponse, error) {
	log.Printf("Received System.Reboot request with method: %v", req.GetMethod())

	// In a real implementation, we would handle the different reboot methods, delay, etc.
	// For this proof of concept, we'll just log the request and initiate a reboot

	// Start a goroutine to perform the reboot after a short delay
	go func() {
		log.Println("Scheduling system reboot in 2 seconds...")
		time.Sleep(2 * time.Second)

		// Check if we should fake the reboot
		if s.fakeReboot {
			log.Println("FAKE REBOOT MODE: Simulating a system reboot without actually rebooting")
			return
		}
		
		log.Println("Executing reboot command on host system")
		
		// We've verified we can access the host's namespaces correctly
		log.Println("Initiating host reboot via nsenter")
		
		// Ensure all log messages are written before the reboot command
		log.Println("--------- REBOOT COMMAND WILL BE EXECUTED NEXT ---------")
		// Force flush log buffers by syncing filesystem
		cmd := exec.Command("sync")
		cmd.Run()
		time.Sleep(1 * time.Second)
		
		// Use the exact command format specified for rebooting the host
		log.Printf("Executing reboot command: nsenter --target 1 --mount --uts --ipc --net --pid reboot")
		
		// Run the command and don't wait for output to avoid being killed mid-execution
		rebootCmd := exec.Command("nsenter", "--target", "1", "--mount", "--uts", "--ipc", "--net", "--pid", "reboot")
		err := rebootCmd.Start()
		
		if err != nil {
			log.Printf("Error starting reboot command: %v", err)
		} else {
			log.Println("Reboot command started successfully, system will reboot momentarily...")
		}
		
		// Give the command a moment to execute
		time.Sleep(1 * time.Second)
		
		// Log immediately after attempt to ensure we see this before any reboot happens
		log.Printf("Reboot command executed. System should be rebooting now.")
		
		// Sleep a bit to ensure logs are flushed
		log.Println("Waiting for reboot to take effect...")
		time.Sleep(5 * time.Second)
	}()

	log.Println("Reboot initiated successfully")
	return &system.RebootResponse{}, nil
}

// RebootStatus implements the gNOI System.RebootStatus RPC
func (s *Service) RebootStatus(ctx context.Context, req *system.RebootStatusRequest) (*system.RebootStatusResponse, error) {
	log.Println("Received System.RebootStatus request")

	// For this simple implementation, we'll just return a fixed response
	// In a real implementation, we would track the reboot status

	if s.fakeReboot {
		log.Println("FAKE REBOOT MODE: Reporting reboot as completed")
	}

	return &system.RebootStatusResponse{
		Active: false, // No reboot is currently active
		Wait:   0,     // No wait time
		Reason: "No reboot scheduled",
		Count:  1,     // Assume at least one reboot has occurred
		Method: system.RebootMethod_COLD, // Assume a cold reboot
	}, nil
}
