<script setup lang="ts">
import { computed } from 'vue'

import type { FeaturePaymentOrder, FeaturePaymentReturnStatus } from '@/api/features'
import type { PaymentReturnState } from '@/composables/usePaymentReturn'
import { useI18n } from '@/i18n'
import { formatDateTime, formatMoney } from '@/utils/format'
import { paymentChannelLabel } from './paymentOptions'

type PaymentReceipt = FeaturePaymentOrder | FeaturePaymentReturnStatus
type ReceiptColor = 'neutral' | 'success' | 'warning' | 'error'

const props = defineProps<{
  title: string
  description: string
  state: PaymentReturnState
  details: PaymentReceipt | null
  canReissue: boolean
  ownerLink?: string
}>()

const emit = defineEmits<{
  reissue: []
}>()

const { t } = useI18n()
const status = computed(() => props.details?.status ?? null)
const stateColors: Record<PaymentReturnState, ReceiptColor> = {
  checking: 'warning',
  pending: 'warning',
  confirmed: 'success',
  terminal: 'error',
  missing: 'neutral',
  unavailable: 'neutral',
}
const statusColor = computed<ReceiptColor>(() => stateColors[props.state])
const statusIcon = computed(() => ({
  checking: 'i-ph-spinner-gap',
  pending: 'i-ph-spinner-gap',
  confirmed: 'i-ph-check-circle-fill',
  terminal: 'i-ph-x-circle-fill',
  missing: 'i-ph-warning-circle',
  unavailable: 'i-ph-warning-circle',
}[props.state]))
const statusLabel = computed(() => status.value ? localized(`payment.status.${status.value}`, status.value) : props.title)
const balanceAmount = computed(() => props.details ? formatMoney(props.details.txb) : t('common.notAvailable'))
const balanceLabel = computed(() => t(status.value === 'paid' ? 'payment.balanceCredited' : 'payment.balanceAmount'))
const paymentAmount = computed(() => props.details ? `${props.details.payableAmount} ${props.details.payableCurrency}` : t('common.notAvailable'))
const cryptoAmount = computed(() => props.details?.actualCryptoAmount && props.details.actualCryptoCurrency
  ? `${props.details.actualCryptoAmount} ${props.details.actualCryptoCurrency}`
  : null)

const receiptFields = computed(() => {
  const details = props.details
  if (!details) return []
  return [
    { key: 'id', label: t('payment.paymentId'), value: details.id, icon: 'i-ph-hash', mono: true },
    { key: 'provider', label: t('payment.provider'), value: localized(`payment.providers.${details.provider}`, details.provider), icon: 'i-ph-storefront' },
    { key: 'channel', label: t('payment.channel'), value: paymentChannelLabel(details.providerRail, details.providerRail), icon: 'i-ph-arrows-left-right' },
    { key: 'paid', label: t('payment.successTime'), value: formatDateTime(details.paidAt ?? undefined), icon: 'i-ph-calendar-check' },
  ]
})

function localized(key: string, fallback: string): string {
  const value = t(key)
  return value === key ? fallback : value
}
</script>

<template>
  <main class="payment-receipt" aria-live="polite">
    <UContainer class="payment-receipt__container">
      <UCard class="payment-receipt__card" variant="outline" :ui="{ body: 'p-0 sm:p-0' }">
        <div class="payment-receipt__body">
          <header class="payment-receipt__header">
            <div class="payment-receipt__heading">
              <p class="payment-receipt__eyebrow">{{ $t('payment.receiptLabel') }}</p>
              <div class="payment-receipt__title-row">
                <h1>{{ title }}</h1>
                <UBadge :color="statusColor" variant="soft" :label="statusLabel" />
              </div>
            </div>
            <UAvatar :icon="statusIcon" size="xl" :color="statusColor" class="payment-receipt__avatar" />
          </header>

          <UAlert
            class="payment-receipt__alert"
            :color="statusColor"
            variant="soft"
            :icon="statusIcon"
            :description="description"
          />

          <section v-if="details || state === 'checking' || state === 'pending'" class="payment-receipt__amount" :aria-label="balanceLabel">
            <div>
              <span class="payment-receipt__label">{{ balanceLabel }}</span>
              <strong>{{ balanceAmount }}</strong>
              <span class="payment-receipt__secondary">{{ $t('payment.paymentAmountValue', { amount: paymentAmount }) }}</span>
            </div>
            <UIcon name="i-ph-wallet" class="payment-receipt__amount-icon" aria-hidden="true" />
          </section>

          <USeparator v-if="details || state === 'checking' || state === 'pending'" />

          <section v-if="details || state === 'checking' || state === 'pending'" class="payment-receipt__details" :aria-labelledby="'payment-details-heading'">
            <div class="payment-receipt__section-heading">
              <div>
                <p class="payment-receipt__eyebrow">{{ $t('payment.receiptLabel') }}</p>
                <h2 id="payment-details-heading">{{ $t('payment.detailsHeading') }}</h2>
              </div>
              <UIcon name="i-ph-receipt" class="payment-receipt__section-icon" aria-hidden="true" />
            </div>

            <dl v-if="receiptFields.length" class="payment-receipt__fields">
              <div v-for="field in receiptFields" :key="field.key" class="payment-receipt__field">
                <dt><UIcon :name="field.icon" aria-hidden="true" />{{ field.label }}</dt>
                <dd :class="{ 'payment-receipt__value--mono': field.mono }">{{ field.value }}</dd>
              </div>
            </dl>
            <div v-else class="payment-receipt__loading" aria-hidden="true">
              <USkeleton v-for="index in 5" :key="index" class="payment-receipt__skeleton" />
            </div>

            <div v-if="cryptoAmount" class="payment-receipt__settled">
              <span>{{ $t('payment.cryptoAmount') }}</span>
              <strong>{{ cryptoAmount }}</strong>
            </div>
          </section>

          <div v-if="canReissue || ownerLink" class="payment-receipt__actions">
            <UButton v-if="canReissue" block color="neutral" variant="outline" trailing-icon="i-ph-arrow-clockwise" :label="$t('payment.reissue')" data-haptic="action" @click="emit('reissue')" />
            <UButton v-if="ownerLink" block :to="ownerLink" external target="_blank" rel="noopener noreferrer" trailing-icon="i-ph-arrow-up-right-bold" :label="$t('browser.memberEntrance')" />
          </div>
        </div>
      </UCard>
    </UContainer>
  </main>
</template>

<style scoped>
.payment-receipt { min-height: 100dvh; display: grid; place-items: center; padding: calc(1.5rem + var(--tg-content-safe-area-inset-top, env(safe-area-inset-top))) 1rem calc(1.5rem + var(--tg-content-safe-area-inset-bottom, env(safe-area-inset-bottom))); background: var(--canvas); }
.payment-receipt__container { width: min(100%, 42rem); }
.payment-receipt__card { overflow: hidden; border-color: var(--line-strong); background: var(--surface); box-shadow: 0 24px 70px rgb(0 0 0 / 0.24); }
.payment-receipt__body { display: grid; gap: 1.25rem; padding: clamp(1.1rem, 4vw, 2rem); }
.payment-receipt__header, .payment-receipt__title-row, .payment-receipt__section-heading, .payment-receipt__settled { display: flex; align-items: center; justify-content: space-between; gap: 1rem; }
.payment-receipt__heading { min-width: 0; }
.payment-receipt__eyebrow { margin: 0 0 0.4rem; color: var(--accent); font-family: var(--font-mono); font-size: 0.66rem; font-weight: 700; letter-spacing: 0.12em; text-transform: uppercase; }
.payment-receipt__title-row { align-items: baseline; flex-wrap: wrap; }
.payment-receipt__title-row h1, .payment-receipt__section-heading h2 { margin: 0; color: var(--text); letter-spacing: 0; }
.payment-receipt__title-row h1 { font-size: 1.6rem; }
.payment-receipt__section-heading h2 { font-size: 1rem; }
.payment-receipt__avatar { flex: 0 0 auto; }
.payment-receipt__alert :deep([data-slot="description"]) { line-height: 1.5; }
.payment-receipt__amount { display: flex; align-items: center; justify-content: space-between; gap: 1rem; padding: 1.1rem; border: 1px solid var(--line); border-radius: var(--radius-panel); background: var(--accent-soft); }
.payment-receipt__amount > div { display: grid; gap: 0.35rem; min-width: 0; }
.payment-receipt__label, .payment-receipt__secondary, .payment-receipt__field dt, .payment-receipt__settled span { color: var(--text-muted); font-size: 0.72rem; }
.payment-receipt__amount strong { color: var(--accent-strong); font-family: var(--font-mono); font-size: 2rem; letter-spacing: 0; line-height: 1.1; overflow-wrap: anywhere; }
.payment-receipt__secondary { line-height: 1.4; overflow-wrap: anywhere; }
.payment-receipt__amount-icon, .payment-receipt__section-icon { color: var(--accent); font-size: 1.35rem; }
.payment-receipt__section-icon { color: var(--text-faint); }
.payment-receipt__fields { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 0.65rem; margin: 0; }
.payment-receipt__field { min-width: 0; display: grid; gap: 0.3rem; padding: 0.7rem; border: 1px solid var(--line); border-radius: var(--radius-control); background: var(--surface-raised); }
.payment-receipt__field dt { display: flex; align-items: center; gap: 0.35rem; }
.payment-receipt__field dt :deep(svg) { color: var(--text-faint); font-size: 0.85rem; }
.payment-receipt__field dd { margin: 0; color: var(--text); font-size: 0.78rem; font-weight: 700; line-height: 1.4; overflow-wrap: anywhere; }
.payment-receipt__value--mono { font-family: var(--font-mono); font-size: 0.7rem !important; }
.payment-receipt__settled { padding-top: 0.8rem; border-top: 1px solid var(--line); }
.payment-receipt__settled strong { color: var(--text); font-family: var(--font-mono); font-size: 0.8rem; text-align: end; overflow-wrap: anywhere; }
.payment-receipt__loading { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 0.65rem; }
.payment-receipt__skeleton { height: 3.5rem; border-radius: var(--radius-control); }
.payment-receipt__actions, .payment-receipt__refreshing { padding-top: 0.15rem; }
@media (max-width: 420px) { .payment-receipt__header { align-items: start; } .payment-receipt__fields, .payment-receipt__loading { grid-template-columns: 1fr; } }
</style>
