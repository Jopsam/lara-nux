package app

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidateLaravelPathRejectsMissingRequirements(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		mutate   func(t *testing.T, project string)
		wantText string
	}{
		{
			name: "missing artisan",
			mutate: func(t *testing.T, project string) {
				t.Helper()
				if err := os.Remove(filepath.Join(project, "artisan")); err != nil {
					t.Fatalf("remove artisan: %v", err)
				}
			},
			wantText: "missing artisan",
		},
		{
			name: "missing public index",
			mutate: func(t *testing.T, project string) {
				t.Helper()
				if err := os.Remove(filepath.Join(project, "public", "index.php")); err != nil {
					t.Fatalf("remove public/index.php: %v", err)
				}
			},
			wantText: "missing public/index.php",
		},
		{
			name: "artisan must be a file",
			mutate: func(t *testing.T, project string) {
				t.Helper()
				artisanPath := filepath.Join(project, "artisan")
				if err := os.Remove(artisanPath); err != nil {
					t.Fatalf("remove artisan file: %v", err)
				}
				if err := os.MkdirAll(artisanPath, 0o755); err != nil {
					t.Fatalf("replace artisan with dir: %v", err)
				}
			},
			wantText: "expected file at artisan",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			project := createSiteRegistryLaravelProjectFixture(t)
			tt.mutate(t, project)

			err := ValidateLaravelPath(project)
			if !errors.Is(err, ErrInvalidLaravelPath) {
				t.Fatalf("expected ErrInvalidLaravelPath, got %v", err)
			}
			if err == nil || !containsText(err.Error(), tt.wantText) {
				t.Fatalf("expected error containing %q, got %v", tt.wantText, err)
			}
		})
	}
}

func TestSiteRegistryRejectsDuplicateDomainCaseInsensitive(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	registry := NewSiteRegistry(filepath.Join(dir, "sites.json"))

	alpha := createSiteRegistryLaravelProjectFixture(t)
	beta := createNamedSiteRegistryLaravelProjectFixture(t, "beta-app")

	if _, err := registry.Register(context.Background(), SiteRegistrationInput{
		RootPath:   alpha,
		Domain:     "Demo.Test",
		PHPVersion: "8.2",
	}); err != nil {
		t.Fatalf("register first site: %v", err)
	}

	_, err := registry.Register(context.Background(), SiteRegistrationInput{
		RootPath:   beta,
		Domain:     "demo.test",
		PHPVersion: "8.2",
	})
	if !errors.Is(err, ErrDuplicateDomain) {
		t.Fatalf("expected ErrDuplicateDomain, got %v", err)
	}

	sites, listErr := registry.List(context.Background())
	if listErr != nil {
		t.Fatalf("list sites: %v", listErr)
	}
	if len(sites) != 1 {
		t.Fatalf("expected 1 persisted site, got %d", len(sites))
	}
	if sites[0].Domain != "demo.test" {
		t.Fatalf("expected normalized persisted domain, got %s", sites[0].Domain)
	}
}

func createSiteRegistryLaravelProjectFixture(t *testing.T) string {
	t.Helper()
	return createNamedSiteRegistryLaravelProjectFixture(t, "demo-app")
}

func createNamedSiteRegistryLaravelProjectFixture(t *testing.T, name string) string {
	t.Helper()

	project := filepath.Join(t.TempDir(), name)
	for rel, content := range map[string]string{
		"artisan":          "#!/usr/bin/env php\n",
		"composer.json":    "{}\n",
		"public/index.php": "<?php\n",
	} {
		path := filepath.Join(project, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", rel, err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatalf("write %s: %v", rel, err)
		}
	}

	return project
}

func containsText(haystack string, needle string) bool {
	return strings.Contains(haystack, needle)
}
