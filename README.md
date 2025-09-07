# BTRFS Backup

A Go implementation of BTRFS backup script that creates snapshots and backs them up using Restic.

## Features

- Creates read-only BTRFS snapshots
- Backs up snapshots using Restic to various backends (B2, S3, etc.)
- Configurable retention policies
- Optional repository verification
- Support for both JSON and YAML configuration files
- Comprehensive logging and error handling
- No external dependencies (uses Go standard library only)

## Installation

```bash
go build -o btrfs-backup
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

## Testing

```bash
go test -v
```

The test suite includes:
- Configuration parsing (YAML and JSON)
- Validation logic
- Command building
- Environment variable handling
- Snapshot listing and sorting