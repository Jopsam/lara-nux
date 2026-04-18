package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/jopsam/lara-nux/daemon/internal/app"
	"github.com/jopsam/lara-nux/daemon/internal/host"
)

func TestRouterSiteManagementRoutes(t *testing.T) {
	t.Parallel()

	testDeps := newRouterTestDeps(t)
	router := NewRouter(testDeps.routerDeps())

	siteA := seedSite(t, testDeps, seedSiteInput{RootPath: testDeps.projectA, Domain: "alpha.test", PHPVersion: "8.2"})
	seedSite(t, testDeps, seedSiteInput{RootPath: testDeps.projectB, Domain: "bravo.test", PHPVersion: "8.2"})

	t.Run("list sites", func(t *testing.T) {
		response := performJSONRequest(t, router, http.MethodGet, "/rpc/sites.list", nil)
		if response.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", response.Code, response.Body.String())
		}

		var payload rpcEnvelope
		decodeEnvelope(t, response, &payload)
		items := payload.Data.([]any)
		if len(items) != 2 {
			t.Fatalf("expected 2 sites, got %d", len(items))
		}
	})

	t.Run("get site detail", func(t *testing.T) {
		response := performJSONRequest(t, router, http.MethodGet, "/rpc/sites.get?siteId="+siteA.ID, nil)
		if response.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", response.Code, response.Body.String())
		}

		var payload struct {
			OK   bool           `json:"ok"`
			Data app.SiteRecord `json:"data"`
		}
		decodeEnvelope(t, response, &payload)
		if payload.Data.ID != siteA.ID {
			t.Fatalf("expected site %s, got %s", siteA.ID, payload.Data.ID)
		}
	})

	t.Run("update site detail", func(t *testing.T) {
		body := map[string]any{
			"siteId":     siteA.ID,
			"domain":     "alpha-updated.test",
			"phpVersion": "8.3",
		}
		response := performJSONRequest(t, router, http.MethodPost, "/rpc/sites.update", body)
		if response.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", response.Code, response.Body.String())
		}

		var payload struct {
			OK   bool           `json:"ok"`
			Data app.SiteRecord `json:"data"`
		}
		decodeEnvelope(t, response, &payload)
		if payload.Data.Domain != "alpha-updated.test" {
			t.Fatalf("expected updated domain, got %s", payload.Data.Domain)
		}
		if payload.Data.PHPVersion != "8.3" {
			t.Fatalf("expected updated runtime, got %s", payload.Data.PHPVersion)
		}
	})

	t.Run("missing site returns not found", func(t *testing.T) {
		response := performJSONRequest(t, router, http.MethodGet, "/rpc/sites.get?siteId=missing", nil)
		if response.Code != http.StatusNotFound {
			t.Fatalf("expected 404, got %d: %s", response.Code, response.Body.String())
		}
	})
}

func TestRouterRuntimeReadRoutes(t *testing.T) {
	t.Parallel()

	testDeps := newRouterTestDeps(t)
	router := NewRouter(testDeps.routerDeps())

	t.Run("list runtimes", func(t *testing.T) {
		response := performJSONRequest(t, router, http.MethodGet, "/rpc/php.list", nil)
		if response.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", response.Code, response.Body.String())
		}

		var payload rpcEnvelope
		decodeEnvelope(t, response, &payload)
		items := payload.Data.([]any)
		if len(items) != 2 {
			t.Fatalf("expected 2 runtimes, got %d", len(items))
		}
	})

	t.Run("read default runtime", func(t *testing.T) {
		response := performJSONRequest(t, router, http.MethodGet, "/rpc/php.default", nil)
		if response.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", response.Code, response.Body.String())
		}

		var payload struct {
			OK   bool `json:"ok"`
			Data struct {
				Runtime *app.PHPRuntimeRecord `json:"runtime"`
			} `json:"data"`
		}
		decodeEnvelope(t, response, &payload)
		if payload.Data.Runtime == nil || payload.Data.Runtime.Version != "8.2" {
			t.Fatalf("expected default runtime 8.2, got %+v", payload.Data.Runtime)
		}
	})

	t.Run("change default runtime", func(t *testing.T) {
		response := performJSONRequest(t, router, http.MethodPost, "/rpc/php.default", map[string]any{"version": "8.3"})
		if response.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", response.Code, response.Body.String())
		}

		var payload struct {
			OK   bool `json:"ok"`
			Data struct {
				Runtime app.PHPRuntimeRecord `json:"runtime"`
			} `json:"data"`
		}
		decodeEnvelope(t, response, &payload)
		if payload.Data.Runtime.Version != "8.3" {
			t.Fatalf("expected default runtime 8.3, got %s", payload.Data.Runtime.Version)
		}
	})

	t.Run("runtime inventory", func(t *testing.T) {
		response := performJSONRequest(t, router, http.MethodGet, "/rpc/php.inventory", nil)
		if response.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", response.Code, response.Body.String())
		}

		var payload struct {
			OK   bool               `json:"ok"`
			Data app.RuntimeCatalog `json:"data"`
		}
		decodeEnvelope(t, response, &payload)
		if len(payload.Data.SupportedPackages) == 0 {
			t.Fatal("expected supported packages in inventory")
		}
		if len(payload.Data.DetectedRuntimes) == 0 {
			t.Fatal("expected detected runtimes in inventory")
		}
	})
}

type routerTestDeps struct {
	projectA    string
	projectB    string
	sites       *app.SiteRegistry
	runtimes    *app.PHPRegistry
	services    *app.ServiceManager
	phpManager  *app.PHPManager
	activation  *app.SiteActivationService
	siteMgmt    *app.SiteManagementService
	onboarding  *app.RuntimeOnboardingService
	runtimeView *app.RuntimeCatalogService
	health      *app.HealthService
	resolver    *fakeResolverManager
	packages    *fakePackageManager
}

func newRouterTestDeps(t *testing.T) *routerTestDeps {
	t.Helper()

	dir := t.TempDir()
	sites := app.NewSiteRegistry(filepath.Join(dir, "sites.json"))
	runtimes := app.NewPHPRegistry(filepath.Join(dir, "runtimes.json"))

	binary82 := filepath.Join(dir, "php82")
	writeVersionExecutable(t, binary82, "8.2")
	if _, err := runtimes.Register(context.Background(), app.RuntimeRegistrationInput{Version: "8.2", BinaryPath: binary82, FPMService: "php8.2-fpm", Source: "test"}); err != nil {
		t.Fatalf("register runtime 8.2: %v", err)
	}

	binary83 := filepath.Join(dir, "php83")
	writeVersionExecutable(t, binary83, "8.3")
	if _, err := runtimes.Register(context.Background(), app.RuntimeRegistrationInput{Version: "8.3", BinaryPath: binary83, FPMService: "php8.3-fpm", Source: "test"}); err != nil {
		t.Fatalf("register runtime 8.3: %v", err)
	}

	if _, err := runtimes.SetDefault(context.Background(), "8.2"); err != nil {
		t.Fatalf("set default runtime: %v", err)
	}

	projectA := createLaravelProject(t, filepath.Join(dir, "alpha"))
	projectB := createLaravelProject(t, filepath.Join(dir, "bravo"))

	fakeServices := &fakeHostServiceManager{}
	serviceManager := app.NewServiceManager(fakeServices)
	fakePackages := &fakePackageManager{}
	fakeResolver := &fakeResolverManager{}
	fakeWeb := &fakeWebManager{}
	fakePHP := &fakePHPManager{}

	return &routerTestDeps{
		projectA:    projectA,
		projectB:    projectB,
		sites:       sites,
		runtimes:    runtimes,
		services:    serviceManager,
		phpManager:  app.NewPHPManager(sites, runtimes, serviceManager, fakePHP),
		activation:  app.NewSiteActivationService(sites, runtimes, fakeResolver, fakeWeb, fakePHP, serviceManager),
		siteMgmt:    app.NewSiteManagementService(sites, runtimes, fakeWeb, fakePHP, serviceManager),
		onboarding:  app.NewRuntimeOnboardingService(runtimes, fakePackages, runtimes, fakePHP, serviceManager),
		runtimeView: app.NewRuntimeCatalogService(runtimes, fakePackages),
		health:      app.NewHealthService(sites, runtimes, serviceManager, fakeResolver, filepath.Join(dir, "lara-nux.sock")),
		resolver:    fakeResolver,
		packages:    fakePackages,
	}
}

func (d *routerTestDeps) routerDeps() RouterDependencies {
	return RouterDependencies{
		HealthService:         d.health,
		PHPManager:            d.phpManager,
		ServiceManager:        d.services,
		SiteActivationService: d.activation,
		SiteManagementService: d.siteMgmt,
		RuntimeOnboarding:     d.onboarding,
		RuntimeCatalogService: d.runtimeView,
		ResolverManager:       d.resolver,
	}
}

type seedSiteInput struct {
	RootPath   string
	Domain     string
	PHPVersion string
}

func seedSite(t *testing.T, deps *routerTestDeps, input seedSiteInput) app.SiteRecord {
	t.Helper()
	record, err := deps.sites.Register(context.Background(), app.SiteRegistrationInput{
		RootPath:   input.RootPath,
		Domain:     input.Domain,
		PHPVersion: input.PHPVersion,
	})
	if err != nil {
		t.Fatalf("seed site: %v", err)
	}
	return record
}

func performJSONRequest(t *testing.T, handler http.Handler, method string, target string, body any) *httptest.ResponseRecorder {
	t.Helper()

	var payload []byte
	if body != nil {
		var err error
		payload, err = json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal body: %v", err)
		}
	}

	request := httptest.NewRequest(method, target, bytes.NewReader(payload))
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	return recorder
}

func decodeEnvelope[T any](t *testing.T, response *httptest.ResponseRecorder, target *T) {
	t.Helper()
	if err := json.Unmarshal(response.Body.Bytes(), target); err != nil {
		t.Fatalf("decode response: %v\nbody=%s", err, response.Body.String())
	}
}

func createLaravelProject(t *testing.T, path string) string {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(path, "public"), 0o755); err != nil {
		t.Fatalf("mkdir project: %v", err)
	}
	for rel, content := range map[string]string{
		"artisan":          "#!/usr/bin/env php\n",
		"composer.json":    "{}\n",
		"public/index.php": "<?php\n",
	} {
		full := filepath.Join(path, rel)
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatalf("write %s: %v", rel, err)
		}
	}
	return path
}

func writeVersionExecutable(t *testing.T, path string, version string) {
	t.Helper()
	if err := os.WriteFile(path, []byte("#!/bin/sh\nprintf '"+version+"'\n"), 0o755); err != nil {
		t.Fatalf("write executable: %v", err)
	}
}

type fakeResolverManager struct{}

func (*fakeResolverManager) Inspect(context.Context, host.ResolverStubSpec) (host.ResolverStatus, error) {
	return host.ResolverStatus{Managed: true, Owner: "lara-nux", Summary: "managed"}, nil
}
func (*fakeResolverManager) EnsureTestStub(context.Context, host.ResolverStubSpec) (host.ResolverStatus, error) {
	return host.ResolverStatus{Managed: true, Owner: "lara-nux", Summary: "managed"}, nil
}
func (*fakeResolverManager) RemoveManagedStub(context.Context) error { return nil }

type fakeWebManager struct{}

func (*fakeWebManager) ActivateSite(context.Context, host.WebSite) (host.WebActivationResult, error) {
	return host.WebActivationResult{ConfigPath: "/tmp/site.caddy", Validated: true, Reloaded: true, HTTPURL: "http://example.test", HTTPSURL: "https://example.test"}, nil
}
func (*fakeWebManager) RemoveSite(context.Context, string) error { return nil }
func (*fakeWebManager) Validate(context.Context) error           { return nil }

type fakePHPManager struct{}

func (*fakePHPManager) MaterializeRuntime(context.Context, host.PHPRuntime) (host.PHPMaterialization, error) {
	return host.PHPMaterialization{Version: "8.2", ServiceName: "php-fpm", SocketPath: "/run/php/test.sock", PoolConfigPath: "/tmp/pool.conf", OverridePath: "/tmp/override.conf", Active: true}, nil
}
func (*fakePHPManager) SwitchRuntime(_ context.Context, request host.PHPSwitchRequest) (host.PHPMaterialization, error) {
	return host.PHPMaterialization{Version: request.Target.Version, ServiceName: request.Target.ServiceName, SocketPath: request.Target.SocketPath, PoolConfigPath: "/tmp/pool.conf", OverridePath: "/tmp/override.conf", Active: true}, nil
}
func (*fakePHPManager) RemoveRuntime(context.Context, host.PHPRuntime) error { return nil }

type fakeHostServiceManager struct{}

func (*fakeHostServiceManager) Action(_ context.Context, service string, action host.ServiceAction) (host.ServiceStatus, error) {
	return host.ServiceStatus{Service: service, State: host.ServiceStateActive, Summary: string(action), UpdatedAt: time.Unix(100, 0).UTC()}, nil
}
func (f *fakeHostServiceManager) Start(ctx context.Context, service string) (host.ServiceStatus, error) {
	return f.Action(ctx, service, host.ServiceActionStart)
}
func (f *fakeHostServiceManager) Stop(ctx context.Context, service string) (host.ServiceStatus, error) {
	return f.Action(ctx, service, host.ServiceActionStop)
}
func (f *fakeHostServiceManager) Restart(ctx context.Context, service string) (host.ServiceStatus, error) {
	return f.Action(ctx, service, host.ServiceActionRestart)
}
func (f *fakeHostServiceManager) Status(ctx context.Context, service string) (host.ServiceStatus, error) {
	return f.Action(ctx, service, host.ServiceActionStatus)
}

type fakePackageManager struct{}

func (*fakePackageManager) SupportedPackages() []host.SupportedPackage {
	return []host.SupportedPackage{{Key: "php-8.2", Description: "PHP 8.2", RuntimeVersion: "8.2", Packages: []string{"php8.2", "php8.2-fpm"}}}
}
func (*fakePackageManager) Acquire(context.Context, host.PackageRequest) (host.PackageReceipt, error) {
	return host.PackageReceipt{}, errors.New("not implemented")
}
func (*fakePackageManager) Verify(context.Context, string) (host.PackageVerification, error) {
	return host.PackageVerification{}, errors.New("not implemented")
}
func (*fakePackageManager) RefreshRuntimeInventory(context.Context) ([]host.PHPRuntime, error) {
	return []host.PHPRuntime{{Version: "8.2", BinaryPath: "/usr/bin/php8.2", FPMBinaryPath: "/usr/sbin/php-fpm8.2", ServiceName: "php8.2-fpm", SocketPath: "/run/php/lara-nux-php8.2-fpm.sock"}}, nil
}
