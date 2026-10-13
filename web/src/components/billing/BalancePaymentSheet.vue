<script setup lang="ts">
import { computed, shallowRef, watch } from 'vue'
import type { CouponRedemption, FeaturePaymentMethod, FeaturePaymentOrder } from '@/api/features'
import BalancePaymentConfiguration from '@/components/billing/BalancePaymentConfiguration.vue'
import CryptoPaymentInstructions from '@/components/billing/CryptoPaymentInstructions.vue'
import { usePaymentOrder } from '@/composables/usePaymentOrder'
import { useTelegramBackButton } from '@/composables/useTelegramBackButton'
import { useI18n } from '@/i18n'
import { formatDateTime, formatMoney } from '@/utils/format'

const props = defineProps<{
  methods: readonly FeaturePaymentMethod[]
  pendingOrder?: FeaturePaymentOrder | null
  reissueOrder?: FeaturePaymentOrder | null
  minimumMinor?: string
  maximumMinor?: string
}>()
const emit = defineEmits<{ paid: [] }>()
const open = defineModel<boolean>('open', { required: true })
const { t } = useI18n()

const methods = computed(() => props.methods)
const externalMethods = computed(() => methods.value.filter((method) => method.mode === 'order'))
const couponRedemption = shallowRef<CouponRedemption | null>(null)
const paymentStep = shallowRef<'provider' | 'channel'>('provider')

const {
  amount,
  selectedMethodId,
  stage,
  order,
  qrDataUrl,
  error,
  canReissue,
  canRecreate,
  amountValid,
  canCreate,
  reset,
  hydrateReissueOrder,
  hydratePendingOrder,
  chooseMethod,
  createOrder,
  cancelOrder,
  recreateOrder,
  retryOperation,
  openPaymentTarget,
  stopPolling,
} = usePaymentOrder({
  onPaid: () => {
    open.value = false
    emit('paid')
  },
  minimumMinor: () => props.minimumMinor ?? '100',
  maximumMinor: () => props.maximumMinor ?? '10000000000',
})

const description = computed(() => stage.value === 'configure'
  ? t('payment.configureHint')
  : stage.value === 'cancelled'
    ? t('payment.cancelledHint')
    : t('payment.providerHint'))

async function prepareOrder(): Promise<void> {
  couponRedemption.value = null
  paymentStep.value = 'provider'
  reset(externalMethods.value)
  if (props.pendingOrder && await hydratePendingOrder(props.pendingOrder, externalMethods.value)) return
  if (props.reissueOrder) hydrateReissueOrder(props.reissueOrder, externalMethods.value)
}

function closeSheet(): void {
  open.value = false
}

const showTelegramBack = computed(() => open.value)
useTelegramBackButton(showTelegramBack, closeSheet)

watch(open, (next, previous) => {
  if (next) void prepareOrder()
  else {
    stopPolling()
    if (previous && couponRedemption.value) emit('paid')
  }
})
</script>

<template>
  <UModal
    v-model:open="open"
    :title="$t('payment.addTxb')"
    :description="description"
    :dismissible="!['creating', 'pending', 'cancelling'].includes(stage)"
    :close="{ 'data-haptic': 'dismiss' }"
  >
    <template #actions>
      <UButton
        v-if="paymentStep === 'channel'"
        class="payment-sheet-header-back"
        color="neutral"
        variant="ghost"
        icon="i-ph-arrow-left"
        :aria-label="t('payment.backToProvider')"
        data-haptic="navigate"
        @click="paymentStep = 'provider'"
      />
    </template>

    <template #body>
      <template v-if="!couponRedemption && (stage === 'configure' || stage === 'creating')">
        <BalancePaymentConfiguration
          v-model:amount="amount"
          v-model:step="paymentStep"
          :methods="methods"
          :selected-method-id="selectedMethodId"
          :stage="stage"
          :error="error"
          :amount-valid="amountValid"
          :can-create="canCreate"
          :can-reissue="canReissue"
          :minimum-minor="minimumMinor ?? '100'"
          :maximum-minor="maximumMinor ?? '10000000000'"
          @choose-method="chooseMethod"
          @create-order="createOrder"
          @retry-operation="retryOperation"
          @coupon-redeemed="couponRedemption = $event"
        />
      </template>

      <template v-else-if="(stage === 'pending' || stage === 'cancelling') && order">
        <CryptoPaymentInstructions
          v-if="order.provider === 'bepusdt' || order.provider === 'builtin_usdt'"
          :order="order"
          :stage="stage"
          :qr-data-url="qrDataUrl"
          :error="error"
          :can-recreate="canRecreate"
          @cancel="cancelOrder"
          @recreate="recreateOrder"
          @retry="retryOperation"
        />
        <template v-else>
          <div class="payment-amount">
            <span>{{ $t('payment.exactAmount') }}</span>
            <strong>{{ order.payableAmount }} {{ order.payableCurrency }}</strong>
            <small>{{ $t('payment.creditsAfter', { amount: formatMoney(order.txb) }) }}</small>
          </div>
          <div v-if="qrDataUrl" class="qr-frame"><img :src="qrDataUrl" :alt="$t('payment.qrAlt')" width="260" height="260" /></div>
          <p class="payment-expiry">{{ $t('payment.expires', { date: formatDateTime(order.expiresAt) }) }}</p>
          <UButton v-if="order.paymentUrl" block color="neutral" variant="outline" trailing-icon="i-ph-arrow-square-out" :label="$t('payment.openPage')" data-haptic="open" @click="openPaymentTarget" />
          <div class="payment-waiting" role="status"><span class="payment-waiting__pulse" />{{ stage === 'cancelling' ? $t('payment.cancelling') : $t('payment.waiting') }}</div>
          <UButton v-if="stage === 'cancelling' && error" block color="neutral" variant="outline" icon="i-ph-arrows-clockwise" :label="$t('operations.checkStatus')" data-haptic="retry" @click="retryOperation" />
          <UButton block color="error" variant="ghost" :disabled="stage === 'cancelling'" :label="$t('payment.cancelOrder')" data-haptic="destructive" @click="cancelOrder" />
          <p class="field-hint">{{ $t('payment.callbackHint') }}</p>
          <UAlert v-if="error" color="error" variant="soft" :description="error" />
        </template>
      </template>

      <div v-else-if="couponRedemption" class="payment-success payment-success--coupon" role="status">
        <UIcon name="i-ph-check-circle-fill" class="payment-success__coupon-icon" aria-hidden="true" />
        <h2>{{ $t('payment.added') }}</h2>
        <div class="payment-amount">
          <span>{{ $t('payment.couponGain') }}</span>
          <strong>{{ formatMoney({ currency: 'TXB', minor: couponRedemption.balanceDeltaMinor, display: '' }) }}</strong>
          <small>{{ $t('payment.balanceAfter', { amount: formatMoney({ currency: 'TXB', minor: couponRedemption.balanceAfterMinor, display: '' }) }) }}</small>
        </div>
        <UButton block :label="$t('common.close')" data-haptic="dismiss" @click="closeSheet" />
      </div>
      <div v-else-if="stage === 'cancelled'" class="payment-success payment-success--cancelled" role="status">
        <UIcon name="i-ph-x-circle-fill" class="payment-success__error-icon" aria-hidden="true" />
        <h2>{{ $t('payment.cancelled') }}</h2>
        <p>{{ $t('payment.cancelledClose') }}</p>
        <UAlert v-if="error" color="warning" variant="soft" :description="error" />
      </div>
      <div v-else-if="stage === 'review'" class="payment-success payment-success--cancelled" role="status">
        <UIcon name="i-ph-warning-circle-fill" class="payment-success__error-icon" aria-hidden="true" />
        <h2>{{ $t('payment.reviewTitle') }}</h2>
        <p>{{ $t('payment.reviewDescription') }}</p>
        <UAlert v-if="error" color="warning" variant="soft" :description="error" />
        <UButton block color="neutral" variant="outline" :label="$t('common.close')" data-haptic="dismiss" @click="closeSheet" />
      </div>
      <div v-else class="payment-success" role="status"><UIcon name="i-ph-check-circle-fill" /><h2>{{ $t('payment.added') }}</h2><p>{{ $t('payment.ready') }}</p></div>
    </template>
  </UModal>
</template>

<style scoped>
.payment-sheet-header-back { order: -1; margin-inline-start: -0.65rem; }
.payment-success--coupon { gap: 0.8rem; }
.payment-success--coupon > .payment-amount { width: 100%; }
.payment-success__coupon-icon { width: 5rem; height: 5rem; color: var(--success); font-size: 5rem; }
.payment-success__error-icon { width: 5rem; height: 5rem; color: var(--danger); font-size: 5rem; }
</style>
