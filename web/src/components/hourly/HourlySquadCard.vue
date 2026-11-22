<script setup lang="ts">
import { computed } from 'vue'
import type { HourlySquad } from '@/api/hourly'
import SquadPricingCard from '@/components/catalog/SquadPricingCard.vue'
import { useI18n } from '@/i18n'
import { formatMemberMoney } from '@/utils/displayCurrency'
import { formatDateTime } from '@/utils/format'
import { hourlyActivationQuote } from './hourlyHours'
const props = defineProps<{ item: HourlySquad; busy: boolean; salesEnabled: boolean; hours: number }>()
defineEmits<{ activate: []; disable: [] }>()
const { t } = useI18n()
const enrolled = computed(() => props.item.enrollment?.status === 'active' || props.item.enrollment?.status === 'preparing')
const canActivate = computed(() => !enrolled.value && props.salesEnabled && props.item.ready && props.item.offer.enabled && hourlyActivationQuote(props.item, props.hours) !== null)
// A preparing enrollment can coexist with a full stock limit: the paid hour
// continues while only new activations are refused.
const capacityContinues = computed(() => enrolled.value && props.item.reasonCode === 'capacity')
const reason = computed(() => {
  if (props.item.cooldownUntil && Date.parse(props.item.cooldownUntil) > Date.now()) return t('hourly.cooldownUntil', { at: formatDateTime(props.item.cooldownUntil) })
  const key = `hourly.reasons.${props.item.reasonCode}`
  return props.item.reasonCode && t(key) !== key ? t(key) : t('hourly.notReady')
})
</script>

<template>
  <SquadPricingCard :squad="item.squad" :selected="false" :included="false" informational class="hourly-squad">
    <template #price>
      <span class="hourly-squad__price"><strong>{{ formatMemberMoney(item.offer.hourlyPrice) }}</strong><small>{{ t('hourly.perHour') }}</small></span>
    </template>
    <div class="hourly-squad__facts">
      <span v-if="item.enrollment?.paidThrough"><UIcon name="i-ph-calendar-check" aria-hidden="true" />{{ t('hourly.paidThrough', { at: formatDateTime(item.enrollment.paidThrough) }) }}</span>
      <span v-if="!enrolled && item.requiredBalance"><UIcon name="i-ph-wallet" aria-hidden="true" />{{ t('hourly.requiredBalance', { price: formatMemberMoney(item.requiredBalance), hours }) }}</span>
      <span v-if="capacityContinues" class="hourly-squad__reason"><UIcon name="i-ph-info" aria-hidden="true" />{{ t('hourly.capacityContinuity') }}</span>
      <span v-else-if="!enrolled && !canActivate" class="hourly-squad__reason"><UIcon name="i-ph-info" aria-hidden="true" />{{ reason }}</span>
      <USwitch
        size="lg"
        class="hourly-squad__toggle"
        color="success"
        :model-value="enrolled"
        :label="enrolled ? t('hourly.disable') : t('hourly.activate')"
        :ui="{ root: 'items-center', thumb: 'motion-reduce:transition-none', base: 'motion-reduce:transition-none' }"
        :disabled="busy || (!enrolled && !canActivate)"
        :aria-label="t('hourly.toggleFor', { name: item.squad.name })"
        :data-haptic="enrolled ? 'open' : 'confirm'"
        @update:model-value="enrolled ? $emit('disable') : $emit('activate')"
      />
    </div>
  </SquadPricingCard>
</template>

<style scoped>
.hourly-squad { align-self: stretch; }
.hourly-squad__price { display: flex; flex-wrap: wrap; align-items: baseline; justify-content: flex-end; gap: 0.2rem; }
.hourly-squad__price strong { font: 600 1.05rem var(--font-mono); overflow-wrap: anywhere; }
.hourly-squad__price small { color: var(--text-muted); font-size: 0.65rem; white-space: nowrap; }
.hourly-squad__facts { display: flex; flex-wrap: wrap; align-items: center; gap: 0.2rem 0.8rem; color: var(--text-muted); font-size: 0.74rem; line-height: 1.45; }
.hourly-squad__facts > span { display: inline-flex; align-items: center; gap: 0.28rem; min-width: 0; }
.hourly-squad__facts > span .iconify { flex: 0 0 auto; color: var(--text-faint); font-size: 0.85rem; }
.hourly-squad__reason { color: var(--warning); }
.hourly-squad__reason .iconify { color: inherit; }
.hourly-squad__toggle { flex: 0 0 auto; margin-inline-start: auto; min-height: 2.75rem; }
</style>
