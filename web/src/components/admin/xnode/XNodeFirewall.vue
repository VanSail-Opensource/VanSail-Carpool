<script setup lang="ts">
import type { TableColumn } from '@nuxt/ui'
import { computed } from 'vue'
import type { FirewallFilter, FirewallRecord } from '@/api/firewall'
import { useI18n } from '@/i18n'
import XNodeFirewallGrant from './XNodeFirewallGrant.vue'
import { useFirewallSearch } from './useFirewallSearch'
import OperationStatusNotice from '@/components/common/OperationStatusNotice.vue'
import FirewallFilterGroup from './FirewallFilterGroup.vue'
const { t } = useI18n()
const state = useFirewallSearch()
const { items, query, draft, filterValid, loading, error, load } = state
const columns = computed<TableColumn<FirewallRecord>[]>(() => [
  { accessorKey: 'prefix', header: t('firewall.ip') }, { id: 'status', header: t('firewall.status') }, { accessorKey: 'rule', header: t('firewall.rule') },
  { accessorKey: 'count', header: t('firewall.hits') }, { accessorKey: 'detectedAt', header: t('firewall.detectedAt') },
  { accessorKey: 'until', header: t('firewall.deadline') }, { accessorKey: 'pendingNodes', header: t('firewall.pendingNodes') },
  { id: 'actions', header: t('firewall.unban') },
])
function countFilters(filter: FirewallFilter): { nodes: number; conditions: number } {
  const children = (filter.children ?? []).map(countFilters)
  return { nodes: 1 + children.reduce((sum, item) => sum + item.nodes, 0), conditions: filter.op ? children.reduce((sum, item) => sum + item.conditions, 0) : 1 }
}
const counts = computed(() => {
  const value = countFilters(draft.value)
  return { nodes: value.nodes + 1 + Number(Boolean(query.value.trim())), conditions: value.conditions + Number(Boolean(query.value.trim())) }
})
</script>

<template>
  <section class="grid min-w-0 gap-4">
    <XNodeFirewallGrant />
    <div class="flex items-center justify-between gap-4"><p class="text-sm text-muted">{{ t('firewall.reportHint') }}</p><UButton icon="i-ph-arrow-clockwise" variant="ghost" color="neutral" :loading="loading" :aria-label="t('common.refresh')" @click="load" /></div>
    <UFormField :label="t('firewall.search')"><UInput v-model="query" :placeholder="t('firewall.searchHint')" icon="i-ph-magnifying-glass" class="w-full" /></UFormField>
    <div class="grid min-w-0 gap-2">
      <h3 class="text-sm font-medium">{{ t('firewall.filters') }}</h3>
      <FirewallFilterGroup v-model="draft" :depth="2" :nodes="counts.nodes" :conditions="counts.conditions" />
      <UButton class="justify-self-start" variant="soft" :label="t('firewall.applyFilters')" :disabled="!filterValid" @click="state.applyFilters" />
    </div>
    <OperationStatusNotice :receipt="state.release.receipt.value" :error="state.release.error.value" :checking="state.release.checking.value" @refresh="state.release.refresh" />
    <p v-if="state.release.busy.value" role="status" class="text-sm text-muted">{{ t('firewall.releasePending') }}</p>
    <UProgress v-if="loading" />
    <UAlert v-if="error" color="warning" :title="error" />
    <UButton v-if="error" class="justify-self-start" variant="ghost" :label="t('common.tryAgain')" @click="load" />
    <p v-else-if="!loading && !items.length" class="text-muted">{{ t('firewall.empty') }}</p>
    <div v-else-if="items.length" class="overflow-x-auto">
      <UTable :data="items" :columns="columns" class="admin-table">
        <template #prefix-cell="{ row }">{{ row.original.banned ? row.original.prefix : row.original.ip }}</template>
        <template #status-cell="{ row }"><UBadge :color="row.original.banned ? 'error' : 'neutral'" variant="subtle">{{ t(row.original.banned ? 'firewall.banned' : 'firewall.reported') }}</UBadge></template>
        <template #rule-cell="{ row }">{{ t(`firewall.rules.${row.original.rule}`) }}</template>
        <template #detectedAt-cell="{ row }">{{ new Date(row.original.detectedAt).toLocaleString() }}</template>
        <template #until-cell="{ row }">{{ row.original.until ? new Date(row.original.until).toLocaleString() : t('common.notAvailable') }}</template>
        <template #pendingNodes-cell="{ row }">{{ row.original.pendingNodes ?? t('common.notAvailable') }}</template>
        <template #actions-cell="{ row }"><UButton v-if="row.original.banned && row.original.generation" variant="ghost" color="neutral" :label="t('firewall.unban')" :disabled="state.release.blocksMutations.value || state.release.submitting.value" @click="state.unban(row.original)" /></template>
      </UTable>
    </div>
    <div v-if="state.history.value.length || state.nextCursor.value" class="flex justify-end gap-2"><UButton variant="ghost" :label="t('nodeTemplates.previous')" :disabled="loading || !state.history.value.length" @click="state.previous" /><UButton variant="ghost" :label="t('nodeTemplates.next')" :disabled="loading || !state.nextCursor.value" @click="state.next" /></div>
  </section>
</template>
