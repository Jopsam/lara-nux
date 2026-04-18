package resolved

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/jopsam/lara-nux/daemon/internal/host"
)

const managedMarker = "# Managed by Lara Nux"

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
	MainConfigPath string
	DropInDir      string
	StubPath       string
	Runner         commandRunner
}

type Manager struct {
	mainConfigPath string
	dropInDir      string
	stubPath       string
	runner         commandRunner
}

var _ host.ResolverManager = (*Manager)(nil)

func NewManager(config Config) *Manager {
	mainConfigPath := strings.TrimSpace(config.MainConfigPath)
	if mainConfigPath == "" {
		mainConfigPath = "/etc/systemd/resolved.conf"
	}

	dropInDir := strings.TrimSpace(config.DropInDir)
	if dropInDir == "" {
		dropInDir = "/etc/systemd/resolved.conf.d"
	}

	stubPath := strings.TrimSpace(config.StubPath)
	if stubPath == "" {
		stubPath = filepath.Join(dropInDir, "lara-nux-test.conf")
	}

	runner := config.Runner
	if runner == nil {
		runner = execRunner{}
	}

	return &Manager{
		mainConfigPath: mainConfigPath,
		dropInDir:      dropInDir,
		stubPath:       stubPath,
		runner:         runner,
	}
}

func (m *Manager) Inspect(ctx context.Context, spec host.ResolverStubSpec) (host.ResolverStatus, error) {
	spec = normalizeSpec(spec)
	status := host.ResolverStatus{
		StubPath: m.stubPath,
		Domain:   spec.Domain,
		Address:  spec.Address,
		Owner:    "available",
		Summary:  fmt.Sprintf("Resolver domain ~%s is available for Lara Nux management.", spec.Domain),
	}

	select {
	case <-ctx.Done():
		return status, ctx.Err()
	default:
	}

	managedPayload, managedExists, err := readIfExists(m.stubPath)
	if err != nil {
		return status, fmt.Errorf("inspect managed resolver stub: %w", err)
	}

	if managedExists {
		if !isManagedContent(managedPayload) {
			status.Owner = "external"
			status.Conflicts = append(status.Conflicts, host.Conflict{
				Resource:    m.stubPath,
				Owner:       "external",
				Summary:     fmt.Sprintf("Existing resolver stub at %s is not owned by Lara Nux.", m.stubPath),
				Remediation: fmt.Sprintf("Move or merge the external resolver drop-in before Lara Nux manages ~%s.", spec.Domain),
			})
		} else if strings.TrimSpace(string(managedPayload)) == strings.TrimSpace(renderStub(spec)) {
			status.Managed = true
			status.Owner = "lara-nux"
			status.Summary = fmt.Sprintf("Resolver domain ~%s is already managed by Lara Nux.", spec.Domain)
		} else {
			status.Owner = "lara-nux"
			status.Summary = fmt.Sprintf("Resolver domain ~%s is managed by Lara Nux with drifted content.", spec.Domain)
		}
	}

	claimants, err := m.externalClaimants(spec.Domain)
	if err != nil {
		return status, err
	}
	status.Conflicts = append(status.Conflicts, claimants...)

	if len(status.Conflicts) > 0 {
		status.Owner = "conflict"
		status.Managed = false
		status.Summary = fmt.Sprintf("Resolver ownership for ~%s conflicts with existing host configuration.", spec.Domain)
		status.Remediation = fmt.Sprintf("Remove or update the conflicting resolver drop-ins before Lara Nux manages ~%s.", spec.Domain)
	}

	return status, nil
}

func (m *Manager) EnsureTestStub(ctx context.Context, spec host.ResolverStubSpec) (host.ResolverStatus, error) {
	spec = normalizeSpec(spec)
	status, err := m.Inspect(ctx, spec)
	if err != nil {
		return status, err
	}

	if len(status.Conflicts) > 0 {
		return status, fmt.Errorf("%w: %s", host.ErrResolverConflict, status.Summary)
	}

	previous, existed, err := readIfExists(m.stubPath)
	if err != nil {
		return status, fmt.Errorf("read resolver stub backup: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(m.stubPath), 0o755); err != nil {
		return status, fmt.Errorf("create resolver drop-in directory: %w", err)
	}

	if err := writeAtomically(m.stubPath, []byte(renderStub(spec)), 0o644); err != nil {
		return status, fmt.Errorf("write resolver stub: %w", err)
	}

	if err := m.reload(ctx); err != nil {
		_ = restoreFile(m.stubPath, previous, existed, 0o644)
		_ = m.reload(ctx)
		return status, fmt.Errorf("apply resolver stub: %w", err)
	}

	return m.Inspect(ctx, spec)
}

func (m *Manager) RemoveManagedStub(ctx context.Context) error {
	payload, exists, err := readIfExists(m.stubPath)
	if err != nil || !exists {
		return err
	}

	if !isManagedContent(payload) {
		return fmt.Errorf("%w: resolver stub at %s is not owned by Lara Nux", host.ErrManagedAssetConflict, m.stubPath)
	}

	if err := os.Remove(m.stubPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("remove resolver stub: %w", err)
	}

	if err := m.reload(ctx); err != nil {
		_ = writeAtomically(m.stubPath, payload, 0o644)
		_ = m.reload(ctx)
		return fmt.Errorf("reload resolver after removing managed stub: %w", err)
	}

	return nil
}

func (m *Manager) externalClaimants(domain string) ([]host.Conflict, error) {
	paths := []string{m.mainConfigPath}
	dropIns, err := filepath.Glob(filepath.Join(m.dropInDir, "*.conf"))
	if err != nil {
		return nil, fmt.Errorf("list resolver drop-ins: %w", err)
	}
	paths = append(paths, dropIns...)
	sort.Strings(paths)

	conflicts := []host.Conflict{}
	for _, path := range paths {
		if path == m.stubPath {
			continue
		}

		payload, exists, readErr := readIfExists(path)
		if readErr != nil {
			return nil, fmt.Errorf("inspect resolver config %s: %w", path, readErr)
		}
		if !exists || !ownsDomain(payload, domain) {
			continue
		}

		conflicts = append(conflicts, host.Conflict{
			Resource:    path,
			Owner:       "external",
			Summary:     fmt.Sprintf("Resolver config %s already claims routing for ~%s.", path, domain),
			Remediation: fmt.Sprintf("Remove the ~%s claim from %s or let Lara Nux manage a different development domain.", domain, path),
		})
	}

	return conflicts, nil
}

func (m *Manager) reload(ctx context.Context) error {
	output, err := m.runner.Run(ctx, "systemctl", "restart", "systemd-resolved")
	if err != nil {
		return fmt.Errorf("restart systemd-resolved: %v (%s)", err, strings.TrimSpace(output))
	}

	flushOutput, flushErr := m.runner.Run(ctx, "resolvectl", "flush-caches")
	if flushErr != nil {
		return fmt.Errorf("flush resolved caches: %v (%s)", flushErr, strings.TrimSpace(flushOutput))
	}

	return nil
}

func normalizeSpec(spec host.ResolverStubSpec) host.ResolverStubSpec {
	domain := strings.TrimPrefix(strings.ToLower(strings.TrimSpace(spec.Domain)), ".")
	if domain == "" {
		domain = "test"
	}

	address := strings.TrimSpace(spec.Address)
	if address == "" {
		address = "127.0.0.1"
	}

	return host.ResolverStubSpec{Domain: domain, Address: address}
}

func renderStub(spec host.ResolverStubSpec) string {
	return fmt.Sprintf("%s\n[Resolve]\nDNS=%s\nDomains=~%s\n", managedMarker, spec.Address, spec.Domain)
}

func ownsDomain(payload []byte, domain string) bool {
	needle := "~" + strings.TrimPrefix(strings.ToLower(strings.TrimSpace(domain)), ".")
	for _, line := range strings.Split(string(payload), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}

		key, value, found := strings.Cut(line, "=")
		if !found || !strings.EqualFold(strings.TrimSpace(key), "Domains") {
			continue
		}

		for _, token := range strings.Fields(value) {
			if strings.EqualFold(strings.TrimSpace(token), needle) {
				return true
			}
		}
	}

	return false
}

func isManagedContent(payload []byte) bool {
	return strings.Contains(string(payload), managedMarker)
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
	return os.Rename(tempPath, path)
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
