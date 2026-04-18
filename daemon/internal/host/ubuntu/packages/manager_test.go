package packages

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/jopsam/lara-nux/daemon/internal/host"
)

func TestAcquireInstallsSupportedBundleAndVerifiesIt(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.April, 18, 12, 0, 0, 0, time.UTC)
	runner := &packageRunner{results: map[string]packageRunResult{
		"apt-get install -y --no-install-recommends php8.2 php8.2-cli php8.2-fpm php8.2-curl php8.2-mbstring php8.2-xml php8.2-zip php8.2-sqlite3": {output: "ok"},
		"dpkg-query -W -f=${Status}|${Version} php8.2":          {output: "install ok installed|8.2.12"},
		"dpkg-query -W -f=${Status}|${Version} php8.2-cli":      {output: "install ok installed|8.2.12"},
		"dpkg-query -W -f=${Status}|${Version} php8.2-fpm":      {output: "install ok installed|8.2.12"},
		"dpkg-query -W -f=${Status}|${Version} php8.2-curl":     {output: "install ok installed|8.2.12"},
		"dpkg-query -W -f=${Status}|${Version} php8.2-mbstring": {output: "install ok installed|8.2.12"},
		"dpkg-query -W -f=${Status}|${Version} php8.2-xml":      {output: "install ok installed|8.2.12"},
		"dpkg-query -W -f=${Status}|${Version} php8.2-zip":      {output: "install ok installed|8.2.12"},
		"dpkg-query -W -f=${Status}|${Version} php8.2-sqlite3":  {output: "install ok installed|8.2.12"},
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

	runner := &packageRunner{results: map[string]packageRunResult{
		"dpkg-query -W -f=${Status}|${Version} caddy": {output: "deinstall ok config-files|2.7.6", err: errors.New("not installed")},
	}}
	manager := NewManager(Config{Runner: runner})

	verification, err := manager.Verify(context.Background(), "caddy")
	if !errors.Is(err, host.ErrPackageVerification) {
		t.Fatalf("expected ErrPackageVerification, got %v", err)
	}
	if verification.Verified {
		t.Fatal("expected verification to fail")
	}
	if got := verification.Details["caddy"]; got != "deinstall ok config-files|2.7.6" {
		t.Fatalf("unexpected package detail: %q", got)
	}
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
