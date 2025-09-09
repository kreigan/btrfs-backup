package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/viper"
)

func TestLoadConfig(t *testing.T) {
	// Create a temporary config file
	tmpDir, err := os.MkdirTemp("", "btrfs-backup-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	configFile := filepath.Join(tmpDir, "config.yaml")
	configData := `target_dir: /tmp/targets
snapshot_dir: /tmp/snapshots
restic_repo_dir: /tmp/repos
restic_bin: /usr/bin/restic
`
	err = os.WriteFile(configFile, []byte(configData), 0644)
	if err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	// Test loading the config
	config, err := LoadConfig(configFile)
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	if config.TargetDir != "/tmp/targets" {
		t.Errorf("Expected TargetDir '/tmp/targets', got '%s'", config.TargetDir)
	}
	if config.SnapshotDir != "/tmp/snapshots" {
		t.Errorf("Expected SnapshotDir '/tmp/snapshots', got '%s'", config.SnapshotDir)
	}
	if config.ResticRepoDir != "/tmp/repos" {
		t.Errorf("Expected ResticRepoDir '/tmp/repos', got '%s'", config.ResticRepoDir)
	}
	if config.ResticBin != "/usr/bin/restic" {
		t.Errorf("Expected ResticBin '/usr/bin/restic', got '%s'", config.ResticBin)
	}
}

func TestLoadConfigWithEnvironmentVariables(t *testing.T) {
	// Set environment variables
	os.Setenv("BTRFSBACKUP_TARGET_DIR", "/env/targets")
	os.Setenv("BTRFSBACKUP_SNAPSHOT_DIR", "/env/snapshots")
	defer os.Unsetenv("BTRFSBACKUP_TARGET_DIR")
	defer os.Unsetenv("BTRFSBACKUP_SNAPSHOT_DIR")

	// Create a temporary config file with some values
	tmpDir, err := os.MkdirTemp("", "btrfs-backup-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	configFile := filepath.Join(tmpDir, "config.yaml")
	configData := `target_dir: /file/targets
snapshot_dir: /file/snapshots
restic_repo_dir: /tmp/repos
restic_bin: /usr/bin/restic
`
	err = os.WriteFile(configFile, []byte(configData), 0644)
	if err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	config, err := LoadConfig(configFile)
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	// Environment variables should override file values
	if config.TargetDir != "/env/targets" {
		t.Errorf("Expected TargetDir from env '/env/targets', got '%s'", config.TargetDir)
	}
	if config.SnapshotDir != "/env/snapshots" {
		t.Errorf("Expected SnapshotDir from env '/env/snapshots', got '%s'", config.SnapshotDir)
	}
}

func TestLoadTargetConfig(t *testing.T) {
	// Create a temporary target config file
	tmpDir, err := os.MkdirTemp("", "btrfs-backup-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	targetFile := filepath.Join(tmpDir, "target.yaml")
	targetData := `subvolume: /mnt/btrfs/home
prefix: home-backup
repository: b2-home
type: incremental
verify: true
keep_snapshots: 5
`
	err = os.WriteFile(targetFile, []byte(targetData), 0644)
	if err != nil {
		t.Fatalf("Failed to write target file: %v", err)
	}

	// Test loading the target config
	target, err := LoadTargetConfig(targetFile)
	if err != nil {
		t.Fatalf("LoadTargetConfig failed: %v", err)
	}

	if target.Subvolume != "/mnt/btrfs/home" {
		t.Errorf("Expected Subvolume '/mnt/btrfs/home', got '%s'", target.Subvolume)
	}
	if target.Prefix != "home-backup" {
		t.Errorf("Expected Prefix 'home-backup', got '%s'", target.Prefix)
	}
	if target.Repository != "b2-home" {
		t.Errorf("Expected Repository 'b2-home', got '%s'", target.Repository)
	}
	if target.Type != "incremental" {
		t.Errorf("Expected Type 'incremental', got '%s'", target.Type)
	}
	if !target.Verify {
		t.Errorf("Expected Verify true, got %v", target.Verify)
	}
	if target.KeepSnapshots != 5 {
		t.Errorf("Expected KeepSnapshots 5, got %d", target.KeepSnapshots)
	}
}

func TestLoadTargetConfigWithDefaults(t *testing.T) {
	// Create a minimal target config file
	tmpDir, err := os.MkdirTemp("", "btrfs-backup-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	targetFile := filepath.Join(tmpDir, "target.yaml")
	targetData := `subvolume: /mnt/btrfs/home
prefix: home-backup  
repository: b2-home
`
	err = os.WriteFile(targetFile, []byte(targetData), 0644)
	if err != nil {
		t.Fatalf("Failed to write target file: %v", err)
	}

	target, err := LoadTargetConfig(targetFile)
	if err != nil {
		t.Fatalf("LoadTargetConfig failed: %v", err)
	}

	// Check that defaults are applied
	if target.Type != "incremental" {
		t.Errorf("Expected default Type 'incremental', got '%s'", target.Type)
	}
	if target.KeepSnapshots != 3 {
		t.Errorf("Expected default KeepSnapshots 3, got %d", target.KeepSnapshots)
	}
	if target.Verify != false {
		t.Errorf("Expected default Verify false, got %v", target.Verify)
	}
}

func TestSetConfigDefaults(t *testing.T) {
	v := viper.New()
	setConfigDefaults(v)

	if v.GetString("restic_bin") != "/usr/bin/restic" {
		t.Errorf("Expected default restic_bin '/usr/bin/restic', got '%s'", v.GetString("restic_bin"))
	}
}

func TestSetTargetDefaults(t *testing.T) {
	v := viper.New()
	setTargetDefaults(v)

	if v.GetString("type") != "incremental" {
		t.Errorf("Expected default type 'incremental', got '%s'", v.GetString("type"))
	}
	if v.GetInt("keep_snapshots") != 3 {
		t.Errorf("Expected default keep_snapshots 3, got %d", v.GetInt("keep_snapshots"))
	}
	if v.GetBool("verify") != false {
		t.Errorf("Expected default verify false, got %v", v.GetBool("verify"))
	}
}

func TestValidateConfig(t *testing.T) {
	validConfig := &Config{
		TargetDir:     "/tmp/targets",
		SnapshotDir:   "/tmp/snapshots",
		ResticRepoDir: "/tmp/repos",
		ResticBin:     "/usr/bin/restic",
	}

	err := validateConfig(validConfig)
	if err != nil {
		t.Errorf("validateConfig failed for valid config: %v", err)
	}

	// Test missing fields
	invalidConfigs := []*Config{
		{SnapshotDir: "/tmp/snapshots", ResticRepoDir: "/tmp/repos", ResticBin: "/usr/bin/restic"},
		{TargetDir: "/tmp/targets", ResticRepoDir: "/tmp/repos", ResticBin: "/usr/bin/restic"},
		{TargetDir: "/tmp/targets", SnapshotDir: "/tmp/snapshots", ResticBin: "/usr/bin/restic"},
		{TargetDir: "/tmp/targets", SnapshotDir: "/tmp/snapshots", ResticRepoDir: "/tmp/repos"},
	}

	for i, config := range invalidConfigs {
		err := validateConfig(config)
		if err == nil {
			t.Errorf("validateConfig should have failed for invalid config %d", i)
		}
	}
}

func TestValidateTargetConfig(t *testing.T) {
	validTarget := &TargetConfig{
		Subvolume:     "/mnt/btrfs/home",
		Prefix:        "home-backup",
		Repository:    "b2-home",
		Type:          "incremental",
		Verify:        true,
		KeepSnapshots: 3,
	}

	err := validateTargetConfig(validTarget)
	if err != nil {
		t.Errorf("validateTargetConfig failed for valid target: %v", err)
	}

	// Test invalid type
	invalidTarget := &TargetConfig{
		Subvolume:     "/mnt/btrfs/home",
		Prefix:        "home-backup",
		Repository:    "b2-home",
		Type:          "invalid",
		Verify:        true,
		KeepSnapshots: 3,
	}

	err = validateTargetConfig(invalidTarget)
	if err == nil {
		t.Error("validateTargetConfig should have failed for invalid type")
	}

	// Test negative keep_snapshots
	invalidTarget.Type = "incremental"
	invalidTarget.KeepSnapshots = -1
	err = validateTargetConfig(invalidTarget)
	if err == nil {
		t.Error("validateTargetConfig should have failed for negative keep_snapshots")
	}
}

func TestGetConfigPath(t *testing.T) {
	// Test with provided path
	provided := "/custom/config.yaml"
	result := GetConfigPath(provided)
	if result != provided {
		t.Errorf("Expected provided path '%s', got '%s'", provided, result)
	}

	// Test with environment variable
	os.Setenv("BTRFSBACKUP_CONFIG", "/env/config.yaml")
	result = GetConfigPath("")
	if result != "/env/config.yaml" {
		t.Errorf("Expected env path '/env/config.yaml', got '%s'", result)
	}
	os.Unsetenv("BTRFSBACKUP_CONFIG")

	// Test default path
	result = GetConfigPath("")
	home, _ := os.UserHomeDir()
	expected := filepath.Join(home, ".config", "btrfs-backup", "config.yaml")
	if result != expected {
		t.Errorf("Expected default path '%s', got '%s'", expected, result)
	}
}

func TestGetTargetConfigPath(t *testing.T) {
	// Test with provided path
	provided := "/custom/target.yaml"
	result := GetTargetConfigPath(provided, "/targets", "test-target")
	if result != provided {
		t.Errorf("Expected provided path '%s', got '%s'", provided, result)
	}

	// Test with target dir
	result = GetTargetConfigPath("", "/custom/targets", "test-target")
	expected := "/custom/targets/test-target"
	if result != expected {
		t.Errorf("Expected '%s', got '%s'", expected, result)
	}

	// Test default path
	result = GetTargetConfigPath("", "", "test-target")
	home, _ := os.UserHomeDir()
	expected = filepath.Join(home, ".config", "btrfs-backup", "targets", "test-target")
	if result != expected {
		t.Errorf("Expected default path '%s', got '%s'", expected, result)
	}
}