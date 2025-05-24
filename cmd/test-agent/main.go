package main

import (
	"flag"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"upgrade-agent/internal/agent"
	"upgrade-agent/internal/config"

	"gopkg.in/yaml.v3"
)

const (
	defaultConfigPath = "/etc/upgrade-agent/config.yaml"
)

func main() {
	// Parse command-line flags for testing
	configPath := flag.String("config", getEnvOrDefault("CONFIG_PATH", defaultConfigPath), "Path to config file")
	testMode := flag.Bool("test", false, "Run in test mode")
	targetVersion := flag.String("version", "", "Target version to set when in test mode")
	updateDelay := flag.Int("delay", 5, "Seconds to wait before updating version in test mode")
	flag.Parse()

	log.Println("Upgrade agent daemon starting...")
	log.Printf("Using config file: %s", *configPath)

	// Ensure parent directory exists
	if err := ensureConfigDir(*configPath); err != nil {
		log.Fatalf("Failed to create config directory: %v", err)
	}

	// Create the agent
	svc := agent.NewAgent()
	defer svc.Close()

	// Setup config manager with callback
	cfgManager, err := config.NewManager(*configPath, func(cfg config.Config) {
		svc.UpdateConfig(cfg)
	})
	if err != nil {
		log.Fatalf("Failed to initialize config manager: %v", err)
	}

	// Initialize the service with the initial config
	initialConfig := cfgManager.GetConfig()
	if err := svc.Initialize(initialConfig); err != nil {
		log.Fatalf("Failed to initialize service: %v", err)
	}

	// Start watching for config changes
	if err := cfgManager.StartWatcher(); err != nil {
		log.Fatalf("Failed to start config watcher: %v", err)
	}

	// Handle test mode - automatically update the version after delay
	if *testMode && *targetVersion != "" {
		initialVersion := initialConfig.TargetVersion
		log.Printf("Test mode: Current version is %s, will update to %s after %d seconds",
			initialVersion, *targetVersion, *updateDelay)

		time.AfterFunc(time.Duration(*updateDelay)*time.Second, func() {
			// Read current config
			currentConfig := cfgManager.GetConfig()

			// Only update if original version hasn't changed
			if currentConfig.TargetVersion == initialVersion {
				log.Printf("Test mode: Updating target version to %s", *targetVersion)

				// Update the version in the config file
				currentConfig.TargetVersion = *targetVersion

				// Write updated config back to file
				updateConfigFile(*configPath, currentConfig)
			}
		})
	}

	// Wait for termination signal
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	sig := <-sigCh
	log.Printf("Received signal %v, shutting down...", sig)
}

// updateConfigFile writes the updated configuration to the config file
func updateConfigFile(configPath string, cfg config.Config) {
	// Read the existing file to keep any comments or structure
	data, err := os.ReadFile(configPath)
	if err != nil {
		log.Printf("Error reading config file for update: %v", err)
		return
	}

	// Parse the current YAML structure
	var yamlMap map[string]interface{}
	if err := yaml.Unmarshal(data, &yamlMap); err != nil {
		log.Printf("Error parsing config file for update: %v", err)
		return
	}

	// Update only the targetVersion field
	yamlMap["targetVersion"] = cfg.TargetVersion

	// Marshal back to YAML
	updatedData, err := yaml.Marshal(yamlMap)
	if err != nil {
		log.Printf("Error encoding updated config: %v", err)
		return
	}

	// Write the updated config back
	if err := os.WriteFile(configPath, updatedData, 0644); err != nil {
		log.Printf("Error writing updated config: %v", err)
		return
	}

	log.Printf("Successfully updated config file with new version: %s", cfg.TargetVersion)
}

// getEnvOrDefault returns the value of an environment variable or a default value
func getEnvOrDefault(key, defaultValue string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return defaultValue
}

// ensureConfigDir makes sure the directory for the config file exists
func ensureConfigDir(configPath string) error {
	dir := filepath.Dir(configPath)
	return os.MkdirAll(dir, 0755)
}
