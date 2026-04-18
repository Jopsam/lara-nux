# daemon

## Ownership

The `daemon/` package owns every privileged capability in Lara Nux.

- Unix-socket IPC bootstrap and daemon lifecycle
- Ubuntu host mutation (resolver, Caddy, PHP-FPM, packages, systemd)
- Machine-readable journald-style operational events
- Managed asset tracking for install, rollback, and uninstall safety

## Boundaries

- `cmd/lara-nuxd/` contains the daemon entrypoint only.
- `internal/app/` contains host-agnostic application services and bootstrap orchestration.
- `internal/host/ubuntu/` is reserved for Ubuntu-specific adapters so distro logic does not leak into core services.

The daemon MUST remain the only place where privileged filesystem writes, service control, and resolver changes are initiated.
