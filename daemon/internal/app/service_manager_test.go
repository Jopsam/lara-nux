package app

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jopsam/lara-nux/daemon/internal/host"
)

type fakeHostServiceManager struct {
	actionCalls []struct {
		service string
		action  host.ServiceAction
	}
	statusCalls []string
	actionFn    func(context.Context, string, host.ServiceAction) (host.ServiceStatus, error)
	statusFn    func(context.Context, string) (host.ServiceStatus, error)
}

func (f *fakeHostServiceManager) Action(ctx context.Context, service string, action host.ServiceAction) (host.ServiceStatus, error) {
	f.actionCalls = append(f.actionCalls, struct {
		service string
		action  host.ServiceAction
	}{service: service, action: action})
	if f.actionFn != nil {
		return f.actionFn(ctx, service, action)
	}
	return host.ServiceStatus{Service: service, State: host.ServiceStateActive, Summary: string(action), UpdatedAt: time.Now().UTC()}, nil
}

func (f *fakeHostServiceManager) Start(ctx context.Context, service string) (host.ServiceStatus, error) {
	return f.Action(ctx, service, host.ServiceActionStart)
}

func (f *fakeHostServiceManager) Stop(ctx context.Context, service string) (host.ServiceStatus, error) {
	return f.Action(ctx, service, host.ServiceActionStop)
}

func (f *fakeHostServiceManager) Restart(ctx context.Context, service string) (host.ServiceStatus, error) {
	return f.Action(ctx, service, host.ServiceActionRestart)
}

func (f *fakeHostServiceManager) Status(ctx context.Context, service string) (host.ServiceStatus, error) {
	f.statusCalls = append(f.statusCalls, service)
	if f.statusFn != nil {
		return f.statusFn(ctx, service)
	}
	return host.ServiceStatus{Service: service, State: host.ServiceStateActive, Summary: "active", UpdatedAt: time.Now().UTC()}, nil
}

func TestServiceManagerDelegatesToHostBoundary(t *testing.T) {
	t.Parallel()

	fake := &fakeHostServiceManager{}
	manager := NewServiceManager(fake)

	status, err := manager.Action(context.Background(), "caddy", ServiceActionRestart)
	if err != nil {
		t.Fatalf("Action returned error: %v", err)
	}

	if len(fake.actionCalls) != 1 {
		t.Fatalf("expected 1 host action call, got %d", len(fake.actionCalls))
	}
	if fake.actionCalls[0].service != "caddy" || fake.actionCalls[0].action != host.ServiceActionRestart {
		t.Fatalf("unexpected host action call: %+v", fake.actionCalls[0])
	}
	if status.Service != "caddy" || status.Summary != "restart" {
		t.Fatalf("unexpected status: %+v", status)
	}
}

func TestServiceManagerRejectsBlankServiceBeforeHostCall(t *testing.T) {
	t.Parallel()

	fake := &fakeHostServiceManager{}
	manager := NewServiceManager(fake)

	_, err := manager.Status(context.Background(), "   ")
	if !errors.Is(err, ErrInvalidServiceAction) {
		t.Fatalf("expected ErrInvalidServiceAction, got %v", err)
	}
	if len(fake.statusCalls) != 0 {
		t.Fatalf("expected no host status calls, got %d", len(fake.statusCalls))
	}
}
