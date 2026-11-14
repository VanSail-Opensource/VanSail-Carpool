<script setup lang="ts">
import { computed, shallowRef, watch } from 'vue'
import type { HourlySquad } from '@/api/hourly'
import InlineNotice from '@/components/common/InlineNotice.vue'
import OperationStatusNotice from '@/components/common/OperationStatusNotice.vue'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'
import SkeletonBlock from '@/components/common/SkeletonBlock.vue'
import SquadActivationDialog from '@/components/catalog/SquadActivationDialog.vue'
import { localizedError, useI18n } from '@/i18n'
import { txbInputFromMinor } from '@/utils/format'
import { hourlyRequiredHours } from './hourlyHours'
import HourlyLegacyBlock from './HourlyLegacyBlock.vue'
import HourlySummary from './HourlySummary.vue'
import HourlySquadCard from './HourlySquadCard.vue'
import { useHourly } from './useHourly'

const { t } = useI18n()
const hourly = useHourly()
const { value: data, loading, error, liveReady, busy, actionError } = hourly
const disabling = shallowRef<HourlySquad | null>(null)
const activating = shallowRef<HourlySquad | null>(null)
const codeOpen = shallowRef(false)
const extraOpen = shallowRef(false)
const reviewedPolicyVersion = shallowRef<string | null>(null)
const consentError = shallowRef<string | null>(null)
const disableOpen = computed({ get: () => disabling.value !== null, set: value => { if (!value) disabling.value = null } })
const cooldownHours = computed(() => String((data.value?.policy.policy.cooldownSeconds ?? 86400) / 3600))
const hasHourlyHistory = computed(() => {
  const snapshot = data.value
  if (!snapshot) return false
  if (snapshot.squads.some(item => item.enrollment)) return true
  const usage = snapshot.usage
  const accounting = usage?.accounting
  const currentAccounting = accounting
    ? positive(accounting.openReservedBytes) || positive(accounting.recoveryUnknownBytes)
    : usage?.accountingPending === true
  return currentAccounting || positive(usage?.freeUsedBytes) || positive(usage?.extraFundedBytes) || positive(usage?.extraUsedBytes)
})
function positive(value: string | undefined): boolean {
  try {
    return BigInt(value ?? '0') > 0n
  } catch {
    return false
  }
}
function hoursFor(item: HourlySquad): number {
  return hourlyRequiredHours(item.offer, data.value?.policy.policy.activationHours ?? 0)
}
const operationMessage = computed(() => {
  const receipt = hourly.operation.receipt.value
  if (!receipt) return ''
  if (receipt.status === 'succeeded') return t('hourly.applied')
  if (receipt.status === 'queued' || receipt.status === 'processing') return t('hourly.applicationPending')
  if (receipt.status !== 'failed') return t('hourly.applicationUnconfirmed')
  const key = `hourly.result.${receipt.errorCode ?? ''}`
  return t(key) !== key ? t(key) : localizedError({ code: receipt.errorCode }, 'hourly.applicationFailed')
})
function beginConsent(): boolean {
  if (busy.value || !data.value || data.value.legacyBlocked) return false
  reviewedPolicyVersion.value = data.value.policy.version
  consentError.value = null
  return true
}
function consentCurrent(): boolean {
  const current = data.value
  if (!liveReady.value || !current || current.legacyBlocked || current.policy.version !== reviewedPolicyVersion.value) return false
  if (activating.value) {
    const target = current.squads.find(item => item.offer.groupId === activating.value?.offer.groupId)
    if (!target?.ready || !target.offer.enabled || !current.policy.policy.salesEnabled || target.offer.revision !== activating.value.offer.revision) return false
  }
  if (disabling.value) {
    const target = current.squads.find(item => item.offer.groupId === disabling.value?.offer.groupId)
    if (!target?.enrollment || target.enrollment.id !== disabling.value.enrollment?.id || !['active', 'preparing'].includes(target.enrollment.status)) return false
  }
  return true
}
// Refreshes cannot silently replace the policy, paid enrollment, or gated offer
// that the member reviewed before opening a destructive/billing dialog.
watch([data, liveReady], () => {
  if ((!codeOpen.value && !extraOpen.value && !disabling.value) || consentCurrent()) return
  codeOpen.value = false
  extraOpen.value = false
  activating.value = null
  disabling.value = null
  if (!hourly.submitting.value) consentError.value = t('hourly.conflict')
}, { flush: 'sync' })
async function activate(item: HourlySquad): Promise<void> {
  if (!beginConsent()) return
  if (item.squad.activationRequired) { activating.value = item; codeOpen.value = true }
  else await hourly.activate(item)
}
async function submitCode(code: string): Promise<void> {
  if (!codeOpen.value || !consentCurrent()) return
  if (activating.value && await hourly.activate(activating.value, code)) { codeOpen.value = false; activating.value = null }
}
function requestDisable(item: HourlySquad): void {
  if (beginConsent()) disabling.value = item
}
async function disable(): Promise<void> {
  if (!disabling.value || !consentCurrent()) return
  if (disabling.value && await hourly.disable(disabling.value)) disabling.value = null
}
async function extra(enabled: boolean): Promise<void> {
  if (!beginConsent()) return
  if (enabled) extraOpen.value = true
  else await hourly.extra(false)
}
async function enableExtra(): Promise<void> {
  if (!extraOpen.value || !consentCurrent()) return
  if (await hourly.extra(true)) extraOpen.value = false
}
</script>

<template>
  <section class="hourly-page" :aria-label="t('hourly.title')">
    <div v-if="loading" aria-busy="true"><SkeletonBlock height="12rem" /><SkeletonBlock height="9rem" /></div>
    <InlineNotice v-if="error" tone="warning">{{ error }}<UButton variant="link" :label="t('common.tryAgain')" @click="hourly.refresh()" /></InlineNotice>
    <p v-if="data && !liveReady && !error" class="hourly-page__sync" role="status"><UIcon name="i-ph-circle-notch" aria-hidden="true" />{{ t('common.loading') }}</p>
    <HourlyLegacyBlock v-if="data?.legacyBlocked" />
    <template v-else-if="data">
      <HourlySummary v-if="hasHourlyHistory" :data="data" :busy="busy" @extra="extra" />
      <InlineNotice v-if="!data.policy.policy.salesEnabled">{{ t('hourly.salesPaused') }}</InlineNotice>
      <p v-if="!data.squads.length" class="hourly-page__terms hourly-page__empty">{{ t('hourly.empty') }}</p>
      <div v-else class="hourly-page__squads">
        <HourlySquadCard v-for="(item, index) in data.squads" :key="item.offer.groupId" :style="{ '--entry-delay': `${Math.min(index, 5) * 30}ms` }" :item="item" :busy="busy" :sales-enabled="data.policy.policy.salesEnabled" :hours="hoursFor(item)" @activate="activate(item)" @disable="requestDisable(item)" />
      </div>
    </template>
    <InlineNotice v-if="actionError || consentError" tone="warning">{{ actionError || consentError }}</InlineNotice>
    <OperationStatusNotice :receipt="hourly.operation.receipt.value" :error="hourly.operation.error.value" :checking="hourly.operation.checking.value" :message="operationMessage" @refresh="hourly.operation.refresh" />
    <SquadActivationDialog v-model:open="codeOpen" :squad="activating?.squad ?? null" :busy="hourly.submitting.value" :error="actionError" @submit="submitCode" @cancel="activating = null" />
    <ConfirmDialog v-model:open="disableOpen" :title="t('hourly.disableTitle')" :description="t('hourly.disableWarning', { hours: cooldownHours })" :confirm-label="t('hourly.disable')" :busy="hourly.submitting.value" :error="actionError" danger @confirm="disable" />
    <ConfirmDialog v-model:open="extraOpen" :title="t('hourly.extraTitle')" :description="t('hourly.extraWarning', { price: txbInputFromMinor(data?.policy.policy.extraGBPriceTXBMinor ?? '0') })" :confirm-label="t('hourly.extraConfirm')" :busy="hourly.submitting.value" :error="actionError" @confirm="enableExtra" />
  </section>
</template>

<style scoped>
.hourly-page { display: grid; gap: 0.6rem; min-width: 0; }
.hourly-page__sync { display: flex; align-items: center; justify-content: flex-end; gap: .3rem; margin: 0; color: var(--text-faint); font-size: .7rem; }
.hourly-page__sync .iconify { animation: hourly-sync 1s linear infinite; }
.hourly-page__terms { margin: 0.4rem 0 0; color: var(--text-muted); font-size: 0.82rem; line-height: 1.6; }
.hourly-page__empty { padding-block: 1.5rem; text-align: center; }
.hourly-page__squads { display: grid; gap: .65rem; grid-template-columns: repeat(auto-fill, minmax(min(100%, 22rem), 1fr)); align-items: stretch; }
.hourly-page__squads > * { animation: hourly-component 220ms ease-out both; animation-delay: var(--entry-delay, 0ms); }
@media (min-width: 900px) { .hourly-page__squads { gap: .8rem; } }
@keyframes hourly-component { from { opacity: 0; transform: translateY(6px); } to { opacity: 1; transform: translateY(0); } }
@keyframes hourly-sync { to { transform: rotate(360deg); } }
@media (prefers-reduced-motion: reduce) { .hourly-page__squads > *, .hourly-page__sync .iconify { animation: none; } }
</style>
