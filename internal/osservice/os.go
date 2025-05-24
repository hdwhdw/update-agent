// Package osservice implements the gNOI OS service
package osservice

import (
	"context"
	"fmt"
	"log"
	"os"
	"regexp"

	gnoios "github.com/openconfig/gnoi/os"
)

// OSService implements the gNOI OS service
type OSService struct {
	gnoios.UnimplementedOSServer
}

// NewOSService creates a new OS service instance
func NewOSService() *OSService {
	return &OSService{}
}

// Verify implements the gNOI OS.Verify RPC to return the current running OS version
func (s *OSService) Verify(ctx context.Context, req *gnoios.VerifyRequest) (*gnoios.VerifyResponse, error) {
	log.Println("Received OS.Verify request")

	// Get OS version from /proc/cmdline
	version, err := getOSVersionFromCmdline()
	if err != nil {
		log.Printf("Warning: Failed to get SONiC OS version: %v", err)
		// Use a default unknown version if extraction fails
		version = "unknown"

		// Log diagnostic information to help debug mounting issues
		hostExists, _ := fileExists("/host")
		hostProcExists, _ := fileExists("/host/proc")
		hostProcCmdlineExists, _ := fileExists("/host/proc/cmdline")
		procExists, _ := fileExists("/proc")
		procCmdlineExists, _ := fileExists("/proc/cmdline")

		log.Printf("Diagnostic info for mounting: /host exists: %v, /host/proc exists: %v, /host/proc/cmdline exists: %v",
			hostExists, hostProcExists, hostProcCmdlineExists)
		log.Printf("Diagnostic info for proc: /proc exists: %v, /proc/cmdline exists: %v",
			procExists, procCmdlineExists)

		// Try to directly read content of proc/cmdline for debugging
		if hostProcCmdlineExists {
			content, readErr := os.ReadFile("/host/proc/cmdline")
			if readErr == nil {
				log.Printf("Content of /host/proc/cmdline: %s", string(content))
			} else {
				log.Printf("Error reading /host/proc/cmdline: %v", readErr)
			}
		} else if procCmdlineExists {
			content, readErr := os.ReadFile("/proc/cmdline")
			if readErr == nil {
				log.Printf("Content of /proc/cmdline: %s", string(content))
			} else {
				log.Printf("Error reading /proc/cmdline: %v", readErr)
			}
		}
	}

	log.Printf("Responding with SONiC OS version: %v", version)

	// Create a StandbyState to indicate this is not a dual supervisor system
	standbyState := &gnoios.StandbyState{
		State: gnoios.StandbyState_UNSUPPORTED,
	}

	return &gnoios.VerifyResponse{
		Version: version,
		// We don't have dual supervisor support in this implementation
		VerifyStandby: &gnoios.VerifyStandby{
			State: &gnoios.VerifyStandby_StandbyState{
				StandbyState: standbyState,
			},
		},
		// Indicate that this is not a dual supervisor system that requires
		// individual supervisor installation
		IndividualSupervisorInstall: false,
	}, nil
}

// fileExists checks if a file or directory exists
func fileExists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

// getOSVersionFromCmdline reads the OS version from /proc/cmdline
// This typically contains kernel parameters including version information
func getOSVersionFromCmdline() (string, error) {
	// Simple implementation: just use the proc/cmdline method
	return getVersionFromProcCmdline()
}

// getVersionFromProcCmdline tries to extract version from /proc/cmdline
func getVersionFromProcCmdline() (string, error) {
	// Define potential file paths, with host-mounted variants first
	cmdlinePaths := []string{
		"/host/proc/cmdline",  // Host filesystem mounted at /host
		"/proc/cmdline",       // Direct proc access
	}

	var content []byte
	var err error
	var readPath string

	// Try each path until we find one that works
	for _, path := range cmdlinePaths {
		content, err = os.ReadFile(path)
		if err == nil {
			readPath = path
			break
		}
	}

	if err != nil {
		return "", fmt.Errorf("failed to read cmdline from any path: %v", err)
	}

	log.Printf("Successfully read cmdline from %s", readPath)
	cmdline := string(content)

	// Look for SONiC image pattern like /image-master.858213-545f73f0a/
	// This pattern matches the boot image path in SONiC systems
	sonicPattern := regexp.MustCompile(`/image-master\.([0-9]+-[0-9a-f]+)/`)
	matches := sonicPattern.FindStringSubmatch(cmdline)

	if len(matches) > 1 {
		// Format output as: SONiC.master.858213-545f73f0a
		sonicVersion := "SONiC.master." + matches[1]
		log.Printf("Extracted SONiC version from image path: %s", sonicVersion)
		return sonicVersion, nil
	}

	// If not found, return an error instead of a placeholder message
	return "", fmt.Errorf("SONiC version pattern not found in cmdline")
}

// Install is a placeholder for the OS Install RPC
func (s *OSService) Install(stream gnoios.OS_InstallServer) error {
	log.Println("Received OS.Install request, but not implemented")
	return nil
}

// Activate is a placeholder for the OS Activate RPC
func (s *OSService) Activate(ctx context.Context, req *gnoios.ActivateRequest) (*gnoios.ActivateResponse, error) {
	log.Println("Received OS.Activate request, but not implemented")
	return &gnoios.ActivateResponse{
		Response: &gnoios.ActivateResponse_ActivateError{
			ActivateError: &gnoios.ActivateError{
				Type: gnoios.ActivateError_UNSPECIFIED,
				Detail: "OS.Activate not implemented",
			},
		},
	}, nil
}
