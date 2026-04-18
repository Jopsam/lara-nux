## Verification Report

**Change**: lara-nux-v1
**Scope**: Final pre-UI readiness verification after the API/contract batch
**Mode**: Standard

---

### Completeness

| Metric | Value |
|--------|-------|
| Tasks total | 23 |
| Tasks complete | 11 |
| Tasks incomplete | 12 |

Completed scope verified in repo: 1.1-1.3, 2.1-2.4, 3.1-3.4.

Incomplete tasks are Phase 4 UI implementation, Phase 5 packaging, Phase 6 expanded testing, and Phase 7 OSS hardening. For this gate, the relevant question is whether backend/contracts still block Phase 4.

---

### Build & Tests Execution

**Build / static quality**: ✅ `go vet ./...` passed

**Tests**: ✅ `go test ./...` passed

```text
?    github.com/jopsam/lara-nux/daemon/cmd/lara-nuxd                         [no test files]
ok   github.com/jopsam/lara-nux/daemon/internal/api                          (cached)
ok   github.com/jopsam/lara-nux/daemon/internal/app                          (cached)
?    github.com/jopsam/lara-nux/daemon/internal/host                         [no test files]
?    github.com/jopsam/lara-nux/daemon/internal/host/ubuntu/caddy           [no test files]
?    github.com/jopsam/lara-nux/daemon/internal/host/ubuntu/packages        [no test files]
?    github.com/jopsam/lara-nux/daemon/internal/host/ubuntu/php             [no test files]
ok   github.com/jopsam/lara-nux/daemon/internal/host/ubuntu/resolved        (cached)
?    github.com/jopsam/lara-nux/daemon/internal/host/ubuntu/systemd         [no test files]
```

**Coverage**: ⚠️ `go test -cover ./...` passed, but coverage remains uneven

```text
cmd/lara-nuxd                        0.0%
internal/api                        41.7%
internal/app                        24.4%
internal/host/ubuntu/caddy           0.0%
internal/host/ubuntu/packages        0.0%
internal/host/ubuntu/php             0.0%
internal/host/ubuntu/resolved       48.9%
internal/host/ubuntu/systemd         0.0%
```

---

### Spec Compliance Matrix

| Requirement | Scenario | Test / Evidence | Result |
|-------------|----------|-----------------|--------|
| environment-bootstrap | Supported bootstrap | `daemon/internal/app/bootstrap_service.go`, `daemon/cmd/lara-nuxd/main.go` | ✅ COMPLIANT |
| environment-bootstrap | Unsupported host | `daemon/internal/app/bootstrap_service.go` preflight guards | ✅ COMPLIANT |
| environment-bootstrap | Safe uninstall | managed asset tracking in `managed_assets.go`, packaging work still deferred | ⚠️ PARTIAL |
| php-runtime-management | Register supported runtime | `daemon/internal/app/php_registry_test.go`, `/rpc/php.register` | ✅ COMPLIANT |
| php-runtime-management | Reject unsupported runtime | `daemon/internal/app/php_registry_test.go` | ✅ COMPLIANT |
| php-runtime-management | Switch project runtime | `daemon/internal/app/orchestration.go`, `/rpc/php.switch` | ✅ COMPLIANT |
| local-site-serving | Serve a Laravel project | `daemon/internal/app/orchestration.go`, `/rpc/sites.register` | ✅ COMPLIANT |
| local-site-serving | Reject invalid project | `daemon/internal/app/orchestration_test.go` rollback path | ✅ COMPLIANT |
| local-dns-routing | Resolve registered site | `daemon/internal/host/ubuntu/resolved/manager.go`, `manager_test.go` | ✅ COMPLIANT |
| local-dns-routing | Resolver conflict detected | `daemon/internal/host/ubuntu/resolved/manager_test.go` | ✅ COMPLIANT |
| service-orchestration | Start required services | `daemon/internal/app/service_manager.go`, `daemon/internal/api/services_rpc.go` | ✅ COMPLIANT |
| service-orchestration | Port conflict blocks readiness | `daemon/internal/app/health_service_test.go`, `/rpc/health` | ✅ COMPLIANT |

**Compliance summary**: 10/11 scenarios compliant, 1 partial due to packaging/uninstall work intentionally deferred to Phase 5.

---

### Correctness (Static — Structural Evidence)

| Area | Status | Notes |
|------|--------|-------|
| Site management RPC surface | ✅ Implemented | `daemon/internal/api/sites_rpc.go` now exposes `/rpc/sites.list`, `/rpc/sites.get`, `/rpc/sites.update`. |
| Runtime read-model RPC surface | ✅ Implemented | `daemon/internal/api/php_rpc.go` now exposes `/rpc/php.list`, `/rpc/php.default`, `/rpc/php.inventory`. |
| Shared contracts for Phase 4 | ✅ Implemented | `shared/contracts/rpc/v1/contracts.ts` and `contracts.schema.json` include site query/update and runtime read-model DTOs. |
| Router coverage for UI-facing transport | ✅ Implemented | `daemon/internal/api/router_test.go` exercises the new site/runtime read endpoints. |
| Install/uninstall execution path | ⚠️ Partial | Packaging directories exist, but scripts/systemd assets remain placeholders under `packaging/ubuntu/`. |

---

### Coherence (Design)

| Decision | Followed? | Notes |
|----------|-----------|-------|
| Daemon + client split | ✅ Yes | Client remains scaffold-only while backend owns orchestration. |
| Unix socket + HTTP/JSON IPC | ✅ Yes | `daemon/cmd/lara-nuxd/main.go` binds the Unix socket and serves HTTP handlers. |
| Shared contracts for client/backend seam | ✅ Yes | `shared/contracts/rpc/v1/*` now documents and schemas the UI-facing seam. |
| Ubuntu adapter boundaries | ✅ Yes | Host integrations remain isolated under `daemon/internal/host/ubuntu/*`. |

---

### Issues Found

**CRITICAL**

None.

**WARNING**

1. **Packaging/uninstall behavior is still not executable, only modeled.**
   - Spec scenario `environment-bootstrap -> Safe uninstall` remains only partially satisfied.
   - Evidence: packaging assets are still placeholders in `packaging/ubuntu/systemd/.gitkeep`, `packaging/ubuntu/scripts/.gitkeep`, and `packaging/ubuntu/debian/.gitkeep`.

2. **Coverage remains thin in several daemon packages that the Phase 4 UI will indirectly depend on.**
   - `go test -cover ./...` reports 0.0% for `daemon/cmd/lara-nuxd`, `daemon/internal/host/ubuntu/caddy`, `daemon/internal/host/ubuntu/packages`, `daemon/internal/host/ubuntu/php`, and `daemon/internal/host/ubuntu/systemd`.
   - This is not a pre-UI blocker, but it increases regression risk while the UI starts consuming the daemon.

**SUGGESTION**

1. **Add request/response golden tests for shared contracts before Phase 4 expands the client surface.**
   - Evidence: transport coverage exists in `daemon/internal/api/router_test.go`, but contract compatibility is still validated only indirectly.

2. **Add adapter-focused tests for Caddy/PHP/package/systemd managers before Phase 5 packaging.**
   - Evidence: no `*_test.go` files currently exist under `daemon/internal/host/ubuntu/caddy`, `php`, `packages`, or `systemd`.

---

### Verdict

**PASS WITH WARNINGS for Phase 4 readiness.**

The previous two blockers are resolved: the daemon now exposes site list/get/update and runtime read-model endpoints, shared contracts encode those DTOs, and router tests cover the new transport surface. No new critical blocker was found for starting Phase 4 UI work; the remaining concerns are packaging/uninstall completeness and broader backend test depth.
