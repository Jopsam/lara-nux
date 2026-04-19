package caddy

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/jopsam/lara-nux/daemon/internal/host"
)

const (
	managedMarker          = "# Managed by Lara Nux"
	managedImportStartLine = "# Lara Nux managed Caddy import start"
	managedImportEndLine   = "# Lara Nux managed Caddy import end"
	managedImportLine      = "import sites.d/lara-nux/*.caddy"
)

type commandRunner interface {
	Run(ctx context.Context, name string, args ...string) (string, error)
}

type execRunner struct{}

func (execRunner) Run(ctx context.Context, name string, args ...string) (string, error) {
	command := exec.CommandContext(ctx, name, args...)
	output, err := command.CombinedOutput()
	return string(output), err
}

type Config struct {
	SitesDir       string
	RootConfigPath string
	Runner         commandRunner
}

type Manager struct {
	sitesDir       string
	rootConfigPath string
	runner         commandRunner
}

var _ host.WebServerManager = (*Manager)(nil)

func NewManager(config Config) *Manager {
	sitesDir := strings.TrimSpace(config.SitesDir)
	if sitesDir == "" {
		sitesDir = "/etc/caddy/sites.d/lara-nux"
	}

	rootConfigPath := strings.TrimSpace(config.RootConfigPath)
	if rootConfigPath == "" {
		rootConfigPath = "/etc/caddy/Caddyfile"
	}

	runner := config.Runner
	if runner == nil {
		runner = execRunner{}
	}

	return &Manager{
		sitesDir:       sitesDir,
		rootConfigPath: rootConfigPath,
		runner:         runner,
	}
}

func (m *Manager) ActivateSite(ctx context.Context, site host.WebSite) (host.WebActivationResult, error) {
	site, err := normalizeSite(site)
	if err != nil {
		return host.WebActivationResult{}, err
	}

	rootPrevious, rootExisted, err := readIfExists(m.rootConfigPath)
	if err != nil {
		return host.WebActivationResult{}, fmt.Errorf("read caddy root config: %w", err)
	}

	rootChanged := false
	if rootNext, changed := ensureManagedRootImport(rootPrevious, rootExisted); changed {
		if err := ensureDirMode(filepath.Dir(m.rootConfigPath), 0o755); err != nil {
			return host.WebActivationResult{}, fmt.Errorf("create caddy root config directory: %w", err)
		}
		if err := writeAtomically(m.rootConfigPath, rootNext, 0o644); err != nil {
			return host.WebActivationResult{}, fmt.Errorf("write caddy root config: %w", err)
		}
		rootChanged = true
	}

	configPath := filepath.Join(m.sitesDir, siteFilename(site))
	previous, existed, err := readIfExists(configPath)
	if err != nil {
		if rootChanged {
			_ = restoreFile(m.rootConfigPath, rootPrevious, rootExisted, 0o644)
		}
		return host.WebActivationResult{}, fmt.Errorf("read existing caddy site config: %w", err)
	}

	if err := ensureDirMode(filepath.Dir(m.sitesDir), 0o755); err != nil {
		if rootChanged {
			_ = restoreFile(m.rootConfigPath, rootPrevious, rootExisted, 0o644)
		}
		return host.WebActivationResult{}, fmt.Errorf("create caddy sites parent directory: %w", err)
	}

	if err := ensureDirMode(m.sitesDir, 0o755); err != nil {
		if rootChanged {
			_ = restoreFile(m.rootConfigPath, rootPrevious, rootExisted, 0o644)
		}
		return host.WebActivationResult{}, fmt.Errorf("create caddy site directory: %w", err)
	}

	if err := writeAtomically(configPath, []byte(renderSiteConfig(site)), 0o644); err != nil {
		if rootChanged {
			_ = restoreFile(m.rootConfigPath, rootPrevious, rootExisted, 0o644)
		}
		return host.WebActivationResult{}, fmt.Errorf("write caddy site config: %w", err)
	}

	if err := m.Validate(ctx); err != nil {
		_ = restoreFile(configPath, previous, existed, 0o644)
		if rootChanged {
			_ = restoreFile(m.rootConfigPath, rootPrevious, rootExisted, 0o644)
		}
		return host.WebActivationResult{}, fmt.Errorf("%w: %s", host.ErrActivationValidation, err.Error())
	}

	if err := m.reload(ctx); err != nil {
		_ = restoreFile(configPath, previous, existed, 0o644)
		if rootChanged {
			_ = restoreFile(m.rootConfigPath, rootPrevious, rootExisted, 0o644)
		}
		_ = m.Validate(ctx)
		_ = m.reload(ctx)
		return host.WebActivationResult{}, fmt.Errorf("reload caddy after activating %s: %w", site.Domain, err)
	}

	return host.WebActivationResult{
		ConfigPath: configPath,
		Validated:  true,
		Reloaded:   true,
		HTTPURL:    "http://" + site.Domain,
		HTTPSURL:   "https://" + site.Domain,
	}, nil
}

func (m *Manager) RemoveSite(ctx context.Context, siteID string) error {
	fileName := strings.TrimSpace(siteID)
	if fileName == "" {
		return nil
	}

	rootPrevious, rootExisted, err := readIfExists(m.rootConfigPath)
	if err != nil {
		return fmt.Errorf("read caddy root config: %w", err)
	}

	configPath := filepath.Join(m.sitesDir, sanitizeToken(fileName)+".caddy")
	previous, existed, err := readIfExists(configPath)
	if err != nil || !existed {
		return err
	}

	if !strings.Contains(string(previous), managedMarker) {
		return fmt.Errorf("%w: caddy site config %s is not owned by Lara Nux", host.ErrManagedAssetConflict, configPath)
	}

	if err := os.Remove(configPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("remove caddy site config: %w", err)
	}

	rootChanged := false
	remainingConfigs, err := filepath.Glob(filepath.Join(m.sitesDir, "*.caddy"))
	if err != nil {
		_ = writeAtomically(configPath, previous, 0o644)
		return fmt.Errorf("list remaining caddy configs: %w", err)
	}
	if len(remainingConfigs) == 0 {
		rootNext, changed := removeManagedRootImport(rootPrevious, rootExisted)
		if changed {
			if err := restoreFile(m.rootConfigPath, rootNext, rootNext != nil, 0o644); err != nil {
				_ = writeAtomically(configPath, previous, 0o644)
				return fmt.Errorf("remove managed caddy root import: %w", err)
			}
			rootChanged = true
		}
	}

	if err := m.Validate(ctx); err != nil {
		_ = writeAtomically(configPath, previous, 0o644)
		if rootChanged {
			_ = restoreFile(m.rootConfigPath, rootPrevious, rootExisted, 0o644)
		}
		return fmt.Errorf("validate caddy after removing %s: %w", siteID, err)
	}

	if err := m.reload(ctx); err != nil {
		_ = writeAtomically(configPath, previous, 0o644)
		if rootChanged {
			_ = restoreFile(m.rootConfigPath, rootPrevious, rootExisted, 0o644)
		}
		_ = m.Validate(ctx)
		_ = m.reload(ctx)
		return fmt.Errorf("reload caddy after removing %s: %w", siteID, err)
	}

	return nil
}

func (m *Manager) Validate(ctx context.Context) error {
	output, err := m.runner.Run(ctx, "caddy", "validate", "--config", m.rootConfigPath, "--adapter", "caddyfile")
	if err != nil {
		return fmt.Errorf("validate caddy config: %v (%s)", err, strings.TrimSpace(output))
	}
	return nil
}

func (m *Manager) reload(ctx context.Context) error {
	output, err := m.runner.Run(ctx, "systemctl", "reload", "caddy")
	if err != nil {
		return fmt.Errorf("reload caddy: %v (%s)", err, strings.TrimSpace(output))
	}
	return nil
}

func normalizeSite(site host.WebSite) (host.WebSite, error) {
	if strings.TrimSpace(site.Domain) == "" {
		return host.WebSite{}, fmt.Errorf("%w: domain is required", host.ErrActivationValidation)
	}
	if strings.TrimSpace(site.RootPath) == "" {
		return host.WebSite{}, fmt.Errorf("%w: root path is required", host.ErrActivationValidation)
	}
	if strings.TrimSpace(site.PHPSocketPath) == "" {
		return host.WebSite{}, fmt.Errorf("%w: php socket path is required", host.ErrActivationValidation)
	}

	publicDir := strings.TrimSpace(site.PublicDir)
	if publicDir == "" {
		publicDir = filepath.Join(site.RootPath, "public")
	}

	if info, err := os.Stat(publicDir); err != nil || !info.IsDir() {
		if err != nil {
			return host.WebSite{}, fmt.Errorf("%w: inspect public directory: %v", host.ErrActivationValidation, err)
		}
		return host.WebSite{}, fmt.Errorf("%w: public directory %s is not a directory", host.ErrActivationValidation, publicDir)
	}

	site.RootPath = filepath.Clean(site.RootPath)
	site.PublicDir = filepath.Clean(publicDir)
	site.ID = sanitizeToken(firstNonEmpty(site.ID, site.Domain))
	site.Domain = strings.ToLower(strings.TrimSpace(site.Domain))
	site.PHPSocketPath = filepath.Clean(site.PHPSocketPath)
	return site, nil
}

func renderSiteConfig(site host.WebSite) string {
	return fmt.Sprintf(`%s
%s {
	root * %q
	encode zstd gzip
	php_fastcgi unix/%s
	file_server
}

%s {
	tls internal
	root * %q
	encode zstd gzip
	php_fastcgi unix/%s
	file_server
}
`, managedMarker, "http://"+site.Domain, site.PublicDir, site.PHPSocketPath, "https://"+site.Domain, site.PublicDir, site.PHPSocketPath)
}

func ensureManagedRootImport(existing []byte, existed bool) ([]byte, bool) {
	if existed && strings.Contains(string(existing), managedImportLine) {
		return existing, false
	}

	base := strings.TrimRight(string(existing), "\n")
	block := strings.Join([]string{managedImportStartLine, managedImportLine, managedImportEndLine}, "\n")
	if base == "" {
		return []byte(block + "\n"), true
	}
	return []byte(base + "\n\n" + block + "\n"), true
}

func removeManagedRootImport(existing []byte, existed bool) ([]byte, bool) {
	if !existed {
		return nil, false
	}

	content := string(existing)
	if !strings.Contains(content, managedImportStartLine) {
		return existing, false
	}

	lines := strings.Split(content, "\n")
	filtered := make([]string, 0, len(lines))
	skip := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == managedImportStartLine {
			skip = true
			continue
		}
		if trimmed == managedImportEndLine {
			skip = false
			continue
		}
		if skip {
			continue
		}
		filtered = append(filtered, line)
	}

	result := strings.TrimSpace(strings.Join(filtered, "\n"))
	if result == "" {
		return nil, true
	}
	return []byte(result + "\n"), true
}

func siteFilename(site host.WebSite) string {
	return sanitizeToken(firstNonEmpty(site.ID, site.Domain)) + ".caddy"
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed != "" {
			return trimmed
		}
	}
	return "site"
}

func sanitizeToken(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	replacer := strings.NewReplacer("/", "-", " ", "-", ".", "-", "_", "-")
	value = replacer.Replace(value)
	for strings.Contains(value, "--") {
		value = strings.ReplaceAll(value, "--", "-")
	}
	return strings.Trim(value, "-")
}

func readIfExists(path string) ([]byte, bool, error) {
	payload, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, false, nil
		}
		return nil, false, err
	}
	return payload, true, nil
}

func writeAtomically(path string, payload []byte, mode os.FileMode) error {
	tempPath := path + ".tmp"
	if err := os.WriteFile(tempPath, payload, mode); err != nil {
		return err
	}
	if err := os.Chmod(tempPath, mode); err != nil {
		_ = os.Remove(tempPath)
		return err
	}
	if err := os.Rename(tempPath, path); err != nil {
		_ = os.Remove(tempPath)
		return err
	}
	return os.Chmod(path, mode)
}

func restoreFile(path string, payload []byte, existed bool, mode os.FileMode) error {
	if !existed {
		if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		return nil
	}
	return writeAtomically(path, payload, mode)
}

func ensureDirMode(path string, mode os.FileMode) error {
	if err := os.MkdirAll(path, mode); err != nil {
		return err
	}
	return os.Chmod(path, mode)
}
