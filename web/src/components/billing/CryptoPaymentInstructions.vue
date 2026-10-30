<script setup lang="ts">
import type { FeaturePaymentOrder } from '@/api/features'
import type { PaymentStage } from '@/composables/usePaymentOrder'
import { useClipboard } from '@/composables/useClipboard'
import { usePaymentCountdown } from '@/composables/usePaymentCountdown'
import { useI18n } from '@/i18n'
import { formatMoney } from '@/utils/format'
import { paymentChannelLabel } from './paymentOptions'

const props = defineProps<{
  order: FeaturePaymentOrder
  stage: PaymentStage
  qrDataUrl: string | null
  error: string | null
  canRecreate: boolean
}>()
const emit = defineEmits<{ cancel: []; recreate: []; retry: [] }>()
const { t } = useI18n()
const clipboard = useClipboard()
const amountClipboard = useClipboard()
const countdown = usePaymentCountdown(() => props.order.expiresAt)

function copyAddress(): void {
  if (props.order.receivingAddress) void clipboard.copy(props.order.receivingAddress)
}
</script>

<template>
  <div class="crypto-instructions">
    <div class="payment-amount">
      <span>{{ t('payment.exactAmount') }}</span>
      <strong>{{ order.actualCryptoAmount }} {{ order.actualCryptoCurrency }}</strong>
      <small v-if="order.provider === 'builtin_usdt'">{{ t('cryptoPayment.gasFeeHint') }}</small>
      <UButton color="neutral" variant="ghost" :icon="amountClipboard.copied.value ? 'i-ph-check-bold' : 'i-ph-copy'" :label="t('cryptoPayment.copyAmount')" @click="order.actualCryptoAmount && amountClipboard.copy(order.actualCryptoAmount)" />
      <small>{{ paymentChannelLabel(order.providerRail, order.providerRail) }}</small>
      <small>{{ t('cryptoPayment.networkHint') }}</small>
      <small>{{ t('payment.creditsAfter', { amount: formatMoney(order.txb) }) }}</small>
    </div>

    <div v-if="order.receivingAddress" class="qr-frame">
      <img v-if="qrDataUrl" :src="qrDataUrl" :alt="t('payment.qrAlt')" width="260" height="260" />
      <USkeleton v-else class="crypto-instructions__qr-skeleton" />
    </div>

    <div v-if="order.receivingAddress" class="payment-address payment-address--copy">
      <span>{{ t('payment.receivingAddress') }}</span>
      <code>{{ order.receivingAddress }}</code>
      <UTooltip :text="clipboard.copied.value ? t('common.copied') : t('cryptoPayment.copyAddress')">
        <UButton
          class="payment-address__copy-button"
          color="neutral"
          variant="ghost"
          :icon="clipboard.copied.value ? 'i-ph-check-bold' : 'i-ph-copy'"
          :aria-label="clipboard.copied.value ? t('common.copied') : t('cryptoPayment.copyAddress')"
          data-haptic="action"
          @click="copyAddress"
        />
      </UTooltip>
    </div>

    <div class="crypto-countdown" :class="{ 'crypto-countdown--expired': countdown.expired.value }" role="timer">
      <span>{{ countdown.expired.value ? t('cryptoPayment.expired') : t('cryptoPayment.timeRemaining') }}</span>
      <time :datetime="order.expiresAt">{{ countdown.countdown.value }}</time>
    </div>

    <UAlert
      v-if="countdown.expired.value"
      color="warning"
      variant="soft"
      icon="i-ph-clock-countdown"
      :title="t('cryptoPayment.expiredTitle')"
      :description="t('cryptoPayment.expiredDescription')"
    />
    <UButton
      v-if="countdown.expired.value && canRecreate"
      block
      icon="i-ph-arrows-clockwise"
      :label="t('cryptoPayment.recreate')"
      :loading="stage === 'creating'"
      data-haptic="confirm"
      @click="emit('recreate')"
    />
    <div v-else class="payment-waiting" role="status">
      <span class="payment-waiting__pulse" />
      {{ stage === 'cancelling' ? t('payment.cancelling') : t('payment.waiting') }}
    </div>
    <UButton
      v-if="stage === 'cancelling' && error"
      block
      color="neutral"
      variant="outline"
      icon="i-ph-arrows-clockwise"
      :label="t('operations.checkStatus')"
      data-haptic="retry"
      @click="emit('retry')"
    />
    <UButton
      block
      color="error"
      variant="ghost"
      icon="i-ph-x-circle"
      :disabled="stage === 'cancelling'"
      :label="t('payment.cancelOrder')"
      data-haptic="destructive"
      @click="emit('cancel')"
    />
    <p class="field-hint">{{ t(order.provider === 'builtin_usdt' ? 'cryptoPayment.builtinHint' : 'payment.callbackHint') }}</p>
    <UAlert v-if="error" color="error" variant="soft" :description="error" />
  </div>
</template>
