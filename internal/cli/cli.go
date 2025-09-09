// Package cli provides command-line interface functionality for btrfs-backup.
// It uses Cobra for professional CLI with subcommands, automatic help, and completions.
package cli

import (
	"fmt"
	"log"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"btrfs-backup/internal/backup"
	"btrfs-backup/internal/config"
)

// version is set at build time via ldflags
var version = "dev"

var (
	configFile string
	verbose    bool
)

// Run is the main entry point for the CLI application.
// It initializes and executes the root Cobra command.
func Run() {
	rootCmd := createRootCmd()
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

// createRootCmd creates and configures the root Cobra command
func createRootCmd() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:   "btrfs-backup",
		Short: "BTRFS Backup with Restic",
		Long:  `A backup tool that creates BTRFS snapshots and backs them up using Restic.`,
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			if verbose {
				log.SetFlags(log.LstdFlags | log.Lshortfile)
				log.Println("Debug logging enabled")
			}
		},
		CompletionOptions: cobra.CompletionOptions{
			DisableDefaultCmd: true,
		},
	}

	// Global flags
	rootCmd.PersistentFlags().StringVarP(&configFile, "config", "c", "",
		"config file path (default: $HOME/.config/btrfs-backup/config.yaml)")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false,
		"enable debug logging")

	// Bind flags to viper for configuration integration
	_ = viper.BindPFlag("config", rootCmd.PersistentFlags().Lookup("config"))
	_ = viper.BindPFlag("verbose", rootCmd.PersistentFlags().Lookup("verbose"))

	// Add subcommands
	rootCmd.AddCommand(createVersionCmd())
	rootCmd.AddCommand(createBackupCmd())

	return rootCmd
}

// createVersionCmd creates the version subcommand
func createVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Show version information",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("btrfs-backup version %s\n", version)
		},
	}
}

// createBackupCmd creates the backup subcommand
func createBackupCmd() *cobra.Command {
	var targetConfigPath string

	backupCmd := &cobra.Command{
		Use:   "backup <target-name>",
		Short: "Perform backup operation",
		Long: `Perform a complete backup workflow including:
- Environment validation
- BTRFS snapshot creation  
- Restic backup to repository
- Optional repository verification
- Cleanup of old snapshots`,
		Args: cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			targetName := args[0]

			// Determine config path
			finalConfigPath := config.GetConfigPath(configFile)
			if verbose {
				log.Printf("Using config file: %s", finalConfigPath)
			}

			// Load main configuration
			cfg, err := config.LoadConfig(finalConfigPath)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error loading configuration: %v\n", err)
				os.Exit(1)
			}

			// Determine target config path
			finalTargetConfigPath := config.GetTargetConfigPath(targetConfigPath, cfg.TargetDir, targetName)
			if verbose {
				log.Printf("Using target config file: %s", finalTargetConfigPath)
			}

			// Load target configuration
			targetConfig, err := config.LoadTargetConfig(finalTargetConfigPath)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error loading target configuration: %v\n", err)
				os.Exit(1)
			}

			// Run backup
			if err := runBackup(targetName, cfg, targetConfig, verbose); err != nil {
				fmt.Fprintf(os.Stderr, "Backup failed: %v\n", err)
				os.Exit(1)
			}

			fmt.Println("Backup completed successfully")
		},
	}

	// Backup-specific flags
	backupCmd.Flags().StringVarP(&targetConfigPath, "target-config", "t", "",
		"path to target configuration file")

	return backupCmd
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
func validateEnvironmentWithLogging(mgr *backup.Manager, subvolume string, _ *config.Config) error {
	// This would call individual validation steps from the manager
	// For now, we'll use a simplified approach
	return mgr.ValidateEnvironment(subvolume)
}

func createSnapshotWithLogging(mgr *backup.Manager, subvolume, prefix string, _ bool) (string, error) {
	return mgr.CreateSnapshot(subvolume, prefix)
}

func performBackupWithLogging(mgr *backup.Manager, snapshotPath string, target *config.TargetConfig, _ bool) error {
	return mgr.PerformBackup(snapshotPath, target)
}

func verifyRepositoryWithLogging(mgr *backup.Manager, repository string, _ bool) error {
	return mgr.VerifyRepository(repository)
}

func cleanupSnapshotsWithLogging(mgr *backup.Manager, prefix string, retention int) error {
	return mgr.CleanupOldSnapshots(prefix, retention)
}
