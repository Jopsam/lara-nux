package app

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

type ManagedAssetKind string

const (
	ManagedAssetDirectory   ManagedAssetKind = "directory"
	ManagedAssetSocket      ManagedAssetKind = "socket"
	ManagedAssetServiceUnit ManagedAssetKind = "service-unit"
	ManagedAssetResolver    ManagedAssetKind = "resolver"
	ManagedAssetWebConfig   ManagedAssetKind = "web-config"
	ManagedAssetStateFile   ManagedAssetKind = "state-file"
)

type RollbackAction string

const (
	RollbackRemove          RollbackAction = "remove"
	RollbackRestorePrevious RollbackAction = "restore-previous"
	RollbackLeave           RollbackAction = "leave"
)

type ManagedAsset struct {
	ID             string           `json:"id"`
	Kind           ManagedAssetKind `json:"kind"`
	Path           string           `json:"path"`
	Owner          string           `json:"owner"`
	Group          string           `json:"group"`
	Mode           string           `json:"mode,omitempty"`
	RollbackAction RollbackAction   `json:"rollbackAction"`
	Notes          string           `json:"notes,omitempty"`
}

type ManagedAssetManifest struct {
	Format      string         `json:"format"`
	TargetOS    string         `json:"targetOs"`
	GeneratedAt time.Time      `json:"generatedAt"`
	Assets      []ManagedAsset `json:"assets"`
}

func DefaultUbuntuManagedAssets(paths AppPaths) ManagedAssetManifest {
	return ManagedAssetManifest{
		Format:      "lara-nux.managed-assets/v1",
		TargetOS:    "ubuntu",
		GeneratedAt: time.Now().UTC(),
		Assets: []ManagedAsset{
			{
				ID:             "runtime-dir",
				Kind:           ManagedAssetDirectory,
				Path:           paths.RuntimeDir,
				Owner:          "root",
				Group:          "lara-nux",
				Mode:           "0750",
				RollbackAction: RollbackRemove,
				Notes:          "Transient runtime directory for the daemon socket and runtime files.",
			},
			{
				ID:             "daemon-socket",
				Kind:           ManagedAssetSocket,
				Path:           paths.SocketPath,
				Owner:          "root",
				Group:          "lara-nux",
				Mode:           "0660",
				RollbackAction: RollbackRemove,
				Notes:          "Unix socket boundary consumed by the Wails/Nuxt desktop client.",
			},
			{
				ID:             "state-dir",
				Kind:           ManagedAssetDirectory,
				Path:           paths.StateDir,
				Owner:          "root",
				Group:          "lara-nux",
				Mode:           "0750",
				RollbackAction: RollbackRemove,
				Notes:          "Daemon state storage used for install metadata and rollback bookkeeping.",
			},
			{
				ID:             "managed-assets-manifest",
				Kind:           ManagedAssetStateFile,
				Path:           paths.ManagedAssetsPath,
				Owner:          "root",
				Group:          "lara-nux",
				Mode:           "0640",
				RollbackAction: RollbackRemove,
				Notes:          "Manifest consumed during uninstall or failed-install rollback.",
			},
			{
				ID:             "resolved-test-stub",
				Kind:           ManagedAssetResolver,
				Path:           "/etc/systemd/resolved.conf.d/lara-nux-test.conf",
				Owner:          "root",
				Group:          "root",
				Mode:           "0644",
				RollbackAction: RollbackRestorePrevious,
				Notes:          "Managed resolver stub for .test routing without overwriting unrelated DNS assets.",
			},
			{
				ID:             "caddy-sites-dir",
				Kind:           ManagedAssetWebConfig,
				Path:           "/etc/caddy/sites.d/lara-nux",
				Owner:          "root",
				Group:          "root",
				Mode:           "0755",
				RollbackAction: RollbackRestorePrevious,
				Notes:          "Namespace reserved for per-site Caddy definitions managed by the daemon.",
			},
			{
				ID:             "daemon-service",
				Kind:           ManagedAssetServiceUnit,
				Path:           "/etc/systemd/system/lara-nuxd.service",
				Owner:          "root",
				Group:          "root",
				Mode:           "0644",
				RollbackAction: RollbackRemove,
				Notes:          "Privileged systemd unit for the system daemon.",
			},
		},
	}
}

func (m *ManagedAssetManifest) Track(asset ManagedAsset) {
	for i := range m.Assets {
		if m.Assets[i].ID == asset.ID {
			m.Assets[i] = asset
			return
		}
	}

	m.Assets = append(m.Assets, asset)
}

func (m *ManagedAssetManifest) Remove(id string) {
	filtered := m.Assets[:0]
	for _, asset := range m.Assets {
		if asset.ID != id {
			filtered = append(filtered, asset)
		}
	}
	m.Assets = filtered
}

func (m ManagedAssetManifest) RollbackTargets() []ManagedAsset {
	targets := make([]ManagedAsset, 0, len(m.Assets))
	for _, asset := range m.Assets {
		if asset.RollbackAction == RollbackRemove || asset.RollbackAction == RollbackRestorePrevious {
			targets = append(targets, asset)
		}
	}
	return targets
}

func (m ManagedAssetManifest) Save(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return fmt.Errorf("create manifest directory: %w", err)
	}

	payload, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal managed asset manifest: %w", err)
	}

	tempPath := path + ".tmp"
	if err := os.WriteFile(tempPath, payload, 0o640); err != nil {
		return fmt.Errorf("write managed asset manifest: %w", err)
	}

	if err := os.Rename(tempPath, path); err != nil {
		return fmt.Errorf("commit managed asset manifest: %w", err)
	}

	return nil
}

func LoadManagedAssetManifest(path string) (ManagedAssetManifest, error) {
	var manifest ManagedAssetManifest

	payload, err := os.ReadFile(path)
	if err != nil {
		return manifest, fmt.Errorf("read managed asset manifest: %w", err)
	}

	if err := json.Unmarshal(payload, &manifest); err != nil {
		return manifest, fmt.Errorf("decode managed asset manifest: %w", err)
	}

	return manifest, nil
}
