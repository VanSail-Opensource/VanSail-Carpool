<script setup lang="ts">
import type { AdminUserDetail } from '@/api/adminOperations'
import StatusBadge from '@/components/common/StatusBadge.vue'
import { useI18n } from '@/i18n'
import { formatDateTime, formatMoney } from '@/utils/format'

defineProps<{ detail: AdminUserDetail }>()
const { t } = useI18n()
</script>

<template>
  <section class="admin-section">
    <div class="admin-profile-section__heading"><div><h3>{{ t('adminUserProfile.accountContext') }}</h3><p>{{ t('adminUserProfile.accountContextHint') }}</p></div></div>
    <div class="admin-profile-history">
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
  </section>
</template>
