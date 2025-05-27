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

		// Method 1: Use the reboot syscall directly (most direct way to reboot)
		cmd := exec.Command("sync") // Sync filesystem before reboot
		cmd.Run()

		// Attempt direct syscall access to reboot
		log.Println("Attempting reboot via direct syscall")
		cmd = exec.Command("sh", "-c", "echo b > /proc/sysrq-trigger")
		err := cmd.Run()

		if err != nil {
			log.Printf("Error executing direct syscall reboot: %v", err)

			// Method 2: Use nsenter to access the host's namespaces
			log.Println("Attempting reboot via nsenter")
			cmd = exec.Command("nsenter", "-m", "-u", "-i", "-n", "-p", "-t", "1", "reboot", "-f")
			err = cmd.Run()

			if err != nil {
				log.Printf("Error executing reboot via nsenter: %v", err)

				// Method 3: Try various paths to the reboot command
				rebootPaths := []string{
					"/sbin/reboot",
					"/bin/reboot",
					"/usr/sbin/reboot",
					"/host/sbin/reboot",
					"/host/bin/reboot",
					"/host/usr/sbin/reboot",
				}

				for _, path := range rebootPaths {
					log.Printf("Attempting reboot with %s", path)
					cmd = exec.Command(path, "-f")
					err = cmd.Run()
					if err == nil {
						log.Printf("Successfully initiated reboot with %s", path)
						return
					}
				}

				log.Println("All reboot attempts failed, trying force reboot through sysrq")
				// Last resort - force reboot through kernel sysrq
				cmd = exec.Command("sh", "-c", "echo 1 > /proc/sys/kernel/sysrq && echo b > /proc/sysrq-trigger")
				cmd.Run()
			} else {
				log.Println("Successfully initiated reboot with nsenter")
			}
		} else {
			log.Println("Successfully initiated reboot with direct syscall")
		}
	}()

	log.Println("Reboot scheduled successfully")
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
