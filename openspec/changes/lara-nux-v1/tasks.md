# Tasks: lara-nux v1

## Phase 1: Repo / Bootstrap

- [x] 1.1 Create `daemon/`, `client/`, `packaging/ubuntu/`, and `shared/contracts/` skeletons plus ownership notes in `daemon/README.md`, `client/README.md`, and `shared/contracts/README.md`.
- [x] 1.2 Add daemon bootstrap entrypoint in `daemon/cmd/lara-nuxd/main.*` to bind the Unix socket, load config paths, and emit journald-style startup events.
- [x] 1.3 Add bootstrap preflight + managed-asset manifest in `daemon/internal/app/bootstrap_service.*` and `daemon/internal/app/managed_assets.*` for Ubuntu support, privilege checks, and rollback tracking.

## Phase 2: Daemon Core

- [x] 2.1 Implement `daemon/internal/app/site_registry.*` with Laravel path validation, unique `.test` domain rules, persisted `SiteRecord`, and status fields.
- [x] 2.2 Implement `daemon/internal/app/php_registry.*` and `daemon/internal/app/php_manager.*` for supported runtime registration, active-runtime lookup, and transactional site switching.
- [x] 2.3 Implement `daemon/internal/app/service_manager.*` and `daemon/internal/app/health_service.*` for start/stop/restart/status plus port, privilege, and per-site readiness checks.
- [x] 2.4 Expose socket RPC handlers in `daemon/internal/api/{health,sites,php,services}_rpc.*` for `/rpc/health`, site register, PHP switch, and service actions.

## Phase 3: Ubuntu Adapters

- [x] 3.1 Implement `daemon/internal/host/ubuntu/resolved/*` to detect resolver ownership, install/remove the managed `.test` stub, and report conflicts without overwriting unrelated DNS.
- [x] 3.2 Implement `daemon/internal/host/ubuntu/caddy/*` to render per-site HTTP/HTTPS configs, validate before reload, and preserve existing sites on failed activation.
- [x] 3.3 Implement `daemon/internal/host/ubuntu/php/*` to materialize PHP-FPM units/sockets per runtime and rollback failed switches.
- [x] 3.4 Implement `daemon/internal/host/ubuntu/packages/*` for supported package acquisition, verification, and runtime inventory refresh.

## Phase 4: Client Shell / UI

- [x] 4.1 Scaffold the selected native shell in `client/src-tauri/*` or `client/wails.*` and wire tray, window lifecycle, and daemon-socket connectivity against the corrected backend activation/runtime/health RPC flows.
- [x] 4.2 Build `client/ui/pages/sites/*` and `client/ui/components/site-form/*` for add/edit/list flows, Laravel-path errors, and duplicate-domain feedback.
- [x] 4.3 Build `client/ui/pages/runtimes/*` and `client/ui/components/health-panel/*` for runtime switching, service states, and remediation messages.

## Phase 5: Packaging / Install / Uninstall

- [ ] 5.1 Add `packaging/ubuntu/systemd/*.service`, daemon user/group setup, and socket permissions matching `root:lara-nux`.
- [ ] 5.2 Add `packaging/ubuntu/scripts/{postinst,prerm,postrm}` for first install, idempotent upgrade, rollback, and safe uninstall that preserves user projects.
- [ ] 5.3 Add `packaging/ubuntu/debian/*` and repo metadata for a signed `.deb` build path and managed file manifests.

## Phase 6: Testing

- [ ] 6.1 Add unit tests in `daemon/internal/app/*_test.*` for Laravel validation, duplicate-domain rejection, supported-runtime rules, and health/conflict mapping.
- [ ] 6.2 Add adapter integration fixtures in `daemon/internal/host/ubuntu/*/testdata/` plus tests for resolver conflicts, Caddy render/reload, PHP-FPM switching, and package verification.
- [ ] 6.3 Add end-to-end coverage in `testing/e2e/ubuntu/*` for install -> register site -> browse HTTPS -> switch PHP -> uninstall on Ubuntu 22.04 and 24.04.

## Phase 7: OSS / Release Hardening

- [ ] 7.1 Document support matrix, privileges, rollback, and troubleshooting in `README.md` and `docs/ubuntu.md`.
- [ ] 7.2 Add CI/release workflows in `.github/workflows/{test,package,release}.yml` for tests, `.deb` validation, and artifact signing checks.
- [ ] 7.3 Add contributor/reporter guardrails in `.github/ISSUE_TEMPLATE/*`, `.github/pull_request_template.md`, and `CONTRIBUTING.md` for reproducible Ubuntu bug reports.
