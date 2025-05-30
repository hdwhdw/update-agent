package agent

import (
	"encoding/json"
	"log"
	"os"
	"path/filepath"

	"upgrade-agent/internal/config"
)

const (
	stateFilePath = "/var/tmp/upgrade-agent-state.json" // Use /var/tmp which is inside the container
)

// UpgradeState tracks the current upgrade process state
type UpgradeState struct {
	InProgress    bool   `json:"inProgress"`
	TargetVersion string `json:"targetVersion"`
	Config        config.Config `json:"config"`
}

// saveUpgradeState persists the upgrade state to a file
func saveUpgradeState(state UpgradeState) error {
	// Ensure directory exists
	dir := filepath.Dir(stateFilePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		log.Printf("Failed to create state directory: %v", err)
		return err
	}

	// Marshal to JSON
	data, err := json.Marshal(state)
	if err != nil {
		return err
	}

	// Write to file
	if err := os.WriteFile(stateFilePath, data, 0644); err != nil {
		log.Printf("Failed to write state file: %v", err)
		return err
	}

	log.Printf("Successfully saved upgrade state to %s", stateFilePath)
	return nil
}

// loadUpgradeState loads the upgrade state from a file
func loadUpgradeState() (UpgradeState, error) {
	var state UpgradeState

	data, err := os.ReadFile(stateFilePath)
	if err != nil {
		if os.IsNotExist(err) {
			// No state file exists, return empty state
			log.Printf("No upgrade state file found at %s", stateFilePath)
			return state, nil
		}
		log.Printf("Error reading upgrade state file: %v", err)
		return state, err
	}

	// Unmarshal from JSON
	err = json.Unmarshal(data, &state)
	if err != nil {
		log.Printf("Error parsing upgrade state file: %v", err)
		return state, err
	}

	log.Printf("Successfully loaded upgrade state: target version=%s, in progress=%v",
		state.TargetVersion, state.InProgress)
	return state, err
}

// clearUpgradeState removes the upgrade state file
func clearUpgradeState() error {
	err := os.Remove(stateFilePath)
	if err != nil {
		if os.IsNotExist(err) {
			log.Printf("No upgrade state file to clear at %s", stateFilePath)
			return nil
		}
		log.Printf("Error clearing upgrade state file: %v", err)
		return err
	}

	log.Printf("Successfully cleared upgrade state file")
	return nil
}
