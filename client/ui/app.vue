<script setup lang="ts">
const links = [
  { to: '/sites', label: 'Sites' },
  { to: '/runtimes', label: 'Runtimes & health' },
];
</script>

<template>
  <div class="shell-layout">
    <aside class="shell-sidebar">
      <div>
        <p class="eyebrow">Desktop client</p>
        <h1>Lara Nux</h1>
        <p class="sidebar-copy">
          The UI stays unprivileged. Every activation, runtime change, and service action crosses the daemon RPC boundary.
        </p>
      </div>

      <nav class="shell-nav" aria-label="Primary">
        <NuxtLink
          v-for="link in links"
          :key="link.to"
          :to="link.to"
          class="shell-nav__link"
          active-class="shell-nav__link--active"
        >
          {{ link.label }}
        </NuxtLink>
      </nav>
    </aside>

    <main class="shell-content">
      <NuxtPage />
    </main>
  </div>
</template>
