<script setup lang="ts">
import { shallowRef } from 'vue'

import { adminOperationsApi, type AdminUserDetail } from '@/api/adminOperations'
import InlineNotice from '@/components/common/InlineNotice.vue'
import StatusBadge from '@/components/common/StatusBadge.vue'
import { useI18n } from '@/i18n'
import { createUuid } from '@/utils/browserCompatibility'
import { formatDateTime, formatMoney } from '@/utils/format'

defineProps<{ userId: string; detail: AdminUserDetail }>()
const emit = defineEmits<{ changed: [] }>()
const { t } = useI18n()
const deleting = shallowRef<AdminUserDetail['abuseHistory'][number] | null>(null)
const busy = shallowRef(false)
const error = shallowRef<string | null>(null)

function abuseTone(action: string): 'warning' | 'danger' | 'neutral' {
  return action === 'warning' ? 'warning' : action === 'ip_ban' ? 'danger' : 'neutral'
}

async function deleteAbuse(): Promise<void> {
  if (!deleting.value || busy.value) return
  busy.value = true
  try {
    await adminOperationsApi.deleteAbuseRecord(deleting.value.id, createUuid())
    deleting.value = null; emit('changed');
  } catch { error.value = t('adminUserProfile.actionFailed'); } finally { busy.value = false }
}
</script>

<template>
  <section class="admin-section">
    <div class="admin-profile-section__heading"><div><h3>{{ t('adminUserProfile.accountContext') }}</h3><p>{{ t('adminUserProfile.accountContextHint') }}</p></div></div>
    <InlineNotice v-if="error" tone="warning">{{ error }}</InlineNotice>
    <div class="admin-profile-history">
      <section class="admin-profile-history__group">
        <h4><UIcon name="i-ph-shield-warning" />{{ t('adminUserProfile.abuseHistory') }}</h4>
        <UTable class="admin-table" :data="detail.abuseHistory" :columns="[{ accessorKey: 'reason', header: t('adminWorkspace.reason') }, { id: 'occurred', header: t('adminWorkspace.date') }, { id: 'status', header: t('adminWorkspace.status') }, { id: 'actions', header: t('adminWorkspace.actions') }]">
          <template #occurred-cell="{ row }"><div class="admin-cell"><span>{{ formatDateTime(row.original.occurredAt) }}</span><small>{{ row.original.measuredQPS }} QPS</small></div></template>
          <template #status-cell="{ row }"><StatusBadge :tone="abuseTone(row.original.action)" :label="t(`adminUserProfile.abuseAction.${row.original.action}`)" /></template>
          <template #actions-cell="{ row }"><UButton color="error" variant="ghost" icon="i-ph-trash" :aria-label="t('adminUserProfile.deleteAbuse')" :disabled="busy" @click="deleting = row.original" /></template>
          <template #empty>{{ t('adminUserProfile.noAbuseHistory') }}</template>
        </UTable>
      </section>
      <section class="admin-profile-history__group">
        <h4><UIcon name="i-ph-users-three" />{{ t('adminUserProfile.affiliateHistory') }}</h4>
        <UTable class="admin-table" :data="detail.affiliateHistory.items" :columns="[{ id: 'member', header: t('adminWorkspace.member') }, { id: 'date', header: t('adminWorkspace.date') }, { id: 'amount', header: t('adminWorkspace.amount') }, { id: 'status', header: t('adminWorkspace.status') }]">
          <template #member-cell="{ row }">{{ [row.original.firstName, row.original.lastName].filter(Boolean).join(' ') }}</template>
          <template #date-cell="{ row }">{{ formatDateTime(row.original.registeredAt) }}</template>
          <template #amount-cell="{ row }">{{ row.original.commissionAmount ? formatMoney(row.original.commissionAmount) : t('common.notAvailable') }}</template>
          <template #status-cell="{ row }"><StatusBadge :tone="row.original.status === 'successful' ? 'success' : 'warning'" :label="t(`adminUserProfile.affiliateStatus.${row.original.status}`)" /></template>
          <template #empty>{{ t('adminUserProfile.noAffiliateHistory') }}</template>
        </UTable>
      </section>
    </div>
    <UModal :open="deleting !== null" :title="t('adminUserProfile.deleteAbuse')" :description="t('adminUserProfile.deleteAbuseHint')" :dismissible="!busy" :close="false" :ui="{ header: 'admin-overlay-header', wrapper: 'admin-overlay-copy', footer: 'justify-end' }" @update:open="!$event && (deleting = null)"><template #body><InlineNotice v-if="error" tone="warning">{{ error }}</InlineNotice></template><template #footer="{ close }"><UButton color="neutral" variant="outline" :label="t('common.cancel')" @click="close" /><UButton color="error" :label="t('common.delete')" :loading="busy" @click="deleteAbuse" /></template></UModal>
  </section>
</template>
