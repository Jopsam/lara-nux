package app

import (
	"context"
	"fmt"
	"strings"

	"github.com/jopsam/lara-nux/daemon/internal/host"
)

type RuntimeInventoryRefresher interface {
	RefreshRuntimeInventory(ctx context.Context) ([]host.PHPRuntime, error)
}

type RuntimeOnboardingService struct {
	runtimes *PHPRegistry
	packages RuntimeInventoryRefresher
	resolver RuntimeResolver
	material PHPRuntimeMaterializer
	services ServiceController
}

type RuntimeRegistrationRequest struct {
	Version    string `json:"version,omitempty"`
	BinaryPath string `json:"binaryPath,omitempty"`
	FPMService string `json:"fpmService,omitempty"`
	Source     string `json:"source,omitempty"`
	PackageKey string `json:"packageKey,omitempty"`
}

type RuntimeRegistrationResult struct {
	Runtime         PHPRuntimeRecord         `json:"runtime"`
	Materialization *host.PHPMaterialization `json:"materialization,omitempty"`
	InstalledFrom   string                   `json:"installedFrom,omitempty"`
	Refreshed       bool                     `json:"refreshed"`
}

func NewRuntimeOnboardingService(runtimes *PHPRegistry, packages RuntimeInventoryRefresher, resolver RuntimeResolver, material PHPRuntimeMaterializer, services ServiceController) *RuntimeOnboardingService {
	return &RuntimeOnboardingService{
		runtimes: runtimes,
		packages: packages,
		resolver: resolver,
		material: material,
		services: services,
	}
}

func (s *RuntimeOnboardingService) Register(ctx context.Context, request RuntimeRegistrationRequest) (RuntimeRegistrationResult, error) {
	if strings.TrimSpace(request.PackageKey) != "" {
		return s.registerFromPackageInventory(ctx, request)
	}

	record, err := s.runtimes.Register(ctx, RuntimeRegistrationInput{
		Version:    request.Version,
		BinaryPath: request.BinaryPath,
		FPMService: request.FPMService,
		Source:     firstNonEmptyString(request.Source, "manual"),
	})
	if err != nil {
		return RuntimeRegistrationResult{}, err
	}

	result := RuntimeRegistrationResult{Runtime: record}
	if s.material == nil {
		return result, nil
	}

	materialization, err := s.material.MaterializeRuntime(ctx, runtimeToHost(record))
	if err != nil {
		return RuntimeRegistrationResult{}, err
	}
	result.Materialization = &materialization

	if s.services != nil && strings.TrimSpace(record.FPMService) != "" {
		if _, err := s.services.Status(ctx, record.FPMService); err != nil {
			return RuntimeRegistrationResult{}, err
		}
	}

	return result, nil
}

func (s *RuntimeOnboardingService) registerFromPackageInventory(ctx context.Context, request RuntimeRegistrationRequest) (RuntimeRegistrationResult, error) {
	if s.packages == nil {
		return RuntimeRegistrationResult{}, fmt.Errorf("runtime package onboarding is not configured")
	}

	inventory, err := s.packages.RefreshRuntimeInventory(ctx)
	if err != nil {
		return RuntimeRegistrationResult{}, err
	}

	targetVersion := NormalizePHPVersion(request.Version)
	packageKey := strings.ToLower(strings.TrimSpace(request.PackageKey))
	for _, runtime := range inventory {
		if targetVersion != "" && NormalizePHPVersion(runtime.Version) != targetVersion {
			continue
		}
		if targetVersion == "" && packageRuntimeKey(runtime.Version) != packageKey {
			continue
		}

		record, regErr := s.runtimes.Register(ctx, RuntimeRegistrationInput{
			Version:    runtime.Version,
			BinaryPath: runtime.BinaryPath,
			FPMService: runtime.ServiceName,
			Source:     firstNonEmptyString(request.Source, packageKey),
		})
		if regErr != nil {
			return RuntimeRegistrationResult{}, regErr
		}

		result := RuntimeRegistrationResult{
			Runtime:       record,
			InstalledFrom: packageKey,
			Refreshed:     true,
		}

		if s.material != nil {
			materialization, matErr := s.material.MaterializeRuntime(ctx, runtimeToHost(record))
			if matErr != nil {
				return RuntimeRegistrationResult{}, matErr
			}
			result.Materialization = &materialization
		}

		return result, nil
	}

	if targetVersion != "" {
		return RuntimeRegistrationResult{}, fmt.Errorf("%w: runtime %s is not available from package inventory %s", ErrRuntimeNotFound, targetVersion, packageKey)
	}

	return RuntimeRegistrationResult{}, fmt.Errorf("%w: package inventory %s does not expose a supported runtime", ErrRuntimeNotFound, packageKey)
}

func packageRuntimeKey(version string) string {
	version = NormalizePHPVersion(version)
	if version == "" {
		return ""
	}
	return "php-" + version
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}
