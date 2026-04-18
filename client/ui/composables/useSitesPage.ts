import { computed, ref } from 'vue';
import type { PHPRuntimeRecord } from '@contracts';
import { createSiteFormModel } from '~/components/site-form/site-form.constants';
import type { SiteFormErrors, SiteFormModel } from '~/components/site-form/site-form.types';
import { useDesktopClient } from '~/composables/useDesktopClient';
import { mapSiteFormError, toErrorMessage } from '~/utils/rpc-errors';

export function useSitesPage() {
  const client = useDesktopClient();
  const loading = ref(false);
  const saving = ref(false);
  const loadError = ref('');
  const feedback = ref('');
  const formErrors = ref<SiteFormErrors>({});
  const formModel = ref<SiteFormModel>(createSiteFormModel());
  const sites = ref([] as Awaited<ReturnType<typeof client.ListSites>>);
  const runtimeOptions = ref<PHPRuntimeRecord[]>([]);

  const hasSites = computed(() => sites.value.length > 0);

  async function refresh() {
    loading.value = true;
    loadError.value = '';

    try {
      const snapshot = await client.LoadDashboard();
      sites.value = snapshot.sites;
      runtimeOptions.value = snapshot.runtimes.registered;
    } catch (error) {
      loadError.value = toErrorMessage(error);
    } finally {
      loading.value = false;
    }
  }

  async function registerSite() {
    saving.value = true;
    feedback.value = '';
    formErrors.value = {};

    try {
      const payload = {
        rootPath: formModel.value.rootPath,
        domain: emptyToUndefined(formModel.value.domain),
        phpVersion: emptyToUndefined(formModel.value.phpVersion),
      };

      await client.RegisterSite(payload);
      feedback.value = 'Site activated through the daemon and added to the Lara Nux catalog.';
      formModel.value = createSiteFormModel();
      await refresh();
    } catch (error) {
      formErrors.value = mapSiteFormError(error);
    } finally {
      saving.value = false;
    }
  }

  return {
    loading,
    saving,
    loadError,
    feedback,
    formErrors,
    formModel,
    hasSites,
    runtimeOptions,
    sites,
    refresh,
    registerSite,
  };
}

function emptyToUndefined(value: string): string | undefined {
  return value.trim() === '' ? undefined : value.trim();
}
