package packages

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jopsam/lara-nux/daemon/internal/host"
)

func TestAcquireInstallsSupportedBundleAndVerifiesIt(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.April, 18, 12, 0, 0, 0, time.UTC)
	installedStatus := strings.TrimSpace(string(loadPackageFixture(t, "php82_installed.status")))
	runner := &packageRunner{results: map[string]packageRunResult{
		"apt-get install -y --no-install-recommends php8.2 php8.2-cli php8.2-fpm php8.2-curl php8.2-mbstring php8.2-xml php8.2-zip php8.2-sqlite3": {output: "ok"},
		"dpkg-query -W -f=${Status}|${Version} php8.2":          {output: installedStatus},
		"dpkg-query -W -f=${Status}|${Version} php8.2-cli":      {output: installedStatus},
		"dpkg-query -W -f=${Status}|${Version} php8.2-fpm":      {output: installedStatus},
		"dpkg-query -W -f=${Status}|${Version} php8.2-curl":     {output: installedStatus},
		"dpkg-query -W -f=${Status}|${Version} php8.2-mbstring": {output: installedStatus},
		"dpkg-query -W -f=${Status}|${Version} php8.2-xml":      {output: installedStatus},
		"dpkg-query -W -f=${Status}|${Version} php8.2-zip":      {output: installedStatus},
		"dpkg-query -W -f=${Status}|${Version} php8.2-sqlite3":  {output: installedStatus},
	}}
	manager := NewManager(Config{Runner: runner, Clock: func() time.Time { return now }})

	receipt, err := manager.Acquire(context.Background(), host.PackageRequest{Key: "php-8.2"})
	if err != nil {
		t.Fatalf("acquire package bundle: %v", err)
	}
	if receipt.Key != "php-8.2" {
		t.Fatalf("unexpected receipt key: %+v", receipt)
	}
	if !receipt.InstalledAt.Equal(now) {
		t.Fatalf("unexpected InstalledAt: %s", receipt.InstalledAt)
	}
	if len(receipt.Packages) != 8 {
		t.Fatalf("expected 8 packages, got %d", len(receipt.Packages))
	}
	if runner.calls[0] != "apt-get install -y --no-install-recommends php8.2 php8.2-cli php8.2-fpm php8.2-curl php8.2-mbstring php8.2-xml php8.2-zip php8.2-sqlite3" {
		t.Fatalf("expected apt-get install first, got %v", runner.calls)
	}
}

func TestVerifyReturnsPackageDetailsWhenBundleIsIncomplete(t *testing.T) {
	t.Parallel()

	missingStatus := strings.TrimSpace(string(loadPackageFixture(t, "caddy_missing.status")))
	runner := &packageRunner{results: map[string]packageRunResult{
		"dpkg-query -W -f=${Status}|${Version} caddy": {output: missingStatus, err: errors.New("not installed")},
	}}
	manager := NewManager(Config{Runner: runner})

	verification, err := manager.Verify(context.Background(), "caddy")
	if !errors.Is(err, host.ErrPackageVerification) {
		t.Fatalf("expected ErrPackageVerification, got %v", err)
	}
	if verification.Verified {
		t.Fatal("expected verification to fail")
	}
	if got := verification.Details["caddy"]; got != missingStatus {
		t.Fatalf("unexpected package detail: %q", got)
	}
}

func loadPackageFixture(t *testing.T, name string) []byte {
	t.Helper()
	payload, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("read package fixture %s: %v", name, err)
	}
	return payload
}

type packageRunResult struct {
	output string
	err    error
}

type packageRunner struct {
	results map[string]packageRunResult
	calls   []string
}

func (r *packageRunner) Run(_ context.Context, name string, args ...string) (string, error) {
	call := strings.TrimSpace(strings.Join(append([]string{name}, args...), " "))
	r.calls = append(r.calls, call)
	if result, ok := r.results[call]; ok {
		return result.output, result.err
	}
	return "", fmt.Errorf("unexpected command: %s", call)
}
