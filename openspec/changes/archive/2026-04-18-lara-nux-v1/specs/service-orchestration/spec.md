# Service Orchestration Specification

## Purpose

Define lifecycle control and operator-visible health for managed local services.

## Requirements

### Requirement: Managed service lifecycle

The system MUST let operators start, stop, restart, and inspect required local services through the daemon boundary, and MUST expose current status by service and affected site.

#### Scenario: Start required services

- GIVEN a registered site with required managed services
- WHEN the operator starts the environment
- THEN the system reports each service state and site readiness

### Requirement: Health and conflict reporting

The system MUST detect occupied ports, failed processes, missing privileges, and degraded service health, and MUST report actionable remediation tied to the affected capability.

#### Scenario: Port conflict blocks readiness

- GIVEN a required port is already occupied
- WHEN the environment is started or checked
- THEN the system marks readiness as blocked and reports the conflicting resource
