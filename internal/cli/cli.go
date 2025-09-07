// Package cli provides command-line interface functionality for btrfs-backup.
// It handles argument parsing, user interaction, progress logging, and error presentation.
package cli

import (
	"flag"
	"fmt"
	"log"
	"os"

	"btrfs-backup/internal/backup"
	"btrfs-backup/internal/config"
)

const version = "0.1.0"

// Run is the main entry point for the CLI application.
// It parses command-line arguments and dispatches to the appropriate command handler.
func Run() {
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

func handleVersion(_ []string) {
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
	finalConfigPath := config.GetConfigPath(*configPath)
	log.Printf("Using config file: %s", finalConfigPath)

	// Load main configuration
	cfg, err := config.LoadConfig(finalConfigPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading configuration: %v\n", err)
		os.Exit(1)
	}

	// Determine target config path
	finalTargetConfigPath := config.GetTargetConfigPath(*targetConfigPath, cfg.TargetDir, targetName)
	log.Printf("Using target config file: %s", finalTargetConfigPath)

	// Load target configuration
	targetConfig, err := config.LoadTargetConfig(finalTargetConfigPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading target configuration: %v\n", err)
		os.Exit(1)
	}

	// Run backup
	if err := runBackup(targetName, cfg, targetConfig, *verbose); err != nil {
		fmt.Fprintf(os.Stderr, "Backup failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Backup completed successfully")
}

func runBackup(targetName string, cfg *config.Config, target *config.TargetConfig, verbose bool) error {
	log.Printf("=== Starting BTRFS backup process for target: %s ===", targetName)
	log.Printf("Subvolume: %s", target.Subvolume)
	log.Printf("Repository: %s", target.Repository)
	log.Printf("Type: %s", target.Type)
	log.Printf("Verify: %t", target.Verify)
	log.Printf("Keep snapshots: %d", target.KeepSnapshots)

	mgr := backup.NewManager(cfg, verbose)
	
	// Step 1: Environment validation
	log.Println("Validating backup environment")
	err := validateEnvironmentWithLogging(mgr, target.Subvolume, cfg)
	if err != nil {
		return fmt.Errorf("environment validation failed: %w", err)
	}
	log.Println("Environment validation completed successfully")

	// Step 2: Create snapshot
	log.Printf("Creating BTRFS snapshot with prefix: %s", target.Prefix)
	snapshotPath, err := createSnapshotWithLogging(mgr, target.Subvolume, target.Prefix, verbose)
	if err != nil {
		return fmt.Errorf("snapshot creation failed: %w", err)
	}
	log.Printf("Snapshot created successfully: %s", snapshotPath)

	// Step 3: Perform backup
	backupType := "incremental"
	if target.Type == "full" {
		backupType = "full"
	}
	log.Printf("Starting Restic %s backup to repository %s", backupType, target.Repository)
	err = performBackupWithLogging(mgr, snapshotPath, target, verbose)
	if err != nil {
		log.Printf("Backup failed, keeping snapshot for investigation: %s", snapshotPath)
		return fmt.Errorf("backup operation failed: %w", err)
	}
	log.Printf("Restic backup completed successfully")

	// Step 4: Verify repository (if enabled)
	if target.Verify {
		log.Printf("Verifying repository integrity: %s", target.Repository)
		err = verifyRepositoryWithLogging(mgr, target.Repository, verbose)
		if err != nil {
			log.Printf("Repository verification failed (warning): %v", err)
		} else {
			log.Printf("Repository verification completed successfully")
		}
	}

	// Step 5: Clean up old snapshots
	log.Printf("Cleaning up old snapshots, keeping last %d", target.KeepSnapshots)
	err = cleanupSnapshotsWithLogging(mgr, target.Prefix, target.KeepSnapshots)
	if err != nil {
		log.Printf("Failed to cleanup old snapshots (warning): %v", err)
	} else {
		log.Println("Snapshot cleanup completed successfully")
	}

	log.Println("=== Backup process completed successfully ===")
	return nil
}

// Helper functions that call manager methods but handle CLI-specific logging
func validateEnvironmentWithLogging(mgr *backup.Manager, subvolume string, cfg *config.Config) error {
	// This would call individual validation steps from the manager
	// For now, we'll use a simplified approach
	return mgr.ValidateEnvironment(subvolume)
}

func createSnapshotWithLogging(mgr *backup.Manager, subvolume, prefix string, verbose bool) (string, error) {
	return mgr.CreateSnapshot(subvolume, prefix)
}

func performBackupWithLogging(mgr *backup.Manager, snapshotPath string, target *config.TargetConfig, verbose bool) error {
	return mgr.PerformBackup(snapshotPath, target)
}

func verifyRepositoryWithLogging(mgr *backup.Manager, repository string, verbose bool) error {
	return mgr.VerifyRepository(repository)
}

func cleanupSnapshotsWithLogging(mgr *backup.Manager, prefix string, retention int) error {
	return mgr.CleanupOldSnapshots(prefix, retention)
}