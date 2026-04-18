package ubuntu

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jopsam/lara-nux/daemon/internal/app"
	"github.com/jopsam/lara-nux/daemon/internal/host"
	caddyhost "github.com/jopsam/lara-nux/daemon/internal/host/ubuntu/caddy"
	phphost "github.com/jopsam/lara-nux/daemon/internal/host/ubuntu/php"
	resolvedhost "github.com/jopsam/lara-nux/daemon/internal/host/ubuntu/resolved"
)

func TestUbuntuLTSManagedWorkflowEvidence(t *testing.T) {
	t.Parallel()

	for _, fixtureName := range []string{"jammy.json", "noble.json"} {
		fixtureName := fixtureName
		t.Run(strings.TrimSuffix(fixtureName, filepath.Ext(fixtureName)), func(t *testing.T) {
			t.Parallel()

			scenario := loadScenario(t, fixtureName)
			dir := t.TempDir()
			projectRoot := createLaravelProject(t, filepath.Join(dir, "projects", scenario.Codename))

			runtimes := app.NewPHPRegistry(filepath.Join(dir, "state", "runtimes.json"))
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
					return "", os.ErrNotExist
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

			sites := app.NewSiteRegistry(filepath.Join(dir, "state", "sites.json"))
			hostServices := &managedWorkflowServiceManager{}
			serviceManager := app.NewServiceManager(hostServices)
			runner := &managedWorkflowRunner{}

			resolvedMain := filepath.Join(dir, "etc", "systemd", "resolved.conf")
			if err := os.MkdirAll(filepath.Dir(resolvedMain), 0o755); err != nil {
				t.Fatalf("mkdir resolved main dir: %v", err)
			}
			if err := os.WriteFile(resolvedMain, []byte("[Resolve]\n"), 0o644); err != nil {
				t.Fatalf("write resolved main config: %v", err)
			}

			caddyRoot := filepath.Join(dir, "etc", "caddy", "Caddyfile")
			if err := os.MkdirAll(filepath.Dir(caddyRoot), 0o755); err != nil {
				t.Fatalf("mkdir caddy root dir: %v", err)
			}
			if err := os.WriteFile(caddyRoot, []byte("import sites.d/lara-nux/*.caddy\n"), 0o644); err != nil {
				t.Fatalf("write caddy root config: %v", err)
			}

			resolverManager := resolvedhost.NewManager(resolvedhost.Config{
				MainConfigPath: resolvedMain,
				DropInDir:      filepath.Join(dir, "etc", "systemd", "resolved.conf.d"),
				StubPath:       filepath.Join(dir, "etc", "systemd", "resolved.conf.d", "lara-nux-test.conf"),
				Runner:         runner,
			})
			webManager := caddyhost.NewManager(caddyhost.Config{
				SitesDir:       filepath.Join(dir, "etc", "caddy", "sites.d", "lara-nux"),
				RootConfigPath: caddyRoot,
				Runner:         runner,
			})
			phpManagerHost := phphost.NewManager(phphost.Config{
				PHPRootDir: filepath.Join(dir, "etc", "php"),
				SystemdDir: filepath.Join(dir, "etc", "systemd", "system"),
				SocketDir:  filepath.Join(dir, "run", "php"),
				Runner:     runner,
			})

			activationService := app.NewSiteActivationService(sites, runtimes, resolverManager, webManager, phpManagerHost, serviceManager)
			phpManager := app.NewPHPManager(sites, runtimes, serviceManager, phpManagerHost, webManager)

			activation, err := activationService.Activate(context.Background(), app.SiteRegistrationInput{
				RootPath:   projectRoot,
				Domain:     scenario.Domain,
				PHPVersion: scenario.DefaultPHP,
			})
			if err != nil {
				t.Fatalf("Activate returned error: %v", err)
			}
			if activation.Site.Domain != scenario.Domain {
				t.Fatalf("expected domain %s, got %s", scenario.Domain, activation.Site.Domain)
			}
			if activation.Web.HTTPSURL != "https://"+scenario.Domain {
				t.Fatalf("expected HTTPS URL for %s, got %s", scenario.Domain, activation.Web.HTTPSURL)
			}
			if len(activation.Services) != 2 {
				t.Fatalf("expected two started services, got %+v", activation.Services)
			}
			if !hostServices.started("caddy") || !hostServices.started("php"+scenario.DefaultPHP+"-fpm") {
				t.Fatalf("expected managed services to start, got %+v", hostServices.startCalls)
			}

			assertFileContains(t, filepath.Join(dir, "etc", "systemd", "resolved.conf.d", "lara-nux-test.conf"), "Domains=~test")
			assertFileContains(t, filepath.Join(dir, "etc", "systemd", "resolved.conf.d", "lara-nux-test.conf"), "DNS=127.0.0.1")
			assertFileContains(t, activation.Web.ConfigPath, "https://"+scenario.Domain)
			assertFileContains(t, activation.Web.ConfigPath, filepath.Join(projectRoot, "public"))
			assertFileContains(t, filepath.Join(dir, "etc", "php", scenario.DefaultPHP, "fpm", "pool.d", "lara-nux.conf"), activation.Materialization.SocketPath)

			switched, err := phpManager.SwitchSiteRuntime(context.Background(), activation.Site.ID, scenario.SwitchPHP)
			if err != nil {
				t.Fatalf("SwitchSiteRuntime returned error: %v", err)
			}
			if switched.PHPVersion != scenario.SwitchPHP {
				t.Fatalf("expected switched PHP %s, got %s", scenario.SwitchPHP, switched.PHPVersion)
			}
			assertFileContains(t, activation.Web.ConfigPath, "/run/php/lara-nux-php"+scenario.SwitchPHP+"-fpm.sock")
			assertFileContains(t, filepath.Join(dir, "etc", "php", scenario.SwitchPHP, "fpm", "pool.d", "lara-nux.conf"), "/run/php/lara-nux-php"+scenario.SwitchPHP+"-fpm.sock")
			if !runner.called("systemctl restart php" + scenario.SwitchPHP + "-fpm") {
				t.Fatalf("expected runtime restart for %s, got %+v", scenario.SwitchPHP, runner.calls)
			}

			if err := webManager.RemoveSite(context.Background(), activation.Site.ID); err != nil {
				t.Fatalf("RemoveSite returned error: %v", err)
			}
			if err := resolverManager.RemoveManagedStub(context.Background()); err != nil {
				t.Fatalf("RemoveManagedStub returned error: %v", err)
			}
			for _, version := range []string{scenario.DefaultPHP, scenario.SwitchPHP} {
				if err := phpManagerHost.RemoveRuntime(context.Background(), host.PHPRuntime{Version: version, ServiceName: "php" + version + "-fpm"}); err != nil {
					t.Fatalf("RemoveRuntime(%s) returned error: %v", version, err)
				}
			}

			assertPathMissing(t, activation.Web.ConfigPath)
			assertPathMissing(t, filepath.Join(dir, "etc", "systemd", "resolved.conf.d", "lara-nux-test.conf"))
			assertPathMissing(t, filepath.Join(dir, "etc", "php", scenario.DefaultPHP, "fpm", "pool.d", "lara-nux.conf"))
			assertPathMissing(t, filepath.Join(dir, "etc", "php", scenario.SwitchPHP, "fpm", "pool.d", "lara-nux.conf"))
			if _, err := os.Stat(filepath.Join(projectRoot, "artisan")); err != nil {
				t.Fatalf("expected Laravel project to remain after cleanup: %v", err)
			}
		})
	}
}

type managedWorkflowRunner struct {
	calls []string
}

func (r *managedWorkflowRunner) Run(_ context.Context, name string, args ...string) (string, error) {
	r.calls = append(r.calls, strings.TrimSpace(name+" "+strings.Join(args, " ")))
	return "", nil
}

func (r *managedWorkflowRunner) called(fragment string) bool {
	for _, call := range r.calls {
		if strings.Contains(call, fragment) {
			return true
		}
	}
	return false
}

type managedWorkflowServiceManager struct {
	startCalls []string
}

func (m *managedWorkflowServiceManager) Action(_ context.Context, service string, action host.ServiceAction) (host.ServiceStatus, error) {
	if action == host.ServiceActionStart {
		m.startCalls = append(m.startCalls, service)
	}
	return host.ServiceStatus{Service: service, State: host.ServiceStateActive, Summary: string(action), UpdatedAt: time.Date(2026, time.April, 18, 18, 0, 0, 0, time.UTC)}, nil
}

func (m *managedWorkflowServiceManager) Start(ctx context.Context, service string) (host.ServiceStatus, error) {
	return m.Action(ctx, service, host.ServiceActionStart)
}

func (m *managedWorkflowServiceManager) Stop(ctx context.Context, service string) (host.ServiceStatus, error) {
	return m.Action(ctx, service, host.ServiceActionStop)
}

func (m *managedWorkflowServiceManager) Restart(ctx context.Context, service string) (host.ServiceStatus, error) {
	return m.Action(ctx, service, host.ServiceActionRestart)
}

func (m *managedWorkflowServiceManager) Status(ctx context.Context, service string) (host.ServiceStatus, error) {
	return m.Action(ctx, service, host.ServiceActionStatus)
}

func (m *managedWorkflowServiceManager) started(service string) bool {
	for _, call := range m.startCalls {
		if call == service {
			return true
		}
	}
	return false
}

func assertFileContains(t *testing.T, path string, want string) {
	t.Helper()
	payload, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	if !strings.Contains(string(payload), want) {
		t.Fatalf("expected %s to contain %q, got %s", path, want, string(payload))
	}
}

func assertPathMissing(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("expected %s to be removed, stat err=%v", path, err)
	}
}
