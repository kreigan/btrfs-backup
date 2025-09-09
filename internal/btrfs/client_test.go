package btrfs

import (
	"testing"
)

func TestNewDefaultClient(t *testing.T) {
	client := NewDefaultClient()
	if client == nil {
		t.Error("NewDefaultClient should return a non-nil client")
	}
}

// Note: Integration tests for actual BTRFS operations would require a test environment
// with BTRFS filesystem and appropriate permissions. These tests focus on the interface
// and basic construction. Actual BTRFS command testing is done through the mock
// implementations in the backup package tests.

func TestDefaultClientImplementsInterface(t *testing.T) {
	var _ Client = (*DefaultClient)(nil)
}
