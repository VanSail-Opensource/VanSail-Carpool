<script setup lang="ts">
import HourlyPage from '@/components/hourly/HourlyPage.vue'
import HourlyAdminPage from '@/components/admin/hourly/HourlyAdminPage.vue'
import CatalogExperience from '@/components/catalog/CatalogExperience.vue'
defineProps<{ admin: boolean; catalog: boolean }>()
</script>
<template><UApp><main class="hourly-audit" :class="{ 'hourly-audit--admin': admin }"><HourlyAdminPage v-if="admin" /><CatalogExperience v-else-if="catalog" /><HourlyPage v-else /></main></UApp></template>
<style scoped>
.hourly-audit { max-width: 58rem; margin: auto; padding: 1rem; min-width: 0; }
.hourly-audit--admin { max-width: 80rem; padding: clamp(1rem, 3vw, 2.5rem); }
</style>
