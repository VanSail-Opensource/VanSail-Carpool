<script setup lang="ts">
import { computed, shallowRef } from 'vue'

import type { NodeCompensationEvent } from '@/api/contracts/compensation'
import InlineNotice from '@/components/common/InlineNotice.vue'
import { useNodeCompensation } from '@/composables/useNodeCompensation'
import { useI18n } from '@/i18n'
import { formatDateTime } from '@/utils/format'
import { durationParts, multiplierFactor } from './compensation/format'
import AdminSectionState from './AdminSectionState.vue'
import CompensationConfigCard from './compensation/CompensationConfigCard.vue'
import CompensationReviewModal from './compensation/CompensationReviewModal.vue'
import {
  compensationStatusChoices,
  fromCompensationStatus,
  toCompensationStatus,
  type CompensationStatusChoice,
} from './compensation/statusFilter'

const { t } = useI18n()
const state = useNodeCompensation()
const workspace = shallowRef('events')
const tabs = computed(() => [{ value: 'events', label: t('adminWorkspace.records') }, { value: 'config', label: t('adminCompensation.configTitle') }])
const columns = computed(() => [{ id: 'node', header: t('adminWorkspace.name') }, { id: 'time', header: t('adminWorkspace.date') }, { id: 'duration', header: t('adminWorkspace.duration') }, { accessorKey: 'thresholdMinutes', header: t('adminCompensation.thresholdShort') }, { id: 'multiplier', header: t('adminCompensation.multiplierShort') }, { accessorKey: 'frozenRecipientCount', header: t('adminCompensation.recipientsShort') }, { id: 'status', header: t('adminWorkspace.status') }, { id: 'actions', header: t('adminWorkspace.actions') }])
const reviewOpen = shallowRef(false)
const selected = shallowRef<NodeCompensationEvent | null>(null)
const statusChoice = computed<CompensationStatusChoice>({
  get: () => fromCompensationStatus(state.status.value),
  set: value => void state.changeStatus(toCompensationStatus(value)),
})
const statusItems = computed(() => compensationStatusChoices.map(value => ({
  value, label: t(value === 'all' ? 'adminCompensation.allStatuses' : `adminCompensation.status.${value}`),
})))

function squadNames(event: NodeCompensationEvent): string { return event.squads.map(squad => squad.name).join(', ') }

function openReview(event: NodeCompensationEvent): void {
  selected.value = event
  reviewOpen.value = true
}

async function review(action: 'approve' | 'dismiss', minutes: number, reason: string): Promise<void> {
  if (!selected.value) return
  if (await state.review(selected.value, action, minutes, reason)) reviewOpen.value = false
}
</script>

<template>
  <section class="admin-panel">
    <div class="admin-panel__heading"><div><h2>{{ t('adminCompensation.title') }}</h2><p>{{ t('adminCompensation.copy') }}</p></div></div>
    <UTabs v-model="workspace" :items="tabs" :content="false" variant="link" class="admin-workspace-tabs" />
    <AdminSectionState :loading="state.loading.value" :error="state.error.value" @retry="state.load">
      <CompensationConfigCard v-if="state.config.value" v-show="workspace === 'config'" :config="state.config.value" :busy="state.busy.value" @save="state.saveConfig" />
      <div v-show="workspace === 'events'">
        <div class="admin-toolbar"><USelect v-model="statusChoice" :items="statusItems" :aria-label="t('adminCompensation.filter')" /></div>
        <InlineNotice v-if="state.error.value" tone="warning">{{ state.error.value }}</InlineNotice>
        <UTable class="admin-table" :data="state.events.value" :columns="columns">
          <template #node-cell="{ row }"><div class="admin-cell"><strong>{{ row.original.nodeName }}</strong><small>{{ squadNames(row.original) }}</small></div></template>
          <template #time-cell="{ row }">{{ formatDateTime(row.original.offlineObservedAt) }}</template>
          <template #duration-cell="{ row }">{{ row.original.observedDurationSeconds === null ? t('adminCompensation.ongoing') : t('adminCompensation.duration', durationParts(row.original.observedDurationSeconds)) }}</template>
          <template #multiplier-cell="{ row }">{{ multiplierFactor(row.original.multiplierBps) }}</template>
          <template #status-cell="{ row }"><div class="admin-cell"><UBadge :color="row.original.status === 'pending_review' ? 'warning' : 'neutral'" variant="subtle" :label="t(`adminCompensation.status.${row.original.status}`)" /><small v-if="row.original.ineligibleReason">{{ t(`adminCompensation.reason.${row.original.ineligibleReason}`) }}</small><small v-else-if="row.original.operation">{{ t('adminCompensation.operation', { status: row.original.operation.status }) }}</small></div></template>
          <template #actions-cell="{ row }"><UButton v-if="row.original.status === 'pending_review'" color="warning" variant="ghost" icon="i-ph-magnifying-glass" :label="t('adminCompensation.review')" @click="openReview(row.original)" /></template>
          <template #empty>{{ t('adminCompensation.none') }}</template>
        </UTable>
        <div v-if="state.nextCursor.value" class="admin-pagination"><UButton color="neutral" variant="outline" :label="t('adminCompensation.loadMore')" :loading="state.busy.value" @click="state.loadMore" /></div>
      </div>
    </AdminSectionState>
    <CompensationReviewModal v-model:open="reviewOpen" :event="selected" :busy="state.busy.value" :error="state.error.value" @review="review" />
  </section>
</template>
