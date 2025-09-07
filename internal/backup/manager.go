package backup

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"btrfs-backup/internal/config"
)

type Manager struct {
	config  *config.Config
	verbose bool
}

func NewManager(cfg *config.Config, verbose bool) *Manager {
	return &Manager{
		config:  cfg,
		verbose: verbose,
	}
}

func (bm *Manager) RunBackup(targetName string, target *config.TargetConfig) error {
	err := bm.ValidateEnvironment(target.Subvolume)
	if err != nil {
		return fmt.Errorf("environment validation failed: %w", err)
	}

	snapshotPath, err := bm.CreateSnapshot(target.Subvolume, target.Prefix)
	if err != nil {
		return fmt.Errorf("snapshot creation failed: %w", err)
	}

	err = bm.PerformBackup(snapshotPath, target)
	if err != nil {
		return fmt.Errorf("backup operation failed (snapshot preserved at %s): %w", snapshotPath, err)
	}

	if target.Verify {
		err = bm.VerifyRepository(target.Repository)
		if err != nil {
			return fmt.Errorf("repository verification failed: %w", err)
		}
	}

	err = bm.CleanupOldSnapshots(target.Prefix, target.KeepSnapshots)
	if err != nil {
		return fmt.Errorf("snapshot cleanup failed: %w", err)
	}

	return nil
}

func (bm *Manager) ValidateEnvironment(subvolume string) error {
	_, err := os.Stat(bm.config.SnapshotDir)
	if os.IsNotExist(err) {
		return fmt.Errorf("snapshots directory does not exist: %s", bm.config.SnapshotDir)
	}

	cmd := exec.Command("sudo", "btrfs", "subvolume", "show", subvolume)
	err = cmd.Run()
	if err != nil {
		return fmt.Errorf("source subvolume invalid or not BTRFS: %s", subvolume)
	}

	return nil
}

func (bm *Manager) CreateSnapshot(subvolume, prefix string) (string, error) {
	timestamp := time.Now().Format("20060102-150405")
	snapshotName := fmt.Sprintf("%s-%s", prefix, timestamp)
	snapshotPath := filepath.Join(bm.config.SnapshotDir, snapshotName)

	cmd := exec.Command("sudo", "btrfs", "subvolume", "snapshot", "-r", subvolume, snapshotPath)
	err := cmd.Run()
	if err != nil {
		return "", fmt.Errorf("BTRFS snapshot command failed: %w", err)
	}

	_, err = os.Stat(snapshotPath)
	if os.IsNotExist(err) {
		return "", fmt.Errorf("snapshot not found after creation: %s", snapshotPath)
	}

	return snapshotPath, nil
}

func (bm *Manager) PerformBackup(snapshotPath string, target *config.TargetConfig) error {
	_, err := os.Stat(snapshotPath)
	if os.IsNotExist(err) {
		return fmt.Errorf("snapshot path does not exist: %s", snapshotPath)
	}

	env, err := bm.loadRepositoryEnv(target.Repository)
	if err != nil {
		return fmt.Errorf("repository configuration failed: %w", err)
	}

	cmd := bm.buildBackupCommand(snapshotPath, target)
	cmd.Env = env
	err = cmd.Run()
	if err != nil {
		return fmt.Errorf("restic backup command failed: %w", err)
	}

	return nil
}

func (bm *Manager) buildBackupCommand(snapshotPath string, target *config.TargetConfig) *exec.Cmd {
	args := []string{"backup", snapshotPath}
	args = append(args, "--tag", "btrfs-backup")
	args = append(args, "--tag", target.Prefix)
	args = append(args, "--tag", filepath.Base(snapshotPath))
	args = append(args, "--exclude-caches")

	if target.Type == "full" {
		args = append(args, "--force")
	}

	return exec.Command(bm.config.ResticBin, args...)
}

func (bm *Manager) loadRepositoryEnv(repository string) ([]string, error) {
	repoFile := filepath.Join(bm.config.ResticRepoDir, repository)
	_, err := os.Stat(repoFile)
	if os.IsNotExist(err) {
		return nil, fmt.Errorf("repository configuration '%s' not found: %s", repository, repoFile)
	}

	env := os.Environ()

	data, err := os.ReadFile(repoFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read repository config %s: %w", repoFile, err)
	}

	// Parse YAML-style repository config
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
		env = append(env, fmt.Sprintf("%s=%s", key, value))
	}

	return env, nil
}

func (bm *Manager) VerifyRepository(repository string) error {
	env, err := bm.loadRepositoryEnv(repository)
	if err != nil {
		return fmt.Errorf("repository configuration failed for verification: %w", err)
	}

	cmd := exec.Command(bm.config.ResticBin, "check", "--read-data-subset=5%")
	cmd.Env = env

	err = cmd.Run()
	if err != nil {
		return fmt.Errorf("repository verification failed: %s - %w", repository, err)
	}

	return nil
}

func (bm *Manager) CleanupOldSnapshots(prefix string, retention int) error {
	snapshots, err := bm.getSnapshotsByPrefix(prefix)
	if err != nil {
		return fmt.Errorf("failed to list snapshots: %w", err)
	}

	if len(snapshots) <= retention {
		return nil
	}

	snapshotsToDelete := snapshots[retention:]
	var failedDeletions []string
	
	for _, snapshot := range snapshotsToDelete {
		err = bm.deleteSnapshot(snapshot)
		if err != nil {
			failedDeletions = append(failedDeletions, snapshot)
		}
	}

	if len(failedDeletions) > 0 {
		return fmt.Errorf("failed to delete some snapshots: %v", failedDeletions)
	}

	return nil
}

func (bm *Manager) getSnapshotsByPrefix(prefix string) ([]string, error) {
	_, err := os.Stat(bm.config.SnapshotDir)
	if os.IsNotExist(err) {
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

func (bm *Manager) deleteSnapshot(snapshotName string) error {
	snapshotPath := filepath.Join(bm.config.SnapshotDir, snapshotName)

	cmd := exec.Command("sudo", "btrfs", "subvolume", "delete", snapshotPath)
	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("BTRFS delete command failed for snapshot %s: %w", snapshotName, err)
	}

	_, err = os.Stat(snapshotPath)
	if err == nil {
		return fmt.Errorf("snapshot still exists after deletion: %s", snapshotPath)
	}

	return nil
}