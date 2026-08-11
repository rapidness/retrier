package config

import (
	"log"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"github.com/fsnotify/fsnotify"
	"gopkg.in/yaml.v3"
)

// ConfigManager handles configuration loading and hot-reload.
type ConfigManager struct {
	current atomic.Pointer[Config]
	watcher *fsnotify.Watcher
	path    string

	mu       sync.Mutex
	debounce *time.Timer
	onChange func(*Config)
}

// NewConfigManager creates a config manager and loads the initial config.
func NewConfigManager(path string, onChange func(*Config)) (*ConfigManager, error) {
	cfg, err := Load(path)
	if err != nil {
		return nil, err
	}

	cm := &ConfigManager{
		path:     path,
		onChange: onChange,
	}
	cm.current.Store(cfg)

	return cm, nil
}

// Snapshot returns the current config (thread-safe).
func (cm *ConfigManager) Snapshot() *Config {
	return cm.current.Load()
}

// StartWatching begins watching the config file for changes.
// Uses directory watching to handle editor atomic writes (save to temp, rename).
func (cm *ConfigManager) StartWatching() error {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}
	cm.watcher = watcher

	// Watch the directory (not the file) to catch rename-based saves
	dir := filepath.Dir(cm.path)
	if err := watcher.Add(dir); err != nil {
		return err
	}

	go cm.watchLoop()

	log.Printf("[config] watching %s for changes", cm.path)
	return nil
}

// StopWatching stops the file watcher.
func (cm *ConfigManager) StopWatching() {
	if cm.watcher != nil {
		cm.watcher.Close()
	}
}

// watchLoop processes file system events.
func (cm *ConfigManager) watchLoop() {
	for {
		select {
		case event, ok := <-cm.watcher.Events:
			if !ok {
				return
			}
			// Only care about write/create/rename events for our config file
			if event.Name != cm.path {
				// Also check if the event is for our file by basename
				if filepath.Base(event.Name) != filepath.Base(cm.path) {
					continue
				}
			}
			if event.Has(fsnotify.Write) || event.Has(fsnotify.Create) || event.Has(fsnotify.Rename) {
				cm.scheduleReload()
			}

		case err, ok := <-cm.watcher.Errors:
			if !ok {
				return
			}
			log.Printf("[config] watcher error: %v", err)
		}
	}
}

// scheduleReload debounces reload events (500ms window).
func (cm *ConfigManager) scheduleReload() {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	if cm.debounce != nil {
		cm.debounce.Stop()
	}
	cm.debounce = time.AfterFunc(500*time.Millisecond, cm.reload)
}

// reload loads the new config and applies it.
func (cm *ConfigManager) reload() {
	// Small delay to ensure file is fully written
	time.Sleep(100 * time.Millisecond)

	cfg, err := Load(cm.path)
	if err != nil {
		log.Printf("[config] failed to reload: %v", err)
		return
	}

	old := cm.current.Load()
	cm.current.Store(cfg)

	log.Printf("[config] reloaded: %d rules (was %d), logging.enabled=%v (was %v)",
		len(cfg.Rules), len(old.Rules), cfg.Logging.Enabled, old.Logging.Enabled)

	if cm.onChange != nil {
		cm.onChange(cfg)
	}
}

// Reload manually triggers a config reload.
func (cm *ConfigManager) Reload() error {
	cfg, err := Load(cm.path)
	if err != nil {
		return err
	}
	cm.current.Store(cfg)
	if cm.onChange != nil {
		cm.onChange(cfg)
	}
	return nil
}

// SetOnChange sets the callback for config changes.
func (cm *ConfigManager) SetOnChange(fn func(*Config)) {
	cm.onChange = fn
}

// Path returns the config file path.
func (cm *ConfigManager) Path() string {
	return cm.path
}

// SaveAndReload writes the config to YAML file and triggers a reload.
// It temporarily pauses the file watcher to avoid a double-reload.
func (cm *ConfigManager) SaveAndReload(cfg *Config) error {
	// Validate before saving
	if err := cfg.Validate(); err != nil {
		return err
	}

	// Marshal to YAML
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}

	// Pause watcher to avoid double-reload
	cm.StopWatching()

	// Write file
	if err := os.WriteFile(cm.path, data, 0644); err != nil {
		// Try to restart watcher even on write failure
		_ = cm.StartWatching()
		return err
	}

	log.Printf("[config] saved config to %s", cm.path)

	// Reload from disk (which also calls onChange)
	if err := cm.Reload(); err != nil {
		log.Printf("[config] reload after save failed: %v", err)
	}

	// Restart watcher
	if err := cm.StartWatching(); err != nil {
		log.Printf("[config] failed to restart watcher: %v", err)
	}

	return nil
}
