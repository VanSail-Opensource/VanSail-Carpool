<script setup lang="ts">
import { computed, shallowRef, watch } from 'vue'
import type { PricingHealth } from '@/api/pricingAnalysis'
import MetricLineChart from '@/components/common/MetricLineChart.vue'
import { useMetricSamples } from '@/composables/useMetricSamples'
import { useI18n } from '@/i18n'
const props = defineProps<{ health: PricingHealth[] }>()
const { t } = useI18n()
const samples = useMetricSamples()
const selected = shallowRef('')
watch(() => props.health, nodes => { if (!nodes.some(node => node.nodeId === selected.value)) selected.value = nodes[0]?.nodeId ?? '' }, { immediate: true })
const selectedNode = computed(() => props.health.find(node => node.nodeId === selected.value))
watch(selectedNode, node => {
  const values: Record<string, number> = {}
  if (node && node.state !== 'TELEMETRY_STALE') { values[`${node.nodeId}:cpu`] = node.cpu; values[`${node.nodeId}:memory`] = node.memory }
  samples.append(Date.now(), values)
}, { immediate: true })
const columns = computed(() => [{ accessorKey: 'nodeId', header: t('pricing.node') }, { id: 'state', header: t('adminWorkspace.status') }, { id: 'cpu', header: t('pricing.cpu') }, { id: 'memory', header: t('pricing.memory') }])
</script>
<template>
  <section class="pricing-operations">
    <h3>{{ t('pricing.nodeHealth') }}</h3>
    <p>{{ t('pricing.healthHint') }}</p>
    <UTable :data="health" :columns="columns" class="admin-table">
      <template #state-cell="{ row }">{{ t(`pricing.states.${row.original.state}`) }}</template>
      <template #cpu-cell="{ row }">{{ row.original.state === 'TELEMETRY_STALE' ? t('common.notAvailable') : `${row.original.cpu.toFixed(1)}%` }}</template>
      <template #memory-cell="{ row }">{{ row.original.state === 'TELEMETRY_STALE' ? t('common.notAvailable') : `${row.original.memory.toFixed(1)}%` }}</template>
      <template #empty>{{ t('pricing.noNodes') }}</template>
    </UTable>
    <UFormField v-if="health.length" :label="t('pricing.node')"><USelect v-model="selected" :items="health.map(node => ({ value: node.nodeId, label: node.nodeId }))" class="w-full max-w-sm" /></UFormField>
    <section v-if="selectedNode" :key="selected"><div class="pricing-operations__charts"><MetricLineChart :samples="samples.series.value[`${selected}:cpu`] ?? []" :label="t('pricing.cpu')" unit="percent" /><MetricLineChart :samples="samples.series.value[`${selected}:memory`] ?? []" :label="t('pricing.memory')" unit="percent" /></div></section>
  </section>
</template>
<style scoped>
.pricing-operations { display: grid; gap: 1rem; min-width: 0; }.pricing-operations h3, .pricing-operations p { margin: 0; }.pricing-operations h3 { font-size: 1rem; }.pricing-operations p { color: var(--text-muted); font-size: .85rem; }.pricing-operations h3 { padding: .5rem 0; cursor: pointer; overflow-wrap: anywhere; }.pricing-operations__charts { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 1rem; }
@media (max-width: 600px) { .pricing-operations__charts { grid-template-columns: 1fr; } }
</style>
