package config

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"gopkg.in/yaml.v3"
)

// Config holds the application configuration loaded from YAML
type Config struct {
	GrpcTarget       string `yaml:"grpcTarget"`
	FirmwareSource   string `yaml:"firmwareSource"`
	UpdateMlnxCpldFw string `yaml:"updateMlnxCpldFw"`
	TargetVersion    string `yaml:"targetVersion"`
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

	data, err := ioutil.ReadFile(m.configPath)
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

// StartWatcher starts watching the config file for changes
func (m *Manager) StartWatcher() error {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("failed to create watcher: %w", err)
	}

	go func() {
		for {
			select {
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}
				if event.Op&fsnotify.Write == fsnotify.Write {
					log.Println("Config file modified, reloading...")
					if err := m.loadConfig(); err != nil {
						log.Printf("Error reloading config: %v", err)
						continue
					}

					// Call the callback with the new config
					if m.onUpdate != nil {
						m.onUpdate(m.GetConfig())
					}
				}
			case err, ok := <-watcher.Errors:
				if !ok {
					return
				}
				log.Printf("Error watching config file: %v", err)
			}
		}
	}()

	// Start watching the config file
	if err := watcher.Add(m.configPath); err != nil {
		watcher.Close()
		return fmt.Errorf("failed to add file to watcher: %w", err)
	}

	log.Printf("Started watching config file: %s", m.configPath)
	return nil
}
