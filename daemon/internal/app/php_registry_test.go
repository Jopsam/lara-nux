package app

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestPHPRegistryRejectsVersionMismatch(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	binaryPath := filepath.Join(dir, "php82")
	writeExecutable(t, binaryPath)

	registry := NewPHPRegistry(filepath.Join(dir, "state.json"))
	registry.run = func(context.Context, string, ...string) (string, error) {
		return "8.2", nil
	}

	_, err := registry.Register(context.Background(), RuntimeRegistrationInput{
		Version:    "8.3",
		BinaryPath: binaryPath,
	})
	if !errors.Is(err, ErrUnverifiablePHP) {
		t.Fatalf("expected ErrUnverifiablePHP, got %v", err)
	}

	runtimes, listErr := registry.List(context.Background())
	if listErr != nil {
		t.Fatalf("List returned error: %v", listErr)
	}
	if len(runtimes) != 0 {
		t.Fatalf("expected no persisted runtimes after failed verification, got %d", len(runtimes))
	}
}

func writeExecutable(t *testing.T, path string) {
	t.Helper()
	if err := os.WriteFile(path, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatalf("write executable: %v", err)
	}
}
