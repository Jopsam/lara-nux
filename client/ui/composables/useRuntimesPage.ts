import { ref } from 'vue';
import type { HealthReport, RuntimeCatalog, SiteRecord } from '@contracts';
import { useDesktopClient } from '~/composables/useDesktopClient';
import { toErrorMessage } from '~/utils/rpc-errors';

export function useRuntimesPage() {
  const client = useDesktopClient();
  const loading = ref(false);
  const saving = ref(false);
  const feedback = ref('');
  const errorMessage = ref('');
  const health = ref<HealthReport | null>(null);
  const catalog = ref<RuntimeCatalog | null>(null);
  const sites = ref<SiteRecord[]>([]);

  async function refresh() {
    loading.value = true;
    errorMessage.value = '';

    try {
      const snapshot = await client.LoadDashboard();
      health.value = snapshot.health;
      catalog.value = snapshot.runtimes;
      sites.value = snapshot.sites;
    } catch (error) {
      errorMessage.value = toErrorMessage(error);
    } finally {
      loading.value = false;
    }
  }

  async function setDefaultRuntime(version: string) {
    await execute(async () => {
      await client.SetDefaultRuntime({ version });
      feedback.value = `Default PHP runtime changed to ${version}.`;
    });
  }

  async function switchSiteRuntime(siteId: string, phpVersion: string) {
    await execute(async () => {
      await client.SwitchSiteRuntime({ siteId, phpVersion });
      feedback.value = `Site runtime switched to PHP ${phpVersion}.`;
    });
  }

  async function serviceAction(service: string, action: 'start' | 'restart' | 'status') {
    await execute(async () => {
      await client.ServiceAction({ service, action });
      feedback.value = `${service} → ${action} completed through the daemon.`;
    });
  }

  async function execute(work: () => Promise<void>) {
    saving.value = true;
    feedback.value = '';
    errorMessage.value = '';

    try {
      await work();
      await refresh();
    } catch (error) {
      errorMessage.value = toErrorMessage(error);
    } finally {
      saving.value = false;
    }
  }

  return {
    loading,
    saving,
    feedback,
    errorMessage,
    health,
    catalog,
    sites,
    refresh,
    setDefaultRuntime,
    switchSiteRuntime,
    serviceAction,
  };
}
