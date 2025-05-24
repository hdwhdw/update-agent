package agent

import (
	"context"
	"log"
	"sync"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"upgrade-agent/gnoi_sonic"
	"upgrade-agent/internal/config"
	"upgrade-agent/internal/grpcclient"
)

// Agent manages the firmware update process
type Agent struct {
	client        *grpcclient.Client
	currentConfig config.Config
	lastVersion   string
	lock          sync.Mutex
}

// NewAgent creates a new agent instance
func NewAgent() *Agent {
	return &Agent{}
}

// UpdateConfig handles updates to the configuration
func (a *Agent) UpdateConfig(cfg config.Config) {
	a.lock.Lock()
	defer a.lock.Unlock()

	log.Printf("Received config update: target version=%s", cfg.TargetVersion)

	// Save the new config
	a.currentConfig = cfg

	// If target version has changed, trigger update
	if a.lastVersion != "" && a.lastVersion != cfg.TargetVersion {
		log.Printf("Target version changed from %s to %s, triggering update",
			a.lastVersion, cfg.TargetVersion)

		go a.performUpdate(cfg)
	}

	a.lastVersion = cfg.TargetVersion
}

// Initialize sets up the initial agent state
func (a *Agent) Initialize(cfg config.Config) error {
	a.lock.Lock()
	defer a.lock.Unlock()

	// Create a new gRPC client
	client, err := grpcclient.NewClient(cfg.GrpcTarget)
	if err != nil {
		return err
	}

	a.client = client
	a.currentConfig = cfg
	a.lastVersion = cfg.TargetVersion

	log.Printf("Agent initialized with target version: %s", cfg.TargetVersion)
	return nil
}

// performUpdate initiates a firmware update based on the provided config
func (a *Agent) performUpdate(cfg config.Config) {
	// Create a copy of the config to avoid race conditions
	a.lock.Lock()
	client := a.client
	a.lock.Unlock()

	if client == nil {
		log.Println("Cannot perform update: client not initialized")
		return
	}

	log.Printf("Starting firmware update to version %s", cfg.TargetVersion)
	log.Printf("Using target: %s, firmware: %s, updateMlnxCpld: %s",
		cfg.GrpcTarget, cfg.FirmwareSource, cfg.UpdateMlnxCpldFw)

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	// First, get the system time via gNOI.System.Time
	timeCtx, timeCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer timeCancel()

	timeResp, err := client.GetSystemTime(timeCtx)
	if err != nil {
		log.Printf("Warning: Failed to get system time: %v", err)
		// Continue with update even if time request fails
		if !a.shouldIgnoreError(err, cfg) {
			// Only log as warning if not an ignored error type
			log.Printf("Warning: Failed to get system time: %v", err)
		}
		// Continue with update even if time request fails
	} else {
		nanos := int64(timeResp.GetTime())
		systemTime := time.Unix(nanos/1e9, nanos%1e9)
		log.Printf("System time before update: %v (timestamp: %d ns)",
			systemTime, timeResp.GetTime())
	}

	// Get OS version via gNOI.OS.Verify
	osCtx, osCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer osCancel()

	osResp, err := client.GetOSVersion(osCtx)
	if err != nil {
		if !a.shouldIgnoreError(err, cfg) {
			log.Printf("Warning: Failed to get OS version: %v", err)
		}
		// Continue with update even if OS version request fails
	} else {
		log.Printf("OS version before update: %s", osResp.GetVersion())
		if failMsg := osResp.GetActivationFailMessage(); failMsg != "" {
			log.Printf("Previous activation failure message: %s", failMsg)
		}
	}

	// Prepare update parameters
	params := &gnoi_sonic.FirmwareUpdateParams{
		FirmwareSource:   cfg.FirmwareSource,
		UpdateMlnxCpldFw: cfg.UpdateMlnxCpldFw == "true",
	}

	// Initiate the update
	if err := client.UpdateFirmware(ctx, params); err != nil {
		if a.shouldIgnoreError(err, cfg) {
			log.Printf("Firmware update RPC unimplemented, skipping ahead: %v", err)
		} else {
			log.Printf("Firmware update failed: %v", err)
			return
		}
	}

	log.Printf("Firmware update to version %s completed successfully", cfg.TargetVersion)

	// Initiate a system reboot after successful firmware update
	log.Printf("Initiating system reboot to complete firmware update process")
	rebootCtx, rebootCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer rebootCancel()

	if err := client.Reboot(rebootCtx); err != nil {
		if a.shouldIgnoreError(err, cfg) {
			log.Printf("Reboot RPC unimplemented, skipping ahead: %v", err)
		} else {
			log.Printf("Warning: Failed to initiate reboot after firmware update: %v", err)
		}
		// Continue with post-update checks even if reboot request fails
	} else {
		log.Printf("System reboot request sent successfully")

		// Wait for reboot to complete
		log.Printf("Waiting for system to reboot...")		// Give the system some time to start rebooting
		log.Printf("Waiting 10 seconds for system to begin reboot process...")
		time.Sleep(10 * time.Second)

		// Poll for reboot completion with a reasonable timeout
		rebootWaitCtx, rebootWaitCancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer rebootWaitCancel()// Poll the reboot status until either the reboot is complete or timeout occurs
		rebootComplete := false
		rebootPollTimer := time.NewTicker(10 * time.Second)
		defer rebootPollTimer.Stop()

		for !rebootComplete {
			select {
			case <-rebootWaitCtx.Done():
				log.Printf("Timeout waiting for reboot to complete")
				// In case of timeout, just proceed with the rest of the flow
				rebootComplete = true
			case <-rebootPollTimer.C:
				log.Printf("Checking if device has completed reboot...")
				// Try to check reboot status
				statusCtx, statusCancel := context.WithTimeout(context.Background(), 5*time.Second)
				resp, err := client.GetRebootStatus(statusCtx)
				statusCancel()

				if err != nil {
					if a.shouldIgnoreError(err, cfg) {
						log.Printf("Reboot status RPC unimplemented, assuming reboot complete: %v", err)
						rebootComplete = true
					} else {
						log.Printf("Device unreachable during reboot (expected): %v", err)
						// Connection error is expected during reboot, continue polling
					}
					continue
				}

				// If we can contact the device, check reboot status
				if !resp.GetActive() {
					log.Printf("System reboot completed successfully")
					rebootComplete = true
				} else {
					log.Printf("Device reboot still in progress, continuing to wait...")
				}
			}
		}

		// Allow some additional time for all services to fully initialize
		log.Printf("Waiting for system services to stabilize (60 seconds)...")
		time.Sleep(60 * time.Second)
		log.Printf("System stabilization period complete, proceeding with post-update verification")
	}

	// Get OS version after update via gNOI.OS.Verify to confirm successful update
	postUpdateOsCtx, postUpdateOsCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer postUpdateOsCancel()

	postUpdateOsResp, err := client.GetOSVersion(postUpdateOsCtx)
	if err != nil {
		if !a.shouldIgnoreError(err, cfg) {
			log.Printf("Warning: Failed to get OS version after update: %v", err)
		}
	} else {
		log.Printf("OS version after update: %s", postUpdateOsResp.GetVersion())
		if failMsg := postUpdateOsResp.GetActivationFailMessage(); failMsg != "" {
			log.Printf("Update activation failure message: %s", failMsg)
		}
	}
}

// Close cleans up resources
func (a *Agent) Close() error {
	a.lock.Lock()
	defer a.lock.Unlock()

	if a.client != nil {
		return a.client.Close()
	}
	return nil
}

// shouldIgnoreError determines if the given error should be ignored
// based on configuration settings
func (a *Agent) shouldIgnoreError(err error, cfg config.Config) bool {
	if !cfg.IgnoreUnimplementedRPC {
		return false
	}

	// Check if the error is a gRPC Unimplemented error
	if st, ok := status.FromError(err); ok {
		if st.Code() == codes.Unimplemented {
			log.Printf("Ignoring unimplemented RPC error: %v", err)
			return true
		}
	}

	return false
}
