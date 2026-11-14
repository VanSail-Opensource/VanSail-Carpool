<script setup lang="ts">
import { computed, shallowRef } from 'vue'
import { AnimatePresence, motion } from 'motion-v'

import type { AdminHourlySquad } from '@/api/adminOperations'
import InlineNotice from '@/components/common/InlineNotice.vue'
import StatusBadge from '@/components/common/StatusBadge.vue'
import HourlyAccountingStatus from '@/components/hourly/HourlyAccountingStatus.vue'
import { useMotionPreferences } from '@/composables/useMotionPreferences'
import { useI18n } from '@/i18n'
import { formatBytes, formatDateTime, formatMoney } from '@/utils/format'
import { useAdminUserHourly } from './useAdminUserHourly'

const props = defineProps<{ userId: string }>()
const { t } = useI18n()
const { reducedMotion } = useMotionPreferences()
const hourly = useAdminUserHourly(() => props.userId)
const { view, squads, loading, error, actionError, busyEnrollment, queued } = hourly
const selected = shallowRef<AdminHourlySquad | null>(null)
const columns = computed(() => [
  { id: 'squad', header: t('hourly.admin.user.squad') },
  { id: 'status', header: t('adminWorkspace.status') },
  { id: 'price', header: t('hourly.admin.user.price') },
  { id: 'paidThrough', header: t('hourly.admin.user.paidThrough') },
  { id: 'cooldown', header: t('hourly.admin.user.cooldown') },
  { id: 'actions', header: t('adminWorkspace.actions') },
])
const tableKey = computed(() => squads.value.map(squad => `${squad.enrollmentId}:${squad.status}`).join(','))
const usage = computed(() => view.value?.usage ?? null)

function tone(status: AdminHourlySquad['status']): 'neutral' | 'success' | 'warning' | 'danger' {
  if (status === 'active') return 'success'
  if (status === 'funding_suspended') return 'danger'
  if (status === 'preparing') return 'warning'
  return 'neutral'
}

function canDisable(squad: AdminHourlySquad): boolean {
  return ['preparing', 'active', 'funding_suspended'].includes(squad.status)
}

function askDisable(squad: AdminHourlySquad): void {
  hourly.clearActionError()
  selected.value = squad
}

async function confirmDisable(): Promise<void> {
  const squad = selected.value
  if (!squad) return
  if (await hourly.disable(squad)) selected.value = null
}
</script>

<template>
  <section class="admin-section hourly-profile">
    <div class="admin-subsection-heading">
      <div><h3>{{ t('hourly.admin.user.title') }}</h3><p>{{ t('hourly.admin.user.hint') }}</p></div>
      <UButton color="neutral" variant="outline" icon="i-ph-arrow-clockwise" :label="t('common.refresh')" :loading="loading" :disabled="loading || busyEnrollment !== null" @click="hourly.load()" />
    </div>
    <InlineNotice v-if="error" tone="warning">{{ error }}<span v-if="view"> {{ t('hourly.admin.user.stale') }}</span><UButton variant="link" :label="t('common.tryAgain')" :disabled="loading" @click="hourly.load()" /></InlineNotice>
    <div v-if="loading && !view" class="admin-loading" :aria-label="t('hourly.admin.user.title')">
      <USkeleton class="h-20" /><USkeleton class="h-24" />
    </div>
    <div v-else-if="!view" class="empty-inline"><div><h3>{{ t('hourly.admin.user.unavailable') }}</h3><p>{{ t('hourly.admin.user.loadFailed') }}</p></div></div>
    <template v-else>
      <InlineNotice v-if="actionError" tone="warning">{{ actionError }}</InlineNotice>
      <InlineNotice v-else-if="queued" tone="success">{{ t('hourly.admin.user.disableQueued', { id: queued.id }) }}</InlineNotice>
      <dl class="hourly-profile__facts">
        <div><dt>{{ t('hourly.admin.user.balance') }}</dt><dd>{{ formatMoney(view.balance) }}</dd></div>
        <div><dt>{{ t('hourly.admin.salesEnabled') }}</dt><dd>{{ t(view.policy.policy.salesEnabled ? 'common.enabled' : 'hourly.admin.user.salesPausedShort') }}</dd></div>
        <div><dt>{{ t('hourly.admin.activationHours') }}</dt><dd>{{ view.policy.policy.activationHours }}</dd></div>
        <div><dt>{{ t('hourly.dailyAllowance') }}</dt><dd>{{ usage ? t('hourly.admin.user.usageFacts', { used: formatBytes(usage.freeUsedBytes), total: formatBytes(usage.freeAllowanceBytes) }) : t('common.notAvailable') }}</dd></div>
        <div><dt>{{ t('hourly.admin.user.extraPreference') }}</dt><dd>{{ t(view.extraPreference.extraEnabled ? 'hourly.admin.user.extraEnabled' : 'hourly.admin.user.extraDisabled') }}</dd></div>
      </dl>
      <HourlyAccountingStatus :usage="usage" :show-historical="true" />
      <AnimatePresence mode="wait" :initial="false">
        <motion.div :key="tableKey" :initial="reducedMotion ? { opacity: 0 } : { opacity: 0, y: 4 }" :animate="{ opacity: 1, y: 0 }" :exit="{ opacity: 0 }" :transition="{ duration: reducedMotion ? 0.08 : 0.18, ease: 'easeOut' }">
          <UTable class="admin-table" :data="squads" :columns="columns" tabindex="0" role="region" :aria-label="t('hourly.admin.user.title')">
            <template #squad-cell="{ row }"><div class="admin-cell"><strong>{{ row.original.name }}</strong><code>{{ row.original.groupId }}</code></div></template>
            <template #status-cell="{ row }"><div class="admin-cell"><StatusBadge :tone="tone(row.original.status)" :label="t(`hourly.status.${row.original.status}`)" /><small v-if="!row.original.offerEnabled">{{ t('hourly.admin.user.offerClosed') }}</small></div></template>
            <template #price-cell="{ row }">{{ formatMoney(row.original.hourlyPrice) }}</template>
            <template #paidThrough-cell="{ row }">{{ row.original.paidThrough ? formatDateTime(row.original.paidThrough) : t('common.notScheduled') }}</template>
            <template #cooldown-cell="{ row }">{{ row.original.cooldownUntil ? formatDateTime(row.original.cooldownUntil) : t('common.notAvailable') }}</template>
            <template #actions-cell="{ row }"><div class="admin-row-actions"><UButton v-if="canDisable(row.original)" color="warning" variant="ghost" icon="i-ph-prohibit" :label="t('hourly.disable')" :loading="busyEnrollment === row.original.enrollmentId" :disabled="busyEnrollment !== null" @click="askDisable(row.original)" /></div></template>
            <template #empty>{{ t('hourly.admin.user.empty') }}</template>
          </UTable>
        </motion.div>
      </AnimatePresence>
    </template>
    <UModal :open="selected !== null" :title="t('hourly.admin.user.disableTitle')" :description="t('hourly.admin.user.disableHint', { name: selected?.name ?? '' })" :dismissible="busyEnrollment === null" :close="false" :ui="{ header: 'admin-overlay-header', wrapper: 'admin-overlay-copy', footer: 'justify-end' }" @update:open="!$event && (selected = null)">
      <template #body>
        <InlineNotice v-if="actionError" tone="warning">{{ actionError }}</InlineNotice>
      </template>
      <template #footer>
        <UButton color="neutral" variant="outline" :label="t('common.cancel')" :disabled="busyEnrollment !== null" @click="selected = null" />
        <UButton color="warning" icon="i-ph-prohibit" :label="t('hourly.admin.user.disableConfirm')" :loading="busyEnrollment !== null" :disabled="busyEnrollment !== null" @click="confirmDisable" />
      </template>
    </UModal>
  </section>
</template>

<style scoped>
.hourly-profile__facts { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: .9rem 1.5rem; margin: 0 0 1rem; font-size: .8rem; }
.hourly-profile__facts dt { color: var(--text-muted); font-size: .72rem; }
.hourly-profile__facts dd { margin: .2rem 0 0; font-variant-numeric: tabular-nums; overflow-wrap: anywhere; }
@media (min-width: 900px) { .hourly-profile__facts { grid-template-columns: repeat(4, minmax(0, 1fr)); } }
</style>
