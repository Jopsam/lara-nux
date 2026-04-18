package app

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestBootstrapPreflightAcceptsSupportedUbuntuHosts(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	service := newBootstrapServiceForTests(t, dir, "ID=ubuntu\nVERSION_ID=22.04\nVERSION_CODENAME=jammy\nPRETTY_NAME=Ubuntu 22.04 LTS\n")

	report, err := service.Preflight(context.Background())
	if err != nil {
		t.Fatalf("Preflight returned error: %v", err)
	}
	if report.Host.ID != "ubuntu" || report.Host.VersionID != "22.04" {
		t.Fatalf("unexpected host report: %+v", report.Host)
	}
	if report.HasFailures() {
		t.Fatalf("expected supported preflight to pass, got %+v", report.Checks)
	}
	if len(report.Manifest.RollbackTargets()) == 0 {
		t.Fatal("expected rollback targets in managed asset manifest")
	}
	assertBootstrapCheck(t, report, "ubuntu-host", true)
	assertBootstrapCheck(t, report, "ubuntu-release", true)
	assertBootstrapCheck(t, report, "privileges", true)
	assertBootstrapCheck(t, report, "daemon-group", true)
	assertBootstrapCheck(t, report, "dependency-systemctl", true)
	assertBootstrapCheck(t, report, "dependency-resolvectl", true)
	assertBootstrapCheck(t, report, "rollback-manifest", true)
}

func TestBootstrapPreflightRejectsUnsupportedUbuntuRelease(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	service := newBootstrapServiceForTests(t, dir, "ID=ubuntu\nVERSION_ID=23.10\nVERSION_CODENAME=mantic\nPRETTY_NAME=Ubuntu 23.10\n")

	report, err := service.Preflight(context.Background())
	if err == nil {
		t.Fatal("expected unsupported release to fail preflight")
	}
	if !report.HasFailures() {
		t.Fatal("expected failure checks for unsupported release")
	}
	assertBootstrapCheck(t, report, "ubuntu-host", true)
	assertBootstrapCheck(t, report, "ubuntu-release", false)
}

func TestBootstrapPrepareFilesystemAndManifestPersistence(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	paths := AppPaths{
		ConfigDir:         filepath.Join(dir, "etc", "lara-nux"),
		StateDir:          filepath.Join(dir, "var", "lib", "lara-nux"),
		RuntimeDir:        filepath.Join(dir, "run", "lara-nux"),
		SocketPath:        filepath.Join(dir, "run", "lara-nux", "lara-nux.sock"),
		ManagedAssetsPath: filepath.Join(dir, "var", "lib", "lara-nux", "managed-assets.json"),
	}

	service := NewBootstrapService(paths)
	if err := service.PrepareFilesystem(); err != nil {
		t.Fatalf("PrepareFilesystem returned error: %v", err)
	}

	for _, target := range []string{paths.ConfigDir, paths.StateDir, paths.RuntimeDir} {
		info, err := os.Stat(target)
		if err != nil {
			t.Fatalf("stat %s: %v", target, err)
		}
		if !info.IsDir() {
			t.Fatalf("expected directory at %s", target)
		}
	}

	manifest := DefaultUbuntuManagedAssets(paths)
	if err := manifest.Save(paths.ManagedAssetsPath); err != nil {
		t.Fatalf("Save returned error: %v", err)
	}

	loaded, err := LoadManagedAssetManifest(paths.ManagedAssetsPath)
	if err != nil {
		t.Fatalf("LoadManagedAssetManifest returned error: %v", err)
	}
	if loaded.TargetOS != "ubuntu" {
		t.Fatalf("expected ubuntu target OS, got %s", loaded.TargetOS)
	}
	if len(loaded.RollbackTargets()) == 0 {
		t.Fatal("expected persisted manifest rollback targets")
	}
	manifestAssetFound := false
	for _, asset := range loaded.Assets {
		if asset.Path == paths.ManagedAssetsPath {
			manifestAssetFound = true
			break
		}
	}
	if !manifestAssetFound {
		t.Fatalf("expected manifest asset for %s", paths.ManagedAssetsPath)
	}
}

func newBootstrapServiceForTests(t *testing.T, dir string, osRelease string) *BootstrapService {
	t.Helper()

	osReleasePath := filepath.Join(dir, "os-release")
	if err := os.WriteFile(osReleasePath, []byte(osRelease), 0o644); err != nil {
		t.Fatalf("write os-release fixture: %v", err)
	}

	service := NewBootstrapService(AppPaths{
		ConfigDir:         filepath.Join(dir, "etc", "lara-nux"),
		StateDir:          filepath.Join(dir, "var", "lib", "lara-nux"),
		RuntimeDir:        filepath.Join(dir, "run", "lara-nux"),
		SocketPath:        filepath.Join(dir, "run", "lara-nux", "lara-nux.sock"),
		ManagedAssetsPath: filepath.Join(dir, "var", "lib", "lara-nux", "managed-assets.json"),
	})
	service.osReleasePath = osReleasePath
	service.lookPath = func(name string) (string, error) { return "/usr/bin/" + name, nil }
	service.geteuid = func() int { return 0 }
	service.groupExists = func(name string) bool { return name == "lara-nux" }
	return service
}

func assertBootstrapCheck(t *testing.T, report BootstrapReport, name string, passed bool) {
	t.Helper()
	for _, check := range report.Checks {
		if check.Name == name {
			if check.Passed != passed {
				t.Fatalf("check %s passed=%v want %v", name, check.Passed, passed)
			}
			return
		}
	}
	t.Fatalf("missing bootstrap check %s", name)
}
