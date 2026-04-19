# Environment Bootstrap Specification

## Purpose

Define safe Ubuntu-first installation, privilege boundaries, and rollback behavior.

## Requirements

### Requirement: Supported Ubuntu bootstrap

The system MUST validate Ubuntu support, required dependencies, and required privileges before changing machine state, and MUST stop with remediation guidance when validation fails.

#### Scenario: Supported bootstrap

- GIVEN a supported Ubuntu host
- WHEN installation starts
- THEN the system validates prerequisites before applying managed changes

#### Scenario: Unsupported host

- GIVEN an unsupported Ubuntu release or missing prerequisite
- WHEN bootstrap is requested
- THEN the system MUST refuse installation and report why

### Requirement: Privilege boundary and rollback safety

The system MUST restrict privileged actions to installation and daemon-owned operations, MUST keep the desktop client unprivileged for normal use, and MUST remove only managed assets during uninstall or failed-install rollback while preserving user project files and pre-existing resolver settings.

#### Scenario: Safe uninstall

- GIVEN a completed installation
- WHEN the user uninstalls or rollback is triggered
- THEN the system removes managed services, DNS, and web-serving assets without touching user code
