// Package btrfs provides BTRFS operations for snapshot management.
package btrfs

import (
	"os/exec"
)

// Client interface abstracts BTRFS operations for dependency injection and testing.
type Client interface {
	ShowSubvolume(subvolume string) error
	CreateSnapshot(subvolume, snapshotPath string, readonly bool) error
	DeleteSubvolume(subvolumePath string) error
}

type BtrfsCommand struct {
	Name string
	Args []string
	RunAsSudo bool
}

func (c *BtrfsCommand) Exec(args ...string) error {
	commandToRun := []string{}
	if c.RunAsSudo {
		commandToRun = append(commandToRun, "sudo")
	}
	commandToRun = append(commandToRun, c.Name)
	commandToRun = append(commandToRun, c.Args...)
	cmd := exec.Command(commandToRun[0], commandToRun[1:]...)
	return cmd.Run()
}

// DefaultClient is the production implementation of the Client interface
// that executes actual BTRFS commands using sudo.
type DefaultClient struct {
	btrfsBin string
	runAsSudo bool
}

func (c *DefaultClient) Exec(args ...string) error {
	command := &BtrfsCommand{
		Name: c.btrfsBin,
		Args: args,
		RunAsSudo: c.runAsSudo,
	}
	return command.Exec()
}

// NewDefaultClient creates a new DefaultClient instance.
func NewDefaultClient() *DefaultClient {
	return &DefaultClient {
		btrfsBin: "btrfs",
		runAsSudo: true,
	}
}

// ShowSubvolume verifies that the specified path is a valid BTRFS subvolume.
// It runs 'sudo btrfs subvolume show <subvolume>' and returns an error if the command fails.
func (c *DefaultClient) ShowSubvolume(subvolume string) error {
	return c.Exec([]string{"subvolume", "show", subvolume}...)
}

// CreateSnapshot creates a BTRFS snapshot of the specified subvolume.
// If readonly is true, the snapshot will be created as read-only using the -r flag.
// It runs 'sudo btrfs subvolume snapshot [-r] <subvolume> <snapshotPath>'.
func (c *DefaultClient) CreateSnapshot(subvolume, snapshotPath string, readonly bool) error {
	args := []string{"subvolume", "snapshot"}
	if readonly {
		args = append(args, "-r")
	}
	args = append(args, subvolume, snapshotPath)
	return c.Exec(args...)
}

// DeleteSubvolume removes a BTRFS subvolume or snapshot.
// It runs 'sudo btrfs subvolume delete <subvolumePath>'.
func (c *DefaultClient) DeleteSubvolume(subvolumePath string) error {
	return c.Exec([]string{"subvolume", "delete", subvolumePath}...)
}
