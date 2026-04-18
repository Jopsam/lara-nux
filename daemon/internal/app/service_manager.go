package app

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jopsam/lara-nux/daemon/internal/host"
)

var ErrInvalidServiceAction = errors.New("invalid service action")

type ServiceAction = host.ServiceAction

const (
	ServiceActionStart   ServiceAction = host.ServiceActionStart
	ServiceActionStop    ServiceAction = host.ServiceActionStop
	ServiceActionRestart ServiceAction = host.ServiceActionRestart
	ServiceActionStatus  ServiceAction = host.ServiceActionStatus
)

type ServiceState = host.ServiceState

const (
	ServiceStateActive   ServiceState = host.ServiceStateActive
	ServiceStateInactive ServiceState = host.ServiceStateInactive
	ServiceStateFailed   ServiceState = host.ServiceStateFailed
	ServiceStateUnknown  ServiceState = host.ServiceStateUnknown
)

type ServiceStatus = host.ServiceStatus

type ServiceManager struct {
	manager host.ServiceManager
}

func NewServiceManager(manager host.ServiceManager) *ServiceManager {
	if manager == nil {
		panic("app: host service manager is required")
	}

	return &ServiceManager{
		manager: manager,
	}
}

func (m *ServiceManager) Action(ctx context.Context, service string, action ServiceAction) (ServiceStatus, error) {
	service = strings.TrimSpace(service)
	if service == "" {
		return ServiceStatus{}, fmt.Errorf("%w: service name is required", ErrInvalidServiceAction)
	}

	switch action {
	case ServiceActionStatus, ServiceActionStart, ServiceActionStop, ServiceActionRestart:
		return m.manager.Action(ctx, service, host.ServiceAction(action))
	default:
		return ServiceStatus{}, fmt.Errorf("%w: %s", ErrInvalidServiceAction, action)
	}
}

func (m *ServiceManager) Start(ctx context.Context, service string) (ServiceStatus, error) {
	return m.Action(ctx, service, ServiceActionStart)
}

func (m *ServiceManager) Stop(ctx context.Context, service string) (ServiceStatus, error) {
	return m.Action(ctx, service, ServiceActionStop)
}

func (m *ServiceManager) Restart(ctx context.Context, service string) (ServiceStatus, error) {
	return m.Action(ctx, service, ServiceActionRestart)
}

func (m *ServiceManager) Status(ctx context.Context, service string) (ServiceStatus, error) {
	service = strings.TrimSpace(service)
	if service == "" {
		return ServiceStatus{}, fmt.Errorf("%w: service name is required", ErrInvalidServiceAction)
	}

	return m.manager.Status(ctx, service)
}
