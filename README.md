# Lara Nux

Ubuntu-first local Laravel environment for developers who want a fast native workflow without Docker overhead.

## What this project is

Lara Nux is building a Linux-native local development experience for Laravel on Ubuntu, with a clear split between:

- a **privileged Go daemon** that owns machine-level operations like service control, DNS, and local web serving
- an **unprivileged desktop client** that will provide the operator experience through Wails + Nuxt

The goal is simple: make local Laravel setup on Ubuntu predictable, maintainable, and OSS-friendly.

## Current status

This repository is in early foundation stage.

- Core backend architecture and RPC contracts are in place
- Ubuntu-first daemon boundaries are defined
- OpenSpec planning artifacts are versioned in this repo
- The desktop UI shell is still scaffold-level and will be expanded in the next phase

## Scope for v1

In scope:

- Ubuntu-first local Laravel app serving
- PHP runtime registration and switching
- Local DNS routing for development domains
- Service orchestration and health reporting

Out of scope for v1:

- macOS / Windows parity
- cloud deployment workflows
- team sync / remote environments
- generic infrastructure orchestration beyond the Laravel local-dev use case

## Repository layout

- `daemon/` — privileged Go daemon and Ubuntu host adapters
- `client/` — future Wails + Nuxt desktop client
- `shared/` — shared RPC contracts between backend and UI
- `packaging/` — packaging and install assets
- `openspec/` — proposal, design, specs, tasks, and verification artifacts

## Development notes

- The daemon is the only layer allowed to mutate privileged machine state
- Ubuntu-specific system logic stays behind host adapters
- Frontend/client work must cross the IPC boundary instead of touching host state directly

## Contributing

Please read [CONTRIBUTING.md](./CONTRIBUTING.md) before opening issues or pull requests.

## Project policies

- Security reporting: see [SECURITY.md](./SECURITY.md)
- Change history and release notes policy: see [CHANGELOG.md](./CHANGELOG.md)

## License

Licensed under Apache-2.0. See [LICENSE](./LICENSE).
