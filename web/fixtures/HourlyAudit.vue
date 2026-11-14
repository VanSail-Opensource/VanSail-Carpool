<script setup lang="ts">
import HourlyPage from '@/components/hourly/HourlyPage.vue'
import HourlyAdminPage from '@/components/admin/hourly/HourlyAdminPage.vue'
import CatalogExperience from '@/components/catalog/CatalogExperience.vue'
import SettingsHourlyControls from '@/components/settings/SettingsHourlyControls.vue'
defineProps<{ admin: boolean; catalog: boolean; settings: boolean }>()

// Dev-only audit switch: this host cannot emulate prefers-reduced-motion, so
// the fixture overrides matchMedia before the hourly components subscribe.
// `motion-v`'s useReducedMotion then reports the reduced-motion branch.
const params = new globalThis.URLSearchParams(globalThis.location.search)
if (params.has('reduced-motion')) {
  const original = globalThis.matchMedia.bind(globalThis)
  globalThis.matchMedia = (query: string): ReturnType<typeof original> => {
    if (!query.includes('prefers-reduced-motion')) return original(query)
    return {
      matches: true,
      media: query,
      onchange: null,
      addEventListener: () => undefined,
      removeEventListener: () => undefined,
      addListener: () => undefined,
      removeListener: () => undefined,
      dispatchEvent: () => false,
    } as unknown as ReturnType<typeof original>
  }
}
</script>
<template><UApp><main class="hourly-audit" :class="{ 'hourly-audit--admin': admin }"><HourlyAdminPage v-if="admin" /><CatalogExperience v-else-if="catalog" /><SettingsHourlyControls v-else-if="settings" /><HourlyPage v-else /></main></UApp></template>
<style scoped>
.hourly-audit { max-width: 58rem; margin: auto; padding: 1rem; min-width: 0; }
.hourly-audit--admin { max-width: 80rem; padding: clamp(1rem, 3vw, 2.5rem); }
</style>
