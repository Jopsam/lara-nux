# Proposal: lara-nux v1

## Intent

Create an Ubuntu-first local Laravel environment that avoids Docker overhead and brittle manual setup, while staying OSS-maintainable and security-conscious.

## Why This Is Worth Building

- Laravel developers on Ubuntu lack a polished local-first experience comparable to Herd/Laragon.
- Faster onboarding and lower machine overhead improve day-to-day product delivery.
- An OSS-grade Linux-native tool creates a durable foundation for community contribution and future distro expansion.

## Personas

- **Laravel app developer**: needs zero-friction local setup and predictable project serving.
- **OSS contributor/maintainer**: needs clear boundaries, low operational surprise, and maintainable architecture.

## Scope

### In Scope
- Ubuntu-focused v1 for local Laravel app serving, PHP version switching, local DNS, and service lifecycle control.
- Product framing for a privileged system runtime plus unprivileged desktop client.
- Release/rollback direction that supports safe OSS iteration.

### Out of Scope
- Non-Ubuntu distributions, macOS, and Windows parity.
- Cloud deployment, team sync, remote environments, or advanced database orchestration.
- Deep implementation choices beyond proposal-level direction.

## Product Scope

v1 is a local developer tool for Ubuntu that helps a single developer install, run, switch, and troubleshoot Laravel-ready services on one machine.

## Capabilities

### New Capabilities
- `environment-bootstrap`: install and initialize the local development environment safely on Ubuntu.
- `php-runtime-management`: acquire, register, and switch supported PHP runtimes for projects.
- `local-site-serving`: serve Laravel projects over local HTTP/HTTPS with predictable routing.
- `local-dns-routing`: resolve `*.test` domains to the local environment.
- `service-orchestration`: manage required local services and report conflicts or health.

### Modified Capabilities
- None.

## Approach

Adopt a daemon + client product shape. A privileged Ubuntu system daemon owns sensitive operations (DNS, ports, service control, runtime registration). An unprivileged Nuxt-based desktop/tray client manages UX and communicates through a local IPC boundary. Prioritize Ubuntu stability first, then expand only after the architecture proves maintainable.

## Release Strategy

Start with an explicit v1 support matrix for selected Ubuntu LTS versions and a small core feature set. Treat the first public milestone as a controlled OSS beta, gather compatibility feedback, harden packaging/upgrade/uninstall flows, then move to a stable 1.0 only after install/rollback paths are proven.

## Affected Areas

| Area | Impact | Description |
|------|--------|-------------|
| `openspec/changes/lara-nux-v1/` | New | Proposal baseline for downstream specs/design |
| `future system daemon package` | New | Privileged runtime/service orchestration boundary |
| `future desktop client package` | New | User-facing management UI and status surfaces |
| `future packaging/release assets` | New | Ubuntu-first installation and update path |

## Risks

| Risk | Likelihood | Mitigation |
|------|------------|------------|
| Privileged operations degrade trust/UX | Med | Minimize root surface, isolate daemon, document permissions |
| Port/DNS conflicts on developer machines | High | Detect early, surface remediation, preserve rollback path |
| Ubuntu version packaging drift | Med | Narrow v1 support matrix and publish compatibility policy |

## Rollback Plan

Ship v1 behind explicit install/uninstall flows. If rollout fails, disable the daemon, remove installed service/DNS/web-server assets, restore prior host resolver settings where changed, and keep user project files untouched.

## Dependencies

- Ubuntu service/DNS primitives (`systemd`, resolver integration)
- Local web serving and TLS strategy
- Maintainable OSS packaging/distribution workflow

## Success Criteria

- [ ] Proposal clearly defines v1 product boundaries, personas, and architecture direction for specs/design.
- [ ] v1 is framed around Ubuntu-first local Laravel workflows, not generic environment management.
- [ ] Rollout and rollback expectations are explicit enough to guide design decisions.
