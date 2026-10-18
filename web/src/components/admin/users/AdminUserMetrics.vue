<script setup lang="ts">
import { computed } from 'vue'
import InlineNotice from '@/components/common/InlineNotice.vue'
import MetricLineChart from '@/components/common/MetricLineChart.vue'
import { useI18n } from '@/i18n'
import { formatBytes } from '@/utils/format'
import { useAdminUserMetrics } from './useAdminUserMetrics'
import AdminUserScan from './AdminUserScan.vue'
const props = defineProps<{ userId: string }>()
const { t } = useI18n()
const { metrics, loading, error, load, samples } = useAdminUserMetrics(() => props.userId)
const rate = (value: number) => t('statistics.perSecond', { value: formatBytes(value) })
const columns = computed(() => [{ accessorKey: 'name', header: t('adminWorkspace.name') }, { id: 'use', header: t('adminNative.currentUse') }, { id: 'connections', header: t('xnode.connections') }, { id: 'upload', header: t('xnode.upload') }, { id: 'download', header: t('xnode.download') }, { id: 'total', header: t('adminNative.sevenDayUsage') }])
const dailyColumns = computed(() => [{ accessorKey: 'date', header: t('adminWorkspace.date') }, { id: 'upload', header: t('adminNative.nativeUpload') }, { id: 'download', header: t('adminNative.nativeDownload') }, { id: 'legacy', header: t('adminNative.legacyTraffic') }, { id: 'weighted', header: t('adminNative.weighted') }])
const history = computed(() => metrics.value?.usage.map(day => ({ at: Date.parse(day.date), value: Number(day.weightedBytes) })) ?? [])
</script>
<template>
  <section class="profile-metrics">
    <div class="row-actions"><h3>{{ t('adminNative.title') }}</h3><UButton :label="t('common.refresh')" icon="i-ph-arrows-clockwise" variant="ghost" :loading="loading" @click="load" /></div>
    <InlineNotice v-if="error" tone="warning">{{ error }}</InlineNotice>
    <USkeleton v-if="loading" class="h-28" />
    <template v-if="metrics">
      <InlineNotice v-if="metrics.incomplete" tone="info">{{ t('adminNative.incomplete') }}</InlineNotice>
      <dl class="profile-metric-totals"><div><dt>{{ t('adminNative.usedTraffic') }}</dt><dd :title="metrics.usedTrafficBytes">{{ formatBytes(metrics.usedTrafficBytes) }}</dd></div><div><dt>{{ t('adminNative.lifetime') }}</dt><dd :title="metrics.lifetimeTrafficBytes">{{ formatBytes(metrics.lifetimeTrafficBytes) }}</dd></div></dl>
      <UTable :data="metrics.nodes" :columns="columns" class="admin-table">
        <template #use-cell="{ row }">{{ t(!row.original.available ? 'adminNative.unavailable' : row.original.currentUse ? 'adminNative.active' : 'adminNative.idle') }}</template>
        <template #connections-cell="{ row }">{{ row.original.available ? row.original.connections : t('common.notAvailable') }}</template>
        <template #upload-cell="{ row }">{{ row.original.available ? rate(row.original.uploadBytesPerSecond) : t('common.notAvailable') }}</template>
        <template #download-cell="{ row }">{{ row.original.available ? rate(row.original.downloadBytesPerSecond) : t('common.notAvailable') }}</template>
        <template #total-cell="{ row }">{{ formatBytes(row.original.weightedBytes) }}</template>
        <template #empty>{{ t('adminNative.noNodes') }}</template>
      </UTable>
      <div class="profile-live-charts"><MetricLineChart :samples="samples.upload ?? []" :label="t('xnode.upload')" /><MetricLineChart :samples="samples.download ?? []" :label="t('xnode.download')" /></div>
      <MetricLineChart :samples="history" :label="t('adminNative.sevenDayUsage')" unit="bytes" />
      <UTable :data="metrics.usage" :columns="dailyColumns" class="admin-table"><template #upload-cell="{ row }">{{ formatBytes(row.original.uploadBytes) }}</template><template #download-cell="{ row }">{{ formatBytes(row.original.downloadBytes) }}</template><template #legacy-cell="{ row }">{{ formatBytes(row.original.legacyBytes) }}</template><template #weighted-cell="{ row }">{{ formatBytes(row.original.weightedBytes) }}</template></UTable>
    </template>
    <AdminUserScan :user-id="userId" />
  </section>
</template>
<style scoped>
.profile-metrics { min-width: 0; display: grid; gap: 1rem; }
.profile-live-charts, .profile-metric-totals { display: grid; grid-template-columns: repeat(2,minmax(0,1fr)); gap: 1rem; }
.profile-metric-totals { margin: 0; font-size: .8rem; }
dt { color: var(--text-muted); } dd { margin: .2rem 0; font-variant-numeric: tabular-nums; }
@media(max-width:600px) { .profile-live-charts { grid-template-columns: minmax(0,1fr); } }
</style>
