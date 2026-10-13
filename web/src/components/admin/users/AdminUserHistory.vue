<script setup lang="ts">
import { computed, shallowRef } from 'vue'
import type { AdminUserDetail, OperationReceipt } from '@/api/adminOperations'
import StatusBadge from '@/components/common/StatusBadge.vue'
import { useI18n } from '@/i18n'
import { formatDateTime, formatMoney } from '@/utils/format'
type Payment = AdminUserDetail['payments'][number]
defineProps<{ detail: AdminUserDetail; busy: boolean }>()
const emit = defineEmits<{ resolve: [operation: OperationReceipt]; refundPayment: [payment: Payment]; creditPayment: [payment: Payment] }>()
const { t } = useI18n()
const current = shallowRef('payments')
const tabs = computed(() => [
  { value: 'payments', label: t('adminUserProfile.payments') },
  { value: 'refunds', label: t('adminUserProfile.refunds') },
  { value: 'operations', label: t('adminUserProfile.openOperations') },
  { value: 'emby', label: t('adminUserProfile.emby') },
])
const columns = computed(() => [
  { id: 'identity', header: t('adminWorkspace.identity') },
  { id: 'date', header: t('adminWorkspace.date') },
  { id: 'amount', header: t('adminWorkspace.amount') },
  { id: 'status', header: t('adminWorkspace.status') },
  { id: 'actions', header: t('adminWorkspace.actions') },
])
function operationTone(status: OperationReceipt['status']): 'neutral' | 'success' | 'warning' | 'danger' {
  if (status === 'succeeded' || status === 'compensated') return 'success'
  if (status === 'failed') return 'danger'
  return status === 'queued' || status === 'processing' ? 'neutral' : 'warning'
}
</script>
<template>
  <section class="admin-section">
    <UTabs v-model="current" :items="tabs" :content="false" variant="link" class="admin-workspace-tabs" />
    <UTable v-if="current === 'payments'" class="admin-table" :data="detail.payments" :columns="columns">
      <template #identity-cell="{ row }"><div class="admin-cell"><strong>{{ row.original.provider }} / {{ row.original.providerRail }}</strong><code>{{ row.original.id }}</code></div></template>
      <template #date-cell="{ row }">{{ formatDateTime(row.original.createdAt) }}</template>
      <template #amount-cell="{ row }">{{ formatMoney(row.original.txb) }}</template>
      <template #status-cell="{ row }"><StatusBadge :tone="row.original.status === 'paid' ? 'success' : row.original.status === 'failed' ? 'danger' : 'neutral'" :label="t(`adminUserProfile.paymentStatus.${row.original.status}`)" /></template>
      <template #actions-cell="{ row }"><div class="admin-row-actions"><UButton v-if="row.original.status === 'paid'" color="warning" variant="ghost" :disabled="busy" :label="t('adminUserProfile.refundPayment')" @click="emit('refundPayment', row.original)" /><UButton v-if="['failed', 'expired'].includes(row.original.status)" variant="ghost" :disabled="busy" :label="t('adminUserProfile.courtesyCredit')" @click="emit('creditPayment', row.original)" /></div></template>
      <template #empty>{{ t('adminUserProfile.noPayments') }}</template>
    </UTable>
    <UTable v-else-if="current === 'refunds'" class="admin-table" :data="detail.refunds" :columns="columns.slice(0, 3)">
      <template #identity-cell="{ row }"><div class="admin-cell"><strong>{{ row.original.reason }}</strong><code>{{ row.original.paymentOrderId }}</code></div></template>
      <template #date-cell="{ row }">{{ formatDateTime(row.original.createdAt) }}</template>
      <template #amount-cell="{ row }">{{ formatMoney(row.original.txb) }}</template>
      <template #empty>{{ t('adminUserProfile.noRefunds') }}</template>
    </UTable>
    <UTable v-else-if="current === 'operations'" class="admin-table" :data="detail.operations" :columns="columns.filter(column => column.id !== 'amount')">
      <template #identity-cell="{ row }"><div class="admin-cell"><strong>{{ row.original.kind }}</strong><code>{{ row.original.id }}</code></div></template>
      <template #date-cell="{ row }">{{ formatDateTime(row.original.updatedAt) }}</template>
      <template #status-cell="{ row }"><StatusBadge :tone="operationTone(row.original.status)" :label="t(`adminUserProfile.operationStatus.${row.original.status}`)" /></template>
      <template #actions-cell="{ row }"><UButton v-if="['pending_review', 'partial'].includes(row.original.status)" color="warning" variant="ghost" icon="i-ph-gavel" :label="t('adminUserProfile.resolve')" :disabled="busy" @click="emit('resolve', row.original)" /></template>
      <template #empty>{{ t('adminUserProfile.noOpenOperations') }}</template>
    </UTable>
    <UTable v-else class="admin-table" :data="detail.embyAccounts" :columns="columns.slice(0, 4).filter(column => column.id !== 'amount')">
      <template #identity-cell="{ row }"><div class="admin-cell"><strong>{{ row.original.username }}</strong><small>{{ t('adminUserProfile.embyFacts', { libraries: row.original.disabledLibraryIds.length, date: formatDateTime(row.original.updatedAt) }) }}</small></div></template>
      <template #date-cell="{ row }">{{ formatDateTime(row.original.updatedAt) }}</template>
      <template #status-cell="{ row }"><StatusBadge :tone="row.original.status === 'active' ? 'success' : row.original.status === 'failed' ? 'danger' : 'warning'" :label="t(`adminUserProfile.embyStatus.${row.original.status}`)" /></template>
      <template #empty>{{ t('adminUserProfile.noEmby') }}</template>
    </UTable>
  </section>
</template>
