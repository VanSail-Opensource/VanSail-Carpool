<script setup lang="ts">
import type { FeaturePaymentOrder } from '@/api/features'
import type { PaymentStage } from '@/composables/usePaymentOrder'
import { usePaymentCountdown } from '@/composables/usePaymentCountdown'
import { useI18n } from '@/i18n'
import CryptoPaymentTransfer from './CryptoPaymentTransfer.vue'

const props = defineProps<{
  order: FeaturePaymentOrder
  stage: PaymentStage
  qrDataUrl: string | null
  error: string | null
  canRecreate: boolean
}>()
const emit = defineEmits<{ cancel: []; recreate: []; retry: [] }>()
const { t } = useI18n()
const countdown = usePaymentCountdown(() => props.order.expiresAt)
</script>

<template>
  <div class="crypto-instructions">
    <CryptoPaymentTransfer :order="order" :qr-data-url="qrDataUrl" />

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
      :disabled="stage !== 'pending'"
      :loading="stage === 'creating'"
      data-haptic="confirm"
      @click="emit('recreate')"
    />
    <div v-else-if="!countdown.expired.value" class="payment-waiting" role="status">
      <UIcon name="i-ph-clock" aria-hidden="true" />
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

<style scoped>
.crypto-instructions { gap: 0.9rem; }
.crypto-countdown { padding-inline: 0; border-top: 0; }
.payment-waiting { display: flex; align-items: center; justify-content: center; gap: 0.5rem; }
</style>
