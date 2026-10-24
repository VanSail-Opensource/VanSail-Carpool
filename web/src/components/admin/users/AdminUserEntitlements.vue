<script setup lang="ts">
import { computed } from 'vue'
import type { AdminEntitlement } from '@/api/adminOperations'
import StatusBadge from '@/components/common/StatusBadge.vue'
import { useI18n } from '@/i18n'
import { formatBytes, formatDateTime, formatMoney } from '@/utils/format'
import { entitlementTone } from './adminUserFormat'
import { usePurchaseValidity } from '@/composables/usePurchaseValidity'
const props = defineProps<{ items: AdminEntitlement[]; busy: boolean }>()
const { status } = usePurchaseValidity(() => props.items)
const emit = defineEmits<{ edit: [item: AdminEntitlement]; refund: [item: AdminEntitlement] }>()
const { t } = useI18n()
const columns = computed(() => [
  { accessorKey: 'comboName', header: t('adminUserProfile.entitlements') },
  { id: 'validity', header: t('adminWorkspace.validity') },
  { id: 'traffic', header: t('adminWorkspace.traffic') },
  { id: 'price', header: t('adminWorkspace.price') },
  { id: 'status', header: t('adminWorkspace.status') },
  { id: 'actions', header: t('adminWorkspace.actions') },
])
function refundable(item: AdminEntitlement): boolean { return ['activating', 'active', 'queued'].includes(item.status) }
</script>
<template>
  <section class="admin-section">
    <div class="admin-subsection-heading"><div><h3>{{ t('adminUserProfile.entitlements') }}</h3><p>{{ t('adminUserProfile.entitlementsHint') }}</p></div></div>
    <UTable class="admin-table" :data="items" :columns="columns">
      <template #validity-cell="{ row }"><div class="admin-cell"><span>{{ formatDateTime(row.original.validFrom) }}</span><small>{{ formatDateTime(row.original.validUntil) }}</small></div></template>
      <template #traffic-cell="{ row }"><div class="admin-cell"><span>{{ formatBytes(row.original.trafficLimitBytes) }}</span><small>{{ t(`adminUserProfile.cadence.${row.original.resetStrategy}`) }} / {{ row.original.squadUuids.length }}</small></div></template>
      <template #price-cell="{ row }">{{ formatMoney(row.original.price) }}</template>
      <template #status-cell="{ row }"><StatusBadge :tone="entitlementTone(status(row.original))" :label="t(`adminUserProfile.entitlementStatus.${status(row.original)}`)" /></template>
      <template #actions-cell="{ row }"><div class="admin-row-actions"><UButton color="neutral" variant="ghost" icon="i-ph-pencil-simple" :aria-label="t('adminUserProfile.edit')" :disabled="busy" @click="emit('edit', row.original)" /><UButton v-if="refundable(row.original)" color="warning" variant="ghost" icon="i-ph-arrow-u-up-left" :label="t('adminUserProfile.refund')" :disabled="busy" @click="emit('refund', row.original)" /></div></template>
      <template #empty>{{ t('adminUserProfile.noEntitlements') }}</template>
    </UTable>
  </section>
</template>
