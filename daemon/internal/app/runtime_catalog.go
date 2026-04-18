package app

import (
	"context"
	"errors"

	"github.com/jopsam/lara-nux/daemon/internal/host"
)

type RuntimeCatalog struct {
	Registered        []PHPRuntimeRecord      `json:"registered"`
	DefaultRuntime    *PHPRuntimeRecord       `json:"defaultRuntime,omitempty"`
	SupportedPackages []host.SupportedPackage `json:"supportedPackages"`
	DetectedRuntimes  []host.PHPRuntime       `json:"detectedRuntimes"`
}

type RuntimeCatalogService struct {
	registry *PHPRegistry
	packages host.PackageManager
}

func NewRuntimeCatalogService(registry *PHPRegistry, packages host.PackageManager) *RuntimeCatalogService {
	return &RuntimeCatalogService{registry: registry, packages: packages}
}

func (s *RuntimeCatalogService) ListRegistered(ctx context.Context) ([]PHPRuntimeRecord, error) {
	return s.registry.List(ctx)
}

func (s *RuntimeCatalogService) DefaultRuntime(ctx context.Context) (*PHPRuntimeRecord, error) {
	runtime, err := s.registry.DefaultRuntime(ctx)
	if err == nil {
		return &runtime, nil
	}
	if errors.Is(err, ErrRuntimeNotFound) {
		return nil, nil
	}
	return nil, err
}

func (s *RuntimeCatalogService) SetDefault(ctx context.Context, version string) (PHPRuntimeRecord, error) {
	return s.registry.SetDefault(ctx, version)
}

func (s *RuntimeCatalogService) Inventory(ctx context.Context) (RuntimeCatalog, error) {
	registered, err := s.registry.List(ctx)
	if err != nil {
		return RuntimeCatalog{}, err
	}

	defaultRuntime, err := s.DefaultRuntime(ctx)
	if err != nil {
		return RuntimeCatalog{}, err
	}

	catalog := RuntimeCatalog{
		Registered:        registered,
		DefaultRuntime:    defaultRuntime,
		SupportedPackages: []host.SupportedPackage{},
		DetectedRuntimes:  []host.PHPRuntime{},
	}

	if s.packages == nil {
		return catalog, nil
	}

	catalog.SupportedPackages = s.packages.SupportedPackages()
	detected, err := s.packages.RefreshRuntimeInventory(ctx)
	if err != nil {
		return RuntimeCatalog{}, err
	}
	catalog.DetectedRuntimes = detected
	return catalog, nil
}
