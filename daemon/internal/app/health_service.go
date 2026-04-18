package app

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/jopsam/lara-nux/daemon/internal/host"
)

type HealthCheck struct {
	Name        string `json:"name"`
	Capability  string `json:"capability"`
	Passed      bool   `json:"passed"`
	Summary     string `json:"summary"`
	Remediation string `json:"remediation,omitempty"`
}

type SiteReadiness struct {
	Site       SiteRecord    `json:"site"`
	Ready      bool          `json:"ready"`
	Summary    string        `json:"summary"`
	Checks     []HealthCheck `json:"checks"`
	PHPService ServiceStatus `json:"phpService"`
	CheckedAt  time.Time     `json:"checkedAt"`
}

type HealthReport struct {
	Ready       bool                 `json:"ready"`
	GeneratedAt time.Time            `json:"generatedAt"`
	Socket      SocketAvailability   `json:"socket"`
	Resolver    *host.ResolverStatus `json:"resolver,omitempty"`
	Checks      []HealthCheck        `json:"checks"`
	Services    []ServiceStatus      `json:"services"`
	Sites       []SiteReadiness      `json:"sites"`
}

type PortChecker interface {
	CheckAvailable(port int) error
}

type LocalPortChecker struct{}

func (LocalPortChecker) CheckAvailable(port int) error {
	listener, err := net.Listen("tcp", ":"+strconv.Itoa(port))
	if err != nil {
		return err
	}
	defer func() { _ = listener.Close() }()

	return nil
}

type HealthService struct {
	sites       SiteStore
	runtimes    RuntimeResolver
	services    ServiceController
	resolver    ResolverProvisioner
	socketPath  string
	socketCheck SocketInspector
	portChecker PortChecker
	clock       func() time.Time
}

func NewHealthService(sites SiteStore, runtimes RuntimeResolver, services ServiceController, resolver ResolverProvisioner, socketPath string) *HealthService {
	return &HealthService{
		sites:       sites,
		runtimes:    runtimes,
		services:    services,
		resolver:    resolver,
		socketPath:  strings.TrimSpace(socketPath),
		socketCheck: FileSocketInspector{},
		portChecker: LocalPortChecker{},
		clock:       func() time.Time { return time.Now().UTC() },
	}
}

func (s *HealthService) Report(ctx context.Context) (HealthReport, error) {
	generatedAt := s.clock()
	report := HealthReport{GeneratedAt: generatedAt}
	report.Socket = s.socketCheck.Inspect(s.socketPath)

	sites, err := s.sites.List(ctx)
	if err != nil {
		return report, err
	}

	runtimes, err := s.runtimes.List(ctx)
	if err != nil {
		return report, err
	}

	if s.resolver != nil {
		resolverStatus, resolverErr := s.resolver.Inspect(ctx, host.ResolverStubSpec{Domain: "test", Address: "127.0.0.1"})
		if resolverErr != nil {
			resolverStatus.Summary = resolverErr.Error()
		}
		report.Resolver = &resolverStatus
		report.Checks = append(report.Checks, HealthCheck{
			Name:        "resolver-test-routing",
			Capability:  "local-dns-routing",
			Passed:      resolverErr == nil && len(resolverStatus.Conflicts) == 0 && (resolverStatus.Managed || resolverStatus.Owner == "available"),
			Summary:     resolverStatus.Summary,
			Remediation: firstNonEmptyString(resolverStatus.Remediation, "Resolve the conflicting systemd-resolved ownership before managing .test domains."),
		})
	}

	serviceNames := map[string]struct{}{"caddy": {}}
	for _, site := range sites {
		serviceNames[PHPFPMServiceName(site.PHPVersion)] = struct{}{}
	}

	serviceStatuses := make(map[string]ServiceStatus, len(serviceNames))
	for serviceName := range serviceNames {
		if strings.TrimSpace(serviceName) == "" || serviceName == "php-fpm" {
			continue
		}

		status, statusErr := s.services.Status(ctx, serviceName)
		if statusErr != nil && status.State == ServiceStateUnknown {
			status.Summary = statusErr.Error()
		}
		serviceStatuses[serviceName] = status
		report.Services = append(report.Services, status)
	}

	caddyStatus := serviceStatuses["caddy"]
	report.Checks = append(report.Checks,
		HealthCheck{
			Name:        "daemon-socket",
			Capability:  "daemon",
			Passed:      report.Socket.Available,
			Summary:     report.Socket.Summary,
			Remediation: "Start the daemon systemd unit and restore the managed Unix socket path/permissions.",
		},
		HealthCheck{
			Name:        "privileges",
			Capability:  "daemon",
			Passed:      os.Geteuid() == 0,
			Summary:     privilegeSummary(),
			Remediation: "Run the daemon as a privileged system service so it can own the Unix socket, ports, and system units.",
		},
		HealthCheck{
			Name:        "php-runtime-inventory",
			Capability:  "php-runtime-management",
			Passed:      len(runtimes) > 0,
			Summary:     runtimeInventorySummary(len(runtimes)),
			Remediation: "Register at least one supported PHP runtime before activating sites.",
		},
	)

	for _, port := range []int{80, 443} {
		passed, summary, remediation := s.portHealth(port, caddyStatus)
		report.Checks = append(report.Checks, HealthCheck{
			Name:        fmt.Sprintf("port-%d", port),
			Capability:  "service-orchestration",
			Passed:      passed,
			Summary:     summary,
			Remediation: remediation,
		})
	}

	report.Sites = make([]SiteReadiness, 0, len(sites))
	report.Ready = true
	for _, site := range sites {
		readiness := s.siteReadiness(ctx, site, caddyStatus, serviceStatuses, generatedAt)
		report.Sites = append(report.Sites, readiness)
		if !readiness.Ready {
			report.Ready = false
		}
	}

	for _, check := range report.Checks {
		if !check.Passed {
			report.Ready = false
			break
		}
	}

	return report, nil
}

func (s *HealthService) siteReadiness(ctx context.Context, site SiteRecord, caddyStatus ServiceStatus, serviceStatuses map[string]ServiceStatus, checkedAt time.Time) SiteReadiness {
	checks := []HealthCheck{}

	pathErr := ValidateLaravelPath(site.RootPath)
	checks = append(checks, HealthCheck{
		Name:        "laravel-project",
		Capability:  "local-site-serving",
		Passed:      pathErr == nil,
		Summary:     sitePathSummary(pathErr, site.RootPath),
		Remediation: "Point the site at a Laravel project root containing artisan, composer.json, and public/index.php.",
	})

	runtime, runtimeErr := s.runtimes.Get(ctx, site.PHPVersion)
	checks = append(checks, HealthCheck{
		Name:        "php-runtime",
		Capability:  "php-runtime-management",
		Passed:      runtimeErr == nil,
		Summary:     runtimeSummary(runtimeErr, site.PHPVersion),
		Remediation: "Register the requested PHP runtime and retry the site activation.",
	})

	phpService := serviceStatuses[PHPFPMServiceName(site.PHPVersion)]
	if runtimeErr == nil {
		phpService = serviceStatuses[runtime.FPMService]
	}

	checks = append(checks,
		HealthCheck{
			Name:        "caddy-service",
			Capability:  "service-orchestration",
			Passed:      caddyStatus.State == ServiceStateActive,
			Summary:     serviceHealthSummary("caddy", caddyStatus),
			Remediation: "Start or restart the caddy service through /rpc/services.action.",
		},
		HealthCheck{
			Name:        "php-fpm-service",
			Capability:  "service-orchestration",
			Passed:      phpService.State == ServiceStateActive,
			Summary:     serviceHealthSummary(phpService.Service, phpService),
			Remediation: "Start or restart the PHP-FPM service for the selected runtime.",
		},
	)

	ready := true
	status := SiteStatusReady
	summary := "Site is ready to serve local traffic."
	for _, check := range checks {
		if check.Passed {
			continue
		}

		ready = false
		summary = check.Summary
		if check.Name == "laravel-project" || check.Name == "php-runtime" {
			status = SiteStatusConflict
		} else {
			status = SiteStatusDegraded
		}
		break
	}

	site.Status = status
	site.StatusMessage = summary
	site.LastCheckedAt = checkedAt

	return SiteReadiness{
		Site:       site,
		Ready:      ready,
		Summary:    summary,
		Checks:     checks,
		PHPService: phpService,
		CheckedAt:  checkedAt,
	}
}

func (s *HealthService) portHealth(port int, caddyStatus ServiceStatus) (bool, string, string) {
	if caddyStatus.State == ServiceStateActive {
		return true,
			fmt.Sprintf("Port %d is owned by the managed caddy service.", port),
			""
	}

	if err := s.portChecker.CheckAvailable(port); err != nil {
		if errors.Is(err, syscall.EACCES) || errors.Is(err, syscall.EPERM) {
			return false,
				fmt.Sprintf("Port %d requires elevated privileges before Lara Nux can claim it.", port),
				"Run the daemon as root through the managed systemd unit before checking readiness."
		}

		return false,
			fmt.Sprintf("Port %d is unavailable: %v", port, err),
			"Stop the conflicting service or free the port before starting the local environment."
	}

	return true,
		fmt.Sprintf("Port %d is available for Lara Nux to claim.", port),
		""
}

func privilegeSummary() string {
	if os.Geteuid() == 0 {
		return "Daemon has the privileges required for managed services and ports."
	}

	return "Daemon is missing root privileges required for orchestration."
}

func runtimeInventorySummary(count int) string {
	if count > 0 {
		return fmt.Sprintf("%d PHP runtime(s) registered for site assignment.", count)
	}

	return "No supported PHP runtimes have been registered yet."
}

func sitePathSummary(pathErr error, rootPath string) string {
	if pathErr == nil {
		return fmt.Sprintf("Laravel project path is valid: %s", rootPath)
	}

	return pathErr.Error()
}

func runtimeSummary(err error, version string) string {
	if err == nil {
		return fmt.Sprintf("PHP %s is registered for site switching.", version)
	}

	return err.Error()
}

func serviceHealthSummary(service string, status ServiceStatus) string {
	if strings.TrimSpace(service) == "" {
		return "Service name is not configured."
	}

	if status.Service == "" {
		return fmt.Sprintf("Service %s has not been observed yet.", service)
	}

	return fmt.Sprintf("Service %s is %s (%s).", status.Service, status.State, status.Summary)
}
