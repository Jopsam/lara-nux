package app

import (
	"context"
	"fmt"

	"github.com/jopsam/lara-nux/daemon/internal/host"
)

type PHPManager struct {
	sites    SiteStore
	runtimes RuntimeResolver
	services ServiceController
	hostPHP  PHPRuntimeMaterializer
}

func NewPHPManager(sites SiteStore, runtimes RuntimeResolver, services ServiceController, hostPHP PHPRuntimeMaterializer) *PHPManager {
	return &PHPManager{
		sites:    sites,
		runtimes: runtimes,
		services: services,
		hostPHP:  hostPHP,
	}
}

func (m *PHPManager) ActiveRuntime(ctx context.Context, siteID string) (PHPRuntimeRecord, error) {
	site, err := m.sites.Get(ctx, siteID)
	if err != nil {
		return PHPRuntimeRecord{}, err
	}

	if site.PHPVersion == "" {
		return m.runtimes.DefaultRuntime(ctx)
	}

	return m.runtimes.Get(ctx, site.PHPVersion)
}

func (m *PHPManager) SwitchSiteRuntime(ctx context.Context, siteID string, version string) (SiteRecord, error) {
	site, err := m.sites.Get(ctx, siteID)
	if err != nil {
		return SiteRecord{}, err
	}

	targetRuntime, err := m.runtimes.Get(ctx, version)
	if err != nil {
		return SiteRecord{}, err
	}

	if site.PHPVersion == targetRuntime.Version {
		return site, nil
	}

	previousVersion := site.PHPVersion
	if m.hostPHP != nil {
		_, err = m.hostPHP.SwitchRuntime(ctx, host.PHPSwitchRequest{
			SiteID: site.ID,
			Previous: host.PHPRuntime{
				Version:     previousVersion,
				ServiceName: PHPFPMServiceName(previousVersion),
				SocketPath:  phpSocketPath(previousVersion),
			},
			Target: runtimeToHost(targetRuntime),
		})
		if err != nil {
			return SiteRecord{}, fmt.Errorf("switch php runtime %s for site %s: %w", targetRuntime.Version, site.ID, err)
		}
	} else if _, err := m.services.Restart(ctx, targetRuntime.FPMService); err != nil {
		return SiteRecord{}, fmt.Errorf("activate php runtime %s for site %s: %w", targetRuntime.Version, site.ID, err)
	}

	updated := site
	updated.PHPVersion = targetRuntime.Version
	updated.Status = SiteStatusReady
	updated.StatusMessage = fmt.Sprintf("Site switched to PHP %s.", targetRuntime.Version)

	persisted, err := m.sites.Update(ctx, updated)
	if err != nil {
		if previousVersion != "" {
			previousRuntime, runtimeErr := m.runtimes.Get(ctx, previousVersion)
			if runtimeErr == nil {
				_, _ = m.services.Restart(ctx, previousRuntime.FPMService)
			} else {
				_, _ = m.services.Restart(ctx, PHPFPMServiceName(previousVersion))
			}
		}
		return SiteRecord{}, fmt.Errorf("persist php runtime switch for site %s: %w", site.ID, err)
	}

	return persisted, nil
}
