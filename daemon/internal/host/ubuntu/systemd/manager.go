package systemd

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/jopsam/lara-nux/daemon/internal/host"
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
	Runner commandRunner
}

type Manager struct {
	runner commandRunner
	clock  func() time.Time
}

var _ host.ServiceManager = (*Manager)(nil)

func NewManager(config Config) *Manager {
	runner := config.Runner
	if runner == nil {
		runner = execRunner{}
	}

	return &Manager{
		runner: runner,
		clock:  func() time.Time { return time.Now().UTC() },
	}
}

func (m *Manager) Action(ctx context.Context, service string, action host.ServiceAction) (host.ServiceStatus, error) {
	switch action {
	case host.ServiceActionStatus:
		return m.Status(ctx, service)
	case host.ServiceActionStart, host.ServiceActionStop, host.ServiceActionRestart:
		output, err := m.runner.Run(ctx, "systemctl", string(action), service)
		if err != nil {
			status, _ := m.Status(ctx, service)
			if strings.TrimSpace(output) == "" {
				output = err.Error()
			}
			status.Summary = strings.TrimSpace(output)
			return status, fmt.Errorf("%s %s: %w", action, service, err)
		}
		return m.Status(ctx, service)
	default:
		return host.ServiceStatus{}, fmt.Errorf("unsupported service action: %s", action)
	}
}

func (m *Manager) Start(ctx context.Context, service string) (host.ServiceStatus, error) {
	return m.Action(ctx, service, host.ServiceActionStart)
}

func (m *Manager) Stop(ctx context.Context, service string) (host.ServiceStatus, error) {
	return m.Action(ctx, service, host.ServiceActionStop)
}

func (m *Manager) Restart(ctx context.Context, service string) (host.ServiceStatus, error) {
	return m.Action(ctx, service, host.ServiceActionRestart)
}

func (m *Manager) Status(ctx context.Context, service string) (host.ServiceStatus, error) {
	output, err := m.runner.Run(ctx, "systemctl", "is-active", service)
	state, summary := parseServiceState(output, err)

	status := host.ServiceStatus{
		Service:   service,
		State:     state,
		Summary:   summary,
		UpdatedAt: m.clock(),
	}

	if err != nil && state == host.ServiceStateUnknown {
		return status, fmt.Errorf("inspect service %s: %w", service, err)
	}

	return status, nil
}

func parseServiceState(output string, err error) (host.ServiceState, string) {
	trimmed := strings.TrimSpace(output)
	switch trimmed {
	case "active", "activating":
		return host.ServiceStateActive, serviceSummary(trimmed, err)
	case "inactive", "deactivating":
		return host.ServiceStateInactive, serviceSummary(trimmed, err)
	case "failed":
		return host.ServiceStateFailed, serviceSummary(trimmed, err)
	case "":
		if err != nil {
			return host.ServiceStateUnknown, err.Error()
		}
		return host.ServiceStateUnknown, "service state unknown"
	default:
		return host.ServiceStateUnknown, serviceSummary(trimmed, err)
	}
}

func serviceSummary(output string, err error) string {
	if err == nil {
		return output
	}

	if output == "" {
		return err.Error()
	}

	return output
}
