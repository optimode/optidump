# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

OptidDump is a Go-based MySQL backup tool that creates automated backups using `mysqldump`. It supports per-table and per-database backup modes, gzip/bzip2 compression, database/table filtering, structured logging, and email reporting.

## Build & Development Commands

```bash
# Development (native platform, debug symbols kept)
make build                # Dev build for local development

# Production (native platform, hardened: stripped, no build ID)
make build-prod           # Production build for current platform

# Cross-compilation (production, hardened)
make build-linux-amd64    # Linux x86_64 (servers, Docker)
make build-linux-arm64    # Linux ARM64 (AWS Graviton, Docker multiplatform)
make build-darwin-amd64   # macOS Intel
make build-darwin-arm64   # macOS Apple Silicon (M2)
make build-all            # All 4 platforms

# Quality & Security
make test                 # Run all tests: go test -v ./...
make fmt                  # Format code: go fmt ./...
make vet                  # Lint code: go vet ./...
make check                # Run fmt + vet + test sequentially
make security             # Run govulncheck for known vulnerabilities
make clean                # Remove build artifacts

# Run tests for a single package
go test -v optidump/internal/config
go test -v optidump/internal/backup -run TestCompress
```

Build metadata (`version`, `gitCommit`, `buildTime`) is injected via `-ldflags -X`. Version is derived from `git describe --tags --always --dirty`.

All build artifacts go to the `bin/` directory (git-ignored). `make clean` removes it entirely.

## Running

```bash
bin/optidump -config configs/config.example.yml -section <section_name>
bin/optidump -config configs/config.example.yml -section <section_name> -dry-run
bin/optidump -config configs/config.example.yml -section <section_name> -check-config
```

Key CLI flags: `-config` (path to YAML config), `-section` (required, backup profile name), `-dry-run`, `-check-config`, `-level` (debug/info/error), `-console`, `-version`.

## Architecture

Entry point is `cmd/optidump/main.go`. All internal packages live under `internal/`:

- **`internal/config`** — YAML config parsing and validation. Config files are multi-section; each section defines server, backup, logging, report, and filtering (only/exclude) settings.
- **`internal/database`** — MySQL connection management (TCP and Unix socket), database/table discovery, `mysqldump` command generation. Uses `github.com/go-sql-driver/mysql`.
- **`internal/backup`** — Core backup execution. Applies include/exclude filters, handles both `file_per_table` and `file_per_database` modes, creates timestamped output directories, handles gzip compression via `archive/tar` + `compress/gzip`.
- **`internal/logger`** — Structured logging built on `log/slog` with custom handlers for console, text file, and JSON output formats. Includes automatic password masking in log output.
- **`internal/report`** — SMTP email reporting (localhost:25) with backup statistics.

## Configuration

YAML format with named sections. See `configs/config.example.yml` for the full template. Each section contains: `server` (MySQL connection), `backup` (mode/compression/destination), `logging` (file/level/format), `report` (SMTP sender/recipients), and optional `only`/`exclude` maps for database/table filtering.

## Testing

Each internal package has a `_test.go` file with table-driven tests using `t.Run()` subtests. Tests cover the pure logic layer without requiring external services (MySQL, SMTP).

- **config**: `Load()` (YAML parsing, multiple sections, only/exclude), `Validate()` (every field and boundary), `contains()` helper
- **logger**: `parseLevel`, `ConsoleHandler`/`TextFileHandler` (Enabled, Handle, format output), `New()` (console/file/JSON), level filtering, `Close()`
- **database**: `New()`, `SetCommand`/`SetOptions`, `GetConnectionString` (TCP vs socket), `GetTableDumpCommand`/`GetDatabaseDumpCommand`, `HasDatabases`/`GetDatabases`/`GetTables`
- **backup**: `encryptDumpCommand` (password masking), `removeTable`, `compress` (gz round-trip with tar verification, bz2/unsupported error), `makeBackupCommands` (both modes), `doCompression` edge cases
- **report**: `makeMessage` (all scenarios: basic, error, failed save, compression, empty), `buildEmailMessage` (single/multiple recipients, headers), `Send` (nil/empty recipients early return)

## Dependencies

Go 1.25.3. Two direct dependencies: `github.com/go-sql-driver/mysql` and `gopkg.in/yaml.v3`.

## Git Workflow & Versioning

### Branching: GitHub Flow

Single production branch `main` (always production-ready). No develop branch.

- **`main`** — Protected. All code merges via pull request. Direct pushes blocked.
- **Feature branches** — Short-lived, branched from `main`, merged back via PR.
  - Naming: `<type>/<short-description>` (e.g., `feat/table-filtering`, `fix/socket-timeout`)

### Commits: Conventional Commits

```
<type>(<scope>): <description>

[optional body]
```

**No footers.** Commits must not contain footers, signatures, co-authored-by, references to people, software, or anything else after the body.

**Types**: `feat`, `fix`, `docs`, `chore`, `refactor`, `test`, `ci`, `perf`, `build`

**Scopes** (optional): `config`, `backup`, `database`, `logger`, `report`, `ci`, `build`

**Examples**:
```
feat(backup): add bzip2 compression support
fix(database): handle socket connection timeout
docs: update README with new CLI flags
chore: upgrade go-sql-driver/mysql to v1.10.0
```

**Breaking changes**: Add `!` after type:
```
feat(config)!: change YAML structure for filter rules
```

### Versioning: Semantic Versioning 2.0.0

Format: `vMAJOR.MINOR.PATCH` (e.g., `v1.2.3`)

- **MAJOR** — Breaking changes (incompatible config format, removed flags)
- **MINOR** — New features (new backup mode, new CLI flag)
- **PATCH** — Bug fixes, security patches

Pre-release: `v1.2.3-rc.1`, `v1.2.3-rc.2`, etc.

### Release Process

1. Feature branch from `main`, develop with conventional commits
2. Open PR → CI runs automatically (`make check`, `make security`, `make build-all`)
3. Review, approve, merge
4. Tag RC: `git tag v0.2.0-rc.1 && git push origin v0.2.0-rc.1`
5. Release workflow builds binaries, creates GitHub pre-release
6. Test the RC
7. Tag stable: `git tag v0.2.0 && git push origin v0.2.0`
8. Release workflow creates GitHub release with production binaries + checksums
9. Update `CHANGELOG.md`: move `[Unreleased]` items to new version section
10. Deploy manually from GitHub release artifacts

### CI/CD

- **CI** (`.github/workflows/ci.yml`): Runs on PRs to `main` — fmt, vet, test, govulncheck, build-all
- **Release** (`.github/workflows/release.yml`): Runs on `v*` tags — builds all platforms, SHA-256 checksums, .deb/.pkg.tar.zst packages (nfpm), Docker multiplatform image (ghcr.io + Docker Hub), creates GitHub Release (RC = pre-release)

## Deployment

Deployment files live in `deployments/`:

- **`systemd/optidump@.service`** — Template unit. `optidump@section_name.service` runs `--section section_name`. `Type=oneshot`, security-hardened.
- **`docker/Dockerfile`** — Alpine-based with `mysql-client` and `mariadb-client`. COPY-only (no multi-stage build). Uses `TARGETARCH` for multiplatform builds. Config mounted at `/etc/optidump/`.
- **`nfpm.yml`** — Package config for nfpm. Generates `.deb` (Debian/Ubuntu) and `.pkg.tar.zst` (Arch Linux). Includes binary, example config, systemd unit. Arch and version passed via env vars: `VERSION=x.y.z ARCH=amd64 nfpm package --config deployments/nfpm.yml --packager deb`
- **`install.sh`** — One-liner installer. Detects OS/arch, downloads from GitHub Releases, verifies SHA-256 checksum, installs to `/usr/local/bin/`.
