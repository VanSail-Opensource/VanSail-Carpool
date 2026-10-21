<script setup lang="ts">
import type { TableColumn } from '@nuxt/ui'
import { computed } from 'vue'
import type { FirewallBan } from '@/api/firewall'
import { useI18n } from '@/i18n'
import XNodeFirewallGrant from './XNodeFirewallGrant.vue'
import { useFirewallSearch } from './useFirewallSearch'
import OperationStatusNotice from '@/components/common/OperationStatusNotice.vue'
const { t } = useI18n()
const state = useFirewallSearch()
const { items, query, loading, error, load } = state
const columns = computed<TableColumn<FirewallBan>[]>(() => [
  { accessorKey: 'prefix', header: t('firewall.ip') }, { accessorKey: 'rule', header: t('firewall.rule') },
  { accessorKey: 'count', header: t('firewall.hits') }, { accessorKey: 'detectedAt', header: t('firewall.detectedAt') },
  { accessorKey: 'until', header: t('firewall.deadline') }, { accessorKey: 'pendingNodes', header: t('firewall.pendingNodes') },
  { id: 'actions', header: t('firewall.unban') },
])
</script>

<template>
  <section class="grid min-w-0 gap-4">
    <XNodeFirewallGrant />
    <div class="flex items-center justify-between gap-4"><p class="text-sm text-muted">{{ t('firewall.reportHint') }}</p><UButton icon="i-ph-arrow-clockwise" variant="ghost" color="neutral" :loading="loading" :aria-label="t('common.refresh')" @click="load" /></div>
    <UFormField :label="t('firewall.search')"><UInput v-model="query" :placeholder="t('firewall.searchHint')" icon="i-ph-magnifying-glass" class="w-full" /></UFormField>
    <OperationStatusNotice :receipt="state.release.receipt.value" :error="state.release.error.value" :checking="state.release.checking.value" @refresh="state.release.refresh" />
    <p v-if="state.release.busy.value" role="status" class="text-sm text-muted">{{ t('firewall.releasePending') }}</p>
    <UProgress v-if="loading" />
    <UAlert v-if="error" color="warning" :title="error" />
    <UButton v-if="error" class="justify-self-start" variant="ghost" :label="t('common.tryAgain')" @click="load" />
    <p v-else-if="!loading && !items.length" class="text-muted">{{ t('firewall.empty') }}</p>
    <div v-else-if="items.length" class="overflow-x-auto">
      <UTable :data="items" :columns="columns" class="admin-table">
        <template #rule-cell="{ row }">{{ t(`firewall.rules.${row.original.rule}`) }}</template>
        <template #detectedAt-cell="{ row }">{{ new Date(row.original.detectedAt).toLocaleString() }}</template>
        <template #until-cell="{ row }">{{ new Date(row.original.until).toLocaleString() }}</template>
        <template #actions-cell="{ row }"><UButton variant="ghost" color="neutral" :label="t('firewall.unban')" :disabled="state.release.blocksMutations.value || state.release.submitting.value" @click="state.unban(row.original)" /></template>
      </UTable>
    </div>
    <div v-if="state.history.value.length || state.nextCursor.value" class="flex justify-end gap-2"><UButton variant="ghost" :label="t('nodeTemplates.previous')" :disabled="loading || !state.history.value.length" @click="state.previous" /><UButton variant="ghost" :label="t('nodeTemplates.next')" :disabled="loading || !state.nextCursor.value" @click="state.next" /></div>
  </section>
</template>
