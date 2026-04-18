package app

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jopsam/lara-nux/daemon/internal/host"
)

type fakeRuntimeResolver struct {
	runtime PHPRuntimeRecord
	err     error
}

func (f fakeRuntimeResolver) Get(context.Context, string) (PHPRuntimeRecord, error) {
	return f.runtime, f.err
}
func (f fakeRuntimeResolver) DefaultRuntime(context.Context) (PHPRuntimeRecord, error) {
	return f.runtime, f.err
}
func (f fakeRuntimeResolver) List(context.Context) ([]PHPRuntimeRecord, error) {
	if f.err != nil {
		return nil, f.err
	}
	return []PHPRuntimeRecord{f.runtime}, nil
}

type fakeSiteStore struct {
	registered []SiteRecord
	deleted    []string
	updated    []SiteRecord
	list       []SiteRecord
	registerFn func(context.Context, SiteRegistrationInput) (SiteRecord, error)
}

func (f *fakeSiteStore) Register(ctx context.Context, input SiteRegistrationInput) (SiteRecord, error) {
	if f.registerFn != nil {
		return f.registerFn(ctx, input)
	}
	record := SiteRecord{ID: "site-1", Domain: "demo.test", RootPath: "/tmp/demo", PHPVersion: input.PHPVersion}
	f.registered = append(f.registered, record)
	f.list = append(f.list, record)
	return record, nil
}

func (f *fakeSiteStore) Get(context.Context, string) (SiteRecord, error) {
	return SiteRecord{}, ErrSiteNotFound
}
func (f *fakeSiteStore) Update(_ context.Context, record SiteRecord) (SiteRecord, error) {
	f.updated = append(f.updated, record)
	return record, nil
}
func (f *fakeSiteStore) Delete(_ context.Context, siteID string) error {
	f.deleted = append(f.deleted, siteID)
	filtered := f.list[:0]
	for _, site := range f.list {
		if site.ID != siteID {
			filtered = append(filtered, site)
		}
	}
	f.list = filtered
	return nil
}
func (f *fakeSiteStore) List(context.Context) ([]SiteRecord, error) {
	return append([]SiteRecord(nil), f.list...), nil
}

type fakeResolverProvisioner struct {
	ensured bool
	removed bool
}

func (f *fakeResolverProvisioner) Inspect(context.Context, host.ResolverStubSpec) (host.ResolverStatus, error) {
	return host.ResolverStatus{}, nil
}
func (f *fakeResolverProvisioner) EnsureTestStub(context.Context, host.ResolverStubSpec) (host.ResolverStatus, error) {
	f.ensured = true
	return host.ResolverStatus{Managed: true, Owner: "lara-nux", Summary: "managed"}, nil
}
func (f *fakeResolverProvisioner) RemoveManagedStub(context.Context) error {
	f.removed = true
	return nil
}

type fakeWebActivator struct{ err error }

func (f fakeWebActivator) ActivateSite(context.Context, host.WebSite) (host.WebActivationResult, error) {
	return host.WebActivationResult{}, f.err
}
func (f fakeWebActivator) RemoveSite(context.Context, string) error { return nil }
func (f fakeWebActivator) Validate(context.Context) error           { return nil }

type fakePHPMaterializer struct{}

func (fakePHPMaterializer) MaterializeRuntime(context.Context, host.PHPRuntime) (host.PHPMaterialization, error) {
	return host.PHPMaterialization{SocketPath: "/run/php/lara.sock"}, nil
}
func (fakePHPMaterializer) SwitchRuntime(context.Context, host.PHPSwitchRequest) (host.PHPMaterialization, error) {
	return host.PHPMaterialization{}, nil
}

type fakeServiceController struct{}

func (fakeServiceController) Start(context.Context, string) (ServiceStatus, error) {
	return ServiceStatus{State: ServiceStateActive}, nil
}
func (fakeServiceController) Restart(context.Context, string) (ServiceStatus, error) {
	return ServiceStatus{State: ServiceStateActive}, nil
}
func (fakeServiceController) Status(context.Context, string) (ServiceStatus, error) {
	return ServiceStatus{State: ServiceStateActive}, nil
}

func TestSiteActivationRollsBackRegisteredSiteWhenActivationFails(t *testing.T) {
	t.Parallel()

	store := &fakeSiteStore{}
	resolver := &fakeResolverProvisioner{}
	svc := NewSiteActivationService(
		store,
		fakeRuntimeResolver{runtime: PHPRuntimeRecord{Version: "8.2", FPMService: "php82-fpm"}},
		resolver,
		fakeWebActivator{err: errors.New("caddy reload failed")},
		fakePHPMaterializer{},
		fakeServiceController{},
	)
	svc.clock = func() time.Time { return time.Unix(100, 0).UTC() }

	_, err := svc.Activate(context.Background(), SiteRegistrationInput{RootPath: "/tmp/demo", PHPVersion: "8.2"})
	if err == nil {
		t.Fatal("expected activation error")
	}
	if len(store.deleted) != 1 || store.deleted[0] != "site-1" {
		t.Fatalf("expected rollback delete for site-1, got %+v", store.deleted)
	}
	if !resolver.ensured || !resolver.removed {
		t.Fatalf("expected resolver ensure+remove during rollback, got ensured=%v removed=%v", resolver.ensured, resolver.removed)
	}
	if len(store.updated) != 0 {
		t.Fatalf("did not expect site update on failed activation, got %+v", store.updated)
	}
}
