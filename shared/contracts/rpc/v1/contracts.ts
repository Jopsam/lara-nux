export type RpcEnvelope<T> =
  | { ok: true; data: T }
  | { ok: false; error: RpcError };

export interface RpcError {
  code: string;
  message: string;
}

export type SiteStatus = 'ready' | 'degraded' | 'conflict';
export type ServiceState = 'active' | 'inactive' | 'failed' | 'unknown';
export type ServiceAction = 'start' | 'stop' | 'restart' | 'status';

export interface Conflict {
  resource: string;
  owner?: string;
  summary: string;
  remediation?: string;
}

export interface ResolverStatus {
  managed: boolean;
  stubPath: string;
  domain: string;
  address: string;
  owner: string;
  conflicts?: Conflict[];
  summary: string;
  remediation?: string;
}

export interface ServiceStatus {
  service: string;
  state: ServiceState;
  summary: string;
  updatedAt: string;
}

export interface SiteRecord {
  id: string;
  name: string;
  rootPath: string;
  domain: string;
  phpVersion: string;
  tls: 'auto';
  status: SiteStatus;
  statusMessage?: string;
  createdAt: string;
  updatedAt: string;
  lastCheckedAt?: string;
}

export interface PHPRuntimeRecord {
  version: string;
  binaryPath: string;
  fpmService: string;
  source?: string;
  registeredAt: string;
}

export interface SupportedPackage {
  key: string;
  description: string;
  runtimeVersion?: string;
  packages: string[];
}

export interface DetectedPHPRuntime {
  version: string;
  binaryPath?: string;
  fpmBinaryPath?: string;
  serviceName?: string;
  socketPath?: string;
}

export interface PHPMaterialization {
  version: string;
  serviceName: string;
  socketPath: string;
  poolConfigPath: string;
  overridePath: string;
  active: boolean;
}

export interface WebActivationResult {
  configPath: string;
  validated: boolean;
  reloaded: boolean;
  httpUrl: string;
  httpsUrl: string;
}

export interface SocketAvailability {
  path: string;
  available: boolean;
  summary: string;
}

export interface HealthCheck {
  name: string;
  capability: string;
  passed: boolean;
  summary: string;
  remediation?: string;
}

export interface SiteReadiness {
  site: SiteRecord;
  ready: boolean;
  summary: string;
  checks: HealthCheck[];
  phpService: ServiceStatus;
  checkedAt: string;
}

export interface HealthReport {
  ready: boolean;
  generatedAt: string;
  socket: SocketAvailability;
  resolver?: ResolverStatus;
  checks: HealthCheck[];
  services: ServiceStatus[];
  sites: SiteReadiness[];
}

export interface RegisterSiteRequest {
  rootPath: string;
  domain?: string;
  phpVersion?: string;
}

export interface GetSiteRequest {
  siteId: string;
}

export interface UpdateSiteRequest {
  siteId: string;
  rootPath?: string;
  domain?: string;
  phpVersion?: string;
}

export interface ActivationResult {
  site: SiteRecord;
  resolver: ResolverStatus;
  runtime: PHPRuntimeRecord;
  materialization: PHPMaterialization;
  web: WebActivationResult;
  services: ServiceStatus[];
  activatedAt: string;
}

export interface RegisterPHPRequest {
  version?: string;
  binaryPath?: string;
  fpmService?: string;
  source?: string;
  packageKey?: string;
}

export interface RuntimeRegistrationResult {
  runtime: PHPRuntimeRecord;
  materialization?: PHPMaterialization;
  installedFrom?: string;
  refreshed: boolean;
}

export interface SwitchPHPRequest {
  siteId: string;
  phpVersion: string;
}

export interface DefaultRuntimeResponse {
  runtime: PHPRuntimeRecord | null;
}

export interface SetDefaultPHPRequest {
  version: string;
}

export interface RuntimeCatalog {
  registered: PHPRuntimeRecord[];
  defaultRuntime?: PHPRuntimeRecord;
  supportedPackages: SupportedPackage[];
  detectedRuntimes: DetectedPHPRuntime[];
}

export interface ServiceActionRequest {
  service: string;
  action: ServiceAction;
}
