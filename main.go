package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
)

const version = "0.1.0"

type Config struct {
	TargetDir     string `json:"target_dir" yaml:"target_dir"`
	SnapshotDir   string `json:"snapshot_dir" yaml:"snapshot_dir"`
	ResticRepoDir string `json:"restic_repo_dir" yaml:"restic_repo_dir"`
	ResticBin     string `json:"restic_bin" yaml:"restic_bin"`
}

type TargetConfig struct {
	Subvolume     string `json:"subvolume" yaml:"subvolume"`
	Prefix        string `json:"prefix" yaml:"prefix"`
	Repository    string `json:"repository" yaml:"repository"`
	Type          string `json:"type" yaml:"type"`
	Verify        bool   `json:"verify" yaml:"verify"`
	KeepSnapshots int    `json:"keep_snapshots" yaml:"keep_snapshots"`
}

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	command := os.Args[1]
	args := os.Args[2:]

	switch command {
	case "version":
		handleVersion(args)
	case "backup":
		handleBackup(args)
	case "help", "--help", "-h":
		printUsage()
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", command)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Printf("btrfs-backup %s - BTRFS Backup with Restic\n\n", version)
	fmt.Print(`Usage:
  btrfs-backup <command> [options]

Commands:
  version          Show version information
  backup <target>  Perform backup operation

Global Options:
  -c, --config     Config file path (default: $HOME/.config/btrfs-backup/config.yaml)
                   Can also be set via BTRFSBACKUP_CONFIG environment variable
  -v, --verbose    Enable debug logging

Backup Command Options:
  -t, --target-config   Path to target configuration file
                        (default: $HOME/.config/btrfs-backup/targets/<target>)

Examples:
  btrfs-backup version
  btrfs-backup backup my-target
  btrfs-backup backup my-target -v
  btrfs-backup backup my-target -c /path/to/config.yaml
  btrfs-backup backup my-target -t /path/to/target.yaml
`)
}

func handleVersion(args []string) {
	fmt.Printf("btrfs-backup version %s\n", version)
}

func handleBackup(args []string) {
	if len(args) < 1 {
		fmt.Fprintf(os.Stderr, "Error: backup command requires a target name\n")
		fmt.Fprintf(os.Stderr, "Usage: btrfs-backup backup <target-name> [options]\n")
		os.Exit(1)
	}

	targetName := args[0]
	
	fs := flag.NewFlagSet("backup", flag.ExitOnError)
	configPath := fs.String("c", "", "Config file path")
	fs.StringVar(configPath, "config", "", "Config file path")
	verbose := fs.Bool("v", false, "Enable verbose logging")
	fs.BoolVar(verbose, "verbose", false, "Enable verbose logging")
	targetConfigPath := fs.String("t", "", "Target config file path")
	fs.StringVar(targetConfigPath, "target-config", "", "Target config file path")

	fs.Parse(args[1:])

	if *verbose {
		log.SetFlags(log.LstdFlags | log.Lshortfile)
		log.Println("Debug logging enabled")
	}

	// Determine config path
	finalConfigPath := getConfigPath(*configPath)
	log.Printf("Using config file: %s", finalConfigPath)

	// Load main configuration
	config, err := loadConfig(finalConfigPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading configuration: %v\n", err)
		os.Exit(1)
	}

	// Determine target config path
	finalTargetConfigPath := getTargetConfigPath(*targetConfigPath, config.TargetDir, targetName)
	log.Printf("Using target config file: %s", finalTargetConfigPath)

	// Load target configuration
	targetConfig, err := loadTargetConfig(finalTargetConfigPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading target configuration: %v\n", err)
		os.Exit(1)
	}

	// Run backup
	if err := runBackup(targetName, config, targetConfig, *verbose); err != nil {
		fmt.Fprintf(os.Stderr, "Backup failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Backup completed successfully")
}

func getConfigPath(provided string) string {
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

func getTargetConfigPath(provided, targetDir, targetName string) string {
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

func loadConfig(path string) (*Config, error) {
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

func loadTargetConfig(path string) (*TargetConfig, error) {
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
	lines := strings.Split(string(data), "\n")
	
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}
		
		key := strings.TrimSpace(parts[0])
		value := strings.Trim(strings.TrimSpace(parts[1]), "\"'")
		
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
	lines := strings.Split(string(data), "\n")
	
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}
		
		key := strings.TrimSpace(parts[0])
		value := strings.Trim(strings.TrimSpace(parts[1]), "\"'")
		
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

func runBackup(targetName string, config *Config, target *TargetConfig, verbose bool) error {
	log.Printf("Starting backup for target: %s", targetName)
	log.Printf("Subvolume: %s", target.Subvolume)
	log.Printf("Repository: %s", target.Repository)
	log.Printf("Type: %s", target.Type)
	log.Printf("Verify: %t", target.Verify)
	log.Printf("Keep snapshots: %d", target.KeepSnapshots)

	// Create backup manager
	mgr := NewBackupManager(config, verbose)
	
	return mgr.RunBackup(targetName, target)
}