package backup

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"btrfs-backup/internal/config"
)

func TestNewManager(t *testing.T) {
	cfg := &config.Config{
		TargetDir:     "/tmp/targets",
		SnapshotDir:   "/tmp/snapshots",
		ResticRepoDir: "/tmp/repos",
		ResticBin:     "/usr/bin/restic",
	}

	mgr := NewManager(cfg, true)
	if mgr.config != cfg {
		t.Error("Manager config not set correctly")
	}
	if !mgr.verbose {
		t.Error("Manager verbose flag not set correctly")
	}
}

func TestBuildBackupCommand(t *testing.T) {
	cfg := &config.Config{
		ResticBin: "/usr/bin/restic",
	}
	mgr := NewManager(cfg, false)

	target := &config.TargetConfig{
		Prefix: "test-backup",
		Type:   "incremental",
	}

	snapshotPath := "/tmp/snapshots/test-backup-20230101-120000"
	cmd := mgr.buildBackupCommand(snapshotPath, target)

	expectedArgs := []string{
		"/usr/bin/restic",
		"backup",
		snapshotPath,
		"--tag", "btrfs-backup",
		"--tag", target.Prefix,
		"--tag", "test-backup-20230101-120000",
		"--exclude-caches",
	}

	if len(cmd.Args) != len(expectedArgs) {
		t.Errorf("Expected %d args, got %d", len(expectedArgs), len(cmd.Args))
	}

	for i, expected := range expectedArgs {
		if i < len(cmd.Args) && cmd.Args[i] != expected {
			t.Errorf("Arg %d: expected '%s', got '%s'", i, expected, cmd.Args[i])
		}
	}

	// Test full backup
	target.Type = "full"
	cmd = mgr.buildBackupCommand(snapshotPath, target)
	
	if !slices.Contains(cmd.Args, "--force") {
		t.Error("Full backup should include --force flag")
	}
}

func TestLoadRepositoryEnv(t *testing.T) {
	// Create temporary directory and config file
	tmpDir, err := os.MkdirTemp("", "btrfs-backup-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	cfg := &config.Config{
		ResticRepoDir: tmpDir,
	}
	mgr := NewManager(cfg, false)

	// Create test repository config
	repoConfig := `RESTIC_REPOSITORY: b2:bucket/path
RESTIC_PASSWORD: secret123
B2_ACCOUNT_ID: account123
B2_ACCOUNT_KEY: key123
`
	repoPath := filepath.Join(tmpDir, "test-repo")
	err = os.WriteFile(repoPath, []byte(repoConfig), 0644)
	if err != nil {
		t.Fatalf("Failed to write repo config: %v", err)
	}

	env, err := mgr.loadRepositoryEnv("test-repo")
	if err != nil {
		t.Fatalf("loadRepositoryEnv failed: %v", err)
	}

	// Check that environment variables were added
	expectedVars := map[string]string{
		"RESTIC_REPOSITORY": "b2:bucket/path",
		"RESTIC_PASSWORD":   "secret123",
		"B2_ACCOUNT_ID":     "account123",
		"B2_ACCOUNT_KEY":    "key123",
	}

	envMap := make(map[string]string)
	for _, envVar := range env {
		parts := strings.SplitN(envVar, "=", 2)
		if len(parts) == 2 {
			envMap[parts[0]] = parts[1]
		}
	}

	for key, expectedValue := range expectedVars {
		if value, exists := envMap[key]; !exists {
			t.Errorf("Environment variable %s not found", key)
		} else if value != expectedValue {
			t.Errorf("Environment variable %s: expected '%s', got '%s'", key, expectedValue, value)
		}
	}

	// Test missing repository file
	_, err = mgr.loadRepositoryEnv("nonexistent-repo")
	if err == nil {
		t.Error("loadRepositoryEnv should fail for nonexistent repository")
	}
}

func TestGetSnapshotsByPrefix(t *testing.T) {
	// Create temporary directory
	tmpDir, err := os.MkdirTemp("", "btrfs-backup-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	cfg := &config.Config{
		SnapshotDir: tmpDir,
	}
	mgr := NewManager(cfg, false)

	// Create test snapshot directories with different timestamps
	snapshots := []string{
		"test-backup-20230101-120000",
		"test-backup-20230102-120000",
		"other-backup-20230101-120000",
		"test-backup-20230103-120000",
	}

	for i, snapshot := range snapshots {
		snapshotPath := filepath.Join(tmpDir, snapshot)
		err := os.Mkdir(snapshotPath, 0755)
		if err != nil {
			t.Fatalf("Failed to create snapshot dir: %v", err)
		}
		
		// Set different modification times
		modTime := time.Now().Add(time.Duration(-i) * time.Hour)
		err = os.Chtimes(snapshotPath, modTime, modTime)
		if err != nil {
			t.Fatalf("Failed to set modification time: %v", err)
		}
	}

	// Test getting snapshots by prefix
	result, err := mgr.getSnapshotsByPrefix("test-backup")
	if err != nil {
		t.Fatalf("getSnapshotsByPrefix failed: %v", err)
	}

	// Should return 3 snapshots matching "test-backup" prefix, sorted by newest first
	expected := []string{
		"test-backup-20230101-120000", // newest (i=0, least subtracted time)
		"test-backup-20230102-120000",
		"test-backup-20230103-120000", // oldest (i=3, most subtracted time)
	}

	if len(result) != len(expected) {
		t.Errorf("Expected %d snapshots, got %d", len(expected), len(result))
	}

	for i, expectedSnapshot := range expected {
		if i < len(result) && result[i] != expectedSnapshot {
			t.Errorf("Snapshot %d: expected '%s', got '%s'", i, expectedSnapshot, result[i])
		}
	}

	// Test with nonexistent snapshot dir
	cfg.SnapshotDir = "/nonexistent"
	mgr = NewManager(cfg, false)
	result, err = mgr.getSnapshotsByPrefix("test-backup")
	if err != nil {
		t.Fatalf("getSnapshotsByPrefix should not fail for nonexistent dir: %v", err)
	}
	if len(result) != 0 {
		t.Errorf("Expected empty result for nonexistent dir, got %d snapshots", len(result))
	}
}