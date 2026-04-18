export interface SiteFormModel {
  rootPath: string;
  domain: string;
  phpVersion: string;
}

export interface SiteFormErrors {
  general?: string;
  rootPath?: string;
  domain?: string;
  phpVersion?: string;
}

export interface SiteFormSubmitPayload {
  rootPath: string;
  domain?: string;
  phpVersion?: string;
}
