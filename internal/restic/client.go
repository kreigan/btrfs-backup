// Package restic provides Restic backup operations for repository management.
package restic

import (
	"os/exec"
)

// Client interface abstracts Restic operations for dependency injection and testing.
type Client interface {
	Backup(repositoryEnv []string, snapshotPath string, tags []string, excludeCaches bool, force bool) error
	Check(repositoryEnv []string, readDataSubset string) error
}

// DefaultClient is the production implementation of the Client interface
// that executes actual Restic commands.
type DefaultClient struct {
	resticBin string
}

// NewDefaultClient creates a new DefaultClient instance with the specified Restic binary path.
func NewDefaultClient(resticBin string) *DefaultClient {
	return &DefaultClient{resticBin: resticBin}
}

// Backup creates a backup of the specified snapshot path to a Restic repository.
// It runs the restic backup command with the provided environment variables, tags, and options.
func (c *DefaultClient) Backup(repositoryEnv []string, snapshotPath string, tags []string, excludeCaches bool, force bool) error {
	args := []string{"backup", snapshotPath}
	for _, tag := range tags {
		args = append(args, "--tag", tag)
	}
	if excludeCaches {
		args = append(args, "--exclude-caches")
	}
	if force {
		args = append(args, "--force")
	}
	
	cmd := exec.Command(c.resticBin, args...)
	cmd.Env = repositoryEnv
	return cmd.Run()
}

// Check verifies the integrity of a Restic repository.
// It runs 'restic check' with optional data subset verification.
func (c *DefaultClient) Check(repositoryEnv []string, readDataSubset string) error {
	args := []string{"check"}
	if readDataSubset != "" {
		args = append(args, "--read-data-subset="+readDataSubset)
	}
	
	cmd := exec.Command(c.resticBin, args...)
	cmd.Env = repositoryEnv
	return cmd.Run()
}