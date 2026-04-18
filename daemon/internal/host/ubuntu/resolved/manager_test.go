package resolved

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/jopsam/lara-nux/daemon/internal/host"
)

type fakeRunner struct{}

func (fakeRunner) Run(context.Context, string, ...string) (string, error) { return "", nil }

func TestEnsureTestStubRejectsResolverConflictsWithoutMutatingManagedStub(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	dropInDir := filepath.Join(dir, "resolved.conf.d")
	if err := os.MkdirAll(dropInDir, 0o755); err != nil {
		t.Fatalf("mkdir drop-in dir: %v", err)
	}

	mainConfig := filepath.Join(dir, "resolved.conf")
	if err := os.WriteFile(mainConfig, loadResolvedFixture(t, "main_resolved.conf"), 0o644); err != nil {
		t.Fatalf("write main config: %v", err)
	}
	conflictPath := filepath.Join(dropInDir, "external.conf")
	if err := os.WriteFile(conflictPath, loadResolvedFixture(t, "external_test_domain.conf"), 0o644); err != nil {
		t.Fatalf("write conflict config: %v", err)
	}

	stubPath := filepath.Join(dropInDir, "lara-nux-test.conf")
	manager := NewManager(Config{
		MainConfigPath: mainConfig,
		DropInDir:      dropInDir,
		StubPath:       stubPath,
		Runner:         fakeRunner{},
	})

	_, err := manager.EnsureTestStub(context.Background(), host.ResolverStubSpec{Domain: "test", Address: "127.0.0.1"})
	if !errors.Is(err, host.ErrResolverConflict) {
		t.Fatalf("expected ErrResolverConflict, got %v", err)
	}
	if _, statErr := os.Stat(stubPath); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("expected managed stub to remain absent, stat err=%v", statErr)
	}
}

func loadResolvedFixture(t *testing.T, name string) []byte {
	t.Helper()
	payload, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("read resolved fixture %s: %v", name, err)
	}
	return payload
}
