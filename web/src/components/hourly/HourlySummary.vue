<script setup lang="ts">
import { computed } from 'vue'
import type { HourlyView } from '@/api/hourly'
import { useI18n } from '@/i18n'
import { formatBytes, formatMoney, txbInputFromMinor } from '@/utils/format'
import HourlyAllowanceGraph from './HourlyAllowanceGraph.vue'
const props = defineProps<{ data: HourlyView; busy: boolean }>()
defineEmits<{ extra: [enabled: boolean] }>()
const { t } = useI18n()
const consentChanged = computed(() => props.data.extraPreference.extraEnabled && BigInt(props.data.policy.policy.extraGBPriceTXBMinor) > BigInt(props.data.extraPreference.acceptedExtraPriceTXBMinor ?? '0'))
const availableExtra = computed(() => {
  const usage = props.data.usage
  return usage ? (BigInt(usage.extraFundedBytes) - BigInt(usage.extraUsedBytes) - BigInt(usage.extraReservedBytes)).toString() : '0'
})
const allowance = computed(() => props.data.usage?.freeAllowanceBytes ?? props.data.policy.policy.dailyAllowanceBytes)
const used = computed(() => props.data.usage?.freeUsedBytes ?? '0')
const rate = computed(() => props.data.usage?.reducedRateBytesPerSecond ?? props.data.policy.policy.reducedRateBytesPerSecond)
</script>

<template>
  <section class="hourly-summary" :aria-label="t('hourly.account')">
    <div class="hourly-summary__wallet"><span>{{ t('payg.balance') }}</span><strong>{{ formatMoney(data.balance) }}</strong></div>
    <div class="hourly-summary__body">
      <HourlyAllowanceGraph :allowance-bytes="allowance" :used-bytes="used" />
      <dl class="hourly-summary__metrics">
        <div><dt>{{ t('hourly.reducedRate') }}</dt><dd>{{ t('hourly.rate', { rate: formatBytes(rate) }) }}</dd></div>
        <div><dt>{{ t('hourly.extraRemaining') }}</dt><dd>{{ formatBytes(availableExtra) }}</dd></div>
      </dl>
    </div>
    <p class="hourly-summary__hint">{{ t('hourly.allowanceHint') }}</p>
    <div class="hourly-summary__extra">
      <USwitch :model-value="data.extraPreference.extraEnabled" :disabled="busy || (!data.extraPreference.extraEnabled && data.policy.policy.extraGBPriceTXBMinor === '0')" :label="t('hourly.extraTitle')" :description="t('hourly.extraPrice', { price: txbInputFromMinor(data.policy.policy.extraGBPriceTXBMinor) })" @update:model-value="$emit('extra', $event)" />
      <p v-if="data.policy.policy.extraGBPriceTXBMinor === '0'" class="hourly-summary__hint">{{ t('hourly.extraUnavailable') }}</p>
      <p v-else class="hourly-summary__hint">{{ t('hourly.extraExpiry') }}</p>
      <p v-if="data.extraPreference.extraEnabled" class="hourly-summary__hint">{{ t('hourly.acceptedPrice', { price: txbInputFromMinor(data.extraPreference.acceptedExtraPriceTXBMinor ?? '0') }) }}</p>
      <UButton v-if="consentChanged" color="neutral" variant="link" :disabled="busy" :label="t('hourly.reviewPrice')" @click="$emit('extra', true)" />
    </div>
    <p v-if="data.usage?.accountingPending" class="hourly-summary__hint" role="status">{{ t('hourly.accountingPending') }}</p>
  </section>
</template>

<style scoped>
.hourly-summary { padding-block: .75rem 1.25rem; }
.hourly-summary__wallet { display: flex; align-items: baseline; justify-content: space-between; gap: 1rem; padding-bottom: .9rem; border-bottom: 1px solid var(--line); }
.hourly-summary__wallet span, dt, .hourly-summary__hint { color: var(--text-muted); }
.hourly-summary__wallet strong { font-size: 1.3rem; font-variant-numeric: tabular-nums; }
.hourly-summary__body { display: grid; grid-template-columns: minmax(0, 1fr); gap: 1rem; padding-top: .9rem; }
.hourly-summary__metrics { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 0 1.5rem; margin: 0; align-content: start; }
dt { font-size: .78rem; } dd { margin: .25rem 0 0; font-size: .92rem; font-weight: 600; overflow-wrap: anywhere; }
.hourly-summary__hint { font-size: .78rem; line-height: 1.55; margin: .7rem 0 0; }
.hourly-summary__extra { margin-top: 1.15rem; padding-top: .95rem; border-top: 1px solid var(--line); }
@media (min-width: 760px) { .hourly-summary__body { grid-template-columns: minmax(0, 1.15fr) minmax(0, 1fr); align-items: center; gap: 1.5rem; } }
</style>
