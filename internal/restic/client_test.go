package restic

import (
	"testing"
)

func TestNewDefaultClient(t *testing.T) {
	client := NewDefaultClient("/usr/bin/restic")
	if client == nil {
		t.Error("NewDefaultClient should return a non-nil client")
	}
	if client.resticBin != "/usr/bin/restic" {
		t.Errorf("Expected resticBin '/usr/bin/restic', got '%s'", client.resticBin)
	}
}

// Note: Integration tests for actual Restic operations would require a test environment
// with Restic repository setup and appropriate permissions. These tests focus on the interface
// and basic construction. Actual Restic command testing is done through the mock
// implementations in the backup package tests.

func TestDefaultClientImplementsInterface(t *testing.T) {
	var _ Client = (*DefaultClient)(nil)
}