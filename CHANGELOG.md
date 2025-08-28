# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [1.3.0] - 2025-08-28

### Added

- Added a `--limit` flag to the `run` and `results` commands to control the maximum number of results returned.
- Added a `limit` field to the `config.json` file to allow setting a default result limit.

### Changed

- The default behavior for result fetching is now to return all results (`limit=0`) unless specified otherwise by the `--limit` flag or in the config file.

### Fixed

- Fixed a display issue where the "Waiting for job to complete..." message was not printed on a new line.

## [1.2.1] - 2025-08-18

### Fixed

- Fixed an issue where the version information was not correctly embedded in the binary during the `make` build process. The build script now correctly links the Git tag, commit hash, and build date.

## [1.2.0] - 2025-08-14

### Changed

- **Major Refactoring**: The entire codebase has been refactored for better modularity, testability, and maintainability.
  - Core Splunk API interaction logic has been extracted into a new `splunk` package.
  - Command-line interface (CLI) logic has been separated into a new `cmd` package, with each command in its own file.
  - The main application entrypoint (`splunk-cli.go`) is now significantly simplified.

## [1.1.0] - 2025-08-12

### Added

- Added a global `--config` flag to specify a custom configuration file path, overriding the default `~/.config/splunk-cli/config.json`.

## [1.0.0] - 2025-08-05

### Added

- **Initial Release** of `splunk-cli`.
- Core functionalities: `run`, `start`, `status`, `results` commands to interact with Splunk's REST API.
- Flexible authentication via config file, environment variables, or command-line flags.
- Support for reading SPL queries from files or standard input.
- Asynchronous job handling with job cancellation support (`Ctrl+C`).
- App context support for searches (`--app` flag).
- Makefile for simplified building, testing, linting, and vulnerability scanning.
- Cross-platform build support for macOS (Universal), Linux (amd64), and Windows (amd64).
- Version information embedded in the binary (`--version` flag).
- `README.md` and `LICENSE` (MIT) for project documentation.
- `CHANGELOG.md` to track project changes.
- Japanese README (`README.ja.md`).

### Changed

- Switched build system from a shell script (`build.sh`) to a `Makefile`.

### Fixed

- N/A (Initial Release)