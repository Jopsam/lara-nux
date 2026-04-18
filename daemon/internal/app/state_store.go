package app

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

func loadStateFile(path string, target any) error {
	payload, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}

		return fmt.Errorf("read state file %s: %w", path, err)
	}

	if len(payload) == 0 {
		return nil
	}

	if err := json.Unmarshal(payload, target); err != nil {
		return fmt.Errorf("decode state file %s: %w", path, err)
	}

	return nil
}

func saveStateFile(path string, value any, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return fmt.Errorf("create state directory for %s: %w", path, err)
	}

	payload, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return fmt.Errorf("encode state file %s: %w", path, err)
	}

	tempPath := path + ".tmp"
	if err := os.WriteFile(tempPath, payload, mode); err != nil {
		return fmt.Errorf("write state file %s: %w", path, err)
	}

	if err := os.Rename(tempPath, path); err != nil {
		return fmt.Errorf("commit state file %s: %w", path, err)
	}

	return nil
}
