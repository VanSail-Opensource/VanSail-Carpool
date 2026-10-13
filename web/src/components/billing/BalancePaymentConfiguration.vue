<script setup lang="ts">
import { computed, shallowRef, watch } from 'vue'

import type { CouponRedemption, FeaturePaymentMethod } from '@/api/features'
import { featuresApi } from '@/api/features'
import type { PaymentProvider } from '@/api/types'
import type { PaymentStage } from '@/composables/usePaymentOrder'
import { localizedError, useI18n } from '@/i18n'
import { createUuid } from '@/utils/browserCompatibility'
import PaymentChannelStep from './PaymentChannelStep.vue'
import PaymentProviderStep from './PaymentProviderStep.vue'
import { paymentChannelLabel, paymentChannelLogo, type PaymentChannelOption, type PaymentProviderOption } from './paymentOptions'

const props = withDefaults(defineProps<{
  methods: readonly FeaturePaymentMethod[]
  selectedMethodId: string | null
  stage: PaymentStage
  error: string | null
  amountValid: boolean
  canCreate: boolean
  canReissue: boolean
  minimumMinor?: string
  maximumMinor?: string
}>(), {
  minimumMinor: '100',
  maximumMinor: '10000000000',
})
const emit = defineEmits<{
  chooseMethod: [id: string]
  createOrder: []
  retryOperation: []
  couponRedeemed: [redemption: CouponRedemption]
}>()
const amount = defineModel<string>('amount', { required: true })
const paymentStep = defineModel<'provider' | 'channel'>('step', { default: 'provider' })
const { t } = useI18n()
const couponCode = shallowRef('')
const couponBusy = shallowRef(false)
const couponError = shallowRef<string | null>(null)

const externalMethods = computed(() => props.methods.filter((method) => method.mode === 'order'))
const selectedMethod = computed(() => props.methods.find((method) => method.id === props.selectedMethodId) ?? null)
const selectedProvider = computed<PaymentProvider | null>(() => selectedMethod.value?.provider ?? null)
const selectedProfileKey = computed(() => selectedProvider.value === 'coupon'
  ? 'coupon'
  : selectedMethod.value ? profileKey(selectedMethod.value) : undefined)
const providerItems = computed<PaymentProviderOption[]>(() => {
  const items: PaymentProviderOption[] = []
  const seen = new Set<string>()
  for (const method of externalMethods.value) {
    const value = profileKey(method)
    if (seen.has(value)) continue
    seen.add(value)
    const profileMethods = externalMethods.value.filter((candidate) => profileKey(candidate) === value)
    const available = profileMethods.some((candidate) => candidate.available)
    items.push({
      label: method.providerName || t(`payment.providers.${method.provider}`),
      value,
      description: available ? '' : t('payment.rateUnavailable'),
      icon: providerIcon(method.provider),
      available,
    })
  }
  const coupon = props.methods.find((method) => method.provider === 'coupon')
  if (coupon) {
    items.push({
      label: t('payment.providers.coupon'),
      value: 'coupon',
      description: coupon.available ? '' : t('payment.rateUnavailable'),
      icon: providerIcon('coupon'),
      available: coupon.available,
    })
  }
  return items
})
const channels = computed(() => externalMethods.value.filter((method) => profileKey(method) === selectedProfileKey.value))
const channelItems = computed<PaymentChannelOption[]>(() => channels.value.map((method) => ({
  label: paymentChannelLabel(method.rail, method.name),
  value: method.id,
  description: method.available ? '' : methodNote(method),
  disabled: !method.available,
  logo: paymentChannelLogo(method.provider, method.rail),
  cryptoCurrency: method.cryptoCurrency || undefined,
  network: method.network || undefined,
  networkName: method.networkName || undefined,
})))
const canContinue = computed(() => selectedProvider.value !== null && selectedProvider.value !== 'coupon' && channels.value.some((method) => method.available))

function profileKey(method: FeaturePaymentMethod): string {
  return `${method.provider}:${method.profileId || 'legacy'}`
}

function providerIcon(provider: PaymentProvider): string {
  if (provider === 'ezpay') return 'i-ph-credit-card'
  if (provider === 'bepusdt' || provider === 'builtin_usdt') return 'i-ph-currency-circle-dollar'
  if (provider === 'stars') return 'i-ph-telegram-logo'
  return 'i-ph-ticket'
}

function chooseProfile(key: string | undefined): void {
  if (!key) return
  if (key === 'coupon') {
    emit('chooseMethod', 'coupon')
    couponError.value = null
    return
  }
  const first = externalMethods.value.find((method) => profileKey(method) === key && method.available)
    ?? externalMethods.value.find((method) => profileKey(method) === key)
  if (first) {
    emit('chooseMethod', first.id)
    couponError.value = null
  }
}

function continueToChannel(): void {
  if (canContinue.value) paymentStep.value = 'channel'
}

function methodNote(method: FeaturePaymentMethod): string {
  return method.available ? '' : t('payment.rateUnavailable')
}

async function redeemCoupon(): Promise<void> {
  if (!couponCode.value.trim() || couponBusy.value) return
  couponBusy.value = true
  couponError.value = null
  try {
    const redemption = await featuresApi.redeemCoupon(couponCode.value, createUuid())
    emit('couponRedeemed', redemption)
  } catch (caught) {
    couponError.value = localizedError(caught, 'errors.couponRedeem')
  } finally {
    couponBusy.value = false
  }
}

// Preserve the selected channel across create progress, remounts, and rejected creates.
watch(() => props.stage, (stage, previousStage) => {
  if (stage === 'creating') {
    paymentStep.value = 'channel'
    return
  }
  if (stage === 'configure' && previousStage !== 'creating') paymentStep.value = 'provider'
}, { immediate: true })
</script>

<template>
  <PaymentProviderStep
    v-if="paymentStep === 'provider'"
    :options="providerItems"
    :selected-value="selectedProfileKey"
    :can-continue="canContinue"
    @choose="chooseProfile"
    @continue="continueToChannel"
  />
  <div v-if="paymentStep === 'provider' && selectedProvider === 'coupon'" class="coupon-redemption">
    <div class="coupon-redemption__form">
      <UInput v-model="couponCode" :placeholder="$t('payment.couponCode')" :aria-label="$t('payment.couponCode')" />
      <UButton block :disabled="!couponCode.trim() || couponBusy" :loading="couponBusy" :label="$t('payment.redeemCoupon')" data-haptic="action" @click="redeemCoupon" />
    </div>
    <UAlert v-if="couponError" color="error" variant="soft" :description="couponError" />
  </div>
  <PaymentChannelStep
    v-else-if="paymentStep === 'channel'"
    v-model:amount="amount"
    :channels="channelItems"
    :selected-method-id="selectedMethodId"
    :amount-valid="amountValid"
    :stage="stage"
    :error="error"
    :can-create="canCreate"
    :can-reissue="canReissue"
    :minimum-minor="minimumMinor"
    :maximum-minor="maximumMinor"
    @choose-method="emit('chooseMethod', $event)"
    @create-order="emit('createOrder')"
    @retry-operation="emit('retryOperation')"
  />
</template>
