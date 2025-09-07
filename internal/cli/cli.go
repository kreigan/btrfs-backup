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
	log.Printf("Starting backup for target: %s", targetName)
	log.Printf("Subvolume: %s", target.Subvolume)
	log.Printf("Repository: %s", target.Repository)
	log.Printf("Type: %s", target.Type)
	log.Printf("Verify: %t", target.Verify)
	log.Printf("Keep snapshots: %d", target.KeepSnapshots)

	// Create backup manager
	mgr := backup.NewManager(cfg, verbose)
	
	return mgr.RunBackup(targetName, target)
}