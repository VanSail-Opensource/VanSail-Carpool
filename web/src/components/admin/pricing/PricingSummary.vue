<script setup lang="ts">
import type { PricingRecord } from '@/api/pricingAnalysis'
import { useI18n } from '@/i18n'
import { pricingMoney } from '@/utils/pricingDisplay'
defineProps<{ value: PricingRecord }>()
const { t } = useI18n()
const fields = ['netRevenueMinor', 'recordedCostMinor', 'recordedMarginMinor', 'forecastRevenueMinor', 'forecastMarginMinor', 'unallocatedDailyCostMinor']
</script>
<template>
  <section class="pricing-summary">
    <dl><div v-for="field in fields" :key="field"><dt>{{ t(`pricing.summaryLabels.${field}`) }}</dt><dd>{{ pricingMoney(value[field]) }}</dd></div></dl>
    <p>{{ t('pricing.financeScope') }}</p>
    <p>{{ t('pricing.workerSummary', { pending: String(value.pendingJobs ?? 0), failed: String(value.failedJobs ?? 0), changes: String(value.interventions30 ?? 0), dynamic: String(value.dynamicProducts ?? 0) }) }}</p>
  </section>
</template>
<style scoped>
.pricing-summary dl { display: grid; grid-template-columns: repeat(3, minmax(0, 1fr)); gap: 1.5rem; margin: 0; }.pricing-summary dt, .pricing-summary p { color: var(--text-muted); font-size: .8rem; }.pricing-summary dd { margin: .4rem 0 0; font-variant-numeric: tabular-nums; }.pricing-summary p { line-height: 1.6; }
@media (max-width: 600px) { .pricing-summary dl { grid-template-columns: repeat(2, minmax(0, 1fr)); } }
@media (max-width: 360px) { .pricing-summary dl { grid-template-columns: 1fr; } }
</style>
