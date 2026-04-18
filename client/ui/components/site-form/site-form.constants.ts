import type { SiteRecord } from '@contracts';
import type { SiteFormModel } from './site-form.types';

export const SITE_FORM_MODE = {
  CREATE: 'create',
  EDIT: 'edit',
} as const;

export function createSiteFormModel(): SiteFormModel {
  return {
    rootPath: '',
    domain: '',
    phpVersion: '',
  };
}

export function siteRecordToFormModel(record: SiteRecord): SiteFormModel {
  return {
    rootPath: record.rootPath,
    domain: record.domain,
    phpVersion: record.phpVersion,
  };
}
