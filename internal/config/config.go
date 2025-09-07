// Package config provides configuration loading and validation for btrfs-backup.
// It supports both JSON and YAML configuration files for main settings and backup targets.
package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Config represents the main btrfs-backup configuration containing
// paths to directories and executables needed for backup operations.
type Config struct {
	TargetDir     string `json:"target_dir" yaml:"target_dir"`         // Directory containing target configuration files
	SnapshotDir   string `json:"snapshot_dir" yaml:"snapshot_dir"`     // Directory where BTRFS snapshots are created
	ResticRepoDir string `json:"restic_repo_dir" yaml:"restic_repo_dir"` // Directory containing Restic repository configurations
	ResticBin     string `json:"restic_bin" yaml:"restic_bin"`         // Path to the Restic binary
}

// TargetConfig represents configuration for a specific backup target,
// defining the source subvolume, backup settings, and retention policy.
type TargetConfig struct {
	Subvolume     string `json:"subvolume" yaml:"subvolume"`             // BTRFS subvolume to backup
	Prefix        string `json:"prefix" yaml:"prefix"`                   // Prefix for snapshot names
	Repository    string `json:"repository" yaml:"repository"`           // Restic repository identifier
	Type          string `json:"type" yaml:"type"`                       // Backup type: "incremental" or "full"
	Verify        bool   `json:"verify" yaml:"verify"`                   // Whether to verify repository after backup
	KeepSnapshots int    `json:"keep_snapshots" yaml:"keep_snapshots"`   // Number of local snapshots to retain
}

// GetConfigPath determines the main configuration file path using the following priority:
// 1. Provided path parameter (highest priority)
// 2. BTRFSBACKUP_CONFIG environment variable
// 3. Default path: $HOME/.config/btrfs-backup/config.yaml (lowest priority)
func GetConfigPath(provided string) string {
	if provided != "" {
		return provided
	}

	if envConfig := os.Getenv("BTRFSBACKUP_CONFIG"); envConfig != "" {
		return envConfig
	}

	home, err := os.UserHomeDir()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error getting home directory: %v\n", err)
		os.Exit(1)
	}

	return filepath.Join(home, ".config", "btrfs-backup", "config.yaml")
}

// GetTargetConfigPath determines the target configuration file path using the following priority:
// 1. Provided path parameter (highest priority)
// 2. targetDir from main config + targetName
// 3. Default path: $HOME/.config/btrfs-backup/targets/<targetName> (lowest priority)
func GetTargetConfigPath(provided, targetDir, targetName string) string {
	if provided != "" {
		return provided
	}

	home, err := os.UserHomeDir()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error getting home directory: %v\n", err)
		os.Exit(1)
	}

	defaultTargetDir := filepath.Join(home, ".config", "btrfs-backup", "targets")
	if targetDir != "" {
		defaultTargetDir = targetDir
	}

	return filepath.Join(defaultTargetDir, targetName)
}

// LoadConfig loads and validates the main configuration from the specified file path.
// It supports both JSON and YAML formats, trying JSON first then falling back to YAML.
// Returns a validated Config struct or an error if loading/validation fails.
func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config Config
	
	// Try JSON first, then YAML
	if err := json.Unmarshal(data, &config); err != nil {
		// Try as YAML (simple parsing)
		if err := parseYAML(data, &config); err != nil {
			return nil, fmt.Errorf("failed to parse config file as JSON or YAML: %w", err)
		}
	}

	if err := validateConfig(&config); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return &config, nil
}

// LoadTargetConfig loads and validates a target configuration from the specified file path.
// It supports both JSON and YAML formats, applies default values, and validates the configuration.
// Returns a validated TargetConfig struct or an error if loading/validation fails.
func LoadTargetConfig(path string) (*TargetConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read target config file: %w", err)
	}

	var target TargetConfig
	
	// Try JSON first, then YAML
	if err := json.Unmarshal(data, &target); err != nil {
		// Try as YAML (simple parsing)
		if err := parseTargetYAML(data, &target); err != nil {
			return nil, fmt.Errorf("failed to parse target config file as JSON or YAML: %w", err)
		}
	}

	setTargetDefaults(&target)
	
	if err := validateTargetConfig(&target); err != nil {
		return nil, fmt.Errorf("invalid target configuration: %w", err)
	}

	return &target, nil
}

// Simple YAML parser for Config struct
func parseYAML(data []byte, config *Config) error {
	content := string(data)
	
	for len(content) > 0 {
		var line string
		if newlineIdx := strings.Index(content, "\n"); newlineIdx >= 0 {
			line = content[:newlineIdx]
			content = content[newlineIdx+1:]
		} else {
			line = content
			content = ""
		}
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		
		key, value, found := strings.Cut(line, ":")
		if !found {
			continue
		}
		
		key = strings.TrimSpace(key)
		value = strings.Trim(strings.TrimSpace(value), "\"'")
		
		switch key {
		case "target_dir":
			config.TargetDir = value
		case "snapshot_dir":
			config.SnapshotDir = value
		case "restic_repo_dir":
			config.ResticRepoDir = value
		case "restic_bin":
			config.ResticBin = value
		}
	}
	
	return nil
}

// Simple YAML parser for TargetConfig struct
func parseTargetYAML(data []byte, target *TargetConfig) error {
	content := string(data)
	
	for len(content) > 0 {
		var line string
		if newlineIdx := strings.Index(content, "\n"); newlineIdx >= 0 {
			line = content[:newlineIdx]
			content = content[newlineIdx+1:]
		} else {
			line = content
			content = ""
		}
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		
		key, value, found := strings.Cut(line, ":")
		if !found {
			continue
		}
		
		key = strings.TrimSpace(key)
		value = strings.Trim(strings.TrimSpace(value), "\"'")
		
		switch key {
		case "subvolume":
			target.Subvolume = value
		case "prefix":
			target.Prefix = value
		case "repository":
			target.Repository = value
		case "type":
			target.Type = value
		case "verify":
			target.Verify = value == "true" || value == "1"
		case "keep_snapshots":
			if n := parseInt(value); n >= 0 {
				target.KeepSnapshots = n
			}
		}
	}
	
	return nil
}

func parseInt(s string) int {
	var result int
	for _, r := range s {
		if r >= '0' && r <= '9' {
			result = result*10 + int(r-'0')
		} else {
			return -1
		}
	}
	return result
}

func validateConfig(config *Config) error {
	if config.TargetDir == "" {
		return fmt.Errorf("target_dir is required")
	}
	if config.SnapshotDir == "" {
		return fmt.Errorf("snapshot_dir is required")
	}
	if config.ResticRepoDir == "" {
		return fmt.Errorf("restic_repo_dir is required")
	}
	if config.ResticBin == "" {
		return fmt.Errorf("restic_bin is required")
	}
	return nil
}

func validateTargetConfig(target *TargetConfig) error {
	if target.Subvolume == "" {
		return fmt.Errorf("subvolume is required")
	}
	if target.Prefix == "" {
		return fmt.Errorf("prefix is required")
	}
	if target.Repository == "" {
		return fmt.Errorf("repository is required")
	}

	validTypes := map[string]bool{"incremental": true, "full": true}
	if target.Type != "" && !validTypes[target.Type] {
		return fmt.Errorf("invalid backup type '%s', must be 'incremental' or 'full'", target.Type)
	}

	if target.KeepSnapshots < 0 {
		return fmt.Errorf("keep_snapshots must be non-negative")
	}

	return nil
}

func setTargetDefaults(target *TargetConfig) {
	if target.Type == "" {
		target.Type = "incremental"
	}
	if target.KeepSnapshots == 0 {
		target.KeepSnapshots = 3
	}
}