<script setup lang="ts">
import { computed, shallowRef } from 'vue'
import type { HourlySquad } from '@/api/hourly'
import InlineNotice from '@/components/common/InlineNotice.vue'
import OperationStatusNotice from '@/components/common/OperationStatusNotice.vue'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'
import SkeletonBlock from '@/components/common/SkeletonBlock.vue'
import SquadActivationDialog from '@/components/catalog/SquadActivationDialog.vue'
import { localizedError, useI18n } from '@/i18n'
import { txbInputFromMinor } from '@/utils/format'
import HourlySummary from './HourlySummary.vue'
import HourlySquadCard from './HourlySquadCard.vue'
import { useHourly } from './useHourly'

const { t } = useI18n()
const hourly = useHourly()
const { value: data, loading, error, busy, actionError } = hourly
const disabling = shallowRef<HourlySquad | null>(null)
const activating = shallowRef<HourlySquad | null>(null)
const codeOpen = shallowRef(false)
const extraOpen = shallowRef(false)
const disableOpen = computed({ get: () => disabling.value !== null, set: value => { if (!value) disabling.value = null } })
const cooldownHours = computed(() => String((data.value?.policy.policy.cooldownSeconds ?? 86400) / 3600))
const operationMessage = computed(() => {
  const receipt = hourly.operation.receipt.value
  if (!receipt) return ''
  if (receipt.status === 'succeeded') return t('hourly.applied')
  if (receipt.status === 'queued' || receipt.status === 'processing') return t('hourly.applicationPending')
  if (receipt.status !== 'failed') return t('hourly.applicationUnconfirmed')
  const key = `hourly.result.${receipt.errorCode ?? ''}`
  return t(key) !== key ? t(key) : localizedError({ code: receipt.errorCode }, 'hourly.applicationFailed')
})
async function activate(item: HourlySquad): Promise<void> {
  if (item.squad.activationRequired) { activating.value = item; codeOpen.value = true }
  else await hourly.activate(item)
}
async function submitCode(code: string): Promise<void> {
  if (activating.value && await hourly.activate(activating.value, code)) { codeOpen.value = false; activating.value = null }
}
async function disable(): Promise<void> {
  if (disabling.value && await hourly.disable(disabling.value)) disabling.value = null
}
async function extra(enabled: boolean): Promise<void> {
  if (enabled) extraOpen.value = true
  else await hourly.extra(false)
}
async function enableExtra(): Promise<void> { if (await hourly.extra(true)) extraOpen.value = false }
</script>

<template>
  <section class="hourly-page" :aria-label="t('hourly.title')">
    <header class="hourly-page__heading"><div><h2>{{ t('hourly.title') }}</h2><p>{{ t('hourly.intro') }}</p></div><UButton color="neutral" variant="ghost" icon="i-ph-arrow-clockwise" :aria-label="t('common.refresh')" :loading="hourly.refreshing.value" @click="hourly.refresh()" /></header>
    <div v-if="loading" aria-busy="true"><SkeletonBlock height="12rem" /><SkeletonBlock height="9rem" /></div>
    <InlineNotice v-if="error" tone="warning">{{ error }}<UButton variant="link" :label="t('common.tryAgain')" @click="hourly.refresh()" /></InlineNotice>
    <template v-if="data">
      <HourlySummary :data="data" :busy="busy" @extra="extra" />
      <InlineNotice v-if="data.legacyBlocked">{{ t('hourly.legacyBlocked') }}</InlineNotice>
      <InlineNotice v-else-if="!data.policy.policy.salesEnabled">{{ t('hourly.salesPaused') }}</InlineNotice>
      <p class="hourly-page__terms">{{ t('hourly.billingTerms', { hours: data.policy.policy.activationHours }) }}</p>
      <p v-if="!data.squads.length" class="hourly-page__terms">{{ t('hourly.empty') }}</p>
      <div v-else class="hourly-page__squads"><HourlySquadCard v-for="item in data.squads" :key="item.offer.groupId" :item="item" :busy="busy" :sales-enabled="data.policy.policy.salesEnabled" :legacy-blocked="data.legacyBlocked" :hours="data.policy.policy.activationHours" @activate="activate(item)" @disable="disabling = item" /></div>
    </template>
    <InlineNotice v-if="actionError" tone="warning">{{ actionError }}</InlineNotice>
    <OperationStatusNotice :receipt="hourly.operation.receipt.value" :error="hourly.operation.error.value" :checking="hourly.operation.checking.value" :message="operationMessage" @refresh="hourly.operation.refresh" />
    <SquadActivationDialog v-model:open="codeOpen" :squad="activating?.squad ?? null" :busy="hourly.submitting.value" :error="actionError" @submit="submitCode" @cancel="activating = null" />
    <ConfirmDialog v-model:open="disableOpen" :title="t('hourly.disableTitle')" :description="t('hourly.disableWarning', { hours: cooldownHours })" :confirm-label="t('hourly.disable')" :busy="hourly.submitting.value" :error="actionError" danger @confirm="disable" />
    <ConfirmDialog v-model:open="extraOpen" :title="t('hourly.extraTitle')" :description="t('hourly.extraWarning', { price: txbInputFromMinor(data?.policy.policy.extraGBPriceTXBMinor ?? '0') })" :confirm-label="t('hourly.extraConfirm')" :busy="hourly.submitting.value" :error="actionError" @confirm="enableExtra" />
  </section>
</template>

<style scoped>
.hourly-page { min-width: 0; }
.hourly-page__heading { display: flex; justify-content: space-between; align-items: flex-start; gap: .75rem; padding-top: .75rem; }
h2 { margin: 0; font-size: 1.15rem; font-weight: 650; } header p, .hourly-page__terms { margin: .4rem 0 .8rem; color: var(--text-muted); font-size: .85rem; line-height: 1.6; }
.hourly-page__squads { display: grid; gap: 0 1.75rem; }
@media (min-width: 760px) { .hourly-page__squads { grid-template-columns: repeat(2, minmax(0, 1fr)); } }
</style>
