package php

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/jopsam/lara-nux/daemon/internal/host"
)

const managedMarker = "# Managed by Lara Nux"

type commandRunner interface {
	Run(ctx context.Context, name string, args ...string) (string, error)
}

type execRunner struct{}

func (execRunner) Run(ctx context.Context, name string, args ...string) (string, error) {
	command := exec.CommandContext(ctx, name, args...)
	output, err := command.CombinedOutput()
	return string(output), err
}

type Config struct {
	PHPRootDir string
	SystemdDir string
	SocketDir  string
	Runner     commandRunner
}

type Manager struct {
	phpRootDir string
	systemdDir string
	socketDir  string
	runner     commandRunner
}

var _ host.PHPManager = (*Manager)(nil)

func NewManager(config Config) *Manager {
	phpRootDir := strings.TrimSpace(config.PHPRootDir)
	if phpRootDir == "" {
		phpRootDir = "/etc/php"
	}

	systemdDir := strings.TrimSpace(config.SystemdDir)
	if systemdDir == "" {
		systemdDir = "/etc/systemd/system"
	}

	socketDir := strings.TrimSpace(config.SocketDir)
	if socketDir == "" {
		socketDir = "/run/php"
	}

	runner := config.Runner
	if runner == nil {
		runner = execRunner{}
	}

	return &Manager{phpRootDir: phpRootDir, systemdDir: systemdDir, socketDir: socketDir, runner: runner}
}

func (m *Manager) MaterializeRuntime(ctx context.Context, runtime host.PHPRuntime) (host.PHPMaterialization, error) {
	runtime, materialization, err := m.normalizeRuntime(runtime)
	if err != nil {
		return host.PHPMaterialization{}, err
	}

	poolBackup, poolExists, err := readIfExists(materialization.PoolConfigPath)
	if err != nil {
		return host.PHPMaterialization{}, fmt.Errorf("read php-fpm pool backup: %w", err)
	}

	overrideBackup, overrideExists, err := readIfExists(materialization.OverridePath)
	if err != nil {
		return host.PHPMaterialization{}, fmt.Errorf("read php-fpm override backup: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(materialization.PoolConfigPath), 0o755); err != nil {
		return host.PHPMaterialization{}, fmt.Errorf("create php pool directory: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(materialization.OverridePath), 0o755); err != nil {
		return host.PHPMaterialization{}, fmt.Errorf("create systemd override directory: %w", err)
	}
	if err := os.MkdirAll(m.socketDir, 0o755); err != nil {
		return host.PHPMaterialization{}, fmt.Errorf("create php socket directory: %w", err)
	}

	if err := writeAtomically(materialization.PoolConfigPath, []byte(renderPoolConfig(runtime)), 0o644); err != nil {
		return host.PHPMaterialization{}, fmt.Errorf("write php-fpm pool config: %w", err)
	}
	if err := writeAtomically(materialization.OverridePath, []byte(renderOverrideConfig()), 0o644); err != nil {
		_ = restoreFile(materialization.PoolConfigPath, poolBackup, poolExists, 0o644)
		return host.PHPMaterialization{}, fmt.Errorf("write php-fpm override config: %w", err)
	}

	rollback := func() {
		_ = restoreFile(materialization.PoolConfigPath, poolBackup, poolExists, 0o644)
		_ = restoreFile(materialization.OverridePath, overrideBackup, overrideExists, 0o644)
		_ = m.daemonReload(ctx)
		_, _ = m.runner.Run(ctx, "systemctl", "restart", runtime.ServiceName)
	}

	if err := m.daemonReload(ctx); err != nil {
		rollback()
		return host.PHPMaterialization{}, fmt.Errorf("reload systemd before php-fpm validation: %w", err)
	}

	if err := m.validateRuntime(ctx, runtime); err != nil {
		rollback()
		return host.PHPMaterialization{}, err
	}

	if err := m.restartService(ctx, runtime.ServiceName); err != nil {
		rollback()
		return host.PHPMaterialization{}, err
	}

	materialization.Active = true
	return materialization, nil
}

func (m *Manager) SwitchRuntime(ctx context.Context, request host.PHPSwitchRequest) (host.PHPMaterialization, error) {
	materialization, err := m.MaterializeRuntime(ctx, request.Target)
	if err != nil {
		if strings.TrimSpace(request.Previous.ServiceName) != "" {
			_, _ = m.runner.Run(ctx, "systemctl", "restart", request.Previous.ServiceName)
		}
		return host.PHPMaterialization{}, fmt.Errorf("%w: %s", host.ErrRuntimeSwitchRollback, err.Error())
	}

	return materialization, nil
}

func (m *Manager) RemoveRuntime(ctx context.Context, runtime host.PHPRuntime) error {
	_, materialization, err := m.normalizeRuntime(runtime)
	if err != nil {
		return err
	}

	if err := removeManagedFile(materialization.PoolConfigPath); err != nil {
		return err
	}
	if err := removeManagedFile(materialization.OverridePath); err != nil {
		return err
	}

	if err := m.daemonReload(ctx); err != nil {
		return err
	}

	return m.restartService(ctx, materialization.ServiceName)
}

func (m *Manager) normalizeRuntime(runtime host.PHPRuntime) (host.PHPRuntime, host.PHPMaterialization, error) {
	version := strings.TrimPrefix(strings.TrimSpace(strings.ToLower(runtime.Version)), "php")
	if version == "" {
		return host.PHPRuntime{}, host.PHPMaterialization{}, fmt.Errorf("%w: php version is required", host.ErrActivationValidation)
	}

	serviceName := strings.TrimSpace(runtime.ServiceName)
	if serviceName == "" {
		serviceName = "php" + version + "-fpm"
	}

	fpmBinaryPath := strings.TrimSpace(runtime.FPMBinaryPath)
	if fpmBinaryPath == "" {
		fpmBinaryPath = filepath.Join("/usr/sbin", "php-fpm"+version)
	}

	socketPath := strings.TrimSpace(runtime.SocketPath)
	if socketPath == "" {
		socketPath = filepath.Join(m.socketDir, "lara-nux-php"+version+"-fpm.sock")
	}

	runtime.Version = version
	runtime.ServiceName = serviceName
	runtime.FPMBinaryPath = fpmBinaryPath
	runtime.SocketPath = socketPath

	materialization := host.PHPMaterialization{
		Version:        version,
		ServiceName:    serviceName,
		SocketPath:     socketPath,
		PoolConfigPath: filepath.Join(m.phpRootDir, version, "fpm", "pool.d", "lara-nux.conf"),
		OverridePath:   filepath.Join(m.systemdDir, serviceName+".d", "lara-nux.conf"),
	}

	return runtime, materialization, nil
}

func (m *Manager) validateRuntime(ctx context.Context, runtime host.PHPRuntime) error {
	output, err := m.runner.Run(ctx, runtime.FPMBinaryPath, "-t")
	if err != nil {
		return fmt.Errorf("validate php-fpm runtime %s: %v (%s)", runtime.Version, err, strings.TrimSpace(output))
	}
	return nil
}

func (m *Manager) daemonReload(ctx context.Context) error {
	output, err := m.runner.Run(ctx, "systemctl", "daemon-reload")
	if err != nil {
		return fmt.Errorf("systemd daemon-reload failed: %v (%s)", err, strings.TrimSpace(output))
	}
	return nil
}

func (m *Manager) restartService(ctx context.Context, serviceName string) error {
	output, err := m.runner.Run(ctx, "systemctl", "restart", serviceName)
	if err != nil {
		return fmt.Errorf("restart php-fpm service %s: %v (%s)", serviceName, err, strings.TrimSpace(output))
	}
	return nil
}

func renderPoolConfig(runtime host.PHPRuntime) string {
	return fmt.Sprintf(`%s
[lara-nux]
user = www-data
group = www-data
listen = %s
listen.owner = www-data
listen.group = www-data
pm = dynamic
pm.max_children = 10
pm.start_servers = 2
pm.min_spare_servers = 1
pm.max_spare_servers = 4
chdir = /
`, managedMarker, runtime.SocketPath)
}

func renderOverrideConfig() string {
	return managedMarker + "\n[Service]\nRuntimeDirectory=php\nRuntimeDirectoryMode=0755\n"
}

func readIfExists(path string) ([]byte, bool, error) {
	payload, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, false, nil
		}
		return nil, false, err
	}
	return payload, true, nil
}

func writeAtomically(path string, payload []byte, mode os.FileMode) error {
	tempPath := path + ".tmp"
	if err := os.WriteFile(tempPath, payload, mode); err != nil {
		return err
	}
	return os.Rename(tempPath, path)
}

func restoreFile(path string, payload []byte, existed bool, mode os.FileMode) error {
	if !existed {
		if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		return nil
	}
	return writeAtomically(path, payload, mode)
}

func removeManagedFile(path string) error {
	payload, exists, err := readIfExists(path)
	if err != nil || !exists {
		return err
	}
	if !strings.Contains(string(payload), managedMarker) {
		return fmt.Errorf("%w: php-fpm asset %s is not owned by Lara Nux", host.ErrManagedAssetConflict, path)
	}
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("remove managed php-fpm asset %s: %w", path, err)
	}
	return nil
}
