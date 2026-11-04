<script setup lang="ts">
import { computed } from 'vue'
import type { XNodeProfileMetrics } from '@/api/adminOperations'
import { useI18n } from '@/i18n'
import { formatDateTime } from '@/utils/format'

const props = defineProps<{ nodes: XNodeProfileMetrics['nodes']; stale?: boolean }>()
const { t } = useI18n()
const columns = computed(() => [{ accessorKey: 'name', header: t('adminWorkspace.name') }, { id: 'ip', header: t('adminNative.ipAddress') }, { id: 'lastActive', header: t('adminNative.lastActivity') }, { id: 'observations', header: t('automaticLeases.statistics') }, { id: 'state', header: t('firewall.status') }])
const rows = computed(() => props.nodes.flatMap(node => {
  const status = props.stale && node.activeIpStatus === 'available' ? 'stale' : node.activeIpStatus
  const base = { name: node.name, nodeId: node.nodeId, status, sampledAt: node.sampledAt, truncated: node.activeIpsTruncated }
  return node.activeIps.length ? node.activeIps.map(ip => ({ ...base, ...ip })) : [{ ...base, ip: '', lastActive: '', observations: '' }]
}))
const truncated = computed(() => props.nodes.some(node => node.activeIpsTruncated))
</script>

<template>
  <section class="grid min-w-0 gap-3">
    <h4 class="text-sm font-semibold">{{ t('adminNative.activeIPs') }}</h4>
    <p class="text-xs text-muted">{{ t('adminNative.activeIPsHint') }}</p>
    <UAlert v-if="truncated" color="warning" :title="t('adminNative.ipsTruncated')" />
    <UTable :data="rows" :columns="columns" :virtualize="rows.length > 100 ? { estimateSize: 64, overscan: 6 } : false" class="admin-table" :class="rows.length > 100 ? 'h-[360px] max-h-[55dvh]' : ''" :ui="{ base: 'min-w-[680px]' }" tabindex="0" role="region" :aria-label="t('adminNative.activeIPs')">
      <template #ip-cell="{ row }"><span :class="row.original.ip ? 'font-mono text-xs' : 'text-muted'">{{ row.original.ip || t(row.original.status !== 'available' ? 'adminNative.unavailable' : row.original.truncated ? 'adminNative.ipsLimited' : 'adminNative.noRecentIPs') }}</span></template>
      <template #lastActive-cell="{ row }">{{ row.original.lastActive ? formatDateTime(row.original.lastActive) : t('common.notAvailable') }}</template>
      <template #observations-cell="{ row }">{{ row.original.observations || t('common.notAvailable') }}</template>
      <template #state-cell="{ row }"><UBadge :color="row.original.status === 'available' ? 'neutral' : 'warning'" variant="subtle">{{ t(`adminNative.ipStatus.${row.original.status}`) }}</UBadge><p v-if="row.original.sampledAt" class="mt-1 text-xs text-muted">{{ t('adminNative.sampledAt', { time: formatDateTime(row.original.sampledAt) }) }}</p></template>
      <template #empty>{{ t('adminNative.noNodes') }}</template>
    </UTable>
  </section>
</template>
