<script setup lang="ts">
import { restoreCatalogOptions } from '@/api/cache/catalogOptions'
import { computed, onMounted, onScopeDispose, reactive, shallowRef, watch } from 'vue'
import { useRouter } from 'vue-router'

import { adminOperationsApi, type AdminCatalogOptions, type OperationReceipt } from '@/api/adminOperations'
import type { AdminUserSummary } from '@/api/types'
import InlineNotice from '@/components/common/InlineNotice.vue'
import StatusBadge from '@/components/common/StatusBadge.vue'
import TxbAmountField from '@/components/common/TxbAmountField.vue'
import { useAdminSection } from '@/composables/useAdminSection'
import { localizedError, useI18n } from '@/i18n'
import { formatDate, formatMoney, moneyFromTxbInput } from '@/utils/format'
import AdminSectionState from './AdminSectionState.vue'
import AdminBulkExtensionDialog from './users/AdminBulkExtensionDialog.vue'
import AdminUserSearchFilters, { type AdminUserSearchFiltersValue } from './users/AdminUserSearchFilters.vue'

const { items, nextCursor, loading, busy, error, load, loadMore, perform } = useAdminSection<AdminUserSummary>('users')
const { t } = useI18n()
const router = useRouter()
const query = shallowRef('')
const selected = shallowRef<AdminUserSummary | null>(null)
const form = reactive({ amountTxb: '', reason: '' })
const success = shallowRef<string | null>(null)
const bulkOpen = shallowRef(false)
const filterOptions = shallowRef<AdminCatalogOptions>({ combos: [], squads: [] })
const filters = shallowRef<AdminUserSearchFiltersValue>({ state: '', comboIds: [], squadUuids: [], match: 'and' })
const filterError = shallowRef<string | null>(null)
const columns = computed(() => [
  { id: 'member', header: t('adminWorkspace.member') },
  { id: 'identity', header: t('adminWorkspace.identity') },
  { id: 'balance', header: t('adminUserProfile.balance') },
  { id: 'status', header: t('adminWorkspace.status') },
  { id: 'created', header: t('adminWorkspace.created') },
  { id: 'actions', header: t('adminWorkspace.actions') },
])

let searchTimer: ReturnType<typeof globalThis.setTimeout> | undefined

function userQuery(): Record<string, string | number | readonly string[] | undefined> {
  return {
    search: query.value.trim() || undefined, state: filters.value.state || undefined,
    comboId: filters.value.comboIds, squadUuid: filters.value.squadUuids, match: filters.value.match, limit: 25,
  }
}

async function loadFilterOptions(): Promise<void> {
  restoreCatalogOptions(filterOptions)
  filterError.value = null
  try {
    filterOptions.value = await adminOperationsApi.getCatalogOptions()
  } catch (caught) {
    filterError.value = localizedError(caught, 'adminUsers.filtersFailed')
  }
}

onMounted(() => { void loadFilterOptions() })

watch(query, () => {
  if (searchTimer !== undefined) globalThis.clearTimeout(searchTimer)
  searchTimer = globalThis.setTimeout(() => void load(userQuery()), 250)
})

onScopeDispose(() => {
  if (searchTimer !== undefined) globalThis.clearTimeout(searchTimer)
})

function reloadUsers(): Promise<void> {
  return load(userQuery())
}

function applyFilters(value: AdminUserSearchFiltersValue): void {
  filters.value = value
  void reloadUsers()
}

function displayName(summary: AdminUserSummary): string {
  return [summary.user.firstName, summary.user.lastName].filter(Boolean).join(' ') || t('nav.member')
}

async function adjust(sign: 1 | -1): Promise<void> {
  const amountMinor = moneyFromTxbInput(form.amountTxb)
  if (!selected.value || !amountMinor || form.reason.length < 4) return
  const current = selected.value
  const amount = sign === -1 ? `-${amountMinor}` : amountMinor
  const ok = await perform(() => import('@/api/client').then(({ api }) => api.adjustBalance(current.user.id, amount, form.reason)))
  if (ok) {
    success.value = t('adminUsers.adjusted', { name: displayName(current) })
    selected.value = null
    form.amountTxb = ''
    form.reason = ''
  }
}

function openProfile(summary: AdminUserSummary): void {
  void router.push({ name: 'admin-user', params: { userId: summary.user.id } })
}

function bulkQueued(receipt: OperationReceipt): void {
  success.value = t('adminBulkExtension.queuedNotice', { id: receipt.id })
}
</script>

<template>
  <section class="admin-panel">
    <div class="admin-panel__heading">
      <div><h2>{{ t('adminUsers.title') }}</h2><p>{{ t('adminUsers.copy') }}</p></div>
      <div class="row-actions">
        <UInput v-model="query" type="search" icon="i-ph-magnifying-glass" :placeholder="t('adminUsers.search')" :aria-label="t('adminUsers.search')" />
        <AdminUserSearchFilters :combos="filterOptions.combos" :squads="filterOptions.squads" @apply="applyFilters" />
        <UButton color="neutral" variant="outline" icon="i-ph-calendar-plus" :label="t('adminBulkExtension.open')" @click="bulkOpen = true" />
      </div>
    </div>
    <InlineNotice v-if="success" tone="success">{{ success }}</InlineNotice>
    <InlineNotice v-if="filterError" tone="warning">{{ filterError }}</InlineNotice>
    <AdminSectionState :loading="loading" :error="error" @retry="reloadUsers">
      <UTable class="admin-table" :data="items" :columns="columns" :loading="loading">
        <template #member-cell="{ row }"><UButton color="neutral" variant="link" :label="displayName(row.original)" @click="openProfile(row.original)" /></template>
        <template #identity-cell="{ row }"><div class="admin-cell"><span>{{ row.original.user.username ? `@${row.original.user.username}` : row.original.user.telegramId }}</span><code>{{ row.original.user.id }}</code></div></template>
        <template #balance-cell="{ row }">{{ formatMoney(row.original.balance) }}</template>
        <template #status-cell="{ row }"><StatusBadge :tone="row.original.synchronization.status === 'synchronized' ? 'success' : 'warning'" :label="t(row.original.synchronization.status === 'synchronized' ? 'adminUsers.synchronized' : 'adminUsers.notProvisioned')" /></template>
        <template #created-cell="{ row }">{{ formatDate(row.original.createdAt) }}</template>
        <template #actions-cell="{ row }"><div class="admin-row-actions"><UButton size="sm" color="neutral" variant="ghost" icon="i-ph-user-focus" :aria-label="t('adminUserProfile.open')" @click="openProfile(row.original)" /><UButton size="sm" color="neutral" variant="ghost" icon="i-ph-coins" :label="t('adminUsers.adjust')" @click="selected = row.original" /></div></template>
        <template #empty><div class="empty-inline"><h3>{{ t('adminUsers.none') }}</h3><p>{{ t('adminUsers.noneHint') }}</p></div></template>
      </UTable>
      <UButton v-if="nextCursor" class="database-load-more" color="neutral" variant="outline" icon="i-ph-arrow-down" :loading="loading" :disabled="loading" :label="t('adminUsers.loadMore')" @click="loadMore" />
    </AdminSectionState>

    <USlideover :open="Boolean(selected)" :title="selected ? t('adminUsers.adjustNamed', { name: displayName(selected) }) : t('adminUsers.adjust')" :description="t('adminUsers.adjustHint')" @update:open="!$event && (selected = null)">
      <template #body>
        <InlineNotice v-if="error" tone="warning">{{ error }}</InlineNotice>
        <form v-if="selected" class="admin-editor-form" @submit.prevent>
          <TxbAmountField id="balance-adjustment" v-model="form.amountTxb" :label="t('adminUsers.amount')" min-minor="1" required />
          <UFormField name="reason" :label="t('adminReason.reason')" required>
            <UInput v-model.trim="form.reason" :minlength="4" :maxlength="300" />
          </UFormField>
          <div class="button-row">
            <UButton color="neutral" variant="outline" icon="i-ph-plus" :disabled="busy" :label="t('adminUsers.add')" @click="adjust(1)" />
            <UButton color="error" variant="ghost" icon="i-ph-minus" :disabled="busy" :label="t('adminUsers.deduct')" @click="adjust(-1)" />
          </div>
        </form>
      </template>
    </USlideover>
    <AdminBulkExtensionDialog v-model:open="bulkOpen" @queued="bulkQueued" />
  </section>
</template>
