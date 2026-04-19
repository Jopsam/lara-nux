## Verification Report

**Change**: lara-nux-v1
**Version**: N/A
**Mode**: Standard

---

### Completeness
| Metric | Value |
|--------|-------|
| Tasks total | 23 |
| Tasks complete | 23 |
| Tasks incomplete | 0 |

Resolved from previous verification:
- **4.1 client shell verification blocker is resolved.** `client/wails/go.sum` is checked in and the Linux tray now has a default non-`systray` path, so plain repo-state `go test ./...` in `client/wails/` passes.
- **Task 6.3 is now implemented honestly.** `testing/e2e/ubuntu/real_environment_test.go` runs the packaged install -> register site -> browse HTTPS -> switch PHP -> uninstall workflow inside privileged Ubuntu 22.04 and 24.04 systemd containers, and the matrix was executed successfully with `LARA_NUX_REAL_UBUNTU_E2E=1 go test -count=1 -run TestUbuntuLTSRealEnvironmentWorkflow -v`.

---

### Build & Tests Execution

**Build / static quality**: ✅ `go vet ./...` passed in `daemon/`

**Tests**:
- ✅ `daemon/`: `go test ./...` passed
- ✅ `daemon/internal/app`: `go test -count=1 ./internal/app/...` passed
- ✅ `daemon/`: `go test -cover ./...` passed
- ✅ `testing/e2e/ubuntu/`: `go test ./...` passed
- ✅ `testing/e2e/ubuntu/`: `go test -count=1 ./...` passed
- ✅ `testing/e2e/ubuntu/`: `LARA_NUX_REAL_UBUNTU_E2E=1 go test -count=1 -run TestUbuntuLTSRealEnvironmentWorkflow -v` passed (Jammy + Noble matrix, Docker + systemd, packaged install/uninstall path)
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
| environment-bootstrap | Safe uninstall | `testing/e2e/ubuntu/real_environment_test.go > TestUbuntuLTSRealEnvironmentWorkflow` | ✅ COMPLIANT |
| local-dns-routing | Resolve registered site | `testing/e2e/ubuntu/real_environment_test.go > TestUbuntuLTSRealEnvironmentWorkflow` | ✅ COMPLIANT |
| local-dns-routing | Resolver conflict detected | `daemon/internal/host/ubuntu/resolved/manager_test.go > TestEnsureTestStubRejectsResolverConflictsWithoutMutatingManagedStub` | ✅ COMPLIANT |
| local-site-serving | Serve a Laravel project | `testing/e2e/ubuntu/real_environment_test.go > TestUbuntuLTSRealEnvironmentWorkflow` | ✅ COMPLIANT |
| local-site-serving | Reject invalid project | `daemon/internal/app/site_registry_test.go > TestValidateLaravelPathRejectsMissingRequirements`; `daemon/internal/app/site_registry_test.go > TestSiteRegistryRejectsDuplicateDomainCaseInsensitive`; `daemon/internal/app/orchestration_test.go > TestSiteActivationRollsBackRegisteredSiteWhenActivationFails` | ✅ COMPLIANT |
| php-runtime-management | Register supported runtime | `daemon/internal/app/php_registry_test.go > TestPHPRegistryRegistersSupportedRuntime` | ✅ COMPLIANT |
| php-runtime-management | Reject unsupported runtime | `daemon/internal/app/php_registry_test.go > TestPHPRegistryRejectsUnsupportedRuntime` | ✅ COMPLIANT |
| php-runtime-management | Switch project runtime | `testing/e2e/ubuntu/real_environment_test.go > TestUbuntuLTSRealEnvironmentWorkflow` | ✅ COMPLIANT |
| service-orchestration | Start required services | `testing/e2e/ubuntu/real_environment_test.go > TestUbuntuLTSRealEnvironmentWorkflow` | ✅ COMPLIANT |
| service-orchestration | Port conflict blocks readiness | `daemon/internal/app/health_service_test.go > TestHealthServiceReportsResolverSocketAndRuntimeFailures` | ✅ COMPLIANT |

**Compliance summary**: 12/12 scenarios compliant

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

**WARNING** (should fix):
1. The packaged daemon still runs as `root`, which deviates from the design goal of a dedicated daemon account.
2. Release signing remains intentionally unfinished (`SignWith: CHANGE_ME`, missing configured secrets), so release hardening is not production-ready yet.

**SUGGESTION** (nice to have):
1. Consider promoting the opt-in real Jammy/Noble harness into CI once privileged Docker runners are available.
2. If CI remains unprivileged, keep the fast fixture-backed `managed_workflow_test.go` layer as the default smoke path and reserve the real harness for scheduled/manual verification.

---

### Verdict

**PASS**

All 23 tasks are now honestly complete. The repo has both the fast fixture-backed Ubuntu workflow evidence and an opt-in real Ubuntu 22.04/24.04 Docker+systemd matrix that exercised packaged install -> register site -> browse HTTPS -> switch PHP -> uninstall successfully, so the change is ready for archive once maintainers accept the remaining non-blocking warnings.
