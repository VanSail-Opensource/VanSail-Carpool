<script setup lang="ts">
import { computed, watch } from 'vue'
import type { FeaturePaymentOrder } from '@/api/features'
import { useClipboard } from '@/composables/useClipboard'
import { useI18n } from '@/i18n'
import { formatMoney } from '@/utils/format'
import { paymentChannelLabel, paymentCurrencyLogo } from './paymentOptions'

const props = defineProps<{ order: FeaturePaymentOrder; qrDataUrl: string | null }>()
const { t } = useI18n()
const clipboard = useClipboard()
const amountClipboard = useClipboard()
const currencyLogo = computed(() => props.order.actualCryptoCurrency === 'USDT' || props.order.actualCryptoCurrency === 'USDC'
  ? paymentCurrencyLogo(props.order.actualCryptoCurrency) : undefined)
watch(() => [props.order.id, props.order.receivingAddress], clipboard.reset, { flush: 'sync' })
watch(() => [props.order.id, props.order.actualCryptoAmount, props.order.actualCryptoCurrency], amountClipboard.reset, { flush: 'sync' })
</script>

<template>
  <section class="crypto-transfer" :aria-label="t('cryptoPayment.transferDetails')">
    <div class="crypto-transfer__network">
      <img v-if="currencyLogo" :src="currencyLogo" alt="" width="24" height="24" />
      <strong>{{ paymentChannelLabel(order.providerRail, order.providerRail) }}</strong>
    </div>
    <div class="crypto-transfer__amount payment-amount">
      <span>{{ t('payment.exactAmount') }}</span>
      <strong>{{ order.actualCryptoAmount }} <span>{{ order.actualCryptoCurrency }}</span></strong>
      <UButton
        color="neutral" variant="soft" :disabled="!order.actualCryptoAmount"
        :icon="amountClipboard.copied.value ? 'i-ph-check-bold' : 'i-ph-copy'"
        :label="amountClipboard.copied.value ? t('common.copied') : t('cryptoPayment.copyAmount')"
        @click="order.actualCryptoAmount && amountClipboard.copy(order.actualCryptoAmount)"
      />
      <small>{{ t('payment.creditsAfter', { amount: formatMoney(order.txb) }) }}</small>
    </div>
    <div v-if="order.receivingAddress" class="crypto-transfer__destination">
      <div class="qr-frame crypto-transfer__qr" :aria-busy="!qrDataUrl">
        <img v-if="qrDataUrl" :src="qrDataUrl" :alt="t('payment.qrAlt')" width="200" height="200" />
        <USkeleton v-else class="crypto-instructions__qr-skeleton" />
      </div>
      <div class="crypto-transfer__address payment-address payment-address--copy">
        <span>{{ t('payment.receivingAddress') }}</span>
        <code>{{ order.receivingAddress }}</code>
        <UButton
          class="payment-address__copy-button" color="neutral" variant="ghost"
          :icon="clipboard.copied.value ? 'i-ph-check-bold' : 'i-ph-copy'"
          :aria-label="clipboard.copied.value ? t('common.copied') : t('cryptoPayment.copyAddress')"
          data-haptic="action" @click="order.receivingAddress && clipboard.copy(order.receivingAddress)"
        />
      </div>
    </div>
    <div class="crypto-transfer__notice">
      <UIcon name="i-ph-info" aria-hidden="true" />
      <div>
        <p>{{ t('cryptoPayment.networkHint') }}</p>
        <p v-if="order.provider === 'builtin_usdt'">{{ t('cryptoPayment.gasFeeHint') }}</p>
      </div>
    </div>
  </section>
</template>

<style scoped>
.crypto-transfer { display: grid; gap: 1.25rem; min-width: 0; }
.crypto-transfer__network { display: flex; align-items: center; justify-content: center; gap: 0.5rem; font-size: 0.8rem; }
.crypto-transfer__network strong { font-weight: 500; }
.crypto-transfer__amount { gap: 0.65rem; margin: 0; min-width: 0; }
.crypto-transfer__amount > strong { margin: 0; font-family: var(--font-sans); font-size: clamp(1.5rem, 6vw, 2.25rem); font-weight: 600; letter-spacing: -0.035em; overflow-wrap: anywhere; max-width: 100%; font-variant-numeric: tabular-nums; }
.crypto-transfer__amount > strong > span { color: var(--text-muted); font-size: 0.55em; letter-spacing: 0; }
.crypto-transfer__amount > small { color: var(--text-muted); }
.crypto-transfer__destination { display: grid; gap: 1rem; min-width: 0; }
.crypto-transfer__qr { width: min(100%, 208px); margin: 0 auto; padding: 0.65rem; border-radius: 12px; }
.crypto-transfer__address { background: transparent; border: 0; border-block: 1px solid var(--line); border-radius: 0; padding: 0.9rem 0; }
.crypto-transfer__address code { color: var(--text); font-size: 0.76rem; line-height: 1.6; }
.crypto-transfer__notice { display: flex; align-items: flex-start; gap: 0.55rem; color: var(--text-muted); font-size: 0.73rem; line-height: 1.6; }
.crypto-transfer__notice > span { flex: 0 0 auto; width: 1rem; height: 1rem; margin-top: 0.15rem; }
.crypto-transfer__notice p + p { margin-top: 0.35rem; }
</style>
