# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

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
