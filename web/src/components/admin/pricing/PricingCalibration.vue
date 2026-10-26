<script setup lang="ts">
import { computed, shallowRef } from 'vue'
import { useI18n } from '@/i18n'
import type { PricingRecord } from '@/api/pricingAnalysis'
const props = defineProps<{ items: PricingRecord[] }>()
const { t } = useI18n()
const model = shallowRef('')
const models = computed(() => [...new Set(props.items.map(row => String(row.model_version)))])
const buckets = computed(() => {
  const groups = new Map<string, { kind: string; bucket: number; samples: number; predicted: number; observed: number; brier: number }>()
  for (const row of props.items) {
    if (String(row.model_version) !== (model.value || models.value[0])) continue
    const key = `${row.kind}:${row.bucket}`
    const group = groups.get(key) ?? { kind: String(row.kind), bucket: Number(row.bucket), samples: 0, predicted: 0, observed: 0, brier: 0 }
    group.samples += Number(row.samples); group.predicted += Number(row.probability_sum); group.observed += Number(row.observed_sum); group.brier += Number(row.brier_sum); groups.set(key, group)
  }
  return [...groups.values()].sort((a, b) => a.bucket - b.bucket)
})
const columns = computed(() => [{ accessorKey: 'kind', header: t('pricing.kind') }, { accessorKey: 'bucket', header: t('pricing.probabilityBucket') }, { accessorKey: 'samples', header: t('pricing.samples') }, { id: 'predicted', header: t('pricing.predictedProbability') }, { id: 'observed', header: t('pricing.observedFrequency') }, { id: 'brier', header: t('pricing.brier') }])
</script>
<template>
  <section class="pricing-calibration">
    <h3>{{ t('pricing.calibration') }}</h3><UFormField v-if="models.length" :label="t('pricing.modelVersion')"><USelect :model-value="model || models[0]" :items="models" class="w-full" @update:model-value="model = String($event)" /></UFormField>
    <UTable v-if="buckets.length" :data="buckets" :columns="columns" class="admin-table"><template #kind-cell="{ row }">{{ t(`pricing.${row.original.kind}`) }}</template><template #bucket-cell="{ row }">{{ row.original.bucket * 10 }}–{{ (row.original.bucket + 1) * 10 }}%</template><template #predicted-cell="{ row }">{{ (row.original.predicted / row.original.samples * 100).toFixed(1) }}%</template><template #observed-cell="{ row }">{{ (row.original.observed / row.original.samples * 100).toFixed(1) }}%</template><template #brier-cell="{ row }">{{ (row.original.brier / row.original.samples).toFixed(4) }}</template></UTable><p v-else>{{ t('pricing.noCalibration') }}</p>
  </section>
</template>
<style scoped>.pricing-calibration { display: grid; gap: 1rem; min-width: 0; }.pricing-calibration h3 { font-size: 1rem; margin: 0; }.pricing-calibration p { color: var(--text-muted); font-size: .8rem; }</style>
