package agent

import (
	"log"
	"os"
	"path/filepath"

	"upgrade-agent/internal/config"
)

const (
	// Simple flag file to indicate if post-upgrade check has been completed
	postUpgradeDoneFile = "/etc/sonic/post_upgrade_done"
)

// UpgradeState tracks the current upgrade process state (simplified)
type UpgradeState struct {
	InProgress    bool
	TargetVersion string
	Config        config.Config
}

// saveUpgradeState marks that an upgrade is in progress
func saveUpgradeState(state UpgradeState) error {
	// Just delete the post_upgrade_done file to indicate an upgrade is needed
	if err := os.Remove(postUpgradeDoneFile); err != nil {
		if !os.IsNotExist(err) {
			log.Printf("Failed to remove post upgrade done file: %v", err)
			return err
		}
	}

	// Ensure directory exists
	dir := filepath.Dir(postUpgradeDoneFile)
	if err := os.MkdirAll(dir, 0755); err != nil {
		log.Printf("Failed to create state directory: %v", err)
		return err
	}

	log.Printf("Successfully marked upgrade in progress by removing %s", postUpgradeDoneFile)
	return nil
}

// loadUpgradeState checks if a post-upgrade verification is needed
func loadUpgradeState() (UpgradeState, error) {
	var state UpgradeState

	// Check if the post_upgrade_done file exists
	_, err := os.Stat(postUpgradeDoneFile)
	if err != nil {
		if os.IsNotExist(err) {
			// File does not exist, post-upgrade check is needed
			state.InProgress = true
			log.Printf("Post-upgrade verification needed: %s not found", postUpgradeDoneFile)
			return state, nil
		}
		// Some other error occurred
		log.Printf("Error checking post upgrade done file: %v", err)
		return state, err
	}

	// File exists, no post-upgrade check needed
	state.InProgress = false
	log.Printf("Post-upgrade verification not needed: %s exists", postUpgradeDoneFile)
	return state, nil
}

// clearUpgradeState marks that the post-upgrade check is complete
func clearUpgradeState() error {
	// Create an empty file to indicate post-upgrade is done
	dir := filepath.Dir(postUpgradeDoneFile)
	if err := os.MkdirAll(dir, 0755); err != nil {
		log.Printf("Failed to create state directory: %v", err)
		return err
	}

	// Create the file
	f, err := os.Create(postUpgradeDoneFile)
	if err != nil {
		log.Printf("Failed to create post upgrade done file: %v", err)
		return err
	}
	defer f.Close()

	log.Printf("Successfully marked post-upgrade complete by creating %s", postUpgradeDoneFile)
	return nil
}
