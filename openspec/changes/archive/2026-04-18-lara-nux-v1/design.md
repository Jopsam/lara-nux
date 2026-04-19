# Design: lara-nux v1

## Technical Approach

v1 uses an Ubuntu-first split architecture: a privileged system daemon owns DNS, ports 80/443, service units, runtime inventory, and package-level mutation; an unprivileged Nuxt desktop/tray client owns UX and calls the daemon through a local Unix socket RPC API. This matches the proposal’s root-surface minimization goal while keeping future distro support isolated behind host adapters.

## Architecture Decisions

| Decision | Options | Choice | Rationale |
|---|---|---|---|
| Core topology | monolith GUI, daemon+client | daemon+client | Sensitive ops stay isolated; GUI crashes do not affect routing. |
| IPC | HTTP over loopback, Unix socket JSON-RPC, gRPC | Unix socket + HTTP/JSON | Simple debugging, file-permission auth, no TCP exposure. |
| Web/TLS | nginx, Apache, Caddy | Caddy | Automatic local TLS, simpler per-site config, smaller config surface. |
| DNS | `/etc/hosts`, dnsmasq, systemd-resolved | systemd-resolved + dedicated `test` stub | Wildcard-like local domain handling without per-site hosts churn. |
| PHP model | apt packages, bundled runtimes | daemon-managed runtime registry | Avoids system PHP conflicts and enables project switching. |
| Service manager | custom supervisor, systemd user/system | systemd system units | Native Ubuntu lifecycle, restart policy, journald integration. |
| Packaging | curl script, AppImage, `.deb` | signed `.deb` + apt repo later | Clean install/upgrade/remove semantics for Ubuntu LTS. |

## Data Flow

### Site registration

```text
Client UI -> Socket API -> Site Registry -> Caddy config renderer -> systemd restart caddy
                      \-> PHP resolver -------> PHP-FPM unit renderer -> systemd restart php-fpm@version
```

### Request path

```text
Browser -> systemd-resolved (.test -> 127.0.0.1) -> Caddy :443/:80
        -> matched site config -> php_fastcgi unix socket -> Laravel public/index.php
```

### Install / privileged bootstrap

```text
Desktop setup -> privileged installer helper -> install daemon + units + resolved drop-in + Caddy
             -> health checks -> client marks machine ready
```

### Sequence: register and serve a site

```text
Client -> Daemon API: register(rootPath, phpVersion)
Daemon API -> Site Registry: validate + persist
Daemon API -> Caddy Renderer: write vhost
Daemon API -> PHP Manager: ensure php-fpm unit/socket
Daemon API -> systemd: reload caddy + php-fpm
Browser -> resolved: my-app.test
resolved -> Caddy: 127.0.0.1:443
Caddy -> php-fpm socket: fastcgi
php-fpm -> Laravel: execute app
```

## File Changes

| File | Action | Description |
|------|--------|-------------|
| `openspec/changes/lara-nux-v1/design.md` | Create | v1 technical design artifact. |
| `daemon/cmd/lara-nuxd/main.*` | Create | Daemon entrypoint and socket bootstrap. |
| `daemon/internal/app/service_manager.*` | Create | Start/stop/restart/status abstraction over systemd. |
| `daemon/internal/app/site_registry.*` | Create | Canonical site/runtime metadata model. |
| `daemon/internal/host/ubuntu/{resolved,caddy,php,packages}.*/` | Create | Ubuntu-specific adapters. |
| `client/src-tauri|wails/*` | Create | Native shell for tray/window lifecycle. |
| `client/ui/pages/sites/*` | Create | Site management UX in Nuxt. |
| `packaging/ubuntu/*` | Create | `.deb`, postinst/prerm, unit files, resolver drop-ins. |

## Interfaces / Contracts

```ts
type SiteRecord = {
  id: string; name: string; rootPath: string; domain: `${string}.test`;
  phpVersion: string; tls: 'auto'; status: 'ready'|'degraded'|'conflict';
}

POST /rpc/sites.register { rootPath, domain?, phpVersion }
POST /rpc/php.switch { siteId, phpVersion }
GET  /rpc/health
POST /rpc/services.action { service, action }
```

Socket ownership is `root:lara-nux`; desktop user gains access through group membership, so auth is OS-level first, request validation second.

## Privilege / Runtime Model

- Installer helper runs elevated only for first-time setup, upgrades needing system mutation, and uninstall cleanup.
- Daemon runs as a dedicated system account with only the filesystem paths and unit control it needs.
- Client never shells out with `sudo`; all privileged work crosses the socket boundary.
- Runtime managers are split: `PackageManager` acquires runtimes/bundles, `PHPManager` materializes FPM units/sockets, `ServiceManager` owns lifecycle/status.

## Testing Strategy

| Layer | What to Test | Approach |
|-------|-------------|----------|
| Unit | Registry validation, config rendering, conflict detection | Pure tests around renderers and models. |
| Integration | systemd-resolved/Caddy/PHP-FPM adapters | Ubuntu container/VM fixtures with golden configs. |
| E2E | install -> register site -> browse HTTPS -> uninstall | Fresh Ubuntu LTS VM matrix (22.04, 24.04). |

## Migration / Rollout

No data migration required. Rollout is OSS beta first on supported Ubuntu LTS versions. Upgrades MUST be idempotent; uninstall MUST stop units, remove resolver/Caddy assets, preserve user projects, and optionally keep runtime cache for rollback speed.

## Observability / Health

The daemon exposes `/rpc/health` with checks for socket availability, Caddy active state, resolver routing, runtime presence, and per-site status. Logs go to journald with machine-readable event IDs. The client surfaces degraded states and exact remediation for port conflicts, missing runtime, broken certificates, or resolver drift.

## Failure Handling / Extension Points

- Bootstrap fails closed: if resolver or ports cannot be claimed, installation aborts with rollback.
- Runtime switch is transactional: render config, validate, then reload services; otherwise revert to prior runtime.
- Future distro support enters through `host` adapters (`ResolverManager`, `WebServerManager`, `PackageManager`, `ServiceManager`) so Debian/Fedora differences do not leak into registry or client layers.

## Open Questions

- [ ] Final implementation stack for native shell/runtime layer: Wails+Go vs Tauri+Rust.
- [ ] Whether v1 ships managed MariaDB/Redis or only detects existing services.
