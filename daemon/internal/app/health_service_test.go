package app

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/jopsam/lara-nux/daemon/internal/host"
)

type fakeHealthSiteStore struct{ sites []SiteRecord }

func (f fakeHealthSiteStore) Register(context.Context, SiteRegistrationInput) (SiteRecord, error) {
	return SiteRecord{}, nil
}
func (f fakeHealthSiteStore) Get(context.Context, string) (SiteRecord, error) {
	return SiteRecord{}, nil
}
func (f fakeHealthSiteStore) Update(context.Context, SiteRecord) (SiteRecord, error) {
	return SiteRecord{}, nil
}
func (f fakeHealthSiteStore) Delete(context.Context, string) error { return nil }
func (f fakeHealthSiteStore) List(context.Context) ([]SiteRecord, error) {
	return append([]SiteRecord(nil), f.sites...), nil
}

type fakeHealthRuntimeResolver struct{}

func (fakeHealthRuntimeResolver) Get(context.Context, string) (PHPRuntimeRecord, error) {
	return PHPRuntimeRecord{}, ErrRuntimeNotFound
}
func (fakeHealthRuntimeResolver) DefaultRuntime(context.Context) (PHPRuntimeRecord, error) {
	return PHPRuntimeRecord{}, ErrRuntimeNotFound
}
func (fakeHealthRuntimeResolver) List(context.Context) ([]PHPRuntimeRecord, error) { return nil, nil }

type fakeReadyRuntimeResolver struct {
	runtime PHPRuntimeRecord
}

func (f fakeReadyRuntimeResolver) Get(context.Context, string) (PHPRuntimeRecord, error) {
	return f.runtime, nil
}
func (f fakeReadyRuntimeResolver) DefaultRuntime(context.Context) (PHPRuntimeRecord, error) {
	return f.runtime, nil
}
func (f fakeReadyRuntimeResolver) List(context.Context) ([]PHPRuntimeRecord, error) {
	return []PHPRuntimeRecord{f.runtime}, nil
}

type fakeHealthServiceController struct{}

func (fakeHealthServiceController) Start(context.Context, string) (ServiceStatus, error) {
	return ServiceStatus{}, nil
}

type fakeActiveHealthServiceController struct{}

func (fakeActiveHealthServiceController) Start(context.Context, string) (ServiceStatus, error) {
	return ServiceStatus{}, nil
}
func (fakeActiveHealthServiceController) Restart(context.Context, string) (ServiceStatus, error) {
	return ServiceStatus{}, nil
}
func (fakeActiveHealthServiceController) Status(_ context.Context, service string) (ServiceStatus, error) {
	return ServiceStatus{Service: service, State: ServiceStateActive, Summary: "active"}, nil
}
func (fakeHealthServiceController) Restart(context.Context, string) (ServiceStatus, error) {
	return ServiceStatus{}, nil
}
func (fakeHealthServiceController) Status(_ context.Context, service string) (ServiceStatus, error) {
	return ServiceStatus{Service: service, State: ServiceStateInactive, Summary: "inactive"}, errors.New("inactive")
}

type fakeHealthResolver struct{}

func (fakeHealthResolver) Inspect(context.Context, host.ResolverStubSpec) (host.ResolverStatus, error) {
	return host.ResolverStatus{Owner: "conflict", Summary: "resolver conflict", Conflicts: []host.Conflict{{Resource: "/etc/systemd/resolved.conf.d/external.conf", Summary: "claimed"}}}, nil
}
func (fakeHealthResolver) EnsureTestStub(context.Context, host.ResolverStubSpec) (host.ResolverStatus, error) {
	return host.ResolverStatus{}, nil
}
func (fakeHealthResolver) RemoveManagedStub(context.Context) error { return nil }

type fakeSocketInspector struct{}

func (fakeSocketInspector) Inspect(path string) SocketAvailability {
	return SocketAvailability{Path: path, Available: false, Summary: "socket missing"}
}

type fakePortChecker struct{}

func (fakePortChecker) CheckAvailable(int) error { return errors.New("already in use") }

func TestHealthServiceReportsResolverSocketAndRuntimeFailures(t *testing.T) {
	t.Parallel()

	project := createLaravelProject(t)
	site := SiteRecord{ID: "site-1", Domain: "demo.test", RootPath: project, PHPVersion: "8.3"}
	service := NewHealthService(fakeHealthSiteStore{sites: []SiteRecord{site}}, fakeHealthRuntimeResolver{}, fakeHealthServiceController{}, fakeHealthResolver{}, "/run/lara-nux.sock")
	service.socketCheck = fakeSocketInspector{}
	service.portChecker = fakePortChecker{}
	service.clock = func() time.Time { return time.Unix(123, 0).UTC() }

	report, err := service.Report(context.Background())
	if err != nil {
		t.Fatalf("Report returned error: %v", err)
	}
	if report.Ready {
		t.Fatal("expected health report to be not ready")
	}
	if report.Socket.Summary != "socket missing" {
		t.Fatalf("unexpected socket summary: %+v", report.Socket)
	}
	if report.Resolver == nil || len(report.Resolver.Conflicts) != 1 {
		t.Fatalf("expected resolver conflict in report, got %+v", report.Resolver)
	}
	if len(report.Sites) != 1 || report.Sites[0].Ready {
		t.Fatalf("expected degraded site readiness, got %+v", report.Sites)
	}
	if report.Sites[0].Site.Status != SiteStatusConflict {
		t.Fatalf("expected site conflict status, got %s", report.Sites[0].Site.Status)
	}
	assertHealthCheckPresent(t, report.Checks, "daemon-socket", false)
	assertHealthCheckPresent(t, report.Checks, "resolver-test-routing", false)
	assertHealthCheckPresent(t, report.Checks, "php-runtime-inventory", false)
}

func TestHealthServiceMapsInvalidLaravelPathToConflictStatus(t *testing.T) {
	t.Parallel()

	project := createLaravelProject(t)
	if err := os.Remove(filepath.Join(project, "artisan")); err != nil {
		t.Fatalf("remove artisan: %v", err)
	}

	service := NewHealthService(
		fakeHealthSiteStore{sites: []SiteRecord{{ID: "site-1", Domain: "demo.test", RootPath: project, PHPVersion: "8.2"}}},
		fakeReadyRuntimeResolver{runtime: PHPRuntimeRecord{Version: "8.2", FPMService: "php8.2-fpm"}},
		fakeActiveHealthServiceController{},
		nil,
		"/run/lara-nux.sock",
	)
	service.socketCheck = fakeSocketInspector{}
	service.clock = func() time.Time { return time.Unix(456, 0).UTC() }

	report, err := service.Report(context.Background())
	if err != nil {
		t.Fatalf("Report returned error: %v", err)
	}

	if len(report.Sites) != 1 {
		t.Fatalf("expected 1 site readiness entry, got %d", len(report.Sites))
	}
	if report.Sites[0].Site.Status != SiteStatusConflict {
		t.Fatalf("expected conflict status, got %s", report.Sites[0].Site.Status)
	}
	if report.Sites[0].Ready {
		t.Fatal("expected invalid laravel project to be not ready")
	}
}

func TestHealthServiceMapsInactiveServicesToDegradedStatus(t *testing.T) {
	t.Parallel()

	project := createLaravelProject(t)
	service := NewHealthService(
		fakeHealthSiteStore{sites: []SiteRecord{{ID: "site-1", Domain: "demo.test", RootPath: project, PHPVersion: "8.2"}}},
		fakeReadyRuntimeResolver{runtime: PHPRuntimeRecord{Version: "8.2", FPMService: "php8.2-fpm"}},
		fakeHealthServiceController{},
		nil,
		"/run/lara-nux.sock",
	)
	service.socketCheck = fakeSocketInspector{}
	service.portChecker = fakePortChecker{}

	report, err := service.Report(context.Background())
	if err != nil {
		t.Fatalf("Report returned error: %v", err)
	}

	if len(report.Sites) != 1 {
		t.Fatalf("expected 1 site readiness entry, got %d", len(report.Sites))
	}
	if report.Sites[0].Site.Status != SiteStatusDegraded {
		t.Fatalf("expected degraded status, got %s", report.Sites[0].Site.Status)
	}
	assertSiteHealthCheckPresent(t, report.Sites[0].Checks, "caddy-service", false)
}

func createLaravelProject(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	for _, rel := range []string{"artisan", "composer.json", filepath.Join("public", "index.php")} {
		path := filepath.Join(dir, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		if err := os.WriteFile(path, []byte("stub"), 0o644); err != nil {
			t.Fatalf("write file: %v", err)
		}
	}
	return dir
}

func assertHealthCheckPresent(t *testing.T, checks []HealthCheck, name string, passed bool) {
	t.Helper()
	for _, check := range checks {
		if check.Name == name {
			if check.Passed != passed {
				t.Fatalf("check %s passed=%v want %v", name, check.Passed, passed)
			}
			return
		}
	}
	t.Fatalf("missing health check %s", name)
}

func assertSiteHealthCheckPresent(t *testing.T, checks []HealthCheck, name string, passed bool) {
	t.Helper()
	assertHealthCheckPresent(t, checks, name, passed)
}
