package config

import (
	"fmt"
	"log"
	"os"
	"sync"
	"time"

	"gopkg.in/yaml.v3"
)

// Config holds the application configuration loaded from YAML
type Config struct {
	GrpcTarget              string `yaml:"grpcTarget"`
	FirmwareSource          string `yaml:"firmwareSource"`
	UpdateMlnxCpldFw        string `yaml:"updateMlnxCpldFw"`
	TargetVersion           string `yaml:"targetVersion"`
	IgnoreUnimplementedRPC  bool   `yaml:"ignoreUnimplementedRPC"`  // When true, treat "unimplemented" gRPC errors as success
}

// Manager handles loading and watching configuration
type Manager struct {
	configPath   string
	currentCfg   Config
	lock         sync.RWMutex
	lastModified time.Time
	onUpdate     func(cfg Config)
}

// NewManager creates a new config manager
func NewManager(configPath string, onUpdate func(cfg Config)) (*Manager, error) {
	m := &Manager{
		configPath: configPath,
		onUpdate:   onUpdate,
	}

	// Initial load
	if err := m.loadConfig(); err != nil {
		return nil, err
	}

	return m, nil
}

// GetConfig returns a copy of the current configuration
func (m *Manager) GetConfig() Config {
	m.lock.RLock()
	defer m.lock.RUnlock()
	return m.currentCfg
}

// loadConfig reads and parses the configuration file
func (m *Manager) loadConfig() error {
	m.lock.Lock()
	defer m.lock.Unlock()

	fileInfo, err := os.Stat(m.configPath)
	if err != nil {
		return fmt.Errorf("failed to stat config file: %w", err)
	}

	// Skip if file hasn't been modified
	modTime := fileInfo.ModTime()
	if !modTime.After(m.lastModified) {
		return nil
	}

	data, err := os.ReadFile(m.configPath)
	if err != nil {
		return fmt.Errorf("failed to read config file: %w", err)
	}

	var newCfg Config
	if err := yaml.Unmarshal(data, &newCfg); err != nil {
		return fmt.Errorf("failed to parse config file: %w", err)
	}

	// Update the config and last modified time
	m.currentCfg = newCfg
	m.lastModified = modTime

	return nil
}

// StartWatcher starts polling the config file for changes every 10 seconds
func (m *Manager) StartWatcher() error {
	log.Printf("Started polling config file every 10 seconds: %s", m.configPath)

	go func() {
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()

		for range ticker.C {
			// Read and log the entire file content
			data, err := os.ReadFile(m.configPath)
			if err != nil {
				log.Printf("Error reading config file: %v", err)
				continue
			}

			log.Printf("Config file content: %s", string(data))

			// Check for changes and reload if needed
			if err := m.loadConfig(); err != nil {
				log.Printf("Error reloading config: %v", err)
				continue
			}

			// Call the callback with the new config
			if m.onUpdate != nil {
				m.onUpdate(m.GetConfig())
			}
		}
	}()

	return nil
}
