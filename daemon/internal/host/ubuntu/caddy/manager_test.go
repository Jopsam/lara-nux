package caddy

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jopsam/lara-nux/daemon/internal/host"
)

func TestActivateSiteWritesGoldenConfigAndReloads(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	project := filepath.Join(dir, "demo")
	publicDir := filepath.Join(project, "public")
	if err := os.MkdirAll(publicDir, 0o755); err != nil {
		t.Fatalf("mkdir public dir: %v", err)
	}

	runner := &caddyRunner{}
	manager := NewManager(Config{
		SitesDir:       filepath.Join(dir, "sites"),
		RootConfigPath: filepath.Join(dir, "Caddyfile"),
		Runner:         runner,
	})

	result, err := manager.ActivateSite(context.Background(), host.WebSite{
		ID:            "demo-site",
		Domain:        "Demo.Test",
		RootPath:      project,
		PHPSocketPath: "/run/php/lara-nux-php8.2-fpm.sock",
	})
	if err != nil {
		t.Fatalf("activate site: %v", err)
	}

	if result.ConfigPath == "" || !result.Validated || !result.Reloaded {
		t.Fatalf("unexpected activation result: %+v", result)
	}

	configPayload, err := os.ReadFile(result.ConfigPath)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	rootConfig, err := os.ReadFile(filepath.Join(dir, "Caddyfile"))
	if err != nil {
		t.Fatalf("read root config: %v", err)
	}
	if !strings.Contains(string(rootConfig), managedImportLine) {
		t.Fatalf("expected root config to contain %q, got %s", managedImportLine, string(rootConfig))
	}

	actual := string(configPayload)
	actual = strings.ReplaceAll(actual, publicDir, "__PUBLIC_DIR__")
	actual = strings.ReplaceAll(actual, "/run/php/lara-nux-php8.2-fpm.sock", "__PHP_SOCKET__")
	actual = strings.ReplaceAll(actual, "demo.test", "__DOMAIN__")

	expected := loadGoldenText(t, filepath.Join("testdata", "activate_site.golden"))
	if actual != expected {
		t.Fatalf("config mismatch\nexpected:\n%s\nactual:\n%s", expected, actual)
	}

	expectedCalls := []string{
		"caddy validate --config " + filepath.Join(dir, "Caddyfile") + " --adapter caddyfile",
		"systemctl reload caddy",
	}
	if fmt.Sprint(runner.calls) != fmt.Sprint(expectedCalls) {
		t.Fatalf("unexpected runner calls: %v", runner.calls)
	}
}

func TestActivateSiteRestoresPreviousConfigOnValidationFailure(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	project := filepath.Join(dir, "demo")
	publicDir := filepath.Join(project, "public")
	if err := os.MkdirAll(publicDir, 0o755); err != nil {
		t.Fatalf("mkdir public dir: %v", err)
	}

	sitesDir := filepath.Join(dir, "sites")
	if err := os.MkdirAll(sitesDir, 0o755); err != nil {
		t.Fatalf("mkdir sites dir: %v", err)
	}
	configPath := filepath.Join(sitesDir, "demo-site.caddy")
	previous := loadGoldenBytes(t, filepath.Join("testdata", "existing_site.caddy"))
	if err := os.WriteFile(configPath, previous, 0o644); err != nil {
		t.Fatalf("write existing config: %v", err)
	}

	runner := &caddyRunner{results: map[string]caddyRunResult{
		"caddy validate --config " + filepath.Join(dir, "Caddyfile") + " --adapter caddyfile": {output: "invalid site", err: errors.New("exit status 1")},
	}}
	manager := NewManager(Config{SitesDir: sitesDir, RootConfigPath: filepath.Join(dir, "Caddyfile"), Runner: runner})

	_, err := manager.ActivateSite(context.Background(), host.WebSite{
		ID:            "demo-site",
		Domain:        "demo.test",
		RootPath:      project,
		PHPSocketPath: "/run/php/lara-nux-php8.2-fpm.sock",
	})
	if !errors.Is(err, host.ErrActivationValidation) {
		t.Fatalf("expected ErrActivationValidation, got %v", err)
	}

	restored, readErr := os.ReadFile(configPath)
	if readErr != nil {
		t.Fatalf("read restored config: %v", readErr)
	}
	if string(restored) != string(previous) {
		t.Fatalf("expected previous config to be restored, got %q", string(restored))
	}
	if _, statErr := os.Stat(filepath.Join(dir, "Caddyfile")); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("expected root config rollback to remove generated file, stat err=%v", statErr)
	}
	if len(runner.calls) != 1 || !strings.Contains(runner.calls[0], "caddy validate") {
		t.Fatalf("unexpected runner calls: %v", runner.calls)
	}
}

func TestActivateSiteRestoresPreviousConfigOnReloadFailure(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	project := filepath.Join(dir, "demo")
	publicDir := filepath.Join(project, "public")
	if err := os.MkdirAll(publicDir, 0o755); err != nil {
		t.Fatalf("mkdir public dir: %v", err)
	}

	sitesDir := filepath.Join(dir, "sites")
	if err := os.MkdirAll(sitesDir, 0o755); err != nil {
		t.Fatalf("mkdir sites dir: %v", err)
	}
	configPath := filepath.Join(sitesDir, "demo-site.caddy")
	previous := loadGoldenBytes(t, filepath.Join("testdata", "existing_site.caddy"))
	if err := os.WriteFile(configPath, previous, 0o644); err != nil {
		t.Fatalf("write existing config: %v", err)
	}

	validateCall := "caddy validate --config " + filepath.Join(dir, "Caddyfile") + " --adapter caddyfile"
	reloadCall := "systemctl reload caddy"
	runner := &caddyRunner{results: map[string]caddyRunResult{
		reloadCall: {output: "reload failed", err: errors.New("exit status 1")},
	}}
	manager := NewManager(Config{SitesDir: sitesDir, RootConfigPath: filepath.Join(dir, "Caddyfile"), Runner: runner})

	_, err := manager.ActivateSite(context.Background(), host.WebSite{
		ID:            "demo-site",
		Domain:        "demo.test",
		RootPath:      project,
		PHPSocketPath: "/run/php/lara-nux-php8.2-fpm.sock",
	})
	if err == nil || !strings.Contains(err.Error(), "reload caddy after activating") {
		t.Fatalf("expected reload failure, got %v", err)
	}

	restored, readErr := os.ReadFile(configPath)
	if readErr != nil {
		t.Fatalf("read restored config: %v", readErr)
	}
	if string(restored) != string(previous) {
		t.Fatalf("expected previous config to be restored, got %q", string(restored))
	}
	if _, statErr := os.Stat(filepath.Join(dir, "Caddyfile")); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("expected root config rollback to remove generated file, stat err=%v", statErr)
	}

	expectedCalls := []string{validateCall, reloadCall, validateCall, reloadCall}
	if fmt.Sprint(runner.calls) != fmt.Sprint(expectedCalls) {
		t.Fatalf("unexpected runner calls: %v", runner.calls)
	}
}

func TestRemoveSiteRemovesManagedRootImportWhenLastSiteIsRemoved(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	sitesDir := filepath.Join(dir, "sites")
	if err := os.MkdirAll(sitesDir, 0o755); err != nil {
		t.Fatalf("mkdir sites dir: %v", err)
	}
	configPath := filepath.Join(sitesDir, "demo-site.caddy")
	if err := os.WriteFile(configPath, []byte(managedMarker+"\nhttps://demo.test {\n}\n"), 0o644); err != nil {
		t.Fatalf("write site config: %v", err)
	}
	rootPath := filepath.Join(dir, "Caddyfile")
	if err := os.WriteFile(rootPath, []byte(managedImportStartLine+"\n"+managedImportLine+"\n"+managedImportEndLine+"\n"), 0o644); err != nil {
		t.Fatalf("write root config: %v", err)
	}

	runner := &caddyRunner{}
	manager := NewManager(Config{SitesDir: sitesDir, RootConfigPath: rootPath, Runner: runner})

	if err := manager.RemoveSite(context.Background(), "demo-site"); err != nil {
		t.Fatalf("remove site: %v", err)
	}
	if _, err := os.Stat(rootPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected root config removal, stat err=%v", err)
	}
}

func TestActivateSiteNormalizesPermissionsForManagedPaths(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	project := filepath.Join(dir, "demo")
	publicDir := filepath.Join(project, "public")
	if err := os.MkdirAll(publicDir, 0o755); err != nil {
		t.Fatalf("mkdir public dir: %v", err)
	}

	rootDir := filepath.Join(dir, "root")
	sitesDir := filepath.Join(rootDir, "sites.d", "lara-nux")
	if err := os.MkdirAll(filepath.Dir(sitesDir), 0o700); err != nil {
		t.Fatalf("mkdir sites parent: %v", err)
	}
	if err := os.MkdirAll(sitesDir, 0o700); err != nil {
		t.Fatalf("mkdir sites dir: %v", err)
	}
	rootPath := filepath.Join(rootDir, "Caddyfile")
	if err := os.WriteFile(rootPath, []byte(":80 {}\n"), 0o600); err != nil {
		t.Fatalf("write root config: %v", err)
	}

	runner := &caddyRunner{}
	manager := NewManager(Config{SitesDir: sitesDir, RootConfigPath: rootPath, Runner: runner})

	_, err := manager.ActivateSite(context.Background(), host.WebSite{
		ID:            "demo-site",
		Domain:        "demo.test",
		RootPath:      project,
		PHPSocketPath: "/run/php/lara-nux-php8.2-fpm.sock",
	})
	if err != nil {
		t.Fatalf("activate site: %v", err)
	}

	assertMode := func(path string, want os.FileMode) {
		t.Helper()
		info, statErr := os.Stat(path)
		if statErr != nil {
			t.Fatalf("stat %s: %v", path, statErr)
		}
		if got := info.Mode().Perm(); got != want {
			t.Fatalf("permissions for %s = %o, want %o", path, got, want)
		}
	}

	assertMode(rootDir, 0o755)
	assertMode(filepath.Dir(sitesDir), 0o755)
	assertMode(sitesDir, 0o755)
	assertMode(rootPath, 0o644)
	assertMode(filepath.Join(sitesDir, "demo-site.caddy"), 0o644)
}

type caddyRunResult struct {
	output string
	err    error
}

type caddyRunner struct {
	results map[string]caddyRunResult
	calls   []string
}

func (r *caddyRunner) Run(_ context.Context, name string, args ...string) (string, error) {
	call := strings.TrimSpace(strings.Join(append([]string{name}, args...), " "))
	r.calls = append(r.calls, call)
	if result, ok := r.results[call]; ok {
		return result.output, result.err
	}
	return "", nil
}

func loadGoldenText(t *testing.T, path string) string {
	t.Helper()
	return string(loadGoldenBytes(t, path))
}

func loadGoldenBytes(t *testing.T, path string) []byte {
	t.Helper()
	payload, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read golden file %s: %v", path, err)
	}
	return payload
}
