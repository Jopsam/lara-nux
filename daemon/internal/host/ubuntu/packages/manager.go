package packages

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/jopsam/lara-nux/daemon/internal/host"
)

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
	Runner commandRunner
	Clock  func() time.Time
}

type Manager struct {
	runner commandRunner
	clock  func() time.Time
}

var _ host.PackageManager = (*Manager)(nil)

func NewManager(config Config) *Manager {
	runner := config.Runner
	if runner == nil {
		runner = execRunner{}
	}

	clock := config.Clock
	if clock == nil {
		clock = func() time.Time { return time.Now().UTC() }
	}

	return &Manager{runner: runner, clock: clock}
}

func (m *Manager) SupportedPackages() []host.SupportedPackage {
	packages := supportedCatalog()
	sort.Slice(packages, func(i, j int) bool {
		return packages[i].Key < packages[j].Key
	})
	return packages
}

func (m *Manager) Acquire(ctx context.Context, request host.PackageRequest) (host.PackageReceipt, error) {
	supported, err := findSupported(request.Key)
	if err != nil {
		return host.PackageReceipt{}, err
	}

	args := append([]string{"install", "-y", "--no-install-recommends"}, supported.Packages...)
	output, runErr := m.runner.Run(ctx, "apt-get", args...)
	if runErr != nil {
		return host.PackageReceipt{}, fmt.Errorf("install package bundle %s: %v (%s)", supported.Key, runErr, strings.TrimSpace(output))
	}

	verification, err := m.Verify(ctx, supported.Key)
	if err != nil {
		return host.PackageReceipt{}, err
	}
	if !verification.Verified {
		return host.PackageReceipt{}, fmt.Errorf("%w: package bundle %s did not verify after installation", host.ErrPackageVerification, supported.Key)
	}

	return host.PackageReceipt{
		Key:         supported.Key,
		Packages:    append([]string(nil), supported.Packages...),
		InstalledAt: m.clock(),
	}, nil
}

func (m *Manager) Verify(ctx context.Context, key string) (host.PackageVerification, error) {
	supported, err := findSupported(key)
	if err != nil {
		return host.PackageVerification{}, err
	}

	verification := host.PackageVerification{
		Key:      supported.Key,
		Verified: true,
		Details:  map[string]string{},
	}

	for _, packageName := range supported.Packages {
		status, verifyErr := m.inspectPackage(ctx, packageName)
		verification.Details[packageName] = status
		if verifyErr != nil {
			verification.Verified = false
		}
	}

	if !verification.Verified {
		return verification, fmt.Errorf("%w: bundle %s has missing or unverified packages", host.ErrPackageVerification, supported.Key)
	}

	return verification, nil
}

func (m *Manager) RefreshRuntimeInventory(ctx context.Context) ([]host.PHPRuntime, error) {
	inventory := []host.PHPRuntime{}
	for _, supported := range supportedCatalog() {
		if supported.RuntimeVersion == "" {
			continue
		}

		verification, err := m.Verify(ctx, supported.Key)
		if err != nil || !verification.Verified {
			continue
		}

		binaryPath := filepath.Join("/usr/bin", "php"+supported.RuntimeVersion)
		fpmBinaryPath := filepath.Join("/usr/sbin", "php-fpm"+supported.RuntimeVersion)
		if !fileExists(binaryPath) || !fileExists(fpmBinaryPath) {
			continue
		}

		inventory = append(inventory, host.PHPRuntime{
			Version:       supported.RuntimeVersion,
			BinaryPath:    binaryPath,
			FPMBinaryPath: fpmBinaryPath,
			ServiceName:   "php" + supported.RuntimeVersion + "-fpm",
			SocketPath:    filepath.Join("/run/php", "lara-nux-php"+supported.RuntimeVersion+"-fpm.sock"),
		})
	}

	sort.Slice(inventory, func(i, j int) bool {
		return inventory[i].Version < inventory[j].Version
	})

	return inventory, nil
}

func (m *Manager) inspectPackage(ctx context.Context, packageName string) (string, error) {
	output, err := m.runner.Run(ctx, "dpkg-query", "-W", "-f=${Status}|${Version}", packageName)
	status := strings.TrimSpace(output)
	if err != nil {
		return status, err
	}
	if !strings.HasPrefix(status, "install ok installed|") {
		return status, fmt.Errorf("package %s is not installed", packageName)
	}
	return status, nil
}

func findSupported(key string) (host.SupportedPackage, error) {
	normalized := strings.ToLower(strings.TrimSpace(key))
	for _, supported := range supportedCatalog() {
		if supported.Key == normalized {
			return supported, nil
		}
	}

	return host.SupportedPackage{}, fmt.Errorf("%w: %s", host.ErrUnsupportedPackage, key)
}

func supportedCatalog() []host.SupportedPackage {
	packages := []host.SupportedPackage{{
		Key:         "caddy",
		Description: "Local TLS/web server for Lara Nux sites.",
		Packages:    []string{"caddy"},
	}}

	for _, version := range []string{"8.1", "8.2", "8.3", "8.4"} {
		packages = append(packages, host.SupportedPackage{
			Key:            "php-" + version,
			Description:    "Supported PHP runtime " + version + " for Laravel sites.",
			RuntimeVersion: version,
			Packages: []string{
				"php" + version,
				"php" + version + "-cli",
				"php" + version + "-fpm",
				"php" + version + "-curl",
				"php" + version + "-mbstring",
				"php" + version + "-xml",
				"php" + version + "-zip",
				"php" + version + "-sqlite3",
			},
		})
	}

	return packages
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}
