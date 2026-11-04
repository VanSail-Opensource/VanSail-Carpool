<script setup lang="ts">
import { computed } from 'vue'
import InlineNotice from '@/components/common/InlineNotice.vue'
import MetricLineChart from '@/components/common/MetricLineChart.vue'
import { useI18n } from '@/i18n'
import { formatBytes, formatDateTime } from '@/utils/format'
import { useAdminUserMetrics } from './useAdminUserMetrics'
import AdminUserActiveIPs from './AdminUserActiveIPs.vue'
import AdminUserPAYGHistory from './AdminUserPAYGHistory.vue'
import AdminUserTrafficHistory from './AdminUserTrafficHistory.vue'
const props = defineProps<{ userId: string }>()
const { t } = useI18n()
const { metrics, loading, error, load, samples } = useAdminUserMetrics(() => props.userId)
const rate = (value: number) => t('statistics.perSecond', { value: formatBytes(value) })
const columns = computed(() => [{ accessorKey: 'name', header: t('adminWorkspace.name') }, { id: 'use', header: t('adminNative.currentUse') }, { id: 'connections', header: t('xnode.connections') }, { id: 'upload', header: t('xnode.upload') }, { id: 'download', header: t('xnode.download') }, { id: 'total', header: t('adminNative.nativeSevenDay') }])
</script>
<template>
  <section class="profile-metrics">
    <div class="flex items-center justify-between gap-3"><div><h3>{{ t('adminNative.title') }}</h3><p v-if="metrics" class="text-xs text-muted">{{ t('adminNative.updated', { time: formatDateTime(metrics.fetchedAt) }) }}</p></div><UButton :label="t('common.refresh')" icon="i-ph-arrows-clockwise" variant="ghost" :loading="loading" @click="load" /></div>
    <InlineNotice v-if="error" tone="warning">{{ error }}<span v-if="metrics"> {{ t('adminNative.staleSnapshot') }}</span></InlineNotice>
    <UButton v-if="error" class="justify-self-start" variant="ghost" :label="t('common.tryAgain')" @click="load" />
    <USkeleton v-if="loading && !metrics" class="h-28" />
    <template v-if="metrics">
      <InlineNotice v-if="metrics.incomplete" tone="info">{{ t('adminNative.incomplete') }}</InlineNotice>
      <dl class="profile-metric-totals"><div><dt>{{ t('adminNative.usedTraffic') }}</dt><dd :title="t('adminNative.exactBytes', { bytes: metrics.usedTrafficBytes })">{{ formatBytes(metrics.usedTrafficBytes) }}</dd></div><div><dt>{{ t('adminNative.lifetime') }}</dt><dd :title="t('adminNative.exactBytes', { bytes: metrics.lifetimeTrafficBytes })">{{ formatBytes(metrics.lifetimeTrafficBytes) }}</dd></div></dl>
      <UTable :data="metrics.nodes" :columns="columns" class="admin-table" :ui="{ base: 'min-w-[700px]' }" tabindex="0" role="region" :aria-label="t('adminNative.title')">
        <template #use-cell="{ row }">{{ t(error || !row.original.available ? 'adminNative.unavailable' : row.original.currentUse ? 'adminNative.active' : 'adminNative.idle') }}</template>
        <template #connections-cell="{ row }">{{ !error && row.original.available ? row.original.connections : t('common.notAvailable') }}</template>
        <template #upload-cell="{ row }">{{ !error && row.original.available ? rate(row.original.uploadBytesPerSecond) : t('common.notAvailable') }}</template>
        <template #download-cell="{ row }">{{ !error && row.original.available ? rate(row.original.downloadBytesPerSecond) : t('common.notAvailable') }}</template>
        <template #total-cell="{ row }"><span :title="t('adminNative.exactBytes', { bytes: row.original.nativeWeightedBytes })">{{ formatBytes(row.original.nativeWeightedBytes) }}</span></template>
        <template #empty>{{ t('adminNative.noNodes') }}</template>
      </UTable>
      <div class="profile-live-charts"><MetricLineChart :samples="samples.upload ?? []" :label="t('xnode.upload')" /><MetricLineChart :samples="samples.download ?? []" :label="t('xnode.download')" /></div>
      <AdminUserActiveIPs :nodes="metrics.nodes" :stale="Boolean(error)" />
      <AdminUserPAYGHistory :history="metrics.paygHistory" :nodes="metrics.nodes" :stale="Boolean(error)" />
      <AdminUserTrafficHistory :usage="metrics.usage" />
    </template>
  </section>
</template>
<style scoped>
.profile-metrics { min-width: 0; display: grid; gap: 1rem; }
.profile-live-charts, .profile-metric-totals { display: grid; grid-template-columns: repeat(2,minmax(0,1fr)); gap: 1rem; }
.profile-metric-totals { margin: 0; font-size: .8rem; }
dt { color: var(--text-muted); } dd { margin: .2rem 0; font-variant-numeric: tabular-nums; }
@media(max-width:600px) { .profile-live-charts { grid-template-columns: minmax(0,1fr); } }
</style>
