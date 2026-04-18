package app

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestPHPRegistryRejectsUnsupportedRuntime(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	binaryPath := filepath.Join(dir, "php80")
	writeExecutable(t, binaryPath)

	registry := NewPHPRegistry(filepath.Join(dir, "state.json"))
	registry.run = func(context.Context, string, ...string) (string, error) {
		return "8.0", nil
	}

	_, err := registry.Register(context.Background(), RuntimeRegistrationInput{
		Version:    "8.0",
		BinaryPath: binaryPath,
	})
	if !errors.Is(err, ErrUnsupportedRuntime) {
		t.Fatalf("expected ErrUnsupportedRuntime, got %v", err)
	}

	runtimes, listErr := registry.List(context.Background())
	if listErr != nil {
		t.Fatalf("List returned error: %v", listErr)
	}
	if len(runtimes) != 0 {
		t.Fatalf("expected no persisted runtimes after unsupported registration, got %d", len(runtimes))
	}
}

func TestPHPRegistryRegistersSupportedRuntime(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	binaryPath := filepath.Join(dir, "php82")
	writeExecutable(t, binaryPath)

	registry := NewPHPRegistry(filepath.Join(dir, "state.json"))
	registry.run = func(context.Context, string, ...string) (string, error) {
		return "8.2", nil
	}

	record, err := registry.Register(context.Background(), RuntimeRegistrationInput{
		Version:    "8.2",
		BinaryPath: binaryPath,
		Source:     "ppa:ondrej/php",
	})
	if err != nil {
		t.Fatalf("expected supported runtime registration to pass, got %v", err)
	}
	if record.Version != "8.2" {
		t.Fatalf("expected version 8.2, got %s", record.Version)
	}
	if record.FPMService != "php8.2-fpm" {
		t.Fatalf("expected derived FPM service, got %s", record.FPMService)
	}

	defaultRuntime, err := registry.DefaultRuntime(context.Background())
	if err != nil {
		t.Fatalf("expected default runtime after registration, got %v", err)
	}
	if defaultRuntime.Version != "8.2" {
		t.Fatalf("expected default runtime 8.2, got %s", defaultRuntime.Version)
	}
}

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
