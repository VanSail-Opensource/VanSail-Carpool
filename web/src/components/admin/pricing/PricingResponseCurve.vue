<script setup lang="ts">
import LineChart from '@/components/charts/LineChart.vue'
import { computed } from 'vue'
import { useI18n } from '@/i18n'
import type { PricingRecord } from '@/api/pricingAnalysis'
const props = defineProps<{ record?: PricingRecord }>()
const { t } = useI18n()
interface Candidate { priceMinor: string; probability: number; contributionMinor?: number; contribution?: number; eligible: boolean }
const candidates = computed<Candidate[]>(() => {
  try { const value = JSON.parse(String(props.record?.trace_json ?? '{}')) as { candidates?: Candidate[] }; return (value.candidates ?? []).filter(c => Number.isFinite(Number(c.priceMinor)) && Number.isFinite(c.probability)).sort((a, b) => Number(a.priceMinor) - Number(b.priceMinor)) } catch { return [] }
})
const points = computed(() => candidates.value.map(c => ({ x: Number(c.priceMinor), label: `${(Number(c.priceMinor) / 100).toFixed(2)} TXB`, probability: c.probability * 100 })))
const columns = computed(() => [{ accessorKey: 'priceMinor', header: t('pricing.candidatePrice') }, { accessorKey: 'probability', header: t('pricing.probability') }, { id: 'contribution', header: t('pricing.expectedContribution') }, { accessorKey: 'eligible', header: t('pricing.eligible') }])
</script>
<template>
  <section class="pricing-response">
    <h3>{{ t('pricing.responseCurve') }}</h3><p>{{ t('pricing.responseEvidence') }}</p>
    <LineChart v-if="candidates.length" :data="points" :config="{ probability: { label: t('pricing.probability'), color: 'var(--accent)' } }" :label="t('pricing.responseCurve')" :format-value="(v: number) => `${v.toFixed(1)}%`" :format-x="(v: number) => `${(v / 100).toFixed(2)}`" />
    <UTable v-if="candidates.length" :data="candidates" :columns="columns" class="admin-table"><template #priceMinor-cell="{ row }">{{ (Number(row.original.priceMinor) / 100).toFixed(2) }}</template><template #probability-cell="{ row }">{{ (row.original.probability * 100).toFixed(1) }}%</template><template #contribution-cell="{ row }">{{ ((row.original.contributionMinor ?? row.original.contribution ?? 0) / 100).toFixed(2) }}</template><template #eligible-cell="{ row }">{{ t(row.original.eligible ? 'pricing.eligibleYes' : 'pricing.eligibleNo') }}</template></UTable>
    <p v-else>{{ t('pricing.emptyRecords') }}</p>
  </section>
</template>
<style scoped>
.pricing-response { min-width: 0; }.pricing-response h3 { font-size: 1rem; }.pricing-response p, .pricing-response text { font-size: .8rem; color: var(--text-muted); fill: var(--text-muted); }.pricing-response svg { display: block; width: 100%; max-height: 15rem; }
</style>
