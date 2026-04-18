import type { SiteFormErrors } from '~/components/site-form/site-form.types';

export interface RuntimePageFeedback {
  message: string;
}

export function toErrorMessage(error: unknown): string {
  if (error instanceof Error && error.message.trim() !== '') {
    return error.message;
  }

  if (typeof error === 'string' && error.trim() !== '') {
    return error;
  }

  return 'The Lara Nux desktop bridge could not complete the request.';
}

export function mapSiteFormError(error: unknown): SiteFormErrors {
  const message = toErrorMessage(error);
  const fieldErrors: SiteFormErrors = {
    general: message,
  };

  if (message.includes('invalid laravel path')) {
    fieldErrors.rootPath = 'Pick a Laravel root containing artisan, composer.json, and public/index.php.';
  }

  if (message.includes('duplicate site domain') || message.includes('domain')) {
    fieldErrors.domain = 'That .test domain is already registered. Use a different hostname.';
  }

  if (message.includes('unsupported php runtime') || message.includes('php runtime not found')) {
    fieldErrors.phpVersion = 'Choose a registered PHP runtime before saving this site.';
  }

  return fieldErrors;
}
