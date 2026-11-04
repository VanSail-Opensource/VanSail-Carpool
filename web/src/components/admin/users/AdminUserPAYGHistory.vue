<script setup lang="ts">
import { computed, shallowRef, watch } from 'vue'
import type { XNodeProfileMetrics } from '@/api/adminOperations'
import type { ChartDatum } from '@/components/charts'
import LineChart from '@/components/charts/LineChart.vue'
import { useI18n } from '@/i18n'
import { formatBytes } from '@/utils/format'
import { paygHistorySeries } from './userTrafficHistory'

const props = defineProps<{ history: XNodeProfileMetrics['paygHistory']; nodes: XNodeProfileMetrics['nodes']; stale?: boolean }>()
const { t, locale } = useI18n()
const nodeId = shallowRef('*')
const options = computed(() => [{ value: '*', label: t('adminNative.allNodes') }, ...props.history.nodes.map(node => ({ value: node.nodeId, label: node.name }))])
const rows = computed(() => props.history.nodes.filter(node => nodeId.value === '*' || node.nodeId === nodeId.value))
const data = computed(() => paygHistorySeries(props.history, nodeId.value === '*' ? '' : nodeId.value))
const config = computed(() => ({ upload: { label: t('xnode.upload'), color: 'var(--accent)' }, download: { label: t('xnode.download'), color: 'var(--success)' } }))
const columns = computed(() => [{ accessorKey: 'name', header: t('adminWorkspace.name') }, { id: 'status', header: t('firewall.status') }, { id: 'upload', header: t('xnode.upload') }, { id: 'download', header: t('xnode.download') }, { id: 'total', header: t('adminNative.recordedTotal') }])
const available = computed(() => new Set(props.stale ? [] : props.nodes.filter(node => node.available).map(node => node.nodeId)))
const exact = (row: ChartDatum, key: string) => `${formatBytes(String(row[`${key}Exact`] ?? '0'))} · ${t('adminNative.exactBytes', { bytes: String(row[`${key}Exact`] ?? '0') })}`
const hour = (at: number) => new Intl.DateTimeFormat(locale.value, { month: 'short', day: 'numeric', hour: '2-digit', timeZone: 'UTC' }).format(at)
const total = (upload: string, download: string) => (BigInt(upload) + BigInt(download)).toString()
watch(() => props.history.nodes, nodes => { if (nodeId.value !== '*' && !nodes.some(node => node.nodeId === nodeId.value)) nodeId.value = '*' })
</script>

<template>
  <section class="grid min-w-0 gap-3">
    <div class="flex flex-wrap items-end justify-between gap-3"><h4 class="text-sm font-semibold">{{ t('adminNative.paygHistory') }}</h4><UFormField :label="t('adminNative.nodeFilter')"><USelect v-model="nodeId" :items="options" class="min-w-44 max-w-full" /></UFormField></div>
    <p class="text-xs text-muted">{{ t('adminNative.paygHistoryHint') }}</p>
    <UAlert v-if="history.truncated" color="warning" :title="t('adminNative.historyTruncated')" />
    <p v-if="!history.nodes.length" class="py-4 text-sm text-muted">{{ t('adminNative.noPAYGHistory') }}</p>
    <template v-else>
      <LineChart :data="data" :config="config" :label="t('adminNative.paygHistory')" :height="210" :format-value="formatBytes" :format-x="hour" :format-datum-value="exact" bar />
      <UTable :data="rows" :columns="columns" class="admin-table" :ui="{ base: 'min-w-[600px]' }" tabindex="0" role="region" :aria-label="t('adminNative.paygHistory')">
        <template #name-cell="{ row }"><span :title="row.original.nodeId">{{ row.original.name }}</span></template>
        <template #status-cell="{ row }">{{ t(available.has(row.original.nodeId) ? 'adminNative.available' : 'adminNative.unavailable') }}</template>
        <template #upload-cell="{ row }"><span :title="t('adminNative.exactBytes', { bytes: row.original.uploadBytes })">{{ formatBytes(row.original.uploadBytes) }}</span></template>
        <template #download-cell="{ row }"><span :title="t('adminNative.exactBytes', { bytes: row.original.downloadBytes })">{{ formatBytes(row.original.downloadBytes) }}</span></template>
        <template #total-cell="{ row }"><span :title="t('adminNative.exactBytes', { bytes: total(row.original.uploadBytes, row.original.downloadBytes) })">{{ formatBytes(total(row.original.uploadBytes, row.original.downloadBytes)) }}</span></template>
      </UTable>
    </template>
  </section>
</template>
