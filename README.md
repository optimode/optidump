# OptidDump

A flexible MySQL backup tool written in Go. Supports per-table and per-database backup modes, gzip compression, database/table filtering, structured logging, and email reporting.

## Features

- **Backup modes**: `file_per_table` (separate file per table) or `file_per_database` (single file per database)
- **Gzip compression** with automatic tar archiving
- **Flexible filtering**: `only` and `exclude` rules for databases and tables
- **Structured logging**: text or JSON format with automatic password masking
- **Email reporting**: backup statistics and error notifications via SMTP
- **Dry-run mode**: preview what would be backed up without executing

## Prerequisites

- MySQL server with `mysqldump` available
- SMTP server for email reports (optional)

## Installation

### Quick install (Linux / macOS)

```bash
curl -fsSL https://raw.githubusercontent.com/optimode/optidump/main/deployments/install.sh | bash
```

### Docker

The image includes `mysqldump` and `mariadb-dump` clients. Mount the configuration and backup destination:

```bash
docker run --rm \
  -v /path/to/config.yml:/etc/optidump/config.yml \
  -v /opt/backup:/opt/backup \
  ghcr.io/optimode/optidump --section production
```

Set `logging.file` to a path inside the backup volume (e.g., `/opt/backup/optidump.log`) for persistent audit logging. The log file will be preserved alongside the backups.

Also available on Docker Hub as `optimode/optidump`.

### Debian / Ubuntu

Download the `.deb` package from [Releases](https://github.com/optimode/optidump/releases):

```bash
sudo dpkg -i optidump_*.deb
```

### Arch Linux

Download the `.pkg.tar.zst` package from [Releases](https://github.com/optimode/optidump/releases):

```bash
sudo pacman -U optidump_*.pkg.tar.zst
```

### Build from source

```bash
make build
```

Binaries are placed in the `bin/` directory.

## Configuration

Create a YAML configuration file based on [`configs/config.example.yml`](configs/config.example.yml). The file supports multiple named sections for different backup scenarios.

```yaml
production:
  server:
    host: localhost
    port: 3306
    socket:                    # or Unix socket path
    user: backup_user
    password: secret

  backup:
    mode: file_per_table       # or file_per_database
    compression: gz            # gz or empty for no compression
    destination: /opt/backup
    command: /usr/bin/mysqldump # optional, default path
    options: --opt --routines --triggers --events --skip-lock-tables

  logging:
    file: /var/log/optidump.log
    level: info                # debug, info, error
    format: text               # text or json

  report:
    sender: backup@example.com
    recipient:
      - admin@example.com

  only:                        # optional: backup only these
    app_db:
    analytics_db:
      - events
      - sessions

  exclude:                     # optional: skip these
    information_schema:
    performance_schema:
```

Store configuration files with restricted permissions (`chmod 0600`).

## Usage

```bash
# Run backup
optidump --config /path/to/config.yml --section production

# Validate configuration
optidump --config /path/to/config.yml --check-config

# Dry-run: see what would be backed up
optidump --config /path/to/config.yml --section production --dry-run

# Override log level
optidump --config /path/to/config.yml --section production --level debug

# Show version
optidump --version
```

### Command-line options

| Flag | Description | Default |
|---|---|---|
| `-config` | Path to configuration file | `/etc/optidump/config.yml` |
| `-section` | Section name from config file | (required) |
| `-check-config` | Validate config and exit | |
| `-dry-run` | Preview mode, no actual backup | |
| `-level` | Override log level (debug, info, error) | |
| `-console` | Force console output | |
| `-version` | Show version information | |

### Systemd

The package includes a template service unit. Each instance runs a different config section:

```bash
# Run backup for "production" section
systemctl start optidump@production

# Enable on boot
systemctl enable optidump@production
```

## Output structure

Backups are organized in timestamped directories:

```
/opt/backup/
└── 2025-12-02-10-30/
    ├── database1/
    │   ├── table1.sql.tar.gz
    │   ├── table2.sql.tar.gz
    │   └── table3.sql.tar.gz
    └── database2/
        ├── users.sql.tar.gz
        └── orders.sql.tar.gz
```

## Logging

**Text format** (default):
```
[2025-12-02 10:30:15] INFO: Starting production backup
[2025-12-02 10:30:16] INFO: Backup: mysqldump --user=***** --password=***** ...
```

**JSON format**:
```json
{"timestamp":"2025-12-02 10:30:15","level":"INFO","message":"Starting production backup"}
```

Passwords are automatically masked in all log output.

## License

MIT
