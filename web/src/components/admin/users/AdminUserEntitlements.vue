<script setup lang="ts">
import { computed } from 'vue'
import { AnimatePresence, motion } from 'motion-v'

import type { AdminEntitlement } from '@/api/adminOperations'
import StatusBadge from '@/components/common/StatusBadge.vue'
import { useMotionPreferences } from '@/composables/useMotionPreferences'
import { usePurchaseValidity } from '@/composables/usePurchaseValidity'
import { useI18n } from '@/i18n'
import { formatBytes, formatDateTime, formatMoney } from '@/utils/format'
import { entitlementTone } from './adminUserFormat'

const props = defineProps<{ items: AdminEntitlement[]; busy: boolean }>()
const emit = defineEmits<{ add: []; edit: [item: AdminEntitlement]; refund: [item: AdminEntitlement] }>()
const { t } = useI18n()
const { reducedMotion } = useMotionPreferences()
const { status } = usePurchaseValidity(() => props.items)
const columns = computed(() => [
  { id: 'plan', header: t('adminUserProfile.combo') },
  { id: 'validity', header: t('adminWorkspace.validity') },
  { id: 'traffic', header: t('adminWorkspace.traffic') },
  { id: 'price', header: t('adminWorkspace.price') },
  { id: 'status', header: t('adminWorkspace.status') },
  { id: 'actions', header: t('adminWorkspace.actions') },
])
const tableKey = computed(() => props.items.map(item => `${item.id}:${status(item)}`).join(','))
const hasDiscount = (item: AdminEntitlement) => /^\d+$/.test(item.couponDiscount.minor) && BigInt(item.couponDiscount.minor) > 0n
const refundable = (item: AdminEntitlement) => ['activating', 'active', 'queued'].includes(item.status)
</script>

<template>
  <section class="admin-section">
    <div class="admin-subsection-heading">
      <div><h3>{{ t('adminUserProfile.entitlements') }}</h3><p>{{ t('adminUserProfile.entitlementsHint') }}</p></div>
      <UButton color="primary" icon="i-ph-plus" :label="t('adminUserProfile.addFixedTermPlan')" :disabled="busy" @click="emit('add')" />
    </div>
    <AnimatePresence mode="wait" :initial="false">
      <motion.div :key="tableKey" :initial="reducedMotion ? { opacity: 0 } : { opacity: 0, y: 4 }" :animate="{ opacity: 1, y: 0 }" :exit="{ opacity: 0 }" :transition="{ duration: reducedMotion ? 0.08 : 0.18, ease: 'easeOut' }">
        <UTable class="admin-table" :data="items" :columns="columns" tabindex="0" role="region" :aria-label="t('adminUserProfile.entitlements')">
          <template #plan-cell="{ row }"><div class="admin-cell"><strong>{{ row.original.comboName }}</strong><small>{{ t('adminUserProfile.addedAt', { date: formatDateTime(row.original.createdAt) }) }}</small></div></template>
          <template #validity-cell="{ row }"><div class="admin-cell"><span>{{ t('adminUserProfile.validity', { from: formatDateTime(row.original.validFrom), to: formatDateTime(row.original.validUntil) }) }}</span></div></template>
          <template #traffic-cell="{ row }"><div class="admin-cell"><span>{{ formatBytes(row.original.trafficLimitBytes) }}</span><small>{{ t('adminUserProfile.cadence.' + row.original.resetStrategy) }} · {{ t('adminUserProfile.squadCount', { count: row.original.squadUuids.length }) }}</small></div></template>
          <template #price-cell="{ row }"><div class="admin-cell"><span>{{ formatMoney(row.original.price) }}</span><small v-if="hasDiscount(row.original)">{{ t('adminUserProfile.listPrice', { amount: formatMoney(row.original.grossPrice) }) }}</small></div></template>
          <template #status-cell="{ row }"><StatusBadge :tone="entitlementTone(status(row.original))" :label="t('adminUserProfile.entitlementStatus.' + status(row.original))" /></template>
          <template #actions-cell="{ row }"><div class="admin-row-actions"><UButton color="neutral" variant="ghost" icon="i-ph-pencil-simple" :label="t('adminUserProfile.edit')" :disabled="busy" @click="emit('edit', row.original)" /><UButton v-if="refundable(row.original)" color="warning" variant="ghost" icon="i-ph-arrow-u-up-left" :label="t('adminUserProfile.refund')" :disabled="busy" @click="emit('refund', row.original)" /></div></template>
          <template #empty>{{ t('adminUserProfile.noEntitlements') }}</template>
        </UTable>
      </motion.div>
    </AnimatePresence>
  </section>
</template>
