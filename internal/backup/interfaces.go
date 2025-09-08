// Package backup provides interfaces for dependency injection in testing.
package backup

import (
	"os"

	"btrfs-backup/internal/btrfs"
	"btrfs-backup/internal/restic"
)

// FileSystem interface abstracts file system operations.
type FileSystem interface {
	Stat(name string) (os.FileInfo, error)
	ReadDir(name string) ([]os.DirEntry, error)
	ReadFile(filename string) ([]byte, error)
}

// BtrfsClient interface abstracts BTRFS operations.
type BtrfsClient = btrfs.Client

// ResticClient interface abstracts Restic operations.
type ResticClient = restic.Client

// Production implementations

type DefaultFileSystem struct{}

func (s *DefaultFileSystem) Stat(name string) (os.FileInfo, error) {
	return os.Stat(name)
}

func (s *DefaultFileSystem) ReadDir(name string) ([]os.DirEntry, error) {
	return os.ReadDir(name)
}

func (s *DefaultFileSystem) ReadFile(filename string) ([]byte, error) {
	return os.ReadFile(filename)
}
