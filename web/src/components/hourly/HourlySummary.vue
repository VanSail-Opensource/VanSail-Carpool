<script setup lang="ts">
import { computed } from 'vue'
import type { HourlyView } from '@/api/hourly'
import InlineNotice from '@/components/common/InlineNotice.vue'
import { useI18n } from '@/i18n'
import { formatMemberMoney } from '@/utils/displayCurrency'
import { txbInputFromMinor } from '@/utils/format'
import HourlyAllowanceGraph from './HourlyAllowanceGraph.vue'
import HourlyAccountingStatus from './HourlyAccountingStatus.vue'
const props = defineProps<{ data: HourlyView; busy: boolean }>()
defineEmits<{ extra: [enabled: boolean] }>()
const { t } = useI18n()
const consentChanged = computed(() => props.data.extraPreference.extraEnabled && BigInt(props.data.policy.policy.extraGBPriceTXBMinor) > BigInt(props.data.extraPreference.acceptedExtraPriceTXBMinor ?? '0'))
const fundingSuspended = computed(() => props.data.squads.some(item => item.enrollment?.status === 'funding_suspended'))
</script>

<template>
  <section class="hourly-summary" :aria-label="t('hourly.account')">
    <div class="hourly-summary__wallet"><span>{{ t('payg.balance') }}</span><strong>{{ formatMemberMoney(data.balance) }}</strong></div>
    <InlineNotice v-if="fundingSuspended" class="hourly-summary__notice" tone="warning" :title="t('hourly.fundingSuspendedTitle')">{{ t('hourly.fundingSuspendedBody') }}</InlineNotice>
    <div class="hourly-summary__body">
      <HourlyAllowanceGraph :usage="data.usage" :policy="data.policy.policy" :extra-enabled="data.extraPreference.extraEnabled" />
    </div>
    <div class="hourly-summary__extra">
      <USwitch :model-value="data.extraPreference.extraEnabled" :disabled="busy || (!data.extraPreference.extraEnabled && data.policy.policy.extraGBPriceTXBMinor === '0')" :label="t('hourly.extraTitle')" :description="t('hourly.extraRate', { price: txbInputFromMinor(data.policy.policy.extraGBPriceTXBMinor) })" @update:model-value="$emit('extra', $event)" />
      <p v-if="data.policy.policy.extraGBPriceTXBMinor === '0'" class="hourly-summary__hint">{{ t('hourly.extraUnavailable') }}</p>
      <p v-if="data.extraPreference.extraEnabled" class="hourly-summary__hint">{{ t('hourly.acceptedPrice', { price: txbInputFromMinor(data.extraPreference.acceptedExtraPriceTXBMinor ?? '0') }) }}</p>
      <UButton v-if="consentChanged" color="neutral" variant="link" :disabled="busy" :label="t('hourly.reviewPrice')" @click="$emit('extra', true)" />
    </div>
    <HourlyAccountingStatus :usage="data.usage" />
  </section>
</template>

<style scoped>
.hourly-summary { padding-block: 0.75rem 1.25rem; }
.hourly-summary__wallet { display: flex; align-items: baseline; justify-content: space-between; gap: 1rem; padding-bottom: 0.9rem; border-bottom: 1px solid var(--line); }
.hourly-summary__wallet span, dt, .hourly-summary__hint { color: var(--text-muted); }
.hourly-summary__wallet strong { font-size: 1.3rem; font-variant-numeric: tabular-nums; }
.hourly-summary__notice { margin-top: 0.85rem; }
.hourly-summary__body { display: grid; grid-template-columns: minmax(0, 1fr); gap: 1rem; padding-top: 0.9rem; }
.hourly-summary__hint { font-size: 0.78rem; line-height: 1.55; margin: 0.7rem 0 0; }
.hourly-summary__extra { margin-top: 1.15rem; padding-top: 0.95rem; border-top: 1px solid var(--line); }
</style>
