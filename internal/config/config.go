// Package config provides configuration loading and validation for btrfs-backup.
// It uses Viper for robust configuration management with support for multiple formats,
// environment variables, and configuration watching.
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/viper"
)

// Config represents the main btrfs-backup configuration containing
// paths to directories and executables needed for backup operations.
type Config struct {
	TargetDir     string `json:"target_dir" yaml:"target_dir" mapstructure:"target_dir"`                // Directory containing target configuration files
	SnapshotDir   string `json:"snapshot_dir" yaml:"snapshot_dir" mapstructure:"snapshot_dir"`          // Directory where BTRFS snapshots are created
	ResticRepoDir string `json:"restic_repo_dir" yaml:"restic_repo_dir" mapstructure:"restic_repo_dir"` // Directory containing Restic repository configurations
	ResticBin     string `json:"restic_bin" yaml:"restic_bin" mapstructure:"restic_bin"`                // Path to the Restic binary
}

// TargetConfig represents configuration for a specific backup target,
// defining the source subvolume, backup settings, and retention policy.
type TargetConfig struct {
	Subvolume     string `json:"subvolume" yaml:"subvolume" mapstructure:"subvolume"`                // BTRFS subvolume to backup
	Prefix        string `json:"prefix" yaml:"prefix" mapstructure:"prefix"`                         // Prefix for snapshot names
	Repository    string `json:"repository" yaml:"repository" mapstructure:"repository"`             // Restic repository identifier
	Type          string `json:"type" yaml:"type" mapstructure:"type"`                               // Backup type: "incremental" or "full"
	Verify        bool   `json:"verify" yaml:"verify" mapstructure:"verify"`                         // Whether to verify repository after backup
	KeepSnapshots int    `json:"keep_snapshots" yaml:"keep_snapshots" mapstructure:"keep_snapshots"` // Number of local snapshots to retain
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
// It uses Viper for robust parsing supporting JSON, YAML, TOML, HCL, INI formats.
// Also supports environment variables with BTRFSBACKUP_ prefix.
// Returns a validated Config struct or an error if loading/validation fails.
func LoadConfig(path string) (*Config, error) {
	v := viper.New()

	// Set up environment variables
	v.SetEnvPrefix("BTRFSBACKUP")
	v.AutomaticEnv()
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	// Set defaults
	setConfigDefaults(v)

	// Configure file path
	if path != "" {
		v.SetConfigFile(path)
	} else {
		// Use default config locations
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("failed to get home directory: %w", err)
		}

		v.SetConfigName("config")
		v.SetConfigType("yaml")
		v.AddConfigPath(filepath.Join(home, ".config", "btrfs-backup"))
		v.AddConfigPath(".")
	}

	// Read the configuration
	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	// Unmarshal into struct
	var config Config
	if err := v.Unmarshal(&config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	// Validate
	if err := validateConfig(&config); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return &config, nil
}

// LoadTargetConfig loads and validates a target configuration from the specified file path.
// It uses Viper for robust parsing supporting multiple formats and environment variables.
// Returns a validated TargetConfig struct or an error if loading/validation fails.
func LoadTargetConfig(path string) (*TargetConfig, error) {
	v := viper.New()

	// Set up environment variables (target-specific ones can use TARGET_ prefix)
	v.SetEnvPrefix("BTRFSBACKUP_TARGET")
	v.AutomaticEnv()
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	// Set defaults
	setTargetDefaults(v)

	// Configure file path
	v.SetConfigFile(path)

	// Read the configuration
	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("failed to read target config file: %w", err)
	}

	// Unmarshal into struct
	var target TargetConfig
	if err := v.Unmarshal(&target); err != nil {
		return nil, fmt.Errorf("failed to unmarshal target config: %w", err)
	}

	// Validate
	if err := validateTargetConfig(&target); err != nil {
		return nil, fmt.Errorf("invalid target configuration: %w", err)
	}

	return &target, nil
}

// setConfigDefaults sets default values for main configuration using Viper
func setConfigDefaults(v *viper.Viper) {
	v.SetDefault("restic_bin", "/usr/bin/restic")
}

// setTargetDefaults sets default values for target configuration using Viper
func setTargetDefaults(v *viper.Viper) {
	v.SetDefault("type", "incremental")
	v.SetDefault("keep_snapshots", 3)
	v.SetDefault("verify", false)
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
