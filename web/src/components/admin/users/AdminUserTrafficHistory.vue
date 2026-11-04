<script setup lang="ts">
import { computed } from 'vue'
import type { XNodeProfileMetrics } from '@/api/adminOperations'
import type { ChartDatum } from '@/components/charts'
import LineChart from '@/components/charts/LineChart.vue'
import { useI18n } from '@/i18n'
import { formatBytes, formatAccountingDay } from '@/utils/format'

const props = defineProps<{ usage: XNodeProfileMetrics['usage'] }>()
const { t, locale } = useI18n()
const config = computed(() => ({ upload: { label: t('xnode.upload'), color: 'var(--accent)' }, download: { label: t('xnode.download'), color: 'var(--success)' }, accounted: { label: t('adminNative.nativeAccounted'), color: 'var(--warning)', dashed: true } }))
const data = computed<ChartDatum[]>(() => props.usage.map(day => ({ x: Date.parse(day.date), label: formatAccountingDay(day.date), upload: Number(day.uploadBytes), download: Number(day.downloadBytes), accounted: Number(day.nativeWeightedBytes), uploadExact: day.uploadBytes, downloadExact: day.downloadBytes, accountedExact: day.nativeWeightedBytes })))
const columns = computed(() => [{ accessorKey: 'date', header: t('adminWorkspace.date') }, { id: 'upload', header: t('adminNative.nativeUpload') }, { id: 'download', header: t('adminNative.nativeDownload') }, { id: 'accounted', header: t('adminNative.nativeAccounted') }])
const exact = (row: ChartDatum, key: string) => `${formatBytes(String(row[`${key}Exact`] ?? '0'))} ? ${t('adminNative.exactBytes', { bytes: String(row[`${key}Exact`] ?? '0') })}`
const day = (at: number) => new Intl.DateTimeFormat(locale.value, { month: 'short', day: 'numeric', timeZone: 'UTC' }).format(at)
</script>

<template>
  <section class="grid min-w-0 gap-3">
    <h4 class="text-sm font-semibold">{{ t('adminNative.nativeSevenDay') }}</h4>
    <LineChart :data="data" :config="config" :label="t('adminNative.nativeSevenDay')" :height="190" :format-value="formatBytes" :format-x="day" :format-datum-value="exact" />
    <UTable :data="usage" :columns="columns" class="admin-table" :ui="{ base: 'min-w-[560px]' }" tabindex="0" role="region" :aria-label="t('adminNative.nativeSevenDay')">
      <template #date-cell="{ row }">{{ formatAccountingDay(row.original.date) }}</template>
      <template #upload-cell="{ row }"><span :title="t('adminNative.exactBytes', { bytes: row.original.uploadBytes })">{{ formatBytes(row.original.uploadBytes) }}</span></template>
      <template #download-cell="{ row }"><span :title="t('adminNative.exactBytes', { bytes: row.original.downloadBytes })">{{ formatBytes(row.original.downloadBytes) }}</span></template>
      <template #accounted-cell="{ row }"><span :title="t('adminNative.exactBytes', { bytes: row.original.nativeWeightedBytes })">{{ formatBytes(row.original.nativeWeightedBytes) }}</span></template>
    </UTable>
  </section>
</template>
