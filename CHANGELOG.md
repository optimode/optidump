# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.1.0] - 2026-02-22

### Added
- MySQL backup with `mysqldump` supporting `file_per_table` and `file_per_database` modes
- Gzip compression with automatic tar archiving
- Flexible database and table filtering via `only` and `exclude` rules
- YAML configuration with multiple named sections
- Structured logging (text and JSON formats) with automatic password masking
- Email reporting with backup statistics via SMTP
- Dry-run mode for previewing backup operations
- Cross-platform builds: linux/darwin for amd64/arm64
- Security-hardened production binaries (stripped symbols, trimmed paths, no build ID)
- Build metadata injection (version, git commit, build time) via ldflags
- Unit tests for all internal packages (config, logger, database, backup, report)

[Unreleased]: https://github.com/optimode/optidump/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/optimode/optidump/releases/tag/v0.1.0
