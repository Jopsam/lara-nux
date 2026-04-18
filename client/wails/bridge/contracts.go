package bridge

import "time"

type RpcError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type Conflict struct {
	Resource    string `json:"resource"`
	Owner       string `json:"owner,omitempty"`
	Summary     string `json:"summary"`
	Remediation string `json:"remediation,omitempty"`
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

type ServiceStatus struct {
	Service   string    `json:"service"`
	State     string    `json:"state"`
	Summary   string    `json:"summary"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type SiteRecord struct {
	ID            string    `json:"id"`
	Name          string    `json:"name"`
	RootPath      string    `json:"rootPath"`
	Domain        string    `json:"domain"`
	PHPVersion    string    `json:"phpVersion"`
	TLS           string    `json:"tls"`
	Status        string    `json:"status"`
	StatusMessage string    `json:"statusMessage,omitempty"`
	CreatedAt     time.Time `json:"createdAt"`
	UpdatedAt     time.Time `json:"updatedAt"`
	LastCheckedAt time.Time `json:"lastCheckedAt,omitempty"`
}

type PHPRuntimeRecord struct {
	Version      string    `json:"version"`
	BinaryPath   string    `json:"binaryPath"`
	FPMService   string    `json:"fpmService"`
	Source       string    `json:"source,omitempty"`
	RegisteredAt time.Time `json:"registeredAt"`
}

type SupportedPackage struct {
	Key            string   `json:"key"`
	Description    string   `json:"description"`
	RuntimeVersion string   `json:"runtimeVersion,omitempty"`
	Packages       []string `json:"packages"`
}

type DetectedPHPRuntime struct {
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

type WebActivationResult struct {
	ConfigPath string `json:"configPath"`
	Validated  bool   `json:"validated"`
	Reloaded   bool   `json:"reloaded"`
	HTTPURL    string `json:"httpUrl"`
	HTTPSURL   string `json:"httpsUrl"`
}

type SocketAvailability struct {
	Path      string `json:"path"`
	Available bool   `json:"available"`
	Summary   string `json:"summary"`
}

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
	Ready       bool               `json:"ready"`
	GeneratedAt time.Time          `json:"generatedAt"`
	Socket      SocketAvailability `json:"socket"`
	Resolver    *ResolverStatus    `json:"resolver,omitempty"`
	Checks      []HealthCheck      `json:"checks"`
	Services    []ServiceStatus    `json:"services"`
	Sites       []SiteReadiness    `json:"sites"`
}

type RegisterSiteRequest struct {
	RootPath   string `json:"rootPath"`
	Domain     string `json:"domain,omitempty"`
	PHPVersion string `json:"phpVersion,omitempty"`
}

type UpdateSiteRequest struct {
	SiteID     string  `json:"siteId"`
	RootPath   *string `json:"rootPath,omitempty"`
	Domain     *string `json:"domain,omitempty"`
	PHPVersion *string `json:"phpVersion,omitempty"`
}

type ActivationResult struct {
	Site            SiteRecord         `json:"site"`
	Resolver        ResolverStatus     `json:"resolver"`
	Runtime         PHPRuntimeRecord   `json:"runtime"`
	Materialization PHPMaterialization `json:"materialization"`
	Web             WebActivationResult `json:"web"`
	Services        []ServiceStatus    `json:"services"`
	ActivatedAt     time.Time          `json:"activatedAt"`
}

type RuntimeCatalog struct {
	Registered        []PHPRuntimeRecord   `json:"registered"`
	DefaultRuntime    *PHPRuntimeRecord    `json:"defaultRuntime,omitempty"`
	SupportedPackages []SupportedPackage   `json:"supportedPackages"`
	DetectedRuntimes  []DetectedPHPRuntime `json:"detectedRuntimes"`
}

type DefaultRuntimeResponse struct {
	Runtime *PHPRuntimeRecord `json:"runtime"`
}

type SetDefaultPHPRequest struct {
	Version string `json:"version"`
}

type SwitchPHPRequest struct {
	SiteID     string `json:"siteId"`
	PHPVersion string `json:"phpVersion"`
}

type ServiceActionRequest struct {
	Service string `json:"service"`
	Action  string `json:"action"`
}

type ShellStatus struct {
	SocketPath   string    `json:"socketPath"`
	Connected    bool      `json:"connected"`
	LastSyncedAt time.Time `json:"lastSyncedAt,omitempty"`
	LastError    string    `json:"lastError,omitempty"`
}

type DashboardSnapshot struct {
	Health   HealthReport   `json:"health"`
	Sites    []SiteRecord   `json:"sites"`
	Runtimes RuntimeCatalog `json:"runtimes"`
	Shell    ShellStatus    `json:"shell"`
}
