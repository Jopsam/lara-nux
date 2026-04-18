package app

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strings"
)

type AppPaths struct {
	ConfigDir         string `json:"configDir"`
	StateDir          string `json:"stateDir"`
	RuntimeDir        string `json:"runtimeDir"`
	SocketPath        string `json:"socketPath"`
	ManagedAssetsPath string `json:"managedAssetsPath"`
}

type BootstrapService struct {
	paths AppPaths
}

type UbuntuRelease struct {
	ID              string `json:"id"`
	VersionID       string `json:"versionId"`
	VersionCodename string `json:"versionCodename,omitempty"`
	PrettyName      string `json:"prettyName,omitempty"`
}

type PreflightCheck struct {
	Name        string `json:"name"`
	Passed      bool   `json:"passed"`
	Summary     string `json:"summary"`
	Remediation string `json:"remediation,omitempty"`
}

type BootstrapReport struct {
	Host     UbuntuRelease        `json:"host"`
	Paths    AppPaths             `json:"paths"`
	Checks   []PreflightCheck     `json:"checks"`
	Manifest ManagedAssetManifest `json:"manifest"`
}

func NewBootstrapService(paths AppPaths) *BootstrapService {
	return &BootstrapService{paths: paths}
}

func LoadPathsFromEnv() AppPaths {
	configDir := envOrDefault("LARA_NUXT_CONFIG_DIR", "/etc/lara-nux")
	stateDir := envOrDefault("LARA_NUXT_STATE_DIR", "/var/lib/lara-nux")
	runtimeDir := envOrDefault("LARA_NUXT_RUNTIME_DIR", "/run/lara-nux")

	return AppPaths{
		ConfigDir:         configDir,
		StateDir:          stateDir,
		RuntimeDir:        runtimeDir,
		SocketPath:        envOrDefault("LARA_NUXT_SOCKET_PATH", filepath.Join(runtimeDir, "lara-nux.sock")),
		ManagedAssetsPath: envOrDefault("LARA_NUXT_MANAGED_ASSETS_PATH", filepath.Join(stateDir, "managed-assets.json")),
	}
}

func (s *BootstrapService) Preflight(ctx context.Context) (BootstrapReport, error) {
	report := BootstrapReport{
		Paths:    s.paths,
		Manifest: DefaultUbuntuManagedAssets(s.paths),
	}

	select {
	case <-ctx.Done():
		return report, ctx.Err()
	default:
	}

	release, err := readOSRelease("/etc/os-release")
	if err != nil {
		report.addCheck("os-release", false, "Unable to read Ubuntu release metadata.", "Ensure /etc/os-release exists and the daemon runs on a supported Ubuntu host.")
		return report, fmt.Errorf("bootstrap preflight failed: %w", report.ErrorOr(err))
	}

	report.Host = release
	report.addCheck("ubuntu-host", release.ID == "ubuntu", fmt.Sprintf("Detected host %s %s.", release.ID, release.VersionID), "Run Lara Nuxt only on supported Ubuntu LTS releases.")
	report.addCheck("ubuntu-release", supportedUbuntuRelease(release.VersionID), fmt.Sprintf("Ubuntu release %s is within the supported matrix.", release.VersionID), "Use Ubuntu 22.04 or 24.04 for v1 support.")
	report.addCheck("privileges", os.Geteuid() == 0, "Daemon has the privileges required for system bootstrap.", "Install and run the daemon through a privileged systemd unit or elevated installer helper.")
	report.addCheck("daemon-group", groupExists("lara-nux"), "The lara-nux operator group exists for socket access control.", "Create the lara-nux system group before starting the daemon so client access can be mediated through Unix socket permissions.")

	for _, dependency := range []string{"systemctl", "resolvectl"} {
		_, depErr := exec.LookPath(dependency)
		report.addCheck(
			"dependency-"+dependency,
			depErr == nil,
			fmt.Sprintf("Required host dependency %s is available.", dependency),
			fmt.Sprintf("Install or restore the Ubuntu system package that provides %s before bootstrapping Lara Nuxt.", dependency),
		)
	}

	report.addCheck("rollback-manifest", len(report.Manifest.RollbackTargets()) > 0, "Managed rollback assets are declared for daemon-owned host changes.", "Keep the managed-asset manifest populated before shipping installer mutations.")

	if report.HasFailures() {
		return report, fmt.Errorf("bootstrap preflight failed: %w", report.Error())
	}

	return report, nil
}

func (s *BootstrapService) PrepareFilesystem() error {
	for _, dir := range []string{s.paths.ConfigDir, s.paths.StateDir, s.paths.RuntimeDir} {
		if err := os.MkdirAll(dir, 0o750); err != nil {
			return fmt.Errorf("create bootstrap directory %s: %w", dir, err)
		}
	}

	return nil
}

func (r *BootstrapReport) addCheck(name string, passed bool, successSummary string, remediation string) {
	summary := successSummary
	if !passed {
		summary = remediation
	}

	r.Checks = append(r.Checks, PreflightCheck{
		Name:        name,
		Passed:      passed,
		Summary:     summary,
		Remediation: remediation,
	})
}

func (r BootstrapReport) HasFailures() bool {
	for _, check := range r.Checks {
		if !check.Passed {
			return true
		}
	}

	return false
}

func (r BootstrapReport) Error() error {
	failures := make([]string, 0, len(r.Checks))
	for _, check := range r.Checks {
		if !check.Passed {
			failures = append(failures, fmt.Sprintf("%s: %s", check.Name, check.Summary))
		}
	}

	if len(failures) == 0 {
		return nil
	}

	return errors.New(strings.Join(failures, "; "))
}

func (r BootstrapReport) ErrorOr(fallback error) error {
	if reportErr := r.Error(); reportErr != nil {
		return reportErr
	}

	return fallback
}

func supportedUbuntuRelease(version string) bool {
	switch version {
	case "22.04", "24.04":
		return true
	default:
		return false
	}
}

func readOSRelease(path string) (UbuntuRelease, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return UbuntuRelease{}, err
	}

	values := map[string]string{}
	for _, line := range strings.Split(string(content), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		key, value, found := strings.Cut(line, "=")
		if !found {
			continue
		}

		values[key] = strings.Trim(value, `"`)
	}

	return UbuntuRelease{
		ID:              values["ID"],
		VersionID:       values["VERSION_ID"],
		VersionCodename: values["VERSION_CODENAME"],
		PrettyName:      values["PRETTY_NAME"],
	}, nil
}

func envOrDefault(key string, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}

	return fallback
}

func groupExists(name string) bool {
	_, err := user.LookupGroup(name)
	return err == nil
}
