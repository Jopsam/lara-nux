## Verification Report

**Change**: lara-nux-v1
**Version**: N/A
**Mode**: Standard

---

### Completeness
| Metric | Value |
|--------|-------|
| Tasks total | 23 |
| Tasks complete | 22 |
| Tasks incomplete | 1 |

Incomplete tasks:
- **6.3** Add end-to-end coverage in `testing/e2e/ubuntu/*` for install -> register site -> browse HTTPS -> switch PHP -> uninstall on Ubuntu 22.04 and 24.04.

Resolved from previous verification:
- **4.1 client shell verification blocker is resolved.** `client/wails/go.sum` is checked in and the Linux tray now has a default non-`systray` path, so plain repo-state `go test ./...` in `client/wails/` passes.
- **Task 6.3 overstatement is resolved honestly.** `openspec/changes/lara-nux-v1/tasks.md` now marks 6.3 incomplete, which matches the current evidence level.

---

### Build & Tests Execution

**Build / static quality**: ✅ `go vet ./...` passed in `daemon/`

**Tests**:
- ✅ `daemon/`: `go test ./...` passed
- ✅ `daemon/internal/app`: `go test -count=1 ./internal/app/...` passed
- ✅ `daemon/`: `go test -cover ./...` passed
- ✅ `testing/e2e/ubuntu/`: `go test ./...` passed
- ✅ `testing/e2e/ubuntu/`: `go test -count=1 ./...` passed
- ✅ `client/wails/`: `go test ./...` passed
- ✅ `client/wails/`: `go test -count=1 ./...` passed

**Coverage**: `go test -cover ./...` in `daemon/` passed

```text
github.com/jopsam/lara-nux/daemon/cmd/lara-nuxd              0.0%
github.com/jopsam/lara-nux/daemon/internal/api              58.9%
github.com/jopsam/lara-nux/daemon/internal/app              46.1%
github.com/jopsam/lara-nux/daemon/internal/host/ubuntu/caddy 60.2%
github.com/jopsam/lara-nux/daemon/internal/host/ubuntu/packages 54.8%
github.com/jopsam/lara-nux/daemon/internal/host/ubuntu/php  63.4%
github.com/jopsam/lara-nux/daemon/internal/host/ubuntu/resolved 48.9%
github.com/jopsam/lara-nux/daemon/internal/host/ubuntu/systemd 61.9%
```

---

### Spec Compliance Matrix

| Requirement | Scenario | Test | Result |
|-------------|----------|------|--------|
| environment-bootstrap | Supported bootstrap | `daemon/internal/app/bootstrap_service_test.go > TestBootstrapPreflightAcceptsSupportedUbuntuHosts` | ✅ COMPLIANT |
| environment-bootstrap | Unsupported host | `daemon/internal/app/bootstrap_service_test.go > TestBootstrapPreflightRejectsUnsupportedUbuntuRelease` | ✅ COMPLIANT |
| environment-bootstrap | Safe uninstall | `testing/e2e/ubuntu/managed_workflow_test.go > TestUbuntuLTSManagedWorkflowEvidence` | ⚠️ PARTIAL |
| local-dns-routing | Resolve registered site | `testing/e2e/ubuntu/managed_workflow_test.go > TestUbuntuLTSManagedWorkflowEvidence` | ⚠️ PARTIAL |
| local-dns-routing | Resolver conflict detected | `daemon/internal/host/ubuntu/resolved/manager_test.go > TestEnsureTestStubRejectsResolverConflictsWithoutMutatingManagedStub` | ✅ COMPLIANT |
| local-site-serving | Serve a Laravel project | `testing/e2e/ubuntu/managed_workflow_test.go > TestUbuntuLTSManagedWorkflowEvidence` | ⚠️ PARTIAL |
| local-site-serving | Reject invalid project | `daemon/internal/app/site_registry_test.go > TestValidateLaravelPathRejectsMissingRequirements`; `daemon/internal/app/site_registry_test.go > TestSiteRegistryRejectsDuplicateDomainCaseInsensitive`; `daemon/internal/app/orchestration_test.go > TestSiteActivationRollsBackRegisteredSiteWhenActivationFails` | ✅ COMPLIANT |
| php-runtime-management | Register supported runtime | `daemon/internal/app/php_registry_test.go > TestPHPRegistryRegistersSupportedRuntime` | ✅ COMPLIANT |
| php-runtime-management | Reject unsupported runtime | `daemon/internal/app/php_registry_test.go > TestPHPRegistryRejectsUnsupportedRuntime` | ✅ COMPLIANT |
| php-runtime-management | Switch project runtime | `testing/e2e/ubuntu/managed_workflow_test.go > TestUbuntuLTSManagedWorkflowEvidence` | ⚠️ PARTIAL |
| service-orchestration | Start required services | `testing/e2e/ubuntu/managed_workflow_test.go > TestUbuntuLTSManagedWorkflowEvidence` | ✅ COMPLIANT |
| service-orchestration | Port conflict blocks readiness | `daemon/internal/app/health_service_test.go > TestHealthServiceReportsResolverSocketAndRuntimeFailures` | ✅ COMPLIANT |

**Compliance summary**: 8/12 scenarios compliant

---

### Correctness (Static — Structural Evidence)
| Requirement | Status | Notes |
|------------|--------|-------|
| Supported Ubuntu bootstrap | ✅ Implemented | `daemon/internal/app/bootstrap_service.go` plus focused preflight tests cover supported and unsupported host handling. |
| Managed `*.test` routing | ✅ Implemented | `daemon/internal/host/ubuntu/resolved/manager.go` owns managed stub install, removal, and conflict detection. |
| Predictable site activation | ✅ Implemented | `daemon/internal/app/orchestration.go`, `site_management.go`, and `daemon/internal/host/ubuntu/caddy/manager.go` implement registration, activation, rollback, and config rendering. |
| Supported PHP lifecycle and switching | ✅ Implemented | `php_registry.go`, `php_manager.go`, `runtime_onboarding.go`, and Ubuntu PHP adapters cover registration and switching. |
| Service lifecycle and health reporting | ✅ Implemented | `service_manager.go`, `health_service.go`, `daemon/internal/api/{health,services}_rpc.go`, and `daemon/internal/host/ubuntu/systemd/manager.go` match the intended boundary. |
| Client shell readiness | ✅ Implemented | `client/wails` is now verifiable from repo state with checked-in sums and opt-in `systray` integration. |

---

### Coherence (Design)
| Decision | Followed? | Notes |
|----------|-----------|-------|
| Daemon + client split | ✅ Yes | `daemon/`, `client/`, and `shared/contracts/` reflect the intended split. |
| Unix socket + HTTP/JSON IPC | ✅ Yes | `daemon/cmd/lara-nuxd/main.go` binds a Unix socket and serves HTTP handlers. |
| Caddy for local web/TLS | ✅ Yes | Ubuntu web adapter is implemented under `daemon/internal/host/ubuntu/caddy`. |
| systemd-resolved managed `.test` stub | ✅ Yes | Implemented under `daemon/internal/host/ubuntu/resolved`. |
| systemd system units | ✅ Yes | Service control is behind `daemon/internal/host/ubuntu/systemd`. |
| Dedicated daemon system account | ⚠️ Deviated | Packaging still runs the daemon service as `root` in `packaging/ubuntu/systemd/lara-nuxd.service`, even though maintainer scripts create `lara-nuxd`. |
| Signed `.deb` release path | ⚠️ Deferred honestly | Repo metadata/workflows exist, but release signing is still intentionally unfinished. |

---

### Issues Found

**CRITICAL** (must fix before archive):
1. **Task 6.3 remains incomplete and still blocks archive.** The repo does not yet execute the required install -> register site -> browse HTTPS -> switch PHP -> uninstall workflow on real Ubuntu 22.04 and 24.04 environments.

**WARNING** (should fix):
1. `testing/e2e/ubuntu/managed_workflow_test.go` materially improves evidence by exercising real host managers against temporary filesystem fixtures, but it still does not prove literal VM/container-backed Ubuntu behavior or an actual browser/HTTPS request.
2. The packaged daemon still runs as `root`, which deviates from the design goal of a dedicated daemon account.
3. Release signing remains intentionally unfinished (`SignWith: CHANGE_ME`, missing configured secrets), so release hardening is not production-ready yet.

**SUGGESTION** (nice to have):
1. Add real Jammy/Noble VM or container-backed E2E jobs that execute the full install/serve/switch/uninstall flow.
2. Add explicit HTTPS request assertions in the future real E2E layer rather than inferring behavior from generated config and activation metadata.

---

### Verdict

**FAIL**

The previous `client/wails` verification blocker is fixed, and task 6.3 is now represented honestly as incomplete, but the change is **still not ready to archive** because the remaining core E2E requirement in 6.3 has not been delivered yet.
