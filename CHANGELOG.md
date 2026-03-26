# Changelog

All notable changes to this project will be documented in this file.

## [1.0.1] - 2026-03-26

### Added
- MAC address exposed via `/info` endpoint and displayed in web UI
- Clickable API endpoint paths in web UI
- Version displayed in web UI header

### Changed
- Web UI layout: grid-based header, wider container (720px), fixed-height log area
- Updated screenshot in README

## [1.0.0] - 2026-03-26

### Added
- Version constant exposed via `/info` endpoint
- Web UI screenshot in README

### Changed
- First stable release

## [0.2.0] - 2026-03-25

### Added
- Pure Go ICMP ping implementation (`ping.go`), replacing external `ping` command
- TCP fallback for environments without raw socket privileges
- Comprehensive test suite (`main_test.go`, `ping_test.go`)
- English doc-comments across all source files
- `/info` endpoint listed in web UI
- `README.md` with usage, configuration, and API documentation
- `CHANGELOG.md`
- MIT License

### Changed
- Config and index.html search order: CWD first, then executable directory

## [0.1.0] - 2026-03-25

### Added
- Initial release
- Wake-on-LAN (`/on`) via magic packet
- Shutdown (`/off`) via Synology DSM API
- Status check (`/state`) via external ping command
- Web UI (`index.html`) with live status and action buttons
- `/info` endpoint returning NAS IP
- YAML configuration support (`config.yaml`)
- Private-network-only access restriction
