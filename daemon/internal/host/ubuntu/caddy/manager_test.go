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
	previous := []byte("# previous config\n")
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
	if len(runner.calls) != 1 || !strings.Contains(runner.calls[0], "caddy validate") {
		t.Fatalf("unexpected runner calls: %v", runner.calls)
	}
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
	payload, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read golden file %s: %v", path, err)
	}
	return string(payload)
}
