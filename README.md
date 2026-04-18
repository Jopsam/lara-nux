# Lara Nux

Ubuntu-first local Laravel environment for developers who want a fast native workflow without Docker overhead.

## What this project is

Lara Nux is building a Linux-native local development experience for Laravel on Ubuntu, with a clear split between:

- a **privileged Go daemon** that owns machine-level operations like service control, DNS, package-aware runtime setup, and local web serving
- an **unprivileged desktop client** that provides the operator experience through Wails + Nuxt

The goal is simple: make local Laravel setup on Ubuntu predictable, maintainable, and OSS-friendly.

## Current status

Lara Nux is currently an **OSS beta** with the core v1 slices in place:

- daemon RPCs, Ubuntu host adapters, and runtime/site orchestration exist
- Ubuntu packaging assets and maintainer scripts exist
- Go unit tests plus Ubuntu workflow scaffold coverage exist
- the desktop shell/UI is present, but still not a polished production release

## Supported Ubuntu matrix

| Ubuntu release | Support status | Notes |
| --- | --- | --- |
| 22.04 LTS (Jammy) | Supported beta | Covered by package metadata and Ubuntu workflow fixture coverage |
| 24.04 LTS (Noble) | Supported beta | Covered by package metadata and Ubuntu workflow fixture coverage |
| Other Ubuntu releases | Unsupported | Bugs may be closed unless they reproduce on the supported LTS matrix |
| Non-Ubuntu distros | Out of scope for v1 | Do not assume compatibility |

## Scope for v1

In scope:

- Ubuntu-first local Laravel app serving
- PHP runtime registration and switching
- Local DNS routing for managed `.test` domains
- Service orchestration and health reporting
- Safe install / upgrade / uninstall packaging flows

Out of scope for v1:

- macOS / Windows parity
- cloud deployment workflows
- team sync / remote environments
- generic infrastructure orchestration beyond the Laravel local-dev use case

## Privileges and trust model

- The daemon is the **only** layer allowed to mutate privileged machine state.
- The desktop client never shells out with `sudo`; privileged work crosses the Unix socket boundary.
- The packaged socket contract is `root:lara-nux` with mode `0660`.
- The current packaged daemon unit still runs as `root` while privilege separation inside the daemon is being hardened. That is intentional for now and SHOULD be treated as beta-era behavior, not a finished least-privilege story.

## Install, rollback, and uninstall expectations

- First install creates the `lara-nux` group, the `lara-nuxd` system user, runtime/config/state directories, and the `lara-nuxd.service` unit wiring.
- Failed install/upgrade paths are expected to recover service startup where possible and keep previously managed machine state consistent.
- Uninstall/purge removes only Lara Nux-managed resolver, Caddy, PHP-FPM, socket, and packaging assets.
- User Laravel projects are **not** removed as part of uninstall or rollback.

See [docs/ubuntu.md](./docs/ubuntu.md) for the operational details, troubleshooting commands, and reproducible Ubuntu bug-report checklist.

## CI / release honesty

- `.github/workflows/test.yml` runs daemon and Ubuntu workflow scaffold Go tests.
- `.github/workflows/package.yml` validates Debian metadata by assembling a `.deb` from **placeholder staged artifacts** in CI. That proves packaging wiring, not shipping-quality binaries.
- `.github/workflows/release.yml` is a **signing-readiness** workflow. It intentionally fails until maintainers replace placeholder signing metadata and configure release GPG secrets.

## Repository layout

- `daemon/` — privileged Go daemon and Ubuntu host adapters
- `client/` — Wails + Nuxt desktop client
- `shared/` — shared RPC contracts between backend and UI
- `packaging/` — packaging and install assets
- `testing/` — Ubuntu workflow scaffold coverage and future broader test harnesses
- `openspec/` — proposal, design, specs, tasks, and verification artifacts

## Contributing

Please read [CONTRIBUTING.md](./CONTRIBUTING.md) before opening issues or pull requests.

## Project policies

- Security reporting: see [SECURITY.md](./SECURITY.md)
- Change history and release notes policy: see [CHANGELOG.md](./CHANGELOG.md)

## License

Licensed under Apache-2.0. See [LICENSE](./LICENSE).
