import type {
  ActivationResult,
  DefaultRuntimeResponse,
  HealthReport,
  PHPRuntimeRecord,
  RuntimeCatalog,
  ServiceActionRequest,
  ServiceStatus,
  SiteRecord,
  RegisterSiteRequest,
  SetDefaultPHPRequest,
  SwitchPHPRequest,
  UpdateSiteRequest,
} from '@contracts';

export interface ShellState {
  socketPath: string;
  connected: boolean;
  lastSyncedAt?: string;
  lastError?: string;
}

export interface DashboardSnapshot {
  health: HealthReport;
  sites: SiteRecord[];
  runtimes: RuntimeCatalog;
  shell: ShellState;
}

export interface DesktopClientApi {
  GetShellState: () => Promise<ShellState>;
  LoadDashboard: () => Promise<DashboardSnapshot>;
  ListSites: () => Promise<SiteRecord[]>;
  GetSite: (siteId: string) => Promise<SiteRecord>;
  RegisterSite: (request: RegisterSiteRequest) => Promise<ActivationResult>;
  UpdateSite: (request: UpdateSiteRequest) => Promise<SiteRecord>;
  GetHealth: () => Promise<HealthReport>;
  GetRuntimeCatalog: () => Promise<RuntimeCatalog>;
  SetDefaultRuntime: (request: SetDefaultPHPRequest) => Promise<DefaultRuntimeResponse>;
  SwitchSiteRuntime: (request: SwitchPHPRequest) => Promise<SiteRecord>;
  ServiceAction: (request: ServiceActionRequest) => Promise<ServiceStatus>;
  ShowWindow?: () => Promise<void>;
  HideWindow?: () => Promise<void>;
  QuitApplication?: () => Promise<void>;
}

export interface DesktopWindow {
  go?: {
    main?: {
      App?: DesktopClientApi;
    };
  };
}

export interface RuntimeSelectOption {
  label: string;
  value: string;
  record: PHPRuntimeRecord;
}
