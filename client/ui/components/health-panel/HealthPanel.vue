<script setup lang="ts">
import type { HealthCheck, HealthReport } from '@contracts';

interface HealthPanelProps {
  health: HealthReport | null;
  loading?: boolean;
  errorMessage?: string;
}

const props = withDefaults(defineProps<HealthPanelProps>(), {
  loading: false,
  errorMessage: '',
});

function chipClass(check: HealthCheck): string {
  return check.passed ? 'chip chip--success' : 'chip chip--danger';
}
</script>

<template>
  <section class="panel stack">
    <header class="row-between">
      <div>
        <h2 class="section-title">Environment health</h2>
        <p class="page-copy">
          Resolver, ports, runtime inventory, per-site readiness, and service remediation all come from <code>/rpc/health</code>.
        </p>
      </div>
      <span class="chip" :class="health?.ready ? 'chip--success' : 'chip--warning'">
        {{ health?.ready ? 'Ready' : 'Needs attention' }}
      </span>
    </header>

    <p v-if="props.loading" class="muted">Refreshing daemon health…</p>
    <p v-if="props.errorMessage" class="message message--error">{{ props.errorMessage }}</p>

    <template v-if="props.health">
      <div class="summary-list">
        <span class="muted">Socket: {{ props.health.socket.summary }}</span>
        <span class="muted" v-if="props.health.resolver">Resolver: {{ props.health.resolver.summary }}</span>
      </div>

      <div class="status-grid">
        <article v-for="check in props.health.checks" :key="check.name" class="status-card stack">
          <div class="row-between">
            <h3 class="status-card__title">{{ check.name }}</h3>
            <span :class="chipClass(check)">
              {{ check.passed ? 'OK' : 'Failing' }}
            </span>
          </div>

          <p class="muted">{{ check.summary }}</p>
          <p v-if="check.remediation" class="message message--error">{{ check.remediation }}</p>
        </article>
      </div>

      <div class="panel-grid panel-grid--two">
        <article class="card stack">
          <h3 class="card__title">Managed services</h3>
          <div class="table">
            <div v-for="service in props.health.services" :key="service.service" class="table-row">
              <div class="table-row__meta">
                <strong>{{ service.service }}</strong>
                <span class="muted">{{ service.summary }}</span>
              </div>
              <span class="chip" :class="service.state === 'active' ? 'chip--success' : 'chip--warning'">
                {{ service.state }}
              </span>
            </div>
          </div>
        </article>

        <article class="card stack">
          <h3 class="card__title">Site readiness</h3>
          <div class="table">
            <div v-for="site in props.health.sites" :key="site.site.id" class="table-row">
              <div class="table-row__meta">
                <strong>{{ site.site.domain }}</strong>
                <span class="muted">{{ site.summary }}</span>
              </div>
              <span class="chip" :class="site.ready ? 'chip--success' : 'chip--warning'">
                {{ site.ready ? 'Ready' : site.site.status }}
              </span>
            </div>
          </div>
        </article>
      </div>
    </template>
  </section>
</template>
