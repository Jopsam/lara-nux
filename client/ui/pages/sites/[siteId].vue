<script setup lang="ts">
import { onMounted, ref } from 'vue';
import type { PHPRuntimeRecord } from '@contracts';
import SiteForm from '~/components/site-form/SiteForm.vue';
import { createSiteFormModel, siteRecordToFormModel } from '~/components/site-form/site-form.constants';
import type { SiteFormErrors, SiteFormModel } from '~/components/site-form/site-form.types';
import { useDesktopClient } from '~/composables/useDesktopClient';
import { mapSiteFormError, toErrorMessage } from '~/utils/rpc-errors';

const route = useRoute();
const client = useDesktopClient();
const formModel = ref<SiteFormModel>(createSiteFormModel());
const formErrors = ref<SiteFormErrors>({});
const runtimeOptions = ref<PHPRuntimeRecord[]>([]);
const loading = ref(false);
const saving = ref(false);
const feedback = ref('');
const loadError = ref('');

async function refresh() {
  loading.value = true;
  loadError.value = '';

  try {
    const siteId = String(route.params.siteId ?? '');
    const [site, runtimeCatalog] = await Promise.all([
      client.GetSite(siteId),
      client.GetRuntimeCatalog(),
    ]);

    formModel.value = siteRecordToFormModel(site);
    runtimeOptions.value = runtimeCatalog.registered;
  } catch (error) {
    loadError.value = toErrorMessage(error);
  } finally {
    loading.value = false;
  }
}

async function save() {
  saving.value = true;
  feedback.value = '';
  formErrors.value = {};

  try {
    await client.UpdateSite({
      siteId: String(route.params.siteId ?? ''),
      rootPath: formModel.value.rootPath,
      domain: formModel.value.domain,
      phpVersion: formModel.value.phpVersion,
    });
    feedback.value = 'Site changes were saved through the Lara Nux daemon.';
    await refresh();
  } catch (error) {
    formErrors.value = mapSiteFormError(error);
  } finally {
    saving.value = false;
  }
}

onMounted(async () => {
  await refresh();
});
</script>

<template>
  <div class="stack">
    <header class="page-header stack">
      <NuxtLink class="button-secondary" to="/sites">← Back to sites</NuxtLink>
      <div>
        <p class="eyebrow">Phase 4 · Edit site</p>
        <h2>Edit site configuration</h2>
        <p class="page-copy">
          Update Laravel path, hostname, or runtime without bypassing the daemon’s validation and rollback rules.
        </p>
      </div>
      <p v-if="loadError" class="message message--error">{{ loadError }}</p>
      <p v-if="feedback" class="message message--success">{{ feedback }}</p>
    </header>

    <SiteForm
      v-model="formModel"
      :busy="loading || saving"
      :errors="formErrors"
      :runtime-options="runtimeOptions"
      title="Update site"
      description="Edit flows use /rpc/sites.get and /rpc/sites.update so the client never mutates registry files directly."
      submit-label="Save changes"
      @submit="save"
    />
  </div>
</template>
