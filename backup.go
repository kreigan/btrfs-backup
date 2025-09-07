package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type BackupManager struct {
	config  *Config
	verbose bool
}

func NewBackupManager(config *Config, verbose bool) *BackupManager {
	return &BackupManager{
		config:  config,
		verbose: verbose,
	}
}

func (bm *BackupManager) RunBackup(targetName string, target *TargetConfig) error {
	log.Printf("=== Starting BTRFS backup process for target: %s ===", targetName)

	if err := bm.validateEnvironment(target.Subvolume); err != nil {
		return fmt.Errorf("environment validation failed: %w", err)
	}

	snapshotPath, err := bm.createSnapshot(target.Subvolume, target.Prefix)
	if err != nil {
		return fmt.Errorf("snapshot creation failed: %w", err)
	}
	log.Printf("Snapshot created successfully: %s", snapshotPath)

	if err := bm.performBackup(snapshotPath, target); err != nil {
		log.Printf("Backup failed, keeping snapshot for investigation: %s", snapshotPath)
		return fmt.Errorf("backup operation failed: %w", err)
	}
	log.Println("Restic backup completed successfully")

	if target.Verify {
		if err := bm.verifyRepository(target.Repository); err != nil {
			log.Printf("Repository verification failed (warning): %v", err)
		}
	}

	if err := bm.cleanupOldSnapshots(target.Prefix, target.KeepSnapshots); err != nil {
		log.Printf("Failed to cleanup old snapshots (warning): %v", err)
	}

	log.Println("=== Backup process completed successfully ===")
	return nil
}

func (bm *BackupManager) validateEnvironment(subvolume string) error {
	log.Println("Validating backup environment")

	if _, err := os.Stat(bm.config.SnapshotDir); os.IsNotExist(err) {
		return fmt.Errorf("snapshots directory does not exist: %s", bm.config.SnapshotDir)
	}
	log.Printf("Snapshots directory exists: %s", bm.config.SnapshotDir)

	log.Printf("Validating BTRFS subvolume: %s", subvolume)
	cmd := exec.Command("sudo", "btrfs", "subvolume", "show", subvolume)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("source subvolume invalid or not BTRFS: %s", subvolume)
	}
	log.Printf("Source subvolume is valid: %s", subvolume)

	log.Println("Environment validation completed successfully")
	return nil
}

func (bm *BackupManager) createSnapshot(subvolume, prefix string) (string, error) {
	timestamp := time.Now().Format("20060102-150405")
	snapshotName := fmt.Sprintf("%s-%s", prefix, timestamp)
	snapshotPath := filepath.Join(bm.config.SnapshotDir, snapshotName)

	log.Printf("Creating BTRFS snapshot: %s", snapshotName)
	log.Printf("Source: %s -> Destination: %s", subvolume, snapshotPath)

	cmd := exec.Command("sudo", "btrfs", "subvolume", "snapshot", "-r", subvolume, snapshotPath)
	if bm.verbose {
		log.Printf("Running command: %s", strings.Join(cmd.Args, " "))
	}

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("BTRFS snapshot command failed: %w", err)
	}

	if _, err := os.Stat(snapshotPath); os.IsNotExist(err) {
		return "", fmt.Errorf("snapshot not found after creation: %s", snapshotPath)
	}

	return snapshotPath, nil
}

func (bm *BackupManager) performBackup(snapshotPath string, target *TargetConfig) error {
	log.Printf("Starting Restic backup of %s to repository %s", snapshotPath, target.Repository)

	if _, err := os.Stat(snapshotPath); os.IsNotExist(err) {
		return fmt.Errorf("snapshot path does not exist: %s", snapshotPath)
	}

	env, err := bm.loadRepositoryEnv(target.Repository)
	if err != nil {
		return fmt.Errorf("repository configuration failed: %w", err)
	}

	cmd := bm.buildBackupCommand(snapshotPath, target)
	if bm.verbose {
		log.Printf("Running restic backup command: %s", strings.Join(cmd.Args, " "))
	}

	cmd.Env = env
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("restic backup command failed: %w", err)
	}

	log.Printf("Backup to %s completed successfully", target.Repository)
	return nil
}

func (bm *BackupManager) buildBackupCommand(snapshotPath string, target *TargetConfig) *exec.Cmd {
	args := []string{"backup", snapshotPath}
	args = append(args, "--tag", "btrfs-backup")
	args = append(args, "--tag", target.Prefix)
	args = append(args, "--tag", filepath.Base(snapshotPath))
	args = append(args, "--exclude-caches")

	if target.Type == "full" {
		args = append(args, "--force")
		log.Println("Performing full backup")
	} else {
		log.Println("Performing incremental backup")
	}

	return exec.Command(bm.config.ResticBin, args...)
}

func (bm *BackupManager) loadRepositoryEnv(repository string) ([]string, error) {
	repoFile := filepath.Join(bm.config.ResticRepoDir, repository)
	if _, err := os.Stat(repoFile); os.IsNotExist(err) {
		return nil, fmt.Errorf("repository configuration '%s' not found: %s", repository, repoFile)
	}

	env := os.Environ()

	data, err := os.ReadFile(repoFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read repository config %s: %w", repoFile, err)
	}

	// Parse YAML-style repository config
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
		env = append(env, fmt.Sprintf("%s=%s", key, value))
	}

	return env, nil
}

func (bm *BackupManager) verifyRepository(repository string) error {
	log.Printf("Verifying repository integrity: %s", repository)

	env, err := bm.loadRepositoryEnv(repository)
	if err != nil {
		return fmt.Errorf("repository configuration failed for verification: %w", err)
	}

	cmd := exec.Command(bm.config.ResticBin, "check", "--read-data-subset=5%")
	cmd.Env = env

	if bm.verbose {
		log.Printf("Running restic verify command: %s", strings.Join(cmd.Args, " "))
	}

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("repository verification failed: %s - %w", repository, err)
	}

	log.Printf("Repository verification completed successfully: %s", repository)
	return nil
}

func (bm *BackupManager) cleanupOldSnapshots(prefix string, retention int) error {
	log.Printf("Cleaning up old snapshots, keeping last %d", retention)

	snapshots, err := bm.getSnapshotsByPrefix(prefix)
	if err != nil {
		return fmt.Errorf("failed to list snapshots: %w", err)
	}

	if len(snapshots) <= retention {
		log.Println("No old snapshots to clean up")
		return nil
	}

	snapshotsToDelete := snapshots[retention:]
	log.Printf("%d old snapshots to delete", len(snapshotsToDelete))

	var failedDeletions []string
	for _, snapshot := range snapshotsToDelete {
		if err := bm.deleteSnapshot(snapshot); err != nil {
			log.Printf("Failed to delete snapshot %s: %v", snapshot, err)
			failedDeletions = append(failedDeletions, snapshot)
		}
	}

	if len(failedDeletions) > 0 {
		return fmt.Errorf("failed to delete some snapshots: %v", failedDeletions)
	}

	log.Println("Snapshot cleanup completed successfully")
	return nil
}

func (bm *BackupManager) getSnapshotsByPrefix(prefix string) ([]string, error) {
	if _, err := os.Stat(bm.config.SnapshotDir); os.IsNotExist(err) {
		log.Printf("Snapshots directory does not exist: %s", bm.config.SnapshotDir)
		return []string{}, nil
	}

	entries, err := os.ReadDir(bm.config.SnapshotDir)
	if err != nil {
		return nil, fmt.Errorf("could not list snapshots directory: %w", err)
	}

	type snapshotInfo struct {
		name  string
		mtime time.Time
	}

	var snapshots []snapshotInfo
	searchPrefix := prefix + "-"

	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), searchPrefix) {
			info, err := entry.Info()
			if err != nil {
				log.Printf("Could not get info for %s: %v", entry.Name(), err)
				continue
			}
			snapshots = append(snapshots, snapshotInfo{
				name:  entry.Name(),
				mtime: info.ModTime(),
			})
		}
	}

	// Sort by modification time, newest first
	sort.Slice(snapshots, func(i, j int) bool {
		return snapshots[i].mtime.After(snapshots[j].mtime)
	})

	var result []string
	for _, s := range snapshots {
		result = append(result, s.name)
	}

	return result, nil
}

func (bm *BackupManager) deleteSnapshot(snapshotName string) error {
	snapshotPath := filepath.Join(bm.config.SnapshotDir, snapshotName)
	log.Printf("Deleting old snapshot: %s", snapshotName)

	cmd := exec.Command("sudo", "btrfs", "subvolume", "delete", snapshotPath)
	if bm.verbose {
		log.Printf("Running command: %s", strings.Join(cmd.Args, " "))
	}

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("BTRFS delete command failed for snapshot %s: %w", snapshotName, err)
	}

	if _, err := os.Stat(snapshotPath); err == nil {
		return fmt.Errorf("snapshot still exists after deletion: %s", snapshotPath)
	}

	log.Printf("Successfully deleted snapshot: %s", snapshotName)
	return nil
}