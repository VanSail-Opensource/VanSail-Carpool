<script setup lang="ts">
import { computed } from 'vue'
import type { HourlyView } from '@/api/hourly'
import InlineNotice from '@/components/common/InlineNotice.vue'
import { useI18n } from '@/i18n'
import { formatMemberMoney } from '@/utils/displayCurrency'
import HourlyAllowanceGraph from './HourlyAllowanceGraph.vue'
const props = defineProps<{ data: HourlyView; busy: boolean }>()
defineEmits<{ extra: [enabled: boolean] }>()
const { t } = useI18n()
const fundingSuspended = computed(() => props.data.squads.some(item => item.enrollment?.status === 'funding_suspended'))
</script>

<template>
  <section class="hourly-summary" :aria-label="t('hourly.account')">
    <div class="hourly-summary__wallet"><span>{{ t('payg.balance') }}</span><strong>{{ formatMemberMoney(data.balance) }}</strong></div>
    <InlineNotice v-if="fundingSuspended" class="hourly-summary__notice" tone="warning" :title="t('hourly.fundingSuspendedTitle')">{{ t('hourly.fundingSuspendedBody') }}</InlineNotice>
    <div class="hourly-summary__body">
      <HourlyAllowanceGraph :usage="data.usage" :policy="data.policy.policy" :extra-enabled="data.extraPreference.extraEnabled" :busy="busy" @extra="$emit('extra', $event)" />
    </div>
  </section>
</template>

<style scoped>
.hourly-summary { padding-block: 0.75rem 1.25rem; }
.hourly-summary__wallet { display: flex; align-items: baseline; justify-content: space-between; gap: 1rem; padding-bottom: 0.9rem; border-bottom: 1px solid var(--line); }
.hourly-summary__wallet span { color: var(--text-muted); }
.hourly-summary__wallet strong { font-size: 1.3rem; font-variant-numeric: tabular-nums; }
.hourly-summary__notice { margin-top: 0.85rem; }
.hourly-summary__body { display: grid; grid-template-columns: minmax(0, 1fr); gap: 1rem; padding-top: 0.9rem; }
</style>
