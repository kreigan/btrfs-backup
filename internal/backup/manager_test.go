package backup

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"btrfs-backup/internal/config"
)

// Mock implementations for testing
//
// This file provides mock implementations for the backup package's dependencies,
// allowing comprehensive unit testing without requiring actual BTRFS operations,
// file system access, or Restic commands.
//
// Complete test example:
//   func TestBackupWorkflow(t *testing.T) {
//     cfg := &config.Config{
//       SnapshotDir:   "/snapshots",
//       ResticRepoDir: "/repos",
//     }
//
//     // Set up mocks
//     mockFS := NewMockFileSystem()
//     mockBtrfs := NewMockBtrfsClient(t)
//     mockRestic := NewMockResticClient(t)
//
//     // Mock filesystem state
//     mockFS.AddDir("/snapshots", []MockDirEntry{})
//     mockFS.AddFile("/repos/backup-repo", []byte("RESTIC_REPOSITORY: /backup"))
//
//     // Set up expectations
//     mockBtrfs.ExpectShowSubvolume("/mnt/data", 0)
//     mockBtrfs.ExpectCreateSnapshot("", "", true, 0)
//     mockBtrfs.onCreateSnapshot = func(subvol, snap string) {
//       mockFS.AddFile(snap, []byte{}) // Simulate snapshot creation
//     }
//     mockRestic.ExpectBackup("", []string{"backup-tag"}, true, false, 0)
//
//     // Test the functionality
//     mgr := NewManagerWithDeps(cfg, false, mockFS, mockBtrfs, mockRestic)
//     err := mgr.RunBackup("test", &config.TargetConfig{
//       Subvolume: "/mnt/data", Repository: "backup-repo", Prefix: "test",
//     })
//     assert.NoError(t, err)
//   }

// MockFileSystem implements FileSystem interface for testing.
//
// It simulates a file system in memory, allowing tests to control exactly
// which files and directories exist, their contents, and any errors that
// should be returned.
//
// Example usage:
//
//	mockFS := NewMockFileSystem()
//	mockFS.AddFile("/path/file.txt", []byte("content"))
//	mockFS.AddDir("/path", []MockDirEntry{})
//	mockFS.SetStatError("/missing", os.ErrNotExist)
//
//	// Now mockFS.Stat("/path/file.txt") returns file info
//	// mockFS.ReadFile("/path/file.txt") returns "content"
//	// mockFS.Stat("/missing") returns os.ErrNotExist
type MockFileSystem struct {
	files    map[string][]byte
	dirs     map[string][]MockDirEntry
	statErrs map[string]error
}

// MockDirEntry represents a directory entry for testing.
//
// Used with MockFileSystem.AddDir() to simulate directory contents.
//
// Example:
//
//	entries := []MockDirEntry{
//	  {name: "file1.txt", isDir: false, modTime: time.Now()},
//	  {name: "subdir", isDir: true, modTime: time.Now().Add(-1*time.Hour)},
//	}
//	mockFS.AddDir("/path", entries)
type MockDirEntry struct {
	name    string
	isDir   bool
	modTime time.Time
}

func (m MockDirEntry) Name() string {
	return m.name
}

func (m MockDirEntry) IsDir() bool {
	return m.isDir
}

func (m MockDirEntry) Type() os.FileMode {
	if m.isDir {
		return os.ModeDir
	}
	return 0
}

func (m MockDirEntry) Info() (os.FileInfo, error) {
	return &MockFileInfo{name: m.name, modTime: m.modTime, isDir: m.isDir}, nil
}

type MockFileInfo struct {
	name    string
	modTime time.Time
	isDir   bool
}

func (m *MockFileInfo) Name() string       { return m.name }
func (m *MockFileInfo) Size() int64        { return 0 }
func (m *MockFileInfo) Mode() os.FileMode  { return 0 }
func (m *MockFileInfo) ModTime() time.Time { return m.modTime }
func (m *MockFileInfo) IsDir() bool        { return m.isDir }
func (m *MockFileInfo) Sys() any           { return nil }

func NewMockFileSystem() *MockFileSystem {
	return &MockFileSystem{
		files:    make(map[string][]byte),
		dirs:     make(map[string][]MockDirEntry),
		statErrs: make(map[string]error),
	}
}

// AddFile adds a file to the mock filesystem.
// Subsequent calls to Stat() and ReadFile() will succeed for this path.
func (m *MockFileSystem) AddFile(path string, content []byte) {
	m.files[path] = content
}

// AddDir adds a directory with the specified entries to the mock filesystem.
// Subsequent calls to Stat() and ReadDir() will succeed for this path.
func (m *MockFileSystem) AddDir(path string, entries []MockDirEntry) {
	m.dirs[path] = entries
}

// SetStatError configures Stat() to return the specified error for a path.
// Commonly used to simulate missing files with os.ErrNotExist.
func (m *MockFileSystem) SetStatError(path string, err error) {
	m.statErrs[path] = err
}

func (m *MockFileSystem) Stat(name string) (os.FileInfo, error) {
	if err, exists := m.statErrs[name]; exists {
		return nil, err
	}
	if _, exists := m.files[name]; exists {
		return &MockFileInfo{name: filepath.Base(name)}, nil
	}
	if _, exists := m.dirs[name]; exists {
		return &MockFileInfo{name: filepath.Base(name), isDir: true}, nil
	}
	return nil, os.ErrNotExist
}

func (m *MockFileSystem) ReadDir(name string) ([]os.DirEntry, error) {
	if entries, exists := m.dirs[name]; exists {
		result := make([]os.DirEntry, len(entries))
		for i, entry := range entries {
			result[i] = entry
		}
		return result, nil
	}
	return nil, os.ErrNotExist
}

func (m *MockFileSystem) ReadFile(filename string) ([]byte, error) {
	if content, exists := m.files[filename]; exists {
		return content, nil
	}
	return nil, os.ErrNotExist
}

// MockBtrfsClient implements BtrfsClient interface for testing.
//
// It allows tests to verify that the correct BTRFS commands are executed
// in the expected order with the correct arguments. Use the Expect* methods
// to set up expected operations, then the mock will verify calls match.
//
// Example usage:
//
//	mockBtrfs := NewMockBtrfsClient(t)
//	mockBtrfs.ExpectShowSubvolume("/mnt/btrfs/home", 0)
//	mockBtrfs.ExpectCreateSnapshot("/mnt/btrfs/home", "/snapshots/backup-20230101-120000", true, 0)
//	mockBtrfs.onCreateSnapshot = func(subvol, snap string) {
//	  // Callback executed on successful snapshot creation
//	  mockFS.AddFile(snap, []byte{})
//	}
//
//	// Now calls to ShowSubvolume() and CreateSnapshot() will be verified
type MockBtrfsClient struct {
	expectedCommands []ExpectedBtrfsCommand
	index            int
	t                *testing.T
	onCreateSnapshot func(subvolume, snapshotPath string) // callback for successful snapshot creation
}

type ExpectedBtrfsCommand struct {
	operation string
	args      []string
	exitCode  int
}

func NewMockBtrfsClient(t *testing.T) *MockBtrfsClient {
	return &MockBtrfsClient{t: t}
}

// ExpectShowSubvolume sets up expectation for a 'btrfs subvolume show' command.
// exitCode 0 means success, non-zero means the command will fail.
func (m *MockBtrfsClient) ExpectShowSubvolume(subvolume string, exitCode int) {
	m.expectedCommands = append(m.expectedCommands, ExpectedBtrfsCommand{
		operation: "show",
		args:      []string{subvolume},
		exitCode:  exitCode,
	})
}

// ExpectCreateSnapshot sets up expectation for a 'btrfs subvolume snapshot' command.
// Use empty strings for subvolume and snapshotPath to accept any arguments.
// Set onCreateSnapshot callback to simulate filesystem effects of successful creation.
func (m *MockBtrfsClient) ExpectCreateSnapshot(subvolume, snapshotPath string, readonly bool, exitCode int) {
	args := []string{subvolume, snapshotPath}
	// If both subvolume and snapshotPath are empty, use empty args to accept any
	if subvolume == "" && snapshotPath == "" {
		args = []string{}
	}
	m.expectedCommands = append(m.expectedCommands, ExpectedBtrfsCommand{
		operation: "snapshot",
		args:      args,
		exitCode:  exitCode,
	})
}

// ExpectDeleteSubvolume sets up expectation for a 'btrfs subvolume delete' command.
func (m *MockBtrfsClient) ExpectDeleteSubvolume(subvolumePath string, exitCode int) {
	m.expectedCommands = append(m.expectedCommands, ExpectedBtrfsCommand{
		operation: "delete",
		args:      []string{subvolumePath},
		exitCode:  exitCode,
	})
}

func (m *MockBtrfsClient) ShowSubvolume(subvolume string) error {
	if m.index >= len(m.expectedCommands) {
		m.t.Fatalf("Unexpected btrfs show command for subvolume: %s", subvolume)
	}

	expected := m.expectedCommands[m.index]
	m.index++

	if expected.operation != "show" || len(expected.args) != 1 || expected.args[0] != subvolume {
		m.t.Fatalf("Expected btrfs show %s, got show %s", expected.args[0], subvolume)
	}

	if expected.exitCode != 0 {
		return fmt.Errorf("btrfs command failed with exit code %d", expected.exitCode)
	}
	return nil
}

func (m *MockBtrfsClient) CreateSnapshot(subvolume, snapshotPath string, readonly bool) error {
	if m.index >= len(m.expectedCommands) {
		m.t.Fatalf("Unexpected btrfs snapshot command: %s -> %s", subvolume, snapshotPath)
	}

	expected := m.expectedCommands[m.index]
	m.index++

	if expected.operation != "snapshot" {
		m.t.Fatalf("Expected btrfs snapshot operation, got %s", expected.operation)
	}

	// Allow flexible matching - if args are empty, accept any arguments
	if len(expected.args) > 0 {
		if len(expected.args) != 2 || expected.args[0] != subvolume || expected.args[1] != snapshotPath {
			m.t.Fatalf("Expected btrfs snapshot %s %s, got snapshot %s %s",
				expected.args[0], expected.args[1], subvolume, snapshotPath)
		}
	}

	if expected.exitCode != 0 {
		return fmt.Errorf("btrfs command failed with exit code %d", expected.exitCode)
	}

	// Call callback for successful snapshot creation
	if m.onCreateSnapshot != nil {
		m.onCreateSnapshot(subvolume, snapshotPath)
	}
	return nil
}

func (m *MockBtrfsClient) DeleteSubvolume(subvolumePath string) error {
	if m.index >= len(m.expectedCommands) {
		m.t.Fatalf("Unexpected btrfs delete command for: %s", subvolumePath)
	}

	expected := m.expectedCommands[m.index]
	m.index++

	if expected.operation != "delete" || len(expected.args) != 1 || expected.args[0] != subvolumePath {
		m.t.Fatalf("Expected btrfs delete %s, got delete %s", expected.args[0], subvolumePath)
	}

	if expected.exitCode != 0 {
		return fmt.Errorf("btrfs command failed with exit code %d", expected.exitCode)
	}
	return nil
}

// MockResticClient implements ResticClient interface for testing.
//
// It allows tests to verify that the correct Restic commands are executed
// with the expected parameters. Commands are verified in the order they
// were set up using Expect* methods.
//
// Example usage:
//
//	mockRestic := NewMockResticClient(t)
//	mockRestic.ExpectBackup("/snapshots/backup-20230101", []string{"tag1", "tag2"}, true, false, 0)
//	mockRestic.ExpectCheck("5%", 0)
//
//	// Now calls to Backup() and Check() will be verified against expectations
type MockResticClient struct {
	expectedCommands []ExpectedResticCommand
	index            int
	t                *testing.T
}

type ExpectedResticCommand struct {
	operation      string
	snapshotPath   string
	tags           []string
	exitCode       int
	readDataSubset string
}

func NewMockResticClient(t *testing.T) *MockResticClient {
	return &MockResticClient{t: t}
}

// ExpectBackup sets up expectation for a 'restic backup' command.
// Use empty snapshotPath to accept any path. exitCode 0 means success.
func (m *MockResticClient) ExpectBackup(snapshotPath string, tags []string, excludeCaches bool, force bool, exitCode int) {
	m.expectedCommands = append(m.expectedCommands, ExpectedResticCommand{
		operation:    "backup",
		snapshotPath: snapshotPath,
		tags:         tags,
		exitCode:     exitCode,
	})
}

// ExpectCheck sets up expectation for a 'restic check' command.
// readDataSubset specifies the percentage of data to verify (e.g., "5%").
func (m *MockResticClient) ExpectCheck(readDataSubset string, exitCode int) {
	m.expectedCommands = append(m.expectedCommands, ExpectedResticCommand{
		operation:      "check",
		readDataSubset: readDataSubset,
		exitCode:       exitCode,
	})
}

func (m *MockResticClient) Backup(repositoryEnv []string, snapshotPath string, tags []string, excludeCaches bool, force bool) error {
	if m.index >= len(m.expectedCommands) {
		m.t.Fatalf("Unexpected restic backup command for: %s", snapshotPath)
	}

	expected := m.expectedCommands[m.index]
	m.index++

	if expected.operation != "backup" {
		m.t.Fatalf("Expected restic backup operation, got %s", expected.operation)
	}
	// Allow flexible matching - if snapshotPath is empty, accept any path
	if expected.snapshotPath != "" && expected.snapshotPath != snapshotPath {
		m.t.Fatalf("Expected restic backup %s, got backup %s", expected.snapshotPath, snapshotPath)
	}

	if expected.exitCode != 0 {
		return fmt.Errorf("restic command failed with exit code %d", expected.exitCode)
	}
	return nil
}

func (m *MockResticClient) Check(repositoryEnv []string, readDataSubset string) error {
	if m.index >= len(m.expectedCommands) {
		m.t.Fatalf("Unexpected restic check command")
	}

	expected := m.expectedCommands[m.index]
	m.index++

	if expected.operation != "check" || expected.readDataSubset != readDataSubset {
		m.t.Fatalf("Expected restic check with %s, got check with %s", expected.readDataSubset, readDataSubset)
	}

	if expected.exitCode != 0 {
		return fmt.Errorf("restic command failed with exit code %d", expected.exitCode)
	}
	return nil
}

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

	// Test that real implementations are used by default
	if mgr.fs == nil {
		t.Error("FileSystem not initialized")
	}
	if mgr.btrfs == nil {
		t.Error("BtrfsClient not initialized")
	}
	if mgr.restic == nil {
		t.Error("ResticClient not initialized")
	}
}

func TestNewManagerWithDeps(t *testing.T) {
	cfg := &config.Config{
		TargetDir:     "/tmp/targets",
		SnapshotDir:   "/tmp/snapshots",
		ResticRepoDir: "/tmp/repos",
		ResticBin:     "/usr/bin/restic",
	}

	mockFS := NewMockFileSystem()
	mockBtrfs := NewMockBtrfsClient(t)
	mockRestic := NewMockResticClient(t)

	mgr := NewManagerWithDeps(cfg, false, mockFS, mockBtrfs, mockRestic)

	if mgr.config != cfg {
		t.Error("Manager config not set correctly")
	}
	if mgr.verbose {
		t.Error("Manager verbose flag should be false")
	}
	if mgr.fs != mockFS {
		t.Error("FileSystem dependency not set correctly")
	}
	if mgr.btrfs != mockBtrfs {
		t.Error("BtrfsClient dependency not set correctly")
	}
	if mgr.restic != mockRestic {
		t.Error("ResticClient dependency not set correctly")
	}
}

func TestValidateEnvironment(t *testing.T) {
	cfg := &config.Config{
		SnapshotDir: "/snapshots",
	}

	tests := []struct {
		name           string
		subvolume      string
		snapshotDirErr error
		btrfsExitCode  int
		expectError    bool
		errorContains  string
	}{
		{
			name:           "valid_environment",
			subvolume:      "/mnt/btrfs/home",
			snapshotDirErr: nil,
			btrfsExitCode:  0,
			expectError:    false,
		},
		{
			name:           "snapshot_dir_missing",
			subvolume:      "/mnt/btrfs/home",
			snapshotDirErr: os.ErrNotExist,
			expectError:    true,
			errorContains:  "snapshots directory does not exist",
		},
		{
			name:           "snapshot_dir_permission_denied",
			subvolume:      "/mnt/btrfs/home",
			snapshotDirErr: os.ErrPermission,
			btrfsExitCode:  0,
			expectError:    false, // Non-NotExist errors are ignored
		},
		{
			name:           "invalid_btrfs_subvolume",
			subvolume:      "/invalid/path",
			snapshotDirErr: nil,
			btrfsExitCode:  1,
			expectError:    true,
			errorContains:  "source subvolume invalid or not BTRFS",
		},
		{
			name:           "btrfs_command_not_found",
			subvolume:      "/mnt/btrfs/home",
			snapshotDirErr: nil,
			btrfsExitCode:  127,
			expectError:    true,
			errorContains:  "source subvolume invalid or not BTRFS",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockFS := NewMockFileSystem()
			mockBtrfs := NewMockBtrfsClient(t)
			mockRestic := NewMockResticClient(t)

			// Setup file system mock
			if tt.snapshotDirErr != nil {
				mockFS.SetStatError("/snapshots", tt.snapshotDirErr)
			} else {
				mockFS.AddDir("/snapshots", []MockDirEntry{})
			}

			// Setup btrfs mock - only skip if snapshot dir doesn't exist
			if tt.snapshotDirErr != os.ErrNotExist {
				mockBtrfs.ExpectShowSubvolume(tt.subvolume, tt.btrfsExitCode)
			}

			mgr := NewManagerWithDeps(cfg, false, mockFS, mockBtrfs, mockRestic)
			err := mgr.ValidateEnvironment(tt.subvolume)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error but got none")
				} else if tt.errorContains != "" && !strings.Contains(err.Error(), tt.errorContains) {
					t.Errorf("Expected error containing '%s', got '%s'", tt.errorContains, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error but got: %v", err)
				}
			}
		})
	}
}

func TestCreateSnapshot(t *testing.T) {
	cfg := &config.Config{
		SnapshotDir: "/snapshots",
	}

	t.Run("successful_snapshot_creation", func(t *testing.T) {
		mockFS := NewMockFileSystem()
		mockBtrfs := NewMockBtrfsClient(t)
		mockRestic := NewMockResticClient(t)

		// Set up callback to add file when snapshot is created successfully
		mockBtrfs.onCreateSnapshot = func(subvolume, snapshotPath string) {
			mockFS.AddFile(snapshotPath, []byte{})
		}
		mockBtrfs.ExpectCreateSnapshot("", "", true, 0)

		mgr := NewManagerWithDeps(cfg, false, mockFS, mockBtrfs, mockRestic)
		snapshotPath, err := mgr.CreateSnapshot("/mnt/btrfs/home", "home-backup")

		if err != nil {
			t.Errorf("Expected no error but got: %v", err)
		}
		if !strings.HasPrefix(snapshotPath, "/snapshots/home-backup-") {
			t.Errorf("Expected snapshot path to start with '/snapshots/home-backup-', got '%s'", snapshotPath)
		}
	})

	t.Run("btrfs_command_failure", func(t *testing.T) {
		mockFS := NewMockFileSystem()
		mockBtrfs := NewMockBtrfsClient(t)
		mockRestic := NewMockResticClient(t)
		mockBtrfs.ExpectCreateSnapshot("", "", true, 1)

		mgr := NewManagerWithDeps(cfg, false, mockFS, mockBtrfs, mockRestic)
		_, err := mgr.CreateSnapshot("/mnt/btrfs/home", "home-backup")

		if err == nil {
			t.Error("Expected error but got none")
		}
		if !strings.Contains(err.Error(), "BTRFS snapshot command failed") {
			t.Errorf("Expected error containing 'BTRFS snapshot command failed', got '%s'", err.Error())
		}
	})

	t.Run("snapshot_not_found_after_creation", func(t *testing.T) {
		mockFS := NewMockFileSystem()
		mockBtrfs := NewMockBtrfsClient(t)
		mockRestic := NewMockResticClient(t)

		// Don't set onCreateSnapshot callback, so file won't be created
		mockBtrfs.ExpectCreateSnapshot("", "", true, 0)

		mgr := NewManagerWithDeps(cfg, false, mockFS, mockBtrfs, mockRestic)
		snapshotPath, err := mgr.CreateSnapshot("/mnt/btrfs/home", "home-backup")

		if err == nil {
			t.Error("Expected error when snapshot not found after creation")
		}
		if !strings.Contains(err.Error(), "snapshot not found after creation") {
			t.Errorf("Expected error containing 'snapshot not found after creation', got '%s'", err.Error())
		}
		if snapshotPath != "" {
			t.Errorf("Expected empty snapshot path on error, got '%s'", snapshotPath)
		}
	})
}

func TestPerformBackup(t *testing.T) {
	cfg := &config.Config{
		SnapshotDir:   "/snapshots",
		ResticRepoDir: "/repos",
		ResticBin:     "/usr/bin/restic",
	}

	tests := []struct {
		name              string
		snapshotPath      string
		repository        string
		backupType        string
		snapshotExists    bool
		repoConfigExists  bool
		repoConfigContent string
		resticExitCode    int
		expectError       bool
		errorContains     string
	}{
		{
			name:              "successful_incremental_backup",
			snapshotPath:      "/snapshots/home-20230101-120000",
			repository:        "b2-home",
			backupType:        "incremental",
			snapshotExists:    true,
			repoConfigExists:  true,
			repoConfigContent: "RESTIC_REPOSITORY: b2:bucket/path\nRESTC_PASSWORD: secret123",
			resticExitCode:    0,
			expectError:       false,
		},
		{
			name:           "snapshot_path_missing",
			snapshotPath:   "/snapshots/nonexistent",
			repository:     "b2-home",
			backupType:     "incremental",
			snapshotExists: false,
			expectError:    true,
			errorContains:  "snapshot path does not exist",
		},
		{
			name:             "repository_config_missing",
			snapshotPath:     "/snapshots/home-20230101-120000",
			repository:       "nonexistent-repo",
			backupType:       "incremental",
			snapshotExists:   true,
			repoConfigExists: false,
			expectError:      true,
			errorContains:    "repository configuration failed",
		},
		{
			name:              "restic_backup_failure",
			snapshotPath:      "/snapshots/home-20230101-120000",
			repository:        "b2-home",
			backupType:        "incremental",
			snapshotExists:    true,
			repoConfigExists:  true,
			repoConfigContent: "RESTIC_REPOSITORY: b2:bucket/path",
			resticExitCode:    1,
			expectError:       true,
			errorContains:     "restic backup command failed",
		},
		{
			name:              "full_backup_with_force_flag",
			snapshotPath:      "/snapshots/home-20230101-120000",
			repository:        "b2-home",
			backupType:        "full",
			snapshotExists:    true,
			repoConfigExists:  true,
			repoConfigContent: "RESTIC_REPOSITORY: b2:bucket/path",
			resticExitCode:    0,
			expectError:       false,
		},
		{
			name:              "network_timeout_simulation",
			snapshotPath:      "/snapshots/home-20230101-120000",
			repository:        "b2-home",
			backupType:        "incremental",
			snapshotExists:    true,
			repoConfigExists:  true,
			repoConfigContent: "RESTIC_REPOSITORY: b2:bucket/path",
			resticExitCode:    3, // Common restic network error
			expectError:       true,
			errorContains:     "restic backup command failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockFS := NewMockFileSystem()
			mockBtrfs := NewMockBtrfsClient(t)
			mockRestic := NewMockResticClient(t)

			target := &config.TargetConfig{
				Repository: tt.repository,
				Prefix:     "test-backup",
				Type:       tt.backupType,
			}

			// Setup snapshot existence
			if tt.snapshotExists {
				mockFS.AddFile(tt.snapshotPath, []byte{})
			} else {
				mockFS.SetStatError(tt.snapshotPath, os.ErrNotExist)
			}

			// Setup repository config
			repoConfigPath := filepath.Join("/repos", tt.repository)
			if tt.repoConfigExists {
				mockFS.AddFile(repoConfigPath, []byte(tt.repoConfigContent))
			} else {
				mockFS.SetStatError(repoConfigPath, os.ErrNotExist)
			}

			// Setup restic mock
			if tt.snapshotExists && tt.repoConfigExists {
				tags := []string{"btrfs-backup", target.Prefix, filepath.Base(tt.snapshotPath)}
				force := tt.backupType == "full"
				mockRestic.ExpectBackup(tt.snapshotPath, tags, true, force, tt.resticExitCode)
			}

			mgr := NewManagerWithDeps(cfg, false, mockFS, mockBtrfs, mockRestic)
			err := mgr.PerformBackup(tt.snapshotPath, target)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error but got none")
				} else if tt.errorContains != "" && !strings.Contains(err.Error(), tt.errorContains) {
					t.Errorf("Expected error containing '%s', got '%s'", tt.errorContains, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error but got: %v", err)
				}
			}
		})
	}
}

func TestVerifyRepository(t *testing.T) {
	cfg := &config.Config{
		ResticRepoDir: "/repos",
		ResticBin:     "/usr/bin/restic",
	}

	tests := []struct {
		name              string
		repository        string
		repoConfigExists  bool
		repoConfigContent string
		resticExitCode    int
		expectError       bool
		errorContains     string
	}{
		{
			name:              "successful_verification",
			repository:        "b2-home",
			repoConfigExists:  true,
			repoConfigContent: "RESTIC_REPOSITORY: b2:bucket/path\nRESTC_PASSWORD: secret123",
			resticExitCode:    0,
			expectError:       false,
		},
		{
			name:             "repository_config_missing",
			repository:       "nonexistent-repo",
			repoConfigExists: false,
			expectError:      true,
			errorContains:    "repository configuration failed for verification",
		},
		{
			name:              "verification_finds_corruption",
			repository:        "b2-home",
			repoConfigExists:  true,
			repoConfigContent: "RESTIC_REPOSITORY: b2:bucket/path",
			resticExitCode:    1,
			expectError:       true,
			errorContains:     "repository verification failed",
		},
		{
			name:              "restic_check_command_not_found",
			repository:        "b2-home",
			repoConfigExists:  true,
			repoConfigContent: "RESTIC_REPOSITORY: b2:bucket/path",
			resticExitCode:    127,
			expectError:       true,
			errorContains:     "repository verification failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockFS := NewMockFileSystem()
			mockBtrfs := NewMockBtrfsClient(t)
			mockRestic := NewMockResticClient(t)

			// Setup repository config
			repoConfigPath := filepath.Join("/repos", tt.repository)
			if tt.repoConfigExists {
				mockFS.AddFile(repoConfigPath, []byte(tt.repoConfigContent))
			} else {
				mockFS.SetStatError(repoConfigPath, os.ErrNotExist)
			}

			// Setup restic check mock
			if tt.repoConfigExists {
				mockRestic.ExpectCheck("5%", tt.resticExitCode)
			}

			mgr := NewManagerWithDeps(cfg, false, mockFS, mockBtrfs, mockRestic)
			err := mgr.VerifyRepository(tt.repository)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error but got none")
				} else if tt.errorContains != "" && !strings.Contains(err.Error(), tt.errorContains) {
					t.Errorf("Expected error containing '%s', got '%s'", tt.errorContains, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error but got: %v", err)
				}
			}
		})
	}
}

func TestCleanupOldSnapshots(t *testing.T) {
	cfg := &config.Config{
		SnapshotDir: "/snapshots",
	}

	baseTime := time.Date(2023, 1, 1, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name              string
		prefix            string
		retention         int
		existingSnapshots []MockDirEntry
		deleteFailures    []string
		expectError       bool
		errorContains     string
		expectedDeletes   []string
	}{
		{
			name:      "successful_cleanup",
			prefix:    "backup",
			retention: 2,
			existingSnapshots: []MockDirEntry{
				{name: "backup-20230101-120000", modTime: baseTime.Add(0 * time.Hour)},
				{name: "backup-20230102-120000", modTime: baseTime.Add(-1 * time.Hour)},
				{name: "backup-20230103-120000", modTime: baseTime.Add(-2 * time.Hour)},
				{name: "backup-20230104-120000", modTime: baseTime.Add(-3 * time.Hour)},
			},
			expectedDeletes: []string{"backup-20230103-120000", "backup-20230104-120000"},
			expectError:     false,
		},
		{
			name:      "no_cleanup_needed",
			prefix:    "backup",
			retention: 3,
			existingSnapshots: []MockDirEntry{
				{name: "backup-20230101-120000", modTime: baseTime},
				{name: "backup-20230102-120000", modTime: baseTime.Add(-1 * time.Hour)},
			},
			expectedDeletes: []string{},
			expectError:     false,
		},
		{
			name:      "partial_cleanup_failure",
			prefix:    "backup",
			retention: 1,
			existingSnapshots: []MockDirEntry{
				{name: "backup-20230101-120000", modTime: baseTime},
				{name: "backup-20230102-120000", modTime: baseTime.Add(-1 * time.Hour)},
				{name: "backup-20230103-120000", modTime: baseTime.Add(-2 * time.Hour)},
			},
			deleteFailures:  []string{"backup-20230103-120000"},
			expectedDeletes: []string{"backup-20230102-120000", "backup-20230103-120000"},
			expectError:     true,
			errorContains:   "failed to delete some snapshots",
		},
		{
			name:      "zero_retention",
			prefix:    "backup",
			retention: 0,
			existingSnapshots: []MockDirEntry{
				{name: "backup-20230101-120000", modTime: baseTime},
			},
			expectedDeletes: []string{"backup-20230101-120000"},
			expectError:     false,
		},
		{
			name:      "filter_by_prefix",
			prefix:    "home",
			retention: 1,
			existingSnapshots: []MockDirEntry{
				{name: "home-20230101-120000", modTime: baseTime},
				{name: "other-20230101-120000", modTime: baseTime.Add(-1 * time.Hour)},
				{name: "home-20230102-120000", modTime: baseTime.Add(-2 * time.Hour)},
			},
			expectedDeletes: []string{"home-20230102-120000"},
			expectError:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockFS := NewMockFileSystem()
			mockBtrfs := NewMockBtrfsClient(t)
			mockRestic := NewMockResticClient(t)

			// Setup snapshots directory
			mockFS.AddDir("/snapshots", tt.existingSnapshots)

			// Setup delete btrfs mocks
			for _, snapshotName := range tt.expectedDeletes {
				exitCode := 0
				if slices.Contains(tt.deleteFailures, snapshotName) {
					exitCode = 1
				}
				snapshotPath := filepath.Join("/snapshots", snapshotName)
				mockBtrfs.ExpectDeleteSubvolume(snapshotPath, exitCode)

				// Mock post-delete check
				if exitCode == 0 {
					mockFS.SetStatError(snapshotPath, os.ErrNotExist)
				} else {
					mockFS.AddFile(snapshotPath, []byte{})
				}
			}

			mgr := NewManagerWithDeps(cfg, false, mockFS, mockBtrfs, mockRestic)
			err := mgr.CleanupOldSnapshots(tt.prefix, tt.retention)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error but got none")
				} else if tt.errorContains != "" && !strings.Contains(err.Error(), tt.errorContains) {
					t.Errorf("Expected error containing '%s', got '%s'", tt.errorContains, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error but got: %v", err)
				}
			}
		})
	}
}

func TestRunBackup(t *testing.T) {
	cfg := &config.Config{
		SnapshotDir:   "/snapshots",
		ResticRepoDir: "/repos",
		ResticBin:     "/usr/bin/restic",
	}

	t.Run("successful_workflow", func(t *testing.T) {
		mockFS := NewMockFileSystem()
		mockBtrfs := NewMockBtrfsClient(t)
		mockRestic := NewMockResticClient(t)

		target := &config.TargetConfig{
			Subvolume:     "/mnt/btrfs/home",
			Prefix:        "home-backup",
			Repository:    "b2-home",
			Type:          "incremental",
			Verify:        false,
			KeepSnapshots: 3,
		}

		// Setup successful workflow mocks
		mockFS.AddDir("/snapshots", []MockDirEntry{})
		mockBtrfs.ExpectShowSubvolume("/mnt/btrfs/home", 0)
		mockBtrfs.ExpectCreateSnapshot("", "", true, 0)
		mockBtrfs.onCreateSnapshot = func(subvolume, snapshotPath string) {
			mockFS.AddFile(snapshotPath, []byte{})
		}
		mockFS.AddFile("/repos/b2-home", []byte("RESTIC_REPOSITORY: b2:bucket/path"))
		mockRestic.ExpectBackup("", []string{}, true, false, 0)

		// Mock cleanup
		baseTime := time.Now()
		snapshots := []MockDirEntry{
			{name: "home-backup-old1", modTime: baseTime.Add(-24 * time.Hour)},
			{name: "home-backup-old2", modTime: baseTime.Add(-48 * time.Hour)},
			{name: "home-backup-old3", modTime: baseTime.Add(-72 * time.Hour)},
			{name: "home-backup-old4", modTime: baseTime.Add(-96 * time.Hour)},
		}
		mockFS.AddDir("/snapshots", snapshots)
		mockBtrfs.ExpectDeleteSubvolume("/snapshots/home-backup-old4", 0)
		mockFS.SetStatError("/snapshots/home-backup-old4", os.ErrNotExist)

		mgr := NewManagerWithDeps(cfg, false, mockFS, mockBtrfs, mockRestic)
		err := mgr.RunBackup("home", target)

		if err != nil {
			t.Errorf("Expected no error but got: %v", err)
		}
	})

	t.Run("validation_failure", func(t *testing.T) {
		mockFS := NewMockFileSystem()
		mockBtrfs := NewMockBtrfsClient(t)
		mockRestic := NewMockResticClient(t)

		target := &config.TargetConfig{
			Subvolume:     "/mnt/btrfs/home",
			Prefix:        "home-backup",
			Repository:    "b2-home",
			Type:          "incremental",
			Verify:        false,
			KeepSnapshots: 3,
		}

		mockFS.SetStatError("/snapshots", os.ErrNotExist)

		mgr := NewManagerWithDeps(cfg, false, mockFS, mockBtrfs, mockRestic)
		err := mgr.RunBackup("home", target)

		if err == nil {
			t.Error("Expected error but got none")
		}
		if !strings.Contains(err.Error(), "environment validation failed") {
			t.Errorf("Expected error containing 'environment validation failed', got '%s'", err.Error())
		}
	})
}

func TestLoadRepositoryEnv(t *testing.T) {
	// Create temporary directory and config file
	tmpDir, err := os.MkdirTemp("", "btrfs-backup-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

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
	defer func() { _ = os.RemoveAll(tmpDir) }()

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
