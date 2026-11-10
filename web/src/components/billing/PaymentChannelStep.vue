<script setup lang="ts">
import { computed } from 'vue'
import { AnimatePresence, motion } from 'motion-v'

import type { PaymentStage } from '@/composables/usePaymentOrder'
import TxbAmountField from '@/components/common/TxbAmountField.vue'
import { useI18n } from '@/i18n'
import { useMotionPreferences } from '@/composables/useMotionPreferences'
import { displayInputCurrency, displayInputFromTXBMinor, displayInputMinimumFromTXBMinor, displayInputToTXBMinor } from '@/utils/displayCurrency'
import { formatMoney, moneyFromTxbInput } from '@/utils/format'
import { selectionHaptic } from '@/utils/telegram'
import PaymentCryptoChannelPicker from './PaymentCryptoChannelPicker.vue'
import type { PaymentChannelOption } from './paymentOptions'

const props = defineProps<{
  channels: readonly PaymentChannelOption[]
  selectedMethodId: string | null
  amount: string
  amountMinor: string
  amountValid: boolean
  stage: PaymentStage
  error: string | null
  canCreate: boolean
  canReissue: boolean
  minimumMinor: string
  maximumMinor: string
}>()

const emit = defineEmits<{
  'update:amount': [value: string]
  chooseMethod: [id: string]
  createOrder: []
  retryOperation: []
}>()

const { t } = useI18n()
const { reducedMotion } = useMotionPreferences()
const isCrypto = computed(() => props.channels.some((channel) => channel.cryptoCurrency))
const inputCurrency = computed(displayInputCurrency)
const parseDisplayValue = computed(() => inputCurrency.value === 'TXB' ? moneyFromTxbInput : displayInputToTXBMinor)
const displayMinimum = computed(() => displayInputMinimumFromTXBMinor(props.minimumMinor))
const displayMaximum = computed(() => displayInputFromTXBMinor(props.maximumMinor))
const rangeHint = computed(() => t('payment.amountRange', {
  min: displayMinimum.value,
  max: displayMaximum.value,
  currency: inputCurrency.value,
}))
const creditedTxb = computed(() => /^\d+$/.test(props.amountMinor) && BigInt(props.amountMinor) > 0n
  ? formatMoney({ currency: 'TXB', minor: props.amountMinor, display: '' })
  : '')
const showCredited = computed(() => props.amountValid && Boolean(creditedTxb.value) && inputCurrency.value !== 'TXB')

function chooseMethod(id: string): void {
  if (id === props.selectedMethodId) return
  selectionHaptic()
  emit('chooseMethod', id)
}
</script>

<template>
  <div class="payment-channel-step">
    <TxbAmountField
      id="txb-amount"
      :model-value="amount"
      :label="t('payment.amount')"
      :hint="rangeHint"
      :min-minor="minimumMinor"
      :max-minor="maximumMinor"
      :suffix="inputCurrency"
      :display-value="displayInputFromTXBMinor"
      :display-minimum="displayInputMinimumFromTXBMinor"
      :parse-display-value="parseDisplayValue"
      slider
      required
      @update:model-value="emit('update:amount', String($event))"
    />
    <p v-if="showCredited" class="payment-credit">{{ t('payment.creditEstimate', { amount: creditedTxb }) }}</p>
    <UAlert v-if="!channels.length" color="warning" variant="soft" icon="i-ph-warning-circle" :description="t('payment.noChannel')" />
    <PaymentCryptoChannelPicker
      v-else-if="isCrypto"
      :channels="channels"
      :selected-method-id="selectedMethodId"
      @choose="chooseMethod"
    />
    <fieldset v-else class="provider-picker" role="radiogroup">
      <legend class="sr-only">{{ t('payment.chooseChannel') }}</legend>
      <UButton
        v-for="item in channels"
        :key="item.value"
        class="provider-option"
        :class="{ 'provider-option--selected': selectedMethodId === item.value }"
        color="neutral"
        variant="ghost"
        :disabled="item.disabled"
        :aria-pressed="selectedMethodId === item.value"
        @click="chooseMethod(item.value)"
      >
        <span class="provider-option__icon">
          <img v-if="item.logo" :src="item.logo" alt="" width="28" height="28" />
          <UIcon v-else name="i-ph-credit-card" aria-hidden="true" />
        </span>
        <span><strong>{{ item.label }}</strong><small v-if="item.description">{{ item.description }}</small></span>
        <AnimatePresence :initial="false">
          <motion.span
            v-if="selectedMethodId === item.value"
            :key="item.value"
            class="provider-option__check"
            :initial="reducedMotion ? { opacity: 0 } : { opacity: 0, scale: 0.84 }"
            :animate="{ opacity: 1, scale: 1 }"
            :exit="{ opacity: 0 }"
            :transition="{ duration: reducedMotion ? 0.08 : 0.14, ease: 'easeOut' }"
          >
            <UIcon name="i-ph-check-circle-fill" aria-hidden="true" />
          </motion.span>
        </AnimatePresence>
      </UButton>
    </fieldset>
    <UAlert v-if="error" color="error" variant="soft" :description="error" />
    <UButton
      v-if="stage === 'creating' && error"
      class="payment-submit"
      block
      color="neutral"
      variant="outline"
      icon="i-ph-arrows-clockwise"
      :label="t('operations.checkStatus')"
      data-haptic="retry"
      @click="emit('retryOperation')"
    />
    <UButton
      v-else
      class="payment-submit"
      data-test="payment-submit"
      block
      :disabled="!canCreate || stage === 'creating' || !amountValid"
      :loading="stage === 'creating'"
      :label="stage === 'creating' ? t('payment.creating') : canReissue ? t('payment.reissue') : t('payment.proceedToPayment')"
      data-haptic="confirm"
      @click="emit('createOrder')"
    />
  </div>
</template>

<style scoped>
.payment-channel-step { display: grid; min-width: 0; gap: 0.75rem; }
.payment-channel-step :deep(.amount-field), .payment-channel-step .provider-picker, .payment-channel-step .payment-submit { margin: 0; }
.payment-credit { margin: -0.35rem 0 0; color: var(--text-muted); font-size: 0.78rem; }
</style>
