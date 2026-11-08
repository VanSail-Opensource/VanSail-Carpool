<script setup lang="ts">
import { computed } from 'vue'
import type { HourlyView } from '@/api/hourly'
import { useI18n } from '@/i18n'
import { formatBytes, formatMoney, txbInputFromMinor } from '@/utils/format'
const props = defineProps<{ data: HourlyView; busy: boolean }>()
defineEmits<{ extra: [enabled: boolean] }>()
const { t } = useI18n()
const consentChanged = computed(() => props.data.extraPreference.extraEnabled && BigInt(props.data.policy.policy.extraGBPriceTXBMinor) > BigInt(props.data.extraPreference.acceptedExtraPriceTXBMinor ?? '0'))
const availableExtra = computed(() => {
  const usage = props.data.usage
  return usage ? (BigInt(usage.extraFundedBytes) - BigInt(usage.extraUsedBytes) - BigInt(usage.extraReservedBytes)).toString() : '0'
})
</script>

<template>
  <section class="hourly-summary" :aria-label="t('hourly.account')">
    <div class="hourly-summary__balance"><span>{{ t('payg.balance') }}</span><strong>{{ formatMoney(data.balance) }}</strong></div>
    <dl class="hourly-summary__metrics">
      <div><dt>{{ t('hourly.dailyAllowance') }}</dt><dd>{{ formatBytes(data.usage?.freeAllowanceBytes ?? data.policy.policy.dailyAllowanceBytes) }}</dd></div>
      <div><dt>{{ t('hourly.usedToday') }}</dt><dd>{{ formatBytes(data.usage?.freeUsedBytes ?? '0') }}</dd></div>
      <div><dt>{{ t('hourly.reducedRate') }}</dt><dd>{{ t('hourly.rate', { rate: formatBytes(data.usage?.reducedRateBytesPerSecond ?? data.policy.policy.reducedRateBytesPerSecond) }) }}</dd></div>
      <div><dt>{{ t('hourly.extraRemaining') }}</dt><dd>{{ formatBytes(availableExtra) }}</dd></div>
    </dl>
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
.hourly-summary__balance { display: flex; align-items: baseline; justify-content: space-between; gap: 1rem; padding-bottom: 1rem; }
.hourly-summary__balance span, dt, .hourly-summary__hint { color: var(--text-muted); }
.hourly-summary__balance strong { font-size: 1.3rem; font-variant-numeric: tabular-nums; }
.hourly-summary__metrics { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 1rem; margin: 0; }
dt { font-size: .8rem; } dd { margin: .25rem 0 0; font-size: .95rem; font-weight: 600; overflow-wrap: anywhere; }
.hourly-summary__hint { font-size: .8rem; line-height: 1.55; margin: .75rem 0 0; }
.hourly-summary__extra { margin-top: 1.25rem; padding-top: 1rem; border-top: 1px solid var(--border); }
@media (min-width: 760px) { .hourly-summary__metrics { grid-template-columns: repeat(4, minmax(0, 1fr)); } }
</style>
