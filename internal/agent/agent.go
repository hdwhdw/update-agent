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

	// Check if we need to resume an upgrade after reboot
	state, err := loadUpgradeState()
	if err != nil {
		log.Printf("Warning: Failed to load upgrade state: %v", err)
	} else if state.InProgress {
		log.Printf("Detected incomplete upgrade. Resuming post-reboot verification...")
		go a.performPostRebootVerification(cfg)
	}

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

	// Save the upgrade state before initiating reboot
	state := UpgradeState{
		InProgress:    true,
		TargetVersion: cfg.TargetVersion,
		Config:        cfg,
	}

	if err := saveUpgradeState(state); err != nil {
		log.Printf("Warning: Failed to save upgrade state: %v", err)
	} else {
		log.Printf("Marked upgrade in progress before reboot")
	}

	// Initiate a system reboot after successful firmware update
	log.Printf("Initiating system reboot to complete firmware update process")
	rebootCtx, rebootCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer rebootCancel()

	if err := client.Reboot(rebootCtx); err != nil {
		if a.shouldIgnoreError(err, cfg) {
			log.Printf("Reboot RPC unimplemented, skipping ahead: %v", err)
			// Since we're not actually rebooting, continue with post-reboot verification
			a.performPostRebootVerification(cfg)
		} else {
			log.Printf("Warning: Failed to initiate reboot after firmware update: %v", err)
			// Clear the upgrade state since the reboot failed
			if err := clearUpgradeState(); err != nil {
				log.Printf("Warning: Failed to mark post-upgrade completion: %v", err)
			}
		}
	} else {
		log.Printf("System reboot request sent successfully")
		log.Printf("Agent will be terminated by the reboot. Post-reboot verification will resume after restart.")

		// Give some time for the logs to be written and the reboot to start
		time.Sleep(5 * time.Second)

		// The agent will be terminated here by the system reboot
		// The remaining verification will be performed when the agent restarts
	}
}

// performPostRebootVerification performs the verification steps after a reboot
func (a *Agent) performPostRebootVerification(cfg config.Config) {
	log.Printf("Starting post-reboot verification for version %s", cfg.TargetVersion)

	a.lock.Lock()
	client := a.client
	a.lock.Unlock()

	if client == nil {
		log.Println("Cannot perform post-reboot verification: client not initialized")
		return
	}

	// Wait for system services to stabilize
	log.Printf("Waiting for system services to stabilize (60 seconds)...")
	time.Sleep(60 * time.Second)
	log.Printf("System stabilization period complete, proceeding with post-update verification")

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

	// Clear the upgrade state file since we've completed the verification
	if err := clearUpgradeState(); err != nil {
		log.Printf("Warning: Failed to mark post-upgrade completion: %v", err)
	} else {
		log.Printf("Upgrade completed successfully and marked as done")
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
