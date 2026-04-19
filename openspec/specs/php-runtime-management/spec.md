# PHP Runtime Management Specification

## Purpose

Define supported PHP acquisition, registration, and per-project switching.

## Requirements

### Requirement: Supported PHP lifecycle

The system MUST acquire and register only supported PHP runtimes, MUST record runtime identity for management, and MUST reject unsupported or unverifiable runtimes with guidance.

#### Scenario: Register supported runtime

- GIVEN a supported PHP runtime source
- WHEN the user adds that runtime
- THEN the system registers it for later project assignment

#### Scenario: Reject unsupported runtime

- GIVEN an unsupported or unverifiable runtime
- WHEN registration is requested
- THEN the system refuses activation and reports the reason

### Requirement: Project runtime switching

The system MUST allow a project to select an available PHP runtime and MUST show the active runtime without modifying project source files.

#### Scenario: Switch project runtime

- GIVEN a served project and multiple registered runtimes
- WHEN the user selects another supported runtime
- THEN subsequent requests use the selected runtime
