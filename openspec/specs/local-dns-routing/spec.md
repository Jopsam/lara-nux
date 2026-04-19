# Local DNS Routing Specification

## Purpose

Define managed local domain resolution for development sites.

## Requirements

### Requirement: Managed *.test routing

The system MUST resolve managed `*.test` domains for registered local sites to the local environment and MUST NOT alter unrelated domains.

#### Scenario: Resolve registered site

- GIVEN a registered local site using a managed `*.test` hostname
- WHEN the developer resolves that hostname
- THEN resolution points to the local environment

### Requirement: DNS conflict safety

The system MUST detect conflicting resolver or DNS ownership before taking control and MUST provide remediation guidance instead of silently overwriting external configuration.

#### Scenario: Resolver conflict detected

- GIVEN an existing resolver configuration conflicts with managed local routing
- WHEN DNS setup is requested
- THEN the system refuses takeover and reports the conflict
