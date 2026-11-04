<script setup lang="ts">
import { computed } from 'vue'
import type { TableColumn } from '@nuxt/ui'
import type { AutomaticLease } from '@/api/firewall'
import type { XNode } from '@/api/xnode'
import { useI18n } from '@/i18n'
import { formatDateTime } from '@/utils/format'

const props = defineProps<{ items: AutomaticLease[]; nodes: XNode[] }>()
const { t } = useI18n()
const names = computed(() => new Map(props.nodes.map(node => [node.id, node.name])))
const columns = computed<TableColumn<AutomaticLease>[]>(() => [
  { accessorKey: 'ip', header: t('firewall.ip') },
  { id: 'activity', header: t('automaticLeases.activity') },
  { id: 'expiry', header: t('automaticLeases.expiry') },
  { id: 'renewal', header: t('automaticLeases.renewal') },
  { id: 'counts', header: t('automaticLeases.statistics') },
  { id: 'delivery', header: t('automaticLeases.delivery') },
])
function remaining(seconds: number) {
  const minutes = Math.ceil(seconds / 60)
  return t('automaticLeases.remaining', { hours: Math.floor(minutes / 60), minutes: minutes % 60 })
}
</script>

<template>
  <div class="min-w-0">
    <UTable :data="items" :columns="columns" class="admin-table" :ui="{ base: 'min-w-[1020px]' }" role="region" :aria-label="t('automaticLeases.title')" tabindex="0">
      <template #ip-cell="{ row }"><span class="font-mono text-xs">{{ row.original.ip }}</span></template>
      <template #activity-cell="{ row }">
        <p>{{ row.original.lastActive ? formatDateTime(row.original.lastActive) : t('connections.notObserved') }}</p>
        <p class="text-xs text-muted">{{ t('automaticLeases.firstActive', { time: formatDateTime(row.original.firstActive) }) }}</p>
        <details class="mt-1 text-xs"><summary class="cursor-pointer">{{ t('automaticLeases.sources', { count: `${row.original.sourceCount}${row.original.sourceCountTruncated ? '+' : ''}` }) }}</summary><ul class="mt-1 grid gap-1"><li v-for="id in row.original.sourceNodeIds" :key="id" class="max-w-64 truncate" :title="id">{{ names.get(id) ?? id }}</li></ul></details>
      </template>
      <template #expiry-cell="{ row }">
        <p>{{ formatDateTime(row.original.expiresAt) }}</p><p class="text-xs text-muted">{{ remaining(row.original.remainingSeconds) }}</p>
      </template>
      <template #renewal-cell="{ row }">
        <p>{{ row.original.renewedAt ? formatDateTime(row.original.renewedAt) : t('automaticLeases.notRenewed') }}</p>
        <p class="text-xs text-muted">{{ t('automaticLeases.renewAfter', { time: formatDateTime(row.original.renewAfter) }) }}</p>
        <p class="text-xs text-muted">{{ t('automaticLeases.started', { time: formatDateTime(row.original.leaseStarted) }) }}</p>
      </template>
      <template #counts-cell="{ row }"><p>{{ t('automaticLeases.observations', { count: row.original.observationCount }) }}</p><p class="text-xs text-muted">{{ t('automaticLeases.renewals', { count: row.original.renewalCount }) }}</p></template>
      <template #delivery-cell="{ row }">
        <UBadge :color="row.original.pendingNodes || row.original.banReleasePending ? 'warning' : 'neutral'" variant="subtle">{{ row.original.pendingNodes ? t('automaticLeases.pendingNodes', { count: row.original.pendingNodes }) : t('automaticLeases.delivered') }}</UBadge>
        <p v-if="row.original.banReleasePending" class="mt-1 text-xs text-muted">{{ t('automaticLeases.banPending') }}</p>
      </template>
    </UTable>
  </div>
</template>
