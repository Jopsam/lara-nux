package host

import (
	"context"
	"errors"
	"time"
)

var (
	ErrResolverConflict      = errors.New("resolver conflict detected")
	ErrManagedAssetConflict  = errors.New("managed asset conflict")
	ErrActivationValidation  = errors.New("activation validation failed")
	ErrUnsupportedPackage    = errors.New("unsupported package target")
	ErrPackageVerification   = errors.New("package verification failed")
	ErrRuntimeSwitchRollback = errors.New("php runtime switch rolled back")
)

type Conflict struct {
	Resource    string `json:"resource"`
	Owner       string `json:"owner,omitempty"`
	Summary     string `json:"summary"`
	Remediation string `json:"remediation,omitempty"`
}

type ResolverStubSpec struct {
	Domain  string `json:"domain"`
	Address string `json:"address"`
}

type ResolverStatus struct {
	Managed     bool       `json:"managed"`
	StubPath    string     `json:"stubPath"`
	Domain      string     `json:"domain"`
	Address     string     `json:"address"`
	Owner       string     `json:"owner"`
	Conflicts   []Conflict `json:"conflicts,omitempty"`
	Summary     string     `json:"summary"`
	Remediation string     `json:"remediation,omitempty"`
}

type ResolverManager interface {
	Inspect(ctx context.Context, spec ResolverStubSpec) (ResolverStatus, error)
	EnsureTestStub(ctx context.Context, spec ResolverStubSpec) (ResolverStatus, error)
	RemoveManagedStub(ctx context.Context) error
}

type WebSite struct {
	ID            string `json:"id"`
	Domain        string `json:"domain"`
	RootPath      string `json:"rootPath"`
	PublicDir     string `json:"publicDir,omitempty"`
	PHPSocketPath string `json:"phpSocketPath"`
}

type WebActivationResult struct {
	ConfigPath string `json:"configPath"`
	Validated  bool   `json:"validated"`
	Reloaded   bool   `json:"reloaded"`
	HTTPURL    string `json:"httpUrl"`
	HTTPSURL   string `json:"httpsUrl"`
}

type WebServerManager interface {
	ActivateSite(ctx context.Context, site WebSite) (WebActivationResult, error)
	RemoveSite(ctx context.Context, siteID string) error
	Validate(ctx context.Context) error
}

type PHPRuntime struct {
	Version       string `json:"version"`
	BinaryPath    string `json:"binaryPath,omitempty"`
	FPMBinaryPath string `json:"fpmBinaryPath,omitempty"`
	ServiceName   string `json:"serviceName,omitempty"`
	SocketPath    string `json:"socketPath,omitempty"`
}

type PHPMaterialization struct {
	Version        string `json:"version"`
	ServiceName    string `json:"serviceName"`
	SocketPath     string `json:"socketPath"`
	PoolConfigPath string `json:"poolConfigPath"`
	OverridePath   string `json:"overridePath"`
	Active         bool   `json:"active"`
}

type PHPSwitchRequest struct {
	SiteID   string     `json:"siteId,omitempty"`
	Previous PHPRuntime `json:"previous"`
	Target   PHPRuntime `json:"target"`
}

type PHPManager interface {
	MaterializeRuntime(ctx context.Context, runtime PHPRuntime) (PHPMaterialization, error)
	SwitchRuntime(ctx context.Context, request PHPSwitchRequest) (PHPMaterialization, error)
	RemoveRuntime(ctx context.Context, runtime PHPRuntime) error
}

type SupportedPackage struct {
	Key            string   `json:"key"`
	Description    string   `json:"description"`
	RuntimeVersion string   `json:"runtimeVersion,omitempty"`
	Packages       []string `json:"packages"`
}

type PackageRequest struct {
	Key string `json:"key"`
}

type PackageReceipt struct {
	Key         string    `json:"key"`
	Packages    []string  `json:"packages"`
	InstalledAt time.Time `json:"installedAt"`
}

type PackageVerification struct {
	Key      string            `json:"key"`
	Verified bool              `json:"verified"`
	Details  map[string]string `json:"details,omitempty"`
}

type ServiceAction string

const (
	ServiceActionStart   ServiceAction = "start"
	ServiceActionStop    ServiceAction = "stop"
	ServiceActionRestart ServiceAction = "restart"
	ServiceActionStatus  ServiceAction = "status"
)

type ServiceState string

const (
	ServiceStateActive   ServiceState = "active"
	ServiceStateInactive ServiceState = "inactive"
	ServiceStateFailed   ServiceState = "failed"
	ServiceStateUnknown  ServiceState = "unknown"
)

type ServiceStatus struct {
	Service   string       `json:"service"`
	State     ServiceState `json:"state"`
	Summary   string       `json:"summary"`
	UpdatedAt time.Time    `json:"updatedAt"`
}

type ServiceManager interface {
	Action(ctx context.Context, service string, action ServiceAction) (ServiceStatus, error)
	Start(ctx context.Context, service string) (ServiceStatus, error)
	Stop(ctx context.Context, service string) (ServiceStatus, error)
	Restart(ctx context.Context, service string) (ServiceStatus, error)
	Status(ctx context.Context, service string) (ServiceStatus, error)
}

type PackageManager interface {
	SupportedPackages() []SupportedPackage
	Acquire(ctx context.Context, request PackageRequest) (PackageReceipt, error)
	Verify(ctx context.Context, key string) (PackageVerification, error)
	RefreshRuntimeInventory(ctx context.Context) ([]PHPRuntime, error)
}
