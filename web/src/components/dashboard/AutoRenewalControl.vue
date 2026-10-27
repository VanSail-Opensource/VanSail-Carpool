<script setup lang="ts">
import { computed, shallowRef, watch } from 'vue'
import { AnimatePresence, motion } from 'motion-v'

import { pricingApi } from '@/api/pricing'
import type { PricingRenewalInfo } from '@/api/pricing'
import type { Purchase } from '@/api/types'
import InlineNotice from '@/components/common/InlineNotice.vue'
import { useAutoRenewal } from '@/composables/useAutoRenewal'
import { useMotionPreferences } from '@/composables/useMotionPreferences'
import { t } from '@/i18n'
import { formatDate } from '@/utils/format'
import { formatMemberMoney } from '@/utils/displayCurrency'
import { selectionHaptic } from '@/utils/telegram'

const props = defineProps<{ purchase: Purchase }>()
const emit = defineEmits<{ changed: [] }>()

const open = shallowRef(false)
const pricingInfo = shallowRef<PricingRenewalInfo | null>(null)
const pricingState = computed(() => !pricingInfo.value?.quote ? 'calculating' : Date.parse(pricingInfo.value.quote.validUntil) <= Date.now() ? 'stale' : 'ready')
const switchValue = shallowRef(props.purchase.autoRenewEnabled)
const { renewal, loading, updating, error, enabled, canEnable, ineligibleReason, reset, load, setEnabled } = useAutoRenewal(
  () => props.purchase.id,
  () => props.purchase.autoRenewEnabled,
)

const actionColor = computed(() => enabled.value ? 'success' : 'error')
const actionLabel = computed(() => t(enabled.value ? 'home.autoRenewalEnabledAction' : 'home.autoRenewalDisabledAction'))
const eligibilityMessage = computed(() => localizedReason(ineligibleReason.value, 'home.autoRenewalUnavailable'))
const showSwitch = computed(() => canEnable.value || enabled.value)
const switchState = computed(() => updating.value ? 'updating' : error.value ? `error:${error.value}` : enabled.value ? 'enabled' : 'disabled')
const { reducedMotion } = useMotionPreferences()

watch(open, (next) => {
  if (next) {
    void load()
    const id = props.purchase.id
    void pricingApi.renewalInfo(id).then(info => { if (props.purchase.id === id) pricingInfo.value = info }).catch(() => { if (props.purchase.id === id) pricingInfo.value = null })
  }
})
watch(renewal, (next) => {
  if (next) switchValue.value = next.enabled
})
watch(() => props.purchase.id, () => {
  reset()
  pricingInfo.value = null
  switchValue.value = props.purchase.autoRenewEnabled
  if (open.value) void load()
})

function localizedReason(reason: string | null, fallback: string): string {
  if (!reason) return t(fallback)
  const key = `home.autoRenewalReason.${reason}`
  const localized = t(key)
  return localized === key ? t(fallback) : localized
}

async function updateRenewal(next: boolean): Promise<void> {
  const previous = enabled.value
  if (next && !canEnable.value) {
    switchValue.value = previous
    return
  }
  if (next === previous) return
  switchValue.value = next
  selectionHaptic()
  if (await setEnabled(next)) emit('changed')
  else switchValue.value = previous
}
</script>

<template>
  <div class="auto-renewal-control">
    <UButton block class="auto-renewal-control__action" :color="actionColor" variant="soft" :label="actionLabel" data-haptic="open" @click="open = true" />
    <UModal v-model:open="open" :title="$t('home.autoRenewalTitle')" :description="$t('home.autoRenewalHint')" :close="{ 'data-haptic': 'dismiss' }">
      <template #body>
        <div class="auto-renewal-control__dialog">
          <USkeleton v-if="loading" class="h-36" />
          <template v-else-if="renewal">
            <dl class="auto-renewal-control__quote">
              <div><dt>{{ $t('home.autoRenewalGross') }}</dt><dd>{{ formatMemberMoney(renewal.grossPrice) }}</dd></div>
              <div><dt>{{ $t('home.autoRenewalDiscount') }}</dt><dd>{{ formatMemberMoney(renewal.discount) }}</dd></div>
              <div><dt>{{ $t('home.autoRenewalPrice') }} <small v-if="pricingInfo?.policy.dynamicEnabled && pricingInfo.policy.renewalPricingMode === 'DYNAMIC'">{{ $t(`pricing.priceStates.${pricingState}`) }}</small></dt><dd>{{ pricingInfo?.policy.dynamicEnabled && pricingInfo.policy.renewalPricingMode === 'DYNAMIC' && pricingState !== 'ready' ? $t('common.notAvailable') : formatMemberMoney(renewal.netPrice) }}</dd></div>
              <div><dt>{{ $t('home.autoRenewalChargeDate') }}</dt><dd>{{ formatDate(renewal.scheduledAt) }}</dd></div>
              <div><dt>{{ $t('home.autoRenewalNextCycleDate') }}</dt><dd>{{ formatDate(renewal.nextCycleEndsAt) }}</dd></div>
            </dl>
            <AnimatePresence mode="wait" :initial="false">
              <motion.div :key="switchState" :initial="reducedMotion ? { opacity: 0 } : { opacity: 0, y: 3 }" :animate="{ opacity: 1, y: 0 }" :exit="{ opacity: 0 }" :transition="{ duration: reducedMotion ? 0.08 : 0.16, ease: 'easeOut' }">
                <USwitch v-if="showSwitch" v-model="switchValue" :color="enabled ? 'success' : 'error'" :label="$t('home.autoRenewalSwitch')" :loading="updating" :disabled="updating" @update:model-value="updateRenewal" />
                <InlineNotice v-else tone="warning">{{ eligibilityMessage }}</InlineNotice>
              </motion.div>
            </AnimatePresence>
          </template>
          <InlineNotice v-else tone="warning">{{ error ?? $t('errors.autoRenewalFailed') }}</InlineNotice>
          <InlineNotice v-if="error && renewal" tone="warning">{{ error }}</InlineNotice>
        </div>
      </template>
    </UModal>
  </div>
</template>

<style scoped>
.auto-renewal-control, .auto-renewal-control__dialog { display: grid; gap: 0.75rem; }
.auto-renewal-control__action { justify-content: center; }
.auto-renewal-control__quote { display: grid; gap: 0.45rem; margin: 0; }
.auto-renewal-control__quote > div { display: flex; align-items: baseline; justify-content: space-between; gap: 1rem; padding-bottom: 0.45rem; border-bottom: 1px solid var(--line); }
.auto-renewal-control__quote dt { color: var(--text-faint); font-size: 0.7rem; }
.auto-renewal-control__quote dd { margin: 0; font-family: var(--font-mono); font-size: 0.78rem; font-weight: 700; text-align: right; }
</style>
