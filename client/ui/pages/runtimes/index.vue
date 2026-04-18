<script setup lang="ts">
import { onMounted } from 'vue';
import HealthPanel from '~/components/health-panel/HealthPanel.vue';
import { useRuntimesPage } from '~/composables/useRuntimesPage';

const page = useRuntimesPage();

function onDefaultRuntimeChange(event: Event) {
  const target = event.target as HTMLSelectElement | null;
  if (target === null || target.value === '') {
    return;
  }

  void page.setDefaultRuntime(target.value);
}

function onSiteRuntimeChange(siteId: string, event: Event) {
  const target = event.target as HTMLSelectElement | null;
  if (target === null || target.value === '') {
    return;
  }

  void page.switchSiteRuntime(siteId, target.value);
}

onMounted(async () => {
  await page.refresh();
});
</script>

<template>
  <div class="stack">
    <header class="page-header stack">
      <div class="row-between">
        <div>
          <p class="eyebrow">Phase 4 · Runtimes & health</p>
          <h2>Switch runtimes and remediate health issues</h2>
          <p class="page-copy">
            Default runtime changes, per-site switches, service actions, and health remediation all go through the RPC contracts added before the UI phase.
          </p>
        </div>
        <button class="button-secondary" type="button" @click="page.refresh">
          {{ page.loading ? 'Refreshing…' : 'Refresh' }}
        </button>
      </div>

      <p v-if="page.feedback" class="message message--success">{{ page.feedback }}</p>
      <p v-if="page.errorMessage" class="message message--error">{{ page.errorMessage }}</p>
    </header>

    <div class="panel-grid panel-grid--two">
      <section class="panel stack">
        <h2 class="section-title">Default runtime</h2>
        <p class="page-copy">Pick the runtime Lara Nux should use for new sites when the form leaves PHP blank.</p>

        <div v-if="page.catalog" class="field">
          <label for="default-runtime">Active default</label>
          <select
            id="default-runtime"
            :disabled="page.saving"
            :value="page.catalog.defaultRuntime?.version ?? ''"
            @change="onDefaultRuntimeChange"
          >
            <option disabled value="">Choose a runtime</option>
            <option v-for="runtime in page.catalog.registered" :key="runtime.version" :value="runtime.version">
              PHP {{ runtime.version }} · {{ runtime.binaryPath }}
            </option>
          </select>
        </div>

        <article class="card stack" v-if="page.catalog">
          <h3 class="card__title">Supported package inventory</h3>
          <div class="table">
            <div v-for="pkg in page.catalog.supportedPackages" :key="pkg.key" class="table-row">
              <div class="table-row__meta">
                <strong>{{ pkg.key }}</strong>
                <span class="muted">{{ pkg.description }}</span>
              </div>
              <span class="chip">{{ pkg.runtimeVersion || 'manual' }}</span>
            </div>
          </div>
        </article>
      </section>

      <section class="panel stack">
        <h2 class="section-title">Per-site runtime switching</h2>
        <p class="page-copy">Switch site runtimes without touching project source files.</p>

        <div class="table">
          <div v-for="site in page.sites" :key="site.id" class="table-row">
            <div class="table-row__meta">
              <strong>{{ site.domain }}</strong>
              <span class="muted">Current PHP {{ site.phpVersion }}</span>
            </div>

            <select
              :disabled="page.saving || !page.catalog"
              :value="site.phpVersion"
              @change="onSiteRuntimeChange(site.id, $event)"
            >
              <option
                v-for="runtime in page.catalog?.registered || []"
                :key="runtime.version"
                :value="runtime.version"
              >
                PHP {{ runtime.version }}
              </option>
            </select>
          </div>
        </div>

        <article class="card stack" v-if="page.health">
          <h3 class="card__title">Service actions</h3>
          <div class="table">
            <div v-for="service in page.health.services" :key="service.service" class="table-row">
              <div class="table-row__meta">
                <strong>{{ service.service }}</strong>
                <span class="muted">{{ service.summary }}</span>
              </div>

              <div class="inline-actions">
                <button class="button-secondary" type="button" @click="page.serviceAction(service.service, 'status')">Status</button>
                <button class="button-secondary" type="button" @click="page.serviceAction(service.service, 'start')">Start</button>
                <button class="button" type="button" @click="page.serviceAction(service.service, 'restart')">Restart</button>
              </div>
            </div>
          </div>
        </article>
      </section>
    </div>

    <HealthPanel :health="page.health" :loading="page.loading" :error-message="page.errorMessage" />
  </div>
</template>
