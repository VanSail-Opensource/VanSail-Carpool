<script setup lang="ts">
import { computed, shallowRef, watch } from 'vue'
import { AnimatePresence, motion } from 'motion-v'

import type { AdminEntitlement } from '@/api/types'
import StatusBadge from '@/components/common/StatusBadge.vue'
import { useAdminSection } from '@/composables/useAdminSection'
import { useMotionPreferences } from '@/composables/useMotionPreferences'
import { useI18n } from '@/i18n'
import { formatDate, formatMoney } from '@/utils/format'
import AdminReasonDialog from './AdminReasonDialog.vue'
import AdminSectionState from './AdminSectionState.vue'

const entitlements = useAdminSection<AdminEntitlement>('entitlements', { immediate: false })
const selected = shallowRef<AdminEntitlement | null>(null)
const reason = shallowRef('')
const statusFilter = shallowRef<'all' | AdminEntitlement['status']>('all')
const { t } = useI18n()
const { reducedMotion, offset } = useMotionPreferences()

const statusItems = computed(() => [
  { value: 'all', label: t('adminEntitlements.all') },
  { value: 'activating', label: t('adminEntitlements.activating') },
  { value: 'active', label: t('common.active') },
  { value: 'queued', label: t('common.queued') },
  { value: 'expired', label: t('adminEntitlements.expired') },
  { value: 'cancelled', label: t('adminEntitlements.cancelled') },
  { value: 'failed', label: t('adminEntitlements.failed') },
])
function reloadEntitlements(): Promise<void> {
  return entitlements.load({ status: statusFilter.value === 'all' ? undefined : statusFilter.value, limit: 25 })
}

watch(statusFilter, () => void reloadEntitlements(), { immediate: true })

function canCancel(item: AdminEntitlement): boolean {
  return item.status === 'activating' || item.status === 'active' || item.status === 'queued'
}

function tone(status: AdminEntitlement['status']): 'neutral' | 'success' | 'warning' | 'danger' {
  if (status === 'active') return 'success'
  if (status === 'queued' || status === 'activating') return 'warning'
  if (status === 'failed') return 'danger'
  return 'neutral'
}

function statusLabel(status: AdminEntitlement['status']): string {
  if (status === 'active') return t('common.active')
  if (status === 'queued') return t('common.queued')
  if (status === 'failed') return t('adminEntitlements.failed')
  return t(`adminEntitlements.${status}`)
}

async function cancelEntitlement(): Promise<void> {
  if (!selected.value) return
  const id = selected.value.id
  const { api } = await import('@/api/client')
  const success = await entitlements.perform(() => api.cancelAdminEntitlement(id, reason.value))
  if (success) { selected.value = null; reason.value = '' }
}

function setCancelOpen(open: boolean): void {
  if (!open) { selected.value = null; reason.value = '' }
}
</script>

<template>
  <section class="admin-panel">
    <div class="admin-panel__heading">
      <div><h2>{{ t('adminEntitlements.title') }}</h2><p>{{ t('adminEntitlements.copy') }}</p></div>
      <USelect v-model="statusFilter" :items="statusItems" value-key="value" :aria-label="t('adminEntitlements.filter')" />
    </div>
    <AdminSectionState :loading="entitlements.loading.value" :error="entitlements.error.value" @retry="reloadEntitlements">
      <motion.div layout class="admin-list">
        <AnimatePresence :initial="false" mode="popLayout">
          <motion.article v-for="entitlement in entitlements.items.value" :key="entitlement.id" layout class="admin-list-row admin-list-row--entitlement" :initial="reducedMotion ? false : { opacity: 0, y: offset(6) }" :animate="{ opacity: 1, y: 0 }" :exit="{ opacity: 0 }" :transition="{ duration: reducedMotion ? 0.08 : 0.18, ease: 'easeOut' }">
            <span class="feature-icon feature-icon--small"><UIcon name="i-ph-stack" /></span>
            <div><strong>{{ entitlement.comboName }}</strong><small>{{ t('adminEntitlements.summary', { user: entitlement.userId, from: formatDate(entitlement.validFrom), to: formatDate(entitlement.validUntil) }) }}</small></div>
            <strong>{{ formatMoney(entitlement.price) }}</strong>
            <StatusBadge :tone="tone(entitlement.status)" :label="statusLabel(entitlement.status)" />
            <UButton v-if="canCancel(entitlement)" size="sm" color="error" variant="ghost" icon="i-ph-prohibit" :label="t('common.cancel')" @click="selected = entitlement" />
          </motion.article>
          <motion.div v-if="!entitlements.items.value.length" key="empty-entitlements" class="empty-inline" :initial="{ opacity: 0 }" :animate="{ opacity: 1 }"><div><h3>{{ t('adminEntitlements.none') }}</h3><p>{{ t('adminEntitlements.noneHint') }}</p></div></motion.div>
        </AnimatePresence>
      </motion.div>
      <UButton v-if="entitlements.nextCursor.value" class="database-load-more" color="neutral" variant="outline" icon="i-ph-arrow-down" :loading="entitlements.loading.value" :disabled="entitlements.loading.value" :label="t('adminEntitlements.loadMore')" @click="entitlements.loadMore" />
    </AdminSectionState>
    <AdminReasonDialog :open="selected !== null" v-model:reason="reason" :title="t('adminEntitlements.cancelTitle')" :description="t('adminEntitlements.cancelDescription')" :confirm-label="t('adminEntitlements.cancelEntitlement')" :busy="entitlements.busy.value" danger @update:open="setCancelOpen" @confirm="cancelEntitlement" />
  </section>
</template>
