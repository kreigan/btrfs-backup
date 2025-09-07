package cli

import (
	"testing"
)

func TestGetVersion(t *testing.T) {
	if version == "" {
		t.Error("Version should not be empty")
	}
	if version != "0.1.0" {
		t.Errorf("Expected version '0.1.0', got '%s'", version)
	}
}