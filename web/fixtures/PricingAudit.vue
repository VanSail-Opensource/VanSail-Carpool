<script setup lang="ts">
import PricingCheckoutAudit from './PricingCheckoutAudit.vue'
import ChartsAudit from './ChartsAudit.vue'
import AdminCatalogPanel from '@/components/admin/AdminCatalogPanel.vue'
import ProductPricingSettings from '@/components/admin/pricing/ProductPricingSettings.vue'
import PricingWorkspace from '@/components/admin/pricing/PricingWorkspace.vue'
import AdminUserPredictions from '@/components/admin/pricing/AdminUserPredictions.vue'
defineProps<{ view: string }>()
</script>
<template><UApp><main class="pricing-audit"><PricingCheckoutAudit v-if="view === 'checkout'" /><AdminUserPredictions v-else-if="view === 'predictions'" user-id="fixture-user" /><ChartsAudit v-else-if="view === 'charts'" /><AdminCatalogPanel v-else-if="view === 'catalog'" /><ProductPricingSettings v-else-if="view === 'policy'" kind="combo" product-id="audit-combo" /><PricingWorkspace v-else /></main></UApp></template>
<style scoped>.pricing-audit { max-width: 880px; min-width: 0; margin: auto; padding: 24px; }</style>
