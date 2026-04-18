package app

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jopsam/lara-nux/daemon/internal/host"
)

type SiteUpdateInput struct {
	SiteID     string  `json:"siteId"`
	RootPath   *string `json:"rootPath,omitempty"`
	Domain     *string `json:"domain,omitempty"`
	PHPVersion *string `json:"phpVersion,omitempty"`
}

type SiteManagementService struct {
	sites    SiteStore
	runtimes RuntimeResolver
	web      WebActivator
	hostPHP  PHPRuntimeMaterializer
	services ServiceController
	clock    func() time.Time
}

func NewSiteManagementService(sites SiteStore, runtimes RuntimeResolver, web WebActivator, hostPHP PHPRuntimeMaterializer, services ServiceController) *SiteManagementService {
	return &SiteManagementService{
		sites:    sites,
		runtimes: runtimes,
		web:      web,
		hostPHP:  hostPHP,
		services: services,
		clock:    func() time.Time { return time.Now().UTC() },
	}
}

func (s *SiteManagementService) List(ctx context.Context) ([]SiteRecord, error) {
	return s.sites.List(ctx)
}

func (s *SiteManagementService) Get(ctx context.Context, siteID string) (SiteRecord, error) {
	return s.sites.Get(ctx, strings.TrimSpace(siteID))
}

func (s *SiteManagementService) Update(ctx context.Context, input SiteUpdateInput) (SiteRecord, error) {
	current, err := s.sites.Get(ctx, strings.TrimSpace(input.SiteID))
	if err != nil {
		return SiteRecord{}, err
	}

	next, targetRuntime, err := s.prepareUpdatedSite(ctx, current, input)
	if err != nil {
		return SiteRecord{}, err
	}

	phpChanged := NormalizePHPVersion(current.PHPVersion) != targetRuntime.Version
	configChanged := current.RootPath != next.RootPath || current.Domain != next.Domain || phpChanged

	if !configChanged {
		next.Status = SiteStatusReady
		next.StatusMessage = fmt.Sprintf("Site metadata updated for %s.", next.Domain)
		next.LastCheckedAt = s.clock()
		return s.sites.Update(ctx, next)
	}

	if err := s.applyRuntimeChange(ctx, current, targetRuntime); err != nil {
		return SiteRecord{}, err
	}

	rollbackRuntime := phpChanged
	if s.services != nil {
		if err := startManagedServices(ctx, s.services, "caddy", targetRuntime.FPMService); err != nil {
			if rollbackRuntime {
				_ = s.rollbackRuntime(ctx, current, targetRuntime)
			}
			return SiteRecord{}, err
		}
	}

	if s.web != nil {
		if _, err := s.web.ActivateSite(ctx, siteToWebSite(next, phpSocketPath(targetRuntime.Version))); err != nil {
			if rollbackRuntime {
				_ = s.rollbackRuntime(ctx, current, targetRuntime)
			}
			return SiteRecord{}, err
		}
	}

	next.Status = SiteStatusReady
	next.StatusMessage = fmt.Sprintf("Site updated and ready at https://%s with PHP %s.", next.Domain, targetRuntime.Version)
	next.LastCheckedAt = s.clock()
	persisted, err := s.sites.Update(ctx, next)
	if err != nil {
		if s.web != nil {
			_, _ = s.web.ActivateSite(ctx, siteToWebSite(current, phpSocketPath(current.PHPVersion)))
		}
		if rollbackRuntime {
			_ = s.rollbackRuntime(ctx, current, targetRuntime)
		}
		return SiteRecord{}, fmt.Errorf("persist site update %s: %w", current.ID, err)
	}

	return persisted, nil
}

func (s *SiteManagementService) prepareUpdatedSite(ctx context.Context, current SiteRecord, input SiteUpdateInput) (SiteRecord, PHPRuntimeRecord, error) {
	next := current

	rootPath := current.RootPath
	if input.RootPath != nil {
		var err error
		rootPath, err = normalizeRootPath(*input.RootPath)
		if err != nil {
			return SiteRecord{}, PHPRuntimeRecord{}, err
		}
		if err := ValidateLaravelPath(rootPath); err != nil {
			return SiteRecord{}, PHPRuntimeRecord{}, err
		}
	}

	name := current.Name
	if input.RootPath != nil {
		name = deriveSiteName(rootPath)
	}

	domain := current.Domain
	if input.Domain != nil {
		var err error
		domain, err = normalizeDomain(*input.Domain, name)
		if err != nil {
			return SiteRecord{}, PHPRuntimeRecord{}, err
		}
	}

	runtimeVersion := current.PHPVersion
	if input.PHPVersion != nil && strings.TrimSpace(*input.PHPVersion) != "" {
		runtimeVersion = *input.PHPVersion
	}

	targetRuntime, err := s.resolveRuntime(ctx, runtimeVersion)
	if err != nil {
		return SiteRecord{}, PHPRuntimeRecord{}, err
	}

	existingSites, err := s.sites.List(ctx)
	if err != nil {
		return SiteRecord{}, PHPRuntimeRecord{}, err
	}

	for _, site := range existingSites {
		if site.ID == current.ID {
			continue
		}

		if strings.EqualFold(site.Name, name) {
			return SiteRecord{}, PHPRuntimeRecord{}, fmt.Errorf("%w: site name %q is already registered", ErrDuplicateSiteName, name)
		}

		if strings.EqualFold(site.Domain, domain) {
			return SiteRecord{}, PHPRuntimeRecord{}, fmt.Errorf("%w: domain %q is already registered", ErrDuplicateDomain, domain)
		}
	}

	next.Name = name
	next.RootPath = rootPath
	next.Domain = domain
	next.PHPVersion = targetRuntime.Version
	return next, targetRuntime, nil
}

func (s *SiteManagementService) resolveRuntime(ctx context.Context, version string) (PHPRuntimeRecord, error) {
	if strings.TrimSpace(version) == "" {
		return s.runtimes.DefaultRuntime(ctx)
	}
	return s.runtimes.Get(ctx, version)
}

func (s *SiteManagementService) applyRuntimeChange(ctx context.Context, current SiteRecord, targetRuntime PHPRuntimeRecord) error {
	if NormalizePHPVersion(current.PHPVersion) == targetRuntime.Version {
		return nil
	}

	if s.hostPHP != nil {
		_, err := s.hostPHP.SwitchRuntime(ctx, runtimeSwitchRequest(current, targetRuntime))
		if err != nil {
			return fmt.Errorf("switch php runtime %s for site %s: %w", targetRuntime.Version, current.ID, err)
		}
		return nil
	}

	if s.services == nil {
		return nil
	}

	_, err := s.services.Restart(ctx, targetRuntime.FPMService)
	if err != nil {
		return fmt.Errorf("activate php runtime %s for site %s: %w", targetRuntime.Version, current.ID, err)
	}
	return nil
}

func (s *SiteManagementService) rollbackRuntime(ctx context.Context, previous SiteRecord, failedTarget PHPRuntimeRecord) error {
	previousVersion := NormalizePHPVersion(previous.PHPVersion)
	if previousVersion == "" {
		return nil
	}

	previousRuntime, err := s.runtimes.Get(ctx, previousVersion)
	if err != nil {
		previousRuntime = PHPRuntimeRecord{
			Version:    previousVersion,
			FPMService: PHPFPMServiceName(previousVersion),
		}
	}

	if s.hostPHP != nil {
		_, err = s.hostPHP.SwitchRuntime(ctx, host.PHPSwitchRequest{
			SiteID:   previous.ID,
			Previous: runtimeToHost(failedTarget),
			Target: host.PHPRuntime{
				Version:     previousRuntime.Version,
				BinaryPath:  previousRuntime.BinaryPath,
				ServiceName: previousRuntime.FPMService,
				SocketPath:  phpSocketPath(previousRuntime.Version),
			},
		})
		return err
	}

	if s.services == nil {
		return nil
	}

	_, err = s.services.Restart(ctx, previousRuntime.FPMService)
	return err
}

func runtimeSwitchRequest(previous SiteRecord, target PHPRuntimeRecord) host.PHPSwitchRequest {
	previousVersion := NormalizePHPVersion(previous.PHPVersion)
	return host.PHPSwitchRequest{
		SiteID: previous.ID,
		Previous: host.PHPRuntime{
			Version:     previousVersion,
			ServiceName: PHPFPMServiceName(previousVersion),
			SocketPath:  phpSocketPath(previousVersion),
		},
		Target: runtimeToHost(target),
	}
}

func startManagedServices(ctx context.Context, services ServiceController, names ...string) error {
	seen := map[string]struct{}{}
	for _, name := range names {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		if _, ok := seen[name]; ok {
			continue
		}
		seen[name] = struct{}{}
		if _, err := services.Start(ctx, name); err != nil {
			return err
		}
	}
	return nil
}
