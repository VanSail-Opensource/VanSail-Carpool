<script setup lang="ts">
import { computed } from 'vue'
import type { FeaturePaymentOrder } from '@/api/features'
import type { PaymentConfirmationPhase } from '@/composables/usePaymentConfirmation'
import { useI18n } from '@/i18n'
import { formatMoney } from '@/utils/format'

const props = defineProps<{ order: FeaturePaymentOrder; phase: PaymentConfirmationPhase; progress: number }>()
const emit = defineEmits<{ done: [] }>()
const { t } = useI18n()
const checkProgress = computed(() => Math.max(0, Math.min(1, (props.progress - 0.7) / 0.3)))
const coins = computed(() => Array.from({ length: 7 }, (_, index) => {
  const travel = Math.max(0, Math.min(1, (props.progress / 0.7 - index * 0.065) / 0.58))
  const start = (index % 2 === 0 ? -1 : 1) * (32 + index * 7)
  const x = 120 + start * (1 - travel) ** 2
  const y = 20 + index % 3 * 12 + travel ** 2 * 112
  return { transform: `translate(${x} ${y}) scale(${1 - travel * 0.45})`, opacity: travel === 0 ? 0 : Math.min(1, travel * 8) * (1 - travel ** 6) }
}))
</script>

<template>
  <section class="crypto-success" :data-phase="phase" data-test="crypto-confirmation">
    <svg class="crypto-success__art" viewBox="0 0 240 208" aria-hidden="true">
      <g v-if="phase === 'coins' || phase === 'waiting'" class="crypto-success__wallet">
        <path d="M73 117V105a9 9 0 0 1 9-9h79v20" />
        <rect x="67" y="109" width="106" height="67" rx="14" />
        <path d="M150 132h26v23h-26a11.5 11.5 0 0 1 0-23Z" />
        <circle cx="153" cy="143.5" r="2" fill="currentColor" stroke="none" />
      </g>
      <g v-if="phase === 'coins'" class="crypto-success__coins">
        <g v-for="(coin, index) in coins" :key="index" :transform="coin.transform" :opacity="coin.opacity">
          <circle r="10" /><path d="M-3-4h6M0-4v8M-3 4h6" />
        </g>
      </g>
      <g v-if="phase === 'check' || phase === 'complete'" class="crypto-success__check" :opacity="checkProgress">
        <circle cx="120" cy="104" r="43" />
        <path d="m99 104 15 15 28-30" pathLength="1" stroke-dasharray="1" :stroke-dashoffset="1 - checkProgress" />
      </g>
    </svg>
    <div class="crypto-success__receipt" role="status" aria-live="polite" aria-atomic="true">
      <h2>{{ t('cryptoPayment.confirmed') }}</h2>
      <p class="crypto-success__credit">{{ formatMoney(order.txb) }}</p>
      <p class="crypto-success__detail">{{ t('cryptoPayment.walletCredit') }}</p>
    </div>
    <UButton block color="neutral" :label="t('cryptoPayment.done')" data-haptic="dismiss" @click="emit('done')" />
  </section>
</template>

<style scoped>
.crypto-success { display: grid; justify-items: center; gap: 1.75rem; padding-block: 0.5rem; text-align: center; }
.crypto-success__art { width: min(100%, 240px); height: 208px; color: var(--text-muted); }
.crypto-success__wallet { fill: var(--surface-raised); stroke: currentColor; stroke-width: 2; }
.crypto-success__coins { fill: var(--surface-raised); stroke: var(--accent); stroke-width: 2; }
.crypto-success__coins path { fill: none; stroke-linecap: round; }
.crypto-success__check { fill: none; stroke: var(--success); stroke-width: 3; stroke-linecap: round; stroke-linejoin: round; }
.crypto-success__check path { stroke-width: 4; }
.crypto-success__receipt { display: grid; gap: 0.5rem; }
.crypto-success__receipt h2 { font-size: 1.25rem; font-weight: 600; letter-spacing: -0.025em; }
.crypto-success__credit { font-size: 1.7rem; font-weight: 600; overflow-wrap: anywhere; font-variant-numeric: tabular-nums; }
.crypto-success__detail { color: var(--text-muted); font-size: 0.85rem; }
</style>
