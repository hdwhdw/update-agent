package main

import (
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"upgrade-agent/internal/agent"
	"upgrade-agent/internal/config"
)

const (
	defaultConfigPath = "/etc/upgrade-agent/config.yaml"
)

func main() {
	log.Println("Upgrade agent daemon starting...")

	// Determine config path (default or from env var)
	configPath := getEnvOrDefault("CONFIG_PATH", defaultConfigPath)
	log.Printf("Using config file: %s", configPath)

	// Ensure parent directory exists
	if err := ensureConfigDir(configPath); err != nil {
		log.Fatalf("Failed to create config directory: %v", err)
	}

	// Create the agent
	svc := agent.NewAgent()
	defer svc.Close()

	// Setup config manager with callback
	cfgManager, err := config.NewManager(configPath, func(cfg config.Config) {
		svc.UpdateConfig(cfg)
	})
	if err != nil {
		log.Fatalf("Failed to initialize config manager: %v", err)
	}

	// Initialize the service with the initial config
	if err := svc.Initialize(cfgManager.GetConfig()); err != nil {
		log.Fatalf("Failed to initialize service: %v", err)
	}

	// Start watching for config changes
	if err := cfgManager.StartWatcher(); err != nil {
		log.Fatalf("Failed to start config watcher: %v", err)
	}

	// Wait for termination signal
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	sig := <-sigCh
	log.Printf("Received signal %v, shutting down...", sig)
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
