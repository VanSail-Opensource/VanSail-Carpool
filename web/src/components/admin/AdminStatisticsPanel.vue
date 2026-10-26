<script setup lang="ts">
import LineChart from '@/components/charts/LineChart.vue'
import { computed, onMounted, reactive, shallowRef } from 'vue'

import type { AdminStatistics, StatisticsQuery } from '@/api/features'
import InlineNotice from '@/components/common/InlineNotice.vue'
import { useI18n } from '@/i18n'
import { txbInputFromMinor } from '@/utils/format'

const props = defineProps<{ title: string; load: (query: StatisticsQuery) => Promise<AdminStatistics> }>()
const emit = defineEmits<{ close: [] }>()
const statistics = shallowRef<AdminStatistics | null>(null)
const loading = shallowRef(true)
const error = shallowRef<string | null>(null)
const today = new Date().toISOString().slice(0, 10)
const monthAgo = new Date(Date.now() - 29 * 86_400_000).toISOString().slice(0, 10)
const filters = reactive<Required<Pick<StatisticsQuery, 'from' | 'to' | 'bucket'>>>({ from: monthAgo, to: today, bucket: 'daily' })
const { t } = useI18n()
const bucketItems = computed(() => [
  { value: 'daily', label: t('adminStatistics.daily') },
  { value: 'weekly', label: t('adminStatistics.weekly') },
])
const tableColumns = computed(() => [
  { accessorKey: 'period', header: t('adminStatistics.period') },
  { accessorKey: 'count', header: t('adminStatistics.count') },
  { accessorKey: 'unique', header: t('adminStatistics.unique') },
  { accessorKey: 'input', header: t('adminStatistics.inputTxb') },
  { accessorKey: 'output', header: t('adminStatistics.outputTxb') },
  { accessorKey: 'net', header: t('adminStatistics.netTxb') },
])
const tableData = computed(() => (statistics.value?.series ?? []).map((point) => ({
  period: point.periodStart,
  count: point.count,
  unique: point.uniqueUsers,
  input: txbInputFromMinor(point.inputTxbMinor),
  output: txbInputFromMinor(point.outputTxbMinor),
  net: txbInputFromMinor(point.netTxbMinor),
})))

async function refresh(): Promise<void> {
  loading.value = true
  error.value = null
  try {
    statistics.value = await props.load({ ...filters })
  } catch {
    error.value = t('adminStatistics.error')
  } finally {
    loading.value = false
  }
}

onMounted(() => void refresh())
</script>

<template>
  <section class="statistics-panel">
    <header class="statistics-panel__header">
      <div><h3>{{ title }}</h3><p>{{ t('adminStatistics.serverCalculated') }}</p></div>
      <UButton color="neutral" variant="ghost" square icon="i-ph-x" :aria-label="t('adminStatistics.close')" @click="emit('close')" />
    </header>
    <form class="statistics-filters" @submit.prevent="refresh">
      <UFormField name="statistics-from" :label="t('adminStatistics.from')" required><UInput v-model="filters.from" class="w-full" type="date" /></UFormField>
      <UFormField name="statistics-to" :label="t('adminStatistics.to')" required><UInput v-model="filters.to" class="w-full" type="date" /></UFormField>
      <UFormField name="statistics-bucket" :label="t('adminStatistics.grouping')"><USelect v-model="filters.bucket" class="w-full" :items="bucketItems" /></UFormField>
      <UButton type="submit" color="neutral" variant="outline" icon="i-ph-arrow-clockwise" :loading="loading" :disabled="loading" :label="t('adminStatistics.refresh')" />
    </form>
    <InlineNotice v-if="error" tone="warning">{{ error }}</InlineNotice>
    <div v-if="statistics" class="statistics-content" :aria-busy="loading">
      <dl class="statistics-summary">
        <div><dt>{{ t('adminStatistics.participation') }}</dt><dd>{{ statistics.count }}</dd></div><div><dt>{{ t('adminStatistics.uniqueUsers') }}</dt><dd>{{ statistics.uniqueUsers }}</dd></div>
        <div><dt>{{ t('adminStatistics.input') }}</dt><dd>{{ t('adminStatistics.txbValue', { value: txbInputFromMinor(statistics.inputTxbMinor) }) }}</dd></div><div><dt>{{ t('adminStatistics.output') }}</dt><dd>{{ t('adminStatistics.txbValue', { value: txbInputFromMinor(statistics.outputTxbMinor) }) }}</dd></div>
        <div><dt>{{ t('adminStatistics.net') }}</dt><dd>{{ t('adminStatistics.txbValue', { value: txbInputFromMinor(statistics.netTxbMinor) }) }}</dd></div><div><dt>{{ t('adminStatistics.discountAddons') }}</dt><dd>{{ t('adminStatistics.splitTxbValue', { first: txbInputFromMinor(statistics.discountTxbMinor), second: txbInputFromMinor(statistics.addonTxbMinor) }) }}</dd></div>
      </dl>
      <LineChart :data="statistics.series.map((point, index) => ({ x: index, label: point.periodStart, count: point.count }))" :config="{ count: { label: t('adminStatistics.count'), color: 'var(--accent)' } }" :label="t('adminStatistics.chartLabel', { bucket: t(`adminStatistics.${statistics.bucket}`), from: statistics.from, to: statistics.to })" bar />
      <div class="statistics-table"><p>{{ t('adminStatistics.caption', { grouping: t(`adminStatistics.${statistics.bucket}`) }) }}</p><UTable class="admin-table" :data="tableData" :columns="tableColumns" /></div>
      <UTable v-if="statistics.distribution.length" class="admin-table" :data="statistics.distribution" :columns="[{ accessorKey: 'label', header: t('adminWorkspace.name') }, { accessorKey: 'count', header: t('adminStatistics.count') }]" />
    </div>
    <USkeleton v-else-if="loading" class="h-44 w-full" />
  </section>
</template>

<style scoped>
.statistics-panel { min-width: 0; display: grid; gap: 0.8rem; margin: 0; padding: 0; }
.statistics-content, .statistics-filters, .statistics-filters > * { min-width: 0; }
.statistics-panel__header { display: flex; align-items: flex-start; justify-content: space-between; gap: 0.8rem; }
.statistics-panel__header > div { min-width: 0; }
.statistics-panel__header h3, .statistics-panel__header p { margin: 0; }
.statistics-panel__header p { margin-top: 0.25rem; color: var(--text-muted); font-size: 0.72rem; }
.statistics-panel__header h3 { overflow-wrap: anywhere; }
.statistics-filters { display: grid; grid-template-columns: minmax(0, 1fr); gap: 0.6rem; align-items: end; }
.statistics-filters > .button { width: 100%; }
.statistics-summary { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 0.45rem; margin: 0; }
.statistics-summary div { padding: 0.6rem; border-bottom: 1px solid var(--line); }
.statistics-summary dt { color: var(--text-faint); font-size: 0.62rem; }
.statistics-summary dd { margin: 0.2rem 0 0; font-size: 0.8rem; font-weight: 750; font-variant-numeric: tabular-nums; overflow-wrap: anywhere; }
.statistics-chart { height: 150px; display: flex; min-width: 0; align-items: end; gap: 3px; overflow: hidden; padding: 0.6rem; border-bottom: 1px solid var(--line); }
.statistics-chart__bar { min-width: 4px; flex: 1; border-radius: 3px 3px 0 0; background: var(--accent); opacity: 0.78; }
.statistics-table { min-width: 0; overflow-x: auto; }
.statistics-table :deep(table) { min-width: 600px; }
.statistics-table :deep(th), .statistics-table :deep(td) { white-space: nowrap; }
.statistics-table p { margin: 0; padding: 0.4rem; color: var(--text-faint); font-size: 0.68rem; }
.statistics-distribution { display: grid; gap: 0.35rem; margin: 0; padding: 0; list-style: none; }
.statistics-distribution li { display: flex; justify-content: space-between; padding: 0.45rem 0.6rem; background: var(--surface); }
@media (min-width: 480px) and (max-width: 759px) { .statistics-filters { grid-template-columns: repeat(2, minmax(0, 1fr)); }.statistics-filters > .button { grid-column: 1 / -1; } }
@media (min-width: 760px) { .statistics-filters { grid-template-columns: repeat(3, minmax(150px, 1fr)) auto; }.statistics-filters > .button { width: auto; }.statistics-summary { grid-template-columns: repeat(3, minmax(0, 1fr)); } }
@media (max-width: 360px) { .statistics-summary { grid-template-columns: minmax(0, 1fr); } }
</style>
