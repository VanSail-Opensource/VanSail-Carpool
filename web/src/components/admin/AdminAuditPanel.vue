<script setup lang="ts">
import { computed, shallowRef } from 'vue'
import type { TableColumn } from '@nuxt/ui'
import type { AuditEvent } from '@/api/types'
import { useAdminSection } from '@/composables/useAdminSection'
import { useI18n } from '@/i18n'
import { formatDateTime } from '@/utils/format'
import AdminSectionState from './AdminSectionState.vue'
const { items, loading, error, load, nextCursor, loadMore } = useAdminSection<AuditEvent>('audit-events')
const action = shallowRef('')
const { t } = useI18n()
const columns = computed<TableColumn<AuditEvent>[]>(() => [{ id: 'expand', header: '' }, { accessorKey: 'createdAt', header: t('browser.table.date') }, { accessorKey: 'action', header: t('browser.table.action') }, { accessorKey: 'actorUserId', header: t('browser.table.actor') }, { id: 'target', header: t('browser.table.target') }])
function detailText(detail: AuditEvent['detail']): string { return typeof detail === 'string' ? detail : JSON.stringify(detail, null, 2) }
function applyFilter(): void { void load({ action: action.value || undefined }) }
</script>
<template>
  <section class="admin-panel">
    <header class="admin-panel__heading"><div><h2>{{ t('adminAudit.title') }}</h2><p>{{ t('adminAudit.copy') }}</p></div><form class="button-row" @submit.prevent="applyFilter"><UInput v-model.trim="action" icon="i-ph-funnel" :placeholder="t('adminAudit.filter')" :aria-label="t('adminAudit.filterLabel')" /><UButton type="submit" color="neutral" variant="outline" :label="t('common.refresh')" /></form></header>
    <AdminSectionState :loading="loading" :error="error" @retry="load()">
      <UTable :data="items" :columns="columns" :expanded-options="{ getRowCanExpand: () => true }" class="admin-table">
        <template #expand-cell="{ row }"><UButton :icon="row.getIsExpanded() ? 'i-ph-caret-down' : 'i-ph-caret-right'" color="neutral" variant="ghost" :aria-label="t('browser.table.details')" :aria-expanded="row.getIsExpanded()" @click="row.toggleExpanded()" /></template>
        <template #createdAt-cell="{ row }">{{ formatDateTime(row.original.createdAt) }}</template>
        <template #actorUserId-cell="{ row }"><code>{{ row.original.actorUserId || t('adminAudit.system') }}</code></template>
        <template #target-cell="{ row }"><div class="admin-cell"><strong>{{ row.original.targetType }}</strong><code>{{ row.original.targetId }}</code></div></template>
        <template #expanded="{ row }"><pre class="audit-detail">{{ detailText(row.original.detail) }}</pre></template>
        <template #empty><div class="empty-inline"><h3>{{ t('adminAudit.none') }}</h3><p>{{ t('adminAudit.noneHint') }}</p></div></template>
      </UTable>
      <div v-if="nextCursor" class="admin-pagination"><UButton :loading="loading" :disabled="loading" color="neutral" variant="outline" :label="t('browser.table.loadMore')" @click="loadMore" /></div>
    </AdminSectionState>
  </section>
</template>
<style scoped>
.audit-detail { margin: 0; padding: 16px; white-space: pre-wrap; overflow-wrap: anywhere; font-size: 12px; color: var(--text-muted); }
</style>
