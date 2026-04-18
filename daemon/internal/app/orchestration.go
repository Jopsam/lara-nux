package app

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/jopsam/lara-nux/daemon/internal/host"
)

type RuntimeResolver interface {
	Get(ctx context.Context, version string) (PHPRuntimeRecord, error)
	DefaultRuntime(ctx context.Context) (PHPRuntimeRecord, error)
	List(ctx context.Context) ([]PHPRuntimeRecord, error)
}

type SiteStore interface {
	Register(ctx context.Context, input SiteRegistrationInput) (SiteRecord, error)
	Get(ctx context.Context, siteID string) (SiteRecord, error)
	Update(ctx context.Context, record SiteRecord) (SiteRecord, error)
	Delete(ctx context.Context, siteID string) error
	List(ctx context.Context) ([]SiteRecord, error)
}

type ResolverProvisioner interface {
	Inspect(ctx context.Context, spec host.ResolverStubSpec) (host.ResolverStatus, error)
	EnsureTestStub(ctx context.Context, spec host.ResolverStubSpec) (host.ResolverStatus, error)
	RemoveManagedStub(ctx context.Context) error
}

type WebActivator interface {
	ActivateSite(ctx context.Context, site host.WebSite) (host.WebActivationResult, error)
	RemoveSite(ctx context.Context, siteID string) error
	Validate(ctx context.Context) error
}

type PHPRuntimeMaterializer interface {
	MaterializeRuntime(ctx context.Context, runtime host.PHPRuntime) (host.PHPMaterialization, error)
	SwitchRuntime(ctx context.Context, request host.PHPSwitchRequest) (host.PHPMaterialization, error)
}

type ServiceController interface {
	Start(ctx context.Context, service string) (ServiceStatus, error)
	Restart(ctx context.Context, service string) (ServiceStatus, error)
	Status(ctx context.Context, service string) (ServiceStatus, error)
}

type SocketInspector interface {
	Inspect(path string) SocketAvailability
}

type ActivationResult struct {
	Site            SiteRecord               `json:"site"`
	Resolver        host.ResolverStatus      `json:"resolver"`
	Runtime         PHPRuntimeRecord         `json:"runtime"`
	Materialization host.PHPMaterialization  `json:"materialization"`
	Web             host.WebActivationResult `json:"web"`
	Services        []ServiceStatus          `json:"services"`
	ActivatedAt     time.Time                `json:"activatedAt"`
}

type SiteActivationService struct {
	sites    SiteStore
	runtimes RuntimeResolver
	resolver ResolverProvisioner
	web      WebActivator
	php      PHPRuntimeMaterializer
	services ServiceController
	clock    func() time.Time
	stubSpec host.ResolverStubSpec
}

func NewSiteActivationService(sites SiteStore, runtimes RuntimeResolver, resolver ResolverProvisioner, web WebActivator, php PHPRuntimeMaterializer, services ServiceController) *SiteActivationService {
	return &SiteActivationService{
		sites:    sites,
		runtimes: runtimes,
		resolver: resolver,
		web:      web,
		php:      php,
		services: services,
		clock:    func() time.Time { return time.Now().UTC() },
		stubSpec: host.ResolverStubSpec{Domain: "test", Address: "127.0.0.1"},
	}
}

func (s *SiteActivationService) Activate(ctx context.Context, input SiteRegistrationInput) (ActivationResult, error) {
	var result ActivationResult

	runtime, err := s.resolveRuntime(ctx, input.PHPVersion)
	if err != nil {
		return result, err
	}
	input.PHPVersion = runtime.Version

	site, err := s.sites.Register(ctx, input)
	if err != nil {
		return result, err
	}
	result.Site = site
	result.Runtime = runtime

	committedResolver := false
	committedWeb := false

	defer func() {
		if err == nil {
			return
		}
		if committedWeb && s.web != nil {
			_ = s.web.RemoveSite(ctx, site.ID)
		}
		if committedResolver && s.resolver != nil {
			if hasSites, listErr := s.hasOtherSites(ctx, site.ID); listErr == nil && !hasSites {
				_ = s.resolver.RemoveManagedStub(ctx)
			}
		}
		_ = s.sites.Delete(ctx, site.ID)
	}()

	if s.resolver != nil {
		result.Resolver, err = s.resolver.EnsureTestStub(ctx, s.stubSpec)
		if err != nil {
			return ActivationResult{}, err
		}
		committedResolver = true
	}

	if s.php != nil {
		result.Materialization, err = s.php.MaterializeRuntime(ctx, runtimeToHost(runtime))
		if err != nil {
			return ActivationResult{}, err
		}
	}

	if s.services != nil {
		result.Services, err = s.ensureServices(ctx, "caddy", runtime.FPMService)
		if err != nil {
			return ActivationResult{}, err
		}
	}

	if s.web != nil {
		result.Web, err = s.web.ActivateSite(ctx, siteToWebSite(site, result.Materialization.SocketPath))
		if err != nil {
			return ActivationResult{}, err
		}
		committedWeb = true
	}

	site.Status = SiteStatusReady
	site.StatusMessage = fmt.Sprintf("Site activated at https://%s with PHP %s.", site.Domain, runtime.Version)
	site.LastCheckedAt = s.clock()
	result.Site, err = s.sites.Update(ctx, site)
	if err != nil {
		return ActivationResult{}, err
	}

	result.ActivatedAt = s.clock()
	return result, nil
}

func (s *SiteActivationService) ensureServices(ctx context.Context, names ...string) ([]ServiceStatus, error) {
	statuses := make([]ServiceStatus, 0, len(names))
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

		status, err := s.services.Start(ctx, name)
		if err != nil {
			return nil, err
		}
		statuses = append(statuses, status)
	}
	return statuses, nil
}

func (s *SiteActivationService) resolveRuntime(ctx context.Context, version string) (PHPRuntimeRecord, error) {
	if strings.TrimSpace(version) == "" {
		return s.runtimes.DefaultRuntime(ctx)
	}
	return s.runtimes.Get(ctx, version)
}

func (s *SiteActivationService) hasOtherSites(ctx context.Context, excludeID string) (bool, error) {
	sites, err := s.sites.List(ctx)
	if err != nil {
		return false, err
	}
	for _, site := range sites {
		if site.ID != excludeID {
			return true, nil
		}
	}
	return false, nil
}

type SocketAvailability struct {
	Path      string `json:"path"`
	Available bool   `json:"available"`
	Summary   string `json:"summary"`
}

type FileSocketInspector struct{}

func (FileSocketInspector) Inspect(path string) SocketAvailability {
	path = strings.TrimSpace(path)
	if path == "" {
		return SocketAvailability{Path: path, Available: false, Summary: "Daemon socket path is not configured."}
	}

	info, err := os.Stat(path)
	if err != nil {
		return SocketAvailability{Path: path, Available: false, Summary: fmt.Sprintf("Daemon socket is unavailable: %v", err)}
	}

	if info.Mode()&os.ModeSocket == 0 {
		return SocketAvailability{Path: path, Available: false, Summary: fmt.Sprintf("Configured socket path %s is not a Unix socket.", path)}
	}

	return SocketAvailability{Path: path, Available: true, Summary: fmt.Sprintf("Daemon socket is available at %s.", path)}
}

func runtimeToHost(runtime PHPRuntimeRecord) host.PHPRuntime {
	return host.PHPRuntime{
		Version:     runtime.Version,
		BinaryPath:  runtime.BinaryPath,
		ServiceName: runtime.FPMService,
		SocketPath:  phpSocketPath(runtime.Version),
	}
}

func siteToWebSite(site SiteRecord, socketPath string) host.WebSite {
	return host.WebSite{
		ID:            site.ID,
		Domain:        site.Domain,
		RootPath:      site.RootPath,
		PublicDir:     sitePublicDir(site.RootPath),
		PHPSocketPath: firstNonEmptyString(socketPath, phpSocketPath(site.PHPVersion)),
	}
}

func sitePublicDir(rootPath string) string {
	if strings.TrimSpace(rootPath) == "" {
		return ""
	}
	return filepath.Join(rootPath, "public")
}

func phpSocketPath(version string) string {
	version = NormalizePHPVersion(version)
	if version == "" {
		return ""
	}
	return "/run/php/lara-nux-php" + version + "-fpm.sock"
}
