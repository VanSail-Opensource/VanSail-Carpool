<script setup lang="ts">
import { computed } from 'vue'
import { metricRows } from './metricRows'
import { useI18n } from '@/i18n'

const props = defineProps<{ value: unknown; prefix?: string }>()
const rows = computed(() => metricRows(props.value, props.prefix))
const { t } = useI18n()
const columns = computed(() => [{ accessorKey: 'label', header: t('adminNative.metric') }, { accessorKey: 'value', header: t('adminNative.value') }])
</script>

<template>
  <UTable class="metric-details admin-table" :class="rows.length > 100 ? 'metric-details--large h-[360px] max-h-[55dvh]' : ''" :data="rows" :columns="columns" :virtualize="rows.length > 100 ? { estimateSize: 36, overscan: 6 } : false"><template #value-cell="{ row }"><span :title="row.original.raw">{{ row.original.value }}</span></template></UTable>
</template>

<style scoped>
.metric-details { min-width: 0; font-size: 12px; }
.metric-details :deep(td) { overflow-wrap: anywhere; font-variant-numeric: tabular-nums; }
</style>
