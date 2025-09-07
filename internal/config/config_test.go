package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseYAML(t *testing.T) {
	yamlData := `
target_dir: /tmp/targets
snapshot_dir: /tmp/snapshots
restic_repo_dir: /tmp/repos
restic_bin: /usr/bin/restic
`

	var config Config
	err := parseYAML([]byte(yamlData), &config)
	if err != nil {
		t.Fatalf("parseYAML failed: %v", err)
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

func TestParseTargetYAML(t *testing.T) {
	yamlData := `
subvolume: /mnt/btrfs/home
prefix: home-backup
repository: b2-home
type: incremental
verify: true
keep_snapshots: 5
`

	var target TargetConfig
	err := parseTargetYAML([]byte(yamlData), &target)
	if err != nil {
		t.Fatalf("parseTargetYAML failed: %v", err)
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

func TestParseInt(t *testing.T) {
	tests := []struct {
		input    string
		expected int
	}{
		{"123", 123},
		{"0", 0},
		{"5", 5},
		{"abc", -1},
		{"12abc", -1},
		{"", 0},
	}

	for _, test := range tests {
		result := parseInt(test.input)
		if result != test.expected {
			t.Errorf("parseInt(%q) = %d, expected %d", test.input, result, test.expected)
		}
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

func TestSetTargetDefaults(t *testing.T) {
	target := &TargetConfig{
		Subvolume:  "/mnt/btrfs/home",
		Prefix:     "home-backup",
		Repository: "b2-home",
	}

	setTargetDefaults(target)

	if target.Type != "incremental" {
		t.Errorf("Expected default Type 'incremental', got '%s'", target.Type)
	}
	if target.KeepSnapshots != 3 {
		t.Errorf("Expected default KeepSnapshots 3, got %d", target.KeepSnapshots)
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