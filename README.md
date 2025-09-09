# BTRFS Backup

A Go implementation of BTRFS backup script that creates snapshots and backs them up using Restic.

## Requirements

- **Linux only** - This tool works exclusively on Linux systems with BTRFS support
- **BTRFS filesystem** - Source directories must be on BTRFS filesystems  
- **Restic** - Must be installed and accessible (usually `/usr/bin/restic`)
- **Root/sudo access** - Required for creating BTRFS snapshots

## Features

- Creates read-only BTRFS snapshots
- Backs up snapshots using Restic to various backends (B2, S3, etc.)
- Configurable retention policies
- Optional repository verification
- Support for both JSON and YAML configuration files
- Comprehensive logging and error handling
- Built with modern Go libraries (Cobra for CLI, Viper for configuration)

## Installation

### Pre-built Binaries (Recommended)

Download the latest release from [GitHub Releases](https://github.com/kreigan/btrfs-backup/releases):

```bash
# Linux x86_64
wget https://github.com/kreigan/btrfs-backup/releases/latest/download/btrfs-backup-linux-amd64
chmod +x btrfs-backup-linux-amd64
sudo mv btrfs-backup-linux-amd64 /usr/local/bin/btrfs-backup

# Linux ARM64
wget https://github.com/kreigan/btrfs-backup/releases/latest/download/btrfs-backup-linux-arm64  
chmod +x btrfs-backup-linux-arm64
sudo mv btrfs-backup-linux-arm64 /usr/local/bin/btrfs-backup
```

### Build from Source

```bash
git clone https://github.com/kreigan/btrfs-backup.git
cd btrfs-backup
make build
```

### Using Go

```bash
go install github.com/kreigan/btrfs-backup/cmd/btrfs-backup@latest
```

## Usage

### Commands

- `btrfs-backup version` - Show version information
- `btrfs-backup backup <target>` - Perform backup operation

### Global Options

- `-c, --config` - Config file path (default: `$HOME/.config/btrfs-backup/config.yaml`)
- `-v, --verbose` - Enable debug logging
- Environment variable: `BTRFSBACKUP_CONFIG`

### Backup Command Options

- `-t, --target-config` - Path to target configuration file (default: `$HOME/.config/btrfs-backup/targets/<target>`)

## Configuration

### Main Configuration File

Default location: `$HOME/.config/btrfs-backup/config.yaml`

```yaml
target_dir: /home/user/.config/btrfs-backup/targets
snapshot_dir: /mnt/btrfs/snapshots
restic_repo_dir: /home/user/.config/btrfs-backup/repos
restic_bin: /usr/bin/restic
```

Or in JSON format:

```json
{
  "target_dir": "/home/user/.config/btrfs-backup/targets",
  "snapshot_dir": "/mnt/btrfs/snapshots", 
  "restic_repo_dir": "/home/user/.config/btrfs-backup/repos",
  "restic_bin": "/usr/bin/restic"
}
```

### Target Configuration Files

Default location: `$HOME/.config/btrfs-backup/targets/<target-name>`

```yaml
subvolume: /mnt/btrfs/home
prefix: home-backup
repository: b2-home
type: incremental  # or "full"
verify: true       # or false
keep_snapshots: 3
```

Or in JSON format:

```json
{
  "subvolume": "/mnt/btrfs/home",
  "prefix": "home-backup", 
  "repository": "b2-home",
  "type": "incremental",
  "verify": true,
  "keep_snapshots": 3
}
```

### Repository Configuration Files

Location: `<restic_repo_dir>/<repository-name>`

```yaml
RESTIC_REPOSITORY: b2:my-bucket/home-backup
RESTIC_PASSWORD: my-secure-password
B2_ACCOUNT_ID: my-account-id
B2_ACCOUNT_KEY: my-account-key
```

## Examples

```bash
# Show version
btrfs-backup version

# Backup with default configuration
btrfs-backup backup my-target

# Backup with verbose logging
btrfs-backup backup my-target -v

# Backup with custom config file
btrfs-backup backup my-target -c /path/to/config.yaml

# Backup with custom target config
btrfs-backup backup my-target -t /path/to/target.yaml
```

## Backup Process

1. Validates environment (snapshot directory, BTRFS subvolume)
2. Creates read-only BTRFS snapshot with timestamp
3. Performs Restic backup of the snapshot
4. Optionally verifies repository integrity
5. Cleans up old snapshots based on retention policy
6. Reports success or failure with appropriate exit codes

## Error Handling

- Most failures in the backup process will cause the program to stop and exit with code 1
- Verification failures are logged as warnings but don't fail the backup
- Snapshot cleanup failures are logged as warnings but don't fail the backup
- Failed snapshots are kept for investigation when backup operations fail

## Development

### Prerequisites

- Go 1.25.1 or later
- [golangci-lint](https://golangci-lint.run/) for linting
- [GoReleaser](https://goreleaser.com/) for releases (optional)

### Building

```bash
# Build the binary
make build

# Build and run
make run

# Install to $GOPATH/bin
make install
```

### Testing and Linting

```bash
# Run linting (includes go vet, gofmt, and golangci-lint)
make lint

# Run tests (automatically runs linting first)
make test

# Run tests only
go test -v ./...

# Clean build artifacts
make clean
```

The test suite includes:
- Configuration parsing (YAML and JSON)
- Validation logic
- Command building
- Environment variable handling
- Snapshot listing and sorting
- Backup workflow simulation with mocked dependencies

### Code Quality

This project uses:
- **golangci-lint** for comprehensive static analysis
- **gofmt** for code formatting
- **go vet** for additional checks
- Conventional commits for automated releases

## CI/CD and Releases

This project uses automated releases via GitHub Actions:

- **Automated Releases**: Pushes to `main`/`master` branch trigger automatic version bumps and releases based on conventional commit messages
- **Semantic Versioning**: Version numbers follow semantic versioning (major.minor.patch)
- **Cross-platform Builds**: Releases include Linux binaries for both AMD64 and ARM64 architectures
- **Changelog Generation**: Release notes are automatically generated from commit messages

### Commit Message Format

Follow [Conventional Commits](https://www.conventionalcommits.org/) specification:

```bash
# Patch release (x.x.1)
fix: correct backup verification logic

# Minor release (x.1.0) 
feat: add new retry mechanism for failed backups

# Major release (1.0.0)
feat!: change configuration file format

# or with BREAKING CHANGE footer
feat: add new configuration options

BREAKING CHANGE: configuration file structure has changed
```

### Release Process

1. Push commits to `main` branch using conventional commit messages
2. GitHub Actions automatically:
   - Runs linting and tests
   - Determines next version number
   - Creates GitHub release with changelog
   - Builds and attaches Linux binaries (AMD64/ARM64)
   - Generates checksums for verification