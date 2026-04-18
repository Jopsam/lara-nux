<script setup lang="ts">
import { onMounted } from 'vue';
import SiteForm from '~/components/site-form/SiteForm.vue';
import { useSitesPage } from '~/composables/useSitesPage';

const page = useSitesPage();

onMounted(async () => {
  await page.refresh();
});
</script>

<template>
  <div class="stack">
    <header class="page-header stack">
      <div class="row-between">
        <div>
          <p class="eyebrow">Phase 4 · Sites</p>
          <h2>Register and edit Laravel sites</h2>
          <p class="page-copy">
            Add a Laravel path, validate it through the daemon, and surface duplicate-domain conflicts before operators guess what failed.
          </p>
        </div>
        <button class="button-secondary" type="button" @click="page.refresh">
          {{ page.loading ? 'Refreshing…' : 'Refresh' }}
        </button>
      </div>

      <p v-if="page.loadError" class="message message--error">{{ page.loadError }}</p>
      <p v-if="page.feedback" class="message message--success">{{ page.feedback }}</p>
    </header>

    <div class="page-grid page-grid--two">
      <SiteForm
        v-model="page.formModel"
        :busy="page.saving"
        :errors="page.formErrors"
        :runtime-options="page.runtimeOptions"
        title="Add a new site"
        description="The desktop client sends the request through the Unix socket so Lara Nux can validate the Laravel path and activate Caddy/PHP safely."
        submit-label="Activate site"
        @submit="page.registerSite"
      />

      <section class="panel stack">
        <header class="row-between">
          <div>
            <h2 class="section-title">Registered sites</h2>
            <p class="page-copy">List, readiness, and edit links all come from the daemon read-model routes.</p>
          </div>
          <span class="chip" :class="page.hasSites ? 'chip--success' : 'chip--warning'">
            {{ page.sites.length }} site(s)
          </span>
        </header>

        <div v-if="!page.hasSites" class="card">
          <p class="muted">No sites yet. Add the first Laravel project from the form on the left.</p>
        </div>

        <div v-else class="table">
          <article v-for="site in page.sites" :key="site.id" class="table-row">
            <div class="table-row__meta">
              <strong>{{ site.domain }}</strong>
              <span class="muted">{{ site.rootPath }}</span>
              <span class="muted">PHP {{ site.phpVersion }} · {{ site.statusMessage || site.status }}</span>
            </div>

            <div class="inline-actions">
              <span class="chip" :class="site.status === 'ready' ? 'chip--success' : site.status === 'conflict' ? 'chip--danger' : 'chip--warning'">
                {{ site.status }}
              </span>
              <NuxtLink class="button-secondary" :to="`/sites/${site.id}`">Edit</NuxtLink>
            </div>
          </article>
        </div>
      </section>
    </div>
  </div>
</template>
