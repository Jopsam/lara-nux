# Local Site Serving Specification

## Purpose

Define predictable local Laravel project registration and serving behavior.

## Requirements

### Requirement: Predictable local serving

The system MUST allow a Laravel project to be registered to a stable local hostname and MUST serve that project over local HTTP and HTTPS once activation succeeds.

#### Scenario: Serve a Laravel project

- GIVEN a valid local Laravel project path
- WHEN the user activates the site
- THEN the project becomes reachable at its assigned local hostname

### Requirement: Safe activation failure

The system MUST refuse activation when the path is invalid, not a Laravel project, or conflicts with an existing site name, and MUST report the blocking reason.

#### Scenario: Reject invalid project

- GIVEN a non-Laravel path or duplicate hostname
- WHEN activation is requested
- THEN the system leaves existing sites unchanged and reports the conflict
