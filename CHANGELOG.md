# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [v0.1.1] - 2026-01-15

### Added

- Add CONTRIBUTING.md ([#12](https://github.com/CoHDI/dynamic-device-scaler/pull/12))

### Fixed

- Fix correct missing handling for ResourceSlice with zero devices ([#13](https://github.com/CoHDI/dynamic-device-scaler/pull/13))

## [v0.1.0] - 2025-09-19

### Added

- Initial release of Dynamic Device Scaler
- Operator framework and core controller implementation
- ResourceMonitor controller for monitoring ResourceSlice and ResourceClaim
- Composable DRA Driver for publishing free devices from resource pool to Kubernetes
- Dynamic device attach/detach support via Composable Resource Operator CRs
- RescheduleNotification feature for notifying scheduler after device attachment
- Environment variable processing for configuration
- Unit tests for core components
- Dockerfile and Makefile for building and deployment
- Kubernetes deployment manifests
- LICENSE (Apache-2.0)
- README with architecture documentation

### Fixed

- Fix RBAC configuration
- Fix GetConfiguredDeviceCount function
- Fix RescheduleFailedNotification handling
- Fix DynamicDetach logic
- Fix updateComposableResourceLastUsedTime
- Fix handleDevices processing
- Fix updateNodeLabel
- Fix deploy configuration

[v0.1.1]: https://github.com/CoHDI/dynamic-device-scaler/compare/v0.1.0...v0.1.1
[v0.1.0]: https://github.com/CoHDI/dynamic-device-scaler/releases/tag/v0.1.0
