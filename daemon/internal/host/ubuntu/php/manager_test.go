package php

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

func TestMaterializeRuntimeWritesGoldenManagedConfigs(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	runner := &phpRunner{}
	manager := NewManager(Config{
		PHPRootDir: filepath.Join(dir, "php"),
		SystemdDir: filepath.Join(dir, "systemd"),
		SocketDir:  filepath.Join(dir, "run", "php"),
		Runner:     runner,
	})

	materialization, err := manager.MaterializeRuntime(context.Background(), host.PHPRuntime{Version: "8.2"})
	if err != nil {
		t.Fatalf("materialize runtime: %v", err)
	}
	if !materialization.Active {
		t.Fatal("expected materialization to be active")
	}

	poolConfig, err := os.ReadFile(materialization.PoolConfigPath)
	if err != nil {
		t.Fatalf("read pool config: %v", err)
	}
	overrideConfig, err := os.ReadFile(materialization.OverridePath)
	if err != nil {
		t.Fatalf("read override config: %v", err)
	}

	poolActual := strings.ReplaceAll(string(poolConfig), materialization.SocketPath, "__SOCKET_PATH__")
	poolExpected := loadGoldenText(t, filepath.Join("testdata", "pool_config.golden"))
	if poolActual != poolExpected {
		t.Fatalf("pool config mismatch\nexpected:\n%s\nactual:\n%s", poolExpected, poolActual)
	}

	overrideExpected := loadGoldenText(t, filepath.Join("testdata", "override_config.golden"))
	if string(overrideConfig) != overrideExpected {
		t.Fatalf("override config mismatch\nexpected:\n%s\nactual:\n%s", overrideExpected, string(overrideConfig))
	}

	expectedCalls := []string{
		"systemctl daemon-reload",
		"/usr/sbin/php-fpm8.2 -t",
		"systemctl restart php8.2-fpm",
	}
	if fmt.Sprint(runner.calls) != fmt.Sprint(expectedCalls) {
		t.Fatalf("unexpected runner calls: %v", runner.calls)
	}
}

func TestMaterializeRuntimeRestoresBackupsWhenValidationFails(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	phpRoot := filepath.Join(dir, "php")
	systemdRoot := filepath.Join(dir, "systemd")
	poolPath := filepath.Join(phpRoot, "8.2", "fpm", "pool.d", "lara-nux.conf")
	overridePath := filepath.Join(systemdRoot, "php8.2-fpm.d", "lara-nux.conf")
	if err := os.MkdirAll(filepath.Dir(poolPath), 0o755); err != nil {
		t.Fatalf("mkdir pool dir: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(overridePath), 0o755); err != nil {
		t.Fatalf("mkdir override dir: %v", err)
	}
	if err := os.WriteFile(poolPath, loadGoldenBytes(t, filepath.Join("testdata", "previous_pool.conf")), 0o644); err != nil {
		t.Fatalf("write old pool config: %v", err)
	}
	if err := os.WriteFile(overridePath, loadGoldenBytes(t, filepath.Join("testdata", "previous_override.conf")), 0o644); err != nil {
		t.Fatalf("write old override config: %v", err)
	}

	runner := &phpRunner{results: map[string]phpRunResult{
		"/usr/sbin/php-fpm8.2 -t": {output: "broken config", err: errors.New("exit status 78")},
	}}
	manager := NewManager(Config{PHPRootDir: phpRoot, SystemdDir: systemdRoot, SocketDir: filepath.Join(dir, "run", "php"), Runner: runner})

	_, err := manager.MaterializeRuntime(context.Background(), host.PHPRuntime{Version: "8.2"})
	if err == nil || !strings.Contains(err.Error(), "validate php-fpm runtime 8.2") {
		t.Fatalf("expected validation error, got %v", err)
	}

	restoredPool, readErr := os.ReadFile(poolPath)
	if readErr != nil {
		t.Fatalf("read restored pool config: %v", readErr)
	}
	restoredOverride, readErr := os.ReadFile(overridePath)
	if readErr != nil {
		t.Fatalf("read restored override config: %v", readErr)
	}
	if string(restoredPool) != string(loadGoldenBytes(t, filepath.Join("testdata", "previous_pool.conf"))) {
		t.Fatalf("expected pool config rollback, got %q", string(restoredPool))
	}
	if string(restoredOverride) != string(loadGoldenBytes(t, filepath.Join("testdata", "previous_override.conf"))) {
		t.Fatalf("expected override rollback, got %q", string(restoredOverride))
	}

	expectedCalls := []string{
		"systemctl daemon-reload",
		"/usr/sbin/php-fpm8.2 -t",
		"systemctl daemon-reload",
		"systemctl restart php8.2-fpm",
	}
	if fmt.Sprint(runner.calls) != fmt.Sprint(expectedCalls) {
		t.Fatalf("unexpected runner calls: %v", runner.calls)
	}
}

func TestSwitchRuntimeRestartsPreviousServiceWhenTargetFails(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	runner := &phpRunner{results: map[string]phpRunResult{
		"/usr/sbin/php-fpm8.3 -t": {output: "broken config", err: errors.New("exit status 78")},
	}}
	manager := NewManager(Config{PHPRootDir: filepath.Join(dir, "php"), SystemdDir: filepath.Join(dir, "systemd"), SocketDir: filepath.Join(dir, "run", "php"), Runner: runner})

	_, err := manager.SwitchRuntime(context.Background(), host.PHPSwitchRequest{
		SiteID: "site-1",
		Previous: host.PHPRuntime{
			Version:     "8.2",
			ServiceName: "php8.2-fpm",
		},
		Target: host.PHPRuntime{Version: "8.3"},
	})
	if !errors.Is(err, host.ErrRuntimeSwitchRollback) {
		t.Fatalf("expected ErrRuntimeSwitchRollback, got %v", err)
	}

	expectedCalls := []string{
		"systemctl daemon-reload",
		"/usr/sbin/php-fpm8.3 -t",
		"systemctl daemon-reload",
		"systemctl restart php8.3-fpm",
		"systemctl restart php8.2-fpm",
	}
	if fmt.Sprint(runner.calls) != fmt.Sprint(expectedCalls) {
		t.Fatalf("unexpected runner calls: %v", runner.calls)
	}
}

type phpRunResult struct {
	output string
	err    error
}

type phpRunner struct {
	results map[string]phpRunResult
	calls   []string
}

func (r *phpRunner) Run(_ context.Context, name string, args ...string) (string, error) {
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
