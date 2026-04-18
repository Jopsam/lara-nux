package ubuntu

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
	"unsafe"

	"github.com/jopsam/lara-nux/daemon/internal/api"
	"github.com/jopsam/lara-nux/daemon/internal/app"
	"github.com/jopsam/lara-nux/daemon/internal/host"
)

func TestUbuntuLTSWorkflowScaffold(t *testing.T) {
	t.Parallel()

	for _, fixtureName := range []string{"jammy.json", "noble.json"} {
		fixtureName := fixtureName
		t.Run(strings.TrimSuffix(fixtureName, filepath.Ext(fixtureName)), func(t *testing.T) {
			t.Parallel()

			scenario := loadScenario(t, fixtureName)
			testDeps := newE2EDeps(t, scenario)
			router := api.NewRouter(testDeps.routerDeps())

			assertPackagingAssetsForScenario(t, scenario)

			activation := postJSON(t, router, "/rpc/sites.register", map[string]any{
				"rootPath":   testDeps.projectRoot,
				"domain":     scenario.Domain,
				"phpVersion": scenario.DefaultPHP,
			}, http.StatusCreated)

			var activationPayload struct {
				OK   bool `json:"ok"`
				Data struct {
					Site app.SiteRecord `json:"site"`
					Web  struct {
						HTTPSURL string `json:"httpsUrl"`
					} `json:"web"`
				} `json:"data"`
			}
			decodeBody(t, activation.Body.Bytes(), &activationPayload)
			if activationPayload.Data.Site.Domain != scenario.Domain {
				t.Fatalf("expected domain %s, got %s", scenario.Domain, activationPayload.Data.Site.Domain)
			}
			if activationPayload.Data.Web.HTTPSURL != "https://"+scenario.Domain {
				t.Fatalf("expected HTTPS URL for %s, got %s", scenario.Domain, activationPayload.Data.Web.HTTPSURL)
			}

			health := getJSON(t, router, "/rpc/health", http.StatusOK)
			var healthPayload struct {
				OK   bool `json:"ok"`
				Data struct {
					Ready bool `json:"ready"`
					Sites []struct {
						Ready bool           `json:"ready"`
						Site  app.SiteRecord `json:"site"`
					} `json:"sites"`
				} `json:"data"`
			}
			decodeBody(t, health.Body.Bytes(), &healthPayload)
			if len(healthPayload.Data.Sites) != 1 || !healthPayload.Data.Sites[0].Ready {
				t.Fatalf("expected one ready site, got %s", health.Body.String())
			}

			switchResp := postJSON(t, router, "/rpc/php.switch", map[string]any{
				"siteId":     activationPayload.Data.Site.ID,
				"phpVersion": scenario.SwitchPHP,
			}, http.StatusOK)
			var switchPayload struct {
				OK   bool           `json:"ok"`
				Data app.SiteRecord `json:"data"`
			}
			decodeBody(t, switchResp.Body.Bytes(), &switchPayload)
			if switchPayload.Data.PHPVersion != scenario.SwitchPHP {
				t.Fatalf("expected switched PHP %s, got %s", scenario.SwitchPHP, switchPayload.Data.PHPVersion)
			}

			if !testDeps.php.switchedToScenario(scenario.SwitchPHP) {
				t.Fatalf("expected php switch to %s, got %+v", scenario.SwitchPHP, testDeps.php.switchCalls)
			}
			if !testDeps.web.hasHTTPSDomain(scenario.Domain) {
				t.Fatalf("expected HTTPS browse scaffolding for %s, got %+v", scenario.Domain, testDeps.web.activations)
			}

			if err := testDeps.resolver.RemoveManagedStub(context.Background()); err != nil {
				t.Fatalf("uninstall cleanup remove resolver stub: %v", err)
			}
			if !testDeps.resolver.removed {
				t.Fatal("expected uninstall cleanup to remove resolver stub")
			}
		})
	}
}

type workflowScenario struct {
	Name       string `json:"name"`
	Codename   string `json:"codename"`
	DefaultPHP string `json:"defaultPHP"`
	SwitchPHP  string `json:"switchPHP"`
	Domain     string `json:"domain"`
}

type e2eDeps struct {
	projectRoot string
	sites       *app.SiteRegistry
	runtimes    *app.PHPRegistry
	services    *app.ServiceManager
	phpManager  *app.PHPManager
	activation  *app.SiteActivationService
	siteMgmt    *app.SiteManagementService
	health      *app.HealthService
	onboarding  *app.RuntimeOnboardingService
	runtimeView *app.RuntimeCatalogService
	resolver    *e2eResolverManager
	web         *e2eWebManager
	php         *e2ePHPManager
	packages    *e2ePackageManager
}

func newE2EDeps(t *testing.T, scenario workflowScenario) *e2eDeps {
	t.Helper()

	dir := t.TempDir()
	sites := app.NewSiteRegistry(filepath.Join(dir, "sites.json"))
	runtimes := app.NewPHPRegistry(filepath.Join(dir, "runtimes.json"))
	projectRoot := createLaravelProject(t, filepath.Join(dir, "projects", scenario.Codename))

	setPHPRegistryRun(t, runtimes, func(_ context.Context, binaryPath string, _ ...string) (string, error) {
		base := filepath.Base(binaryPath)
		switch {
		case strings.Contains(base, "82"):
			return "8.2", nil
		case strings.Contains(base, "83"):
			return "8.3", nil
		case strings.Contains(base, "84"):
			return "8.4", nil
		default:
			return "", errors.New("unknown runtime")
		}
	})

	for _, version := range []string{scenario.DefaultPHP, scenario.SwitchPHP} {
		binaryPath := filepath.Join(dir, "bin", "php"+strings.ReplaceAll(version, ".", ""))
		writeExecutable(t, binaryPath)
		if _, err := runtimes.Register(context.Background(), app.RuntimeRegistrationInput{
			Version:    version,
			BinaryPath: binaryPath,
			FPMService: "php" + version + "-fpm",
			Source:     scenario.Codename,
		}); err != nil {
			t.Fatalf("register runtime %s: %v", version, err)
		}
	}
	if _, err := runtimes.SetDefault(context.Background(), scenario.DefaultPHP); err != nil {
		t.Fatalf("set default runtime: %v", err)
	}

	resolver := &e2eResolverManager{}
	web := &e2eWebManager{}
	php := &e2ePHPManager{}
	packages := &e2ePackageManager{}
	hostServices := &e2eHostServiceManager{}
	services := app.NewServiceManager(hostServices)

	deps := &e2eDeps{
		projectRoot: projectRoot,
		sites:       sites,
		runtimes:    runtimes,
		services:    services,
		phpManager:  app.NewPHPManager(sites, runtimes, services, php, web),
		activation:  app.NewSiteActivationService(sites, runtimes, resolver, web, php, services),
		siteMgmt:    app.NewSiteManagementService(sites, runtimes, web, php, services),
		health:      app.NewHealthService(sites, runtimes, services, resolver, filepath.Join(dir, "lara-nux.sock")),
		onboarding:  app.NewRuntimeOnboardingService(runtimes, packages, runtimes, php, services),
		runtimeView: app.NewRuntimeCatalogService(runtimes, packages),
		resolver:    resolver,
		web:         web,
		php:         php,
		packages:    packages,
	}
	return deps
}

func (d *e2eDeps) routerDeps() api.RouterDependencies {
	return api.RouterDependencies{
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

type e2eResolverManager struct{ removed bool }

func (m *e2eResolverManager) Inspect(context.Context, host.ResolverStubSpec) (host.ResolverStatus, error) {
	return host.ResolverStatus{Managed: true, StubPath: "/etc/systemd/resolved.conf.d/lara-nux-test.conf", Domain: "test", Address: "127.0.0.1", Owner: "lara-nux", Summary: "managed"}, nil
}
func (m *e2eResolverManager) EnsureTestStub(context.Context, host.ResolverStubSpec) (host.ResolverStatus, error) {
	return host.ResolverStatus{Managed: true, StubPath: "/etc/systemd/resolved.conf.d/lara-nux-test.conf", Domain: "test", Address: "127.0.0.1", Owner: "lara-nux", Summary: "managed"}, nil
}
func (m *e2eResolverManager) RemoveManagedStub(context.Context) error {
	m.removed = true
	return nil
}

type e2eWebManager struct{ activations []host.WebSite }

func (m *e2eWebManager) ActivateSite(_ context.Context, site host.WebSite) (host.WebActivationResult, error) {
	m.activations = append(m.activations, site)
	return host.WebActivationResult{ConfigPath: "/etc/caddy/sites.d/lara-nux/" + site.ID + ".caddy", Validated: true, Reloaded: true, HTTPURL: "http://" + site.Domain, HTTPSURL: "https://" + site.Domain}, nil
}
func (m *e2eWebManager) RemoveSite(context.Context, string) error { return nil }
func (m *e2eWebManager) Validate(context.Context) error           { return nil }
func (m *e2eWebManager) hasHTTPSDomain(domain string) bool {
	for _, activation := range m.activations {
		if activation.Domain == domain {
			return true
		}
	}
	return false
}

type e2ePHPManager struct{ switchCalls []host.PHPSwitchRequest }

func (m *e2ePHPManager) MaterializeRuntime(_ context.Context, runtime host.PHPRuntime) (host.PHPMaterialization, error) {
	version := runtime.Version
	return host.PHPMaterialization{Version: version, ServiceName: "php" + version + "-fpm", SocketPath: "/run/php/lara-nux-php" + version + "-fpm.sock", PoolConfigPath: "/tmp/lara-nux.conf", OverridePath: "/tmp/lara-nux.service.conf", Active: true}, nil
}
func (m *e2ePHPManager) SwitchRuntime(_ context.Context, request host.PHPSwitchRequest) (host.PHPMaterialization, error) {
	m.switchCalls = append(m.switchCalls, request)
	version := request.Target.Version
	return host.PHPMaterialization{Version: version, ServiceName: "php" + version + "-fpm", SocketPath: "/run/php/lara-nux-php" + version + "-fpm.sock", PoolConfigPath: "/tmp/lara-nux.conf", OverridePath: "/tmp/lara-nux.service.conf", Active: true}, nil
}
func (m *e2ePHPManager) switchedToScenario(version string) bool {
	for _, call := range m.switchCalls {
		if call.Target.Version == version {
			return true
		}
	}
	return false
}

type e2ePackageManager struct{}

func (*e2ePackageManager) SupportedPackages() []host.SupportedPackage {
	return []host.SupportedPackage{{Key: "php-8.2", Description: "PHP 8.2", RuntimeVersion: "8.2", Packages: []string{"php8.2", "php8.2-fpm"}}}
}
func (*e2ePackageManager) Acquire(context.Context, host.PackageRequest) (host.PackageReceipt, error) {
	return host.PackageReceipt{}, nil
}
func (*e2ePackageManager) Verify(context.Context, string) (host.PackageVerification, error) {
	return host.PackageVerification{Verified: true}, nil
}
func (*e2ePackageManager) RefreshRuntimeInventory(context.Context) ([]host.PHPRuntime, error) {
	return []host.PHPRuntime{{Version: "8.2", BinaryPath: "/usr/bin/php8.2", FPMBinaryPath: "/usr/sbin/php-fpm8.2", ServiceName: "php8.2-fpm", SocketPath: "/run/php/lara-nux-php8.2-fpm.sock"}}, nil
}

type e2eHostServiceManager struct{}

func (*e2eHostServiceManager) Action(_ context.Context, service string, action host.ServiceAction) (host.ServiceStatus, error) {
	return host.ServiceStatus{Service: service, State: host.ServiceStateActive, Summary: string(action), UpdatedAt: time.Date(2026, time.April, 18, 18, 0, 0, 0, time.UTC)}, nil
}
func (m *e2eHostServiceManager) Start(ctx context.Context, service string) (host.ServiceStatus, error) {
	return m.Action(ctx, service, host.ServiceActionStart)
}
func (m *e2eHostServiceManager) Stop(ctx context.Context, service string) (host.ServiceStatus, error) {
	return m.Action(ctx, service, host.ServiceActionStop)
}
func (m *e2eHostServiceManager) Restart(ctx context.Context, service string) (host.ServiceStatus, error) {
	return m.Action(ctx, service, host.ServiceActionRestart)
}
func (m *e2eHostServiceManager) Status(ctx context.Context, service string) (host.ServiceStatus, error) {
	return m.Action(ctx, service, host.ServiceActionStatus)
}

func assertPackagingAssetsForScenario(t *testing.T, scenario workflowScenario) {
	t.Helper()

	repoFile := filepath.Join(repoRoot(t), "packaging", "ubuntu", "repo", "distributions")
	payload, err := os.ReadFile(repoFile)
	if err != nil {
		t.Fatalf("read repo distributions: %v", err)
	}
	if !strings.Contains(string(payload), "Codename: "+scenario.Codename) {
		t.Fatalf("expected packaging metadata to include %s", scenario.Codename)
	}

	postinstPath := filepath.Join(repoRoot(t), "packaging", "ubuntu", "scripts", "postinst")
	postinst, err := os.ReadFile(postinstPath)
	if err != nil {
		t.Fatalf("read postinst: %v", err)
	}
	if !strings.Contains(string(postinst), "root:\"$GROUP_NAME\"") {
		t.Fatal("expected install hook to repair root:lara-nux socket ownership")
	}

	postrmPath := filepath.Join(repoRoot(t), "packaging", "ubuntu", "scripts", "postrm")
	postrm, err := os.ReadFile(postrmPath)
	if err != nil {
		t.Fatalf("read postrm: %v", err)
	}
	if !strings.Contains(string(postrm), "remove_glob_if_managed \"${CADDY_DIR}/*.caddy\"") {
		t.Fatal("expected uninstall hook to remove managed Caddy configs")
	}
}

func loadScenario(t *testing.T, name string) workflowScenario {
	t.Helper()
	payload, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("read scenario fixture %s: %v", name, err)
	}

	var scenario workflowScenario
	if err := json.Unmarshal(payload, &scenario); err != nil {
		t.Fatalf("decode scenario fixture %s: %v", name, err)
	}
	return scenario
}

func postJSON(t *testing.T, handler http.Handler, path string, body any, wantStatus int) *httptest.ResponseRecorder {
	t.Helper()
	payload, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}
	request := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(payload))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != wantStatus {
		t.Fatalf("expected %d for %s, got %d: %s", wantStatus, path, response.Code, response.Body.String())
	}
	return response
}

func getJSON(t *testing.T, handler http.Handler, path string, wantStatus int) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequest(http.MethodGet, path, nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != wantStatus {
		t.Fatalf("expected %d for %s, got %d: %s", wantStatus, path, response.Code, response.Body.String())
	}
	return response
}

func decodeBody(t *testing.T, payload []byte, target any) {
	t.Helper()
	if err := json.Unmarshal(payload, target); err != nil {
		t.Fatalf("decode body: %v", err)
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
		filePath := filepath.Join(path, rel)
		if err := os.WriteFile(filePath, []byte(content), 0o644); err != nil {
			t.Fatalf("write %s: %v", rel, err)
		}
	}
	return path
}

func writeExecutable(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir bin dir: %v", err)
	}
	if err := os.WriteFile(path, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatalf("write executable: %v", err)
	}
}

func setPHPRegistryRun(t *testing.T, registry *app.PHPRegistry, run func(context.Context, string, ...string) (string, error)) {
	t.Helper()
	field := reflect.ValueOf(registry).Elem().FieldByName("run")
	reflect.NewAt(field.Type(), unsafe.Pointer(field.UnsafeAddr())).Elem().Set(reflect.ValueOf(run))
}

func repoRoot(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	return filepath.Clean(filepath.Join(wd, "..", "..", ".."))
}
