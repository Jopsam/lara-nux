package systemd

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/jopsam/lara-nux/daemon/internal/host"
)

func TestStatusMapsSystemctlStates(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		output  string
		err     error
		state   host.ServiceState
		summary string
	}{
		{name: "active", output: "active\n", state: host.ServiceStateActive, summary: "active"},
		{name: "inactive", output: "inactive\n", state: host.ServiceStateInactive, summary: "inactive"},
		{name: "failed", output: "failed\n", state: host.ServiceStateFailed, summary: "failed"},
		{name: "unknown with stderr", output: "reloading\n", err: errors.New("exit status 3"), state: host.ServiceStateUnknown, summary: "reloading"},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			runner := &systemdRunner{results: map[string]systemdRunResult{
				"systemctl is-active caddy": {output: tt.output, err: tt.err},
			}}
			manager := NewManager(Config{Runner: runner})
			manager.clock = func() time.Time { return time.Date(2026, time.April, 18, 12, 0, 0, 0, time.UTC) }

			status, err := manager.Status(context.Background(), "caddy")
			if tt.state == host.ServiceStateUnknown && tt.err != nil {
				if err == nil {
					t.Fatal("expected status error for unknown state")
				}
			} else if err != nil {
				t.Fatalf("unexpected status error: %v", err)
			}

			if status.State != tt.state {
				t.Fatalf("expected state %s, got %s", tt.state, status.State)
			}
			if status.Summary != tt.summary {
				t.Fatalf("expected summary %q, got %q", tt.summary, status.Summary)
			}
		})
	}
}

func TestActionReturnsLatestStatusWhenRestartFails(t *testing.T) {
	t.Parallel()

	runner := &systemdRunner{results: map[string]systemdRunResult{
		"systemctl restart caddy":   {output: "job failed", err: errors.New("exit status 1")},
		"systemctl is-active caddy": {output: "failed\n", err: errors.New("exit status 3")},
	}}
	manager := NewManager(Config{Runner: runner})
	manager.clock = func() time.Time { return time.Date(2026, time.April, 18, 12, 0, 0, 0, time.UTC) }

	status, err := manager.Action(context.Background(), "caddy", host.ServiceActionRestart)
	if err == nil {
		t.Fatal("expected restart error")
	}
	if status.State != host.ServiceStateFailed {
		t.Fatalf("expected failed state, got %s", status.State)
	}
	if status.Summary != "job failed" {
		t.Fatalf("expected action output summary, got %q", status.Summary)
	}
	if len(runner.calls) != 2 || !strings.Contains(runner.calls[0], "restart caddy") || !strings.Contains(runner.calls[1], "is-active caddy") {
		t.Fatalf("unexpected runner call order: %v", runner.calls)
	}
}

type systemdRunResult struct {
	output string
	err    error
}

type systemdRunner struct {
	results map[string]systemdRunResult
	calls   []string
}

func (r *systemdRunner) Run(_ context.Context, name string, args ...string) (string, error) {
	call := strings.TrimSpace(strings.Join(append([]string{name}, args...), " "))
	r.calls = append(r.calls, call)
	if result, ok := r.results[call]; ok {
		return result.output, result.err
	}
	return "", nil
}
