<script setup lang="ts">
import { computed, shallowRef } from 'vue'
import { useI18n } from '@/i18n'
import InlineNotice from '@/components/common/InlineNotice.vue'
import type { AffiliateTier } from '@/api/features'
import { affiliateFormSchema } from './affiliateForm'
import AdminAffiliateTierEditor from './AdminAffiliateTierEditor.vue'
import { useAdminAffiliates } from './useAdminAffiliates'

const state = useAdminAffiliates()
const { t } = useI18n()
const selected = shallowRef<number | null>(null)
const columns = computed(() => [{ accessorKey: 'name', header: t('adminAffiliates.name') }, { accessorKey: 'threshold', header: t('adminAffiliates.threshold') }, { id: 'commission', header: t('adminAffiliates.commissionRate') }, { id: 'reward', header: t('adminAffiliates.rewardKind') }, { id: 'status', header: t('adminWorkspace.status') }, { id: 'actions', header: t('adminWorkspace.actions') }])
function update(index: number, tier: AffiliateTier): void { state.replace(state.tiers.value.map((item, itemIndex) => itemIndex === index ? tier : item)) }
function move(index: number, offset: number): void {
  const next = [...state.tiers.value]
  const target = index + offset
  if (target < 0 || target >= next.length) return
  ;[next[index], next[target]] = [next[target]!, next[index]!]
  state.replace(next)
  selected.value = target
}
function remove(index: number): void { state.replace(state.tiers.value.filter((_, itemIndex) => itemIndex !== index)); selected.value = null }
function add(): void {
  const last = state.tiers.value[state.tiers.value.length - 1]
  state.replace([...state.tiers.value, { id: '', name: '', threshold: (last?.threshold ?? 0) + 1, enabled: true, commissionEnabled: false, commissionBps: 0, reward: { kind: 'none' } }])
  selected.value = state.tiers.value.length - 1
}
</script>

<template>
  <section class="admin-panel">
    <div class="admin-panel__heading"><div><h2>{{ t('adminAffiliates.title') }}</h2><p>{{ t('adminAffiliates.copy') }}</p></div><div class="row-actions"><UButton color="neutral" variant="outline" icon="i-ph-plus" :label="t('adminAffiliates.addTier')" @click="add" /><UButton type="submit" form="admin-affiliates" icon="i-ph-floppy-disk" :loading="state.saving.value" :label="t('common.save')" /></div></div>
    <USkeleton v-if="state.loading.value" class="h-64 w-full" />
    <template v-else-if="state.configuration.value">
      <div class="admin-toolbar"><span>{{ t('adminAffiliates.botStatus') }} <UBadge color="neutral" variant="subtle" :label="t(`adminAffiliates.bot.${state.configuration.value.bot.status}`)" /></span><code>{{ state.configuration.value.bot.username ? `@${state.configuration.value.bot.username}` : t('adminAffiliates.notDiscovered') }}</code></div>
      <UForm id="admin-affiliates" :schema="affiliateFormSchema" :state="{ tiers: state.tiers.value }" @submit="state.save">
        <UTable class="admin-table" :data="state.tiers.value" :columns="columns">
          <template #commission-cell="{ row }">{{ row.original.commissionEnabled ? (row.original.commissionBps / 100).toFixed(2) + '%' : t('common.disabled') }}</template>
          <template #reward-cell="{ row }">{{ t(`adminAffiliates.rewardKinds.${row.original.reward.kind}`) }}</template>
          <template #status-cell="{ row }"><UBadge :color="row.original.enabled ? 'success' : 'neutral'" variant="subtle" :label="t(row.original.enabled ? 'common.enabled' : 'common.disabled')" /></template>
          <template #actions-cell="{ row }"><UButton color="neutral" variant="ghost" icon="i-ph-pencil-simple" :aria-label="t('adminWorkspace.details')" @click="selected = row.index" /></template>
        </UTable>
      </UForm>
      <USlideover :open="selected !== null" :title="t('adminAffiliates.title')" :description="t('adminAffiliates.copy')" @update:open="!$event && (selected = null)"><template #body><InlineNotice v-if="state.error.value" tone="warning">{{ state.error.value }}</InlineNotice><AdminAffiliateTierEditor v-if="selected !== null && state.tiers.value[selected]" :tier="state.tiers.value[selected]!" :coupons="state.coupons.value" :index="selected" :count="state.tiers.value.length" @update="update(selected!, $event)" @move="move(selected!, $event)" @remove="remove(selected!)" /></template><template #footer><UButton color="neutral" variant="outline" :label="t('common.close')" @click="selected = null" /><UButton type="submit" form="admin-affiliates" :loading="state.saving.value" :label="t('common.save')" /></template></USlideover>
    </template>
    <UAlert v-if="state.error.value" color="warning" variant="soft" icon="i-ph-warning" :description="state.error.value" />
  </section>
</template>
