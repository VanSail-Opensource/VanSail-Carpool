<script setup lang="ts">
import { computed } from 'vue'
import type { HourlySquad } from '@/api/hourly'
import SquadPricingCard from '@/components/catalog/SquadPricingCard.vue'
import StatusBadge from '@/components/common/StatusBadge.vue'
import { useI18n } from '@/i18n'
import { formatMemberMoney } from '@/utils/displayCurrency'
import { formatDateTime } from '@/utils/format'
const props = defineProps<{ item: HourlySquad; busy: boolean; salesEnabled: boolean; hours: number }>()
defineEmits<{ activate: []; disable: [] }>()
const { t } = useI18n()
const enrolled = computed(() => props.item.enrollment?.status === 'active' || props.item.enrollment?.status === 'preparing')
const canActivate = computed(() => !enrolled.value && props.salesEnabled && props.item.ready && props.item.offer.enabled)
const blocked = computed(() => !enrolled.value && !canActivate.value)
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
    <template #status><StatusBadge v-if="item.enrollment && item.enrollment.status !== 'active'" tone="neutral" :label="t(`hourly.status.${item.enrollment.status}`)" /></template>
    <template #price>
      <span class="hourly-squad__price"><strong>{{ formatMemberMoney(item.offer.hourlyPrice) }}</strong><small>{{ t('hourly.perHour') }}</small></span>
    </template>
    <div class="hourly-squad__facts">
      <span v-if="item.enrollment?.paidThrough"><UIcon name="i-ph-calendar-check" aria-hidden="true" />{{ t('hourly.paidThrough', { at: formatDateTime(item.enrollment.paidThrough) }) }}</span>
      <span v-if="!enrolled && item.requiredBalance"><UIcon name="i-ph-wallet" aria-hidden="true" />{{ t('hourly.requiredBalance', { price: formatMemberMoney(item.requiredBalance), hours }) }}</span>
      <span v-if="capacityContinues" class="hourly-squad__reason"><UIcon name="i-ph-info" aria-hidden="true" />{{ t('hourly.capacityContinuity') }}</span>
      <span v-else-if="!enrolled && !canActivate" class="hourly-squad__reason"><UIcon name="i-ph-info" aria-hidden="true" />{{ reason }}</span>
      <UButton
        size="sm"
        square
        class="hourly-squad__toggle"
        :class="{ 'hourly-squad__toggle--blocked': blocked }"
        :color="enrolled ? 'error' : blocked ? 'neutral' : 'success'"
        variant="soft"
        :icon="enrolled ? 'i-ph-stop-circle' : blocked ? 'i-ph-prohibit' : 'i-ph-play-circle'"
        :disabled="busy || (!enrolled && !canActivate)"
        :aria-label="enrolled ? t('hourly.disable') : t('hourly.activate')"
        :title="enrolled ? t('hourly.disable') : t('hourly.activate')"
        :data-haptic="enrolled ? 'open' : 'confirm'"
        @click="enrolled ? $emit('disable') : $emit('activate')"
      />
    </div>
  </SquadPricingCard>
</template>

<style scoped>
.hourly-squad { align-self: start; }
.hourly-squad__price { display: flex; flex-wrap: nowrap; align-items: baseline; justify-content: flex-end; gap: 0.2rem; }
.hourly-squad__price strong { font: 600 0.95rem var(--font-mono); white-space: nowrap; }
.hourly-squad__price small { color: var(--text-muted); font-size: 0.65rem; white-space: nowrap; }
.hourly-squad__facts { display: flex; flex-wrap: wrap; align-items: center; gap: 0.2rem 0.8rem; color: var(--text-muted); font-size: 0.74rem; line-height: 1.45; }
.hourly-squad__facts > span { display: inline-flex; align-items: center; gap: 0.28rem; min-width: 0; }
.hourly-squad__facts > span .iconify { flex: 0 0 auto; color: var(--text-faint); font-size: 0.85rem; }
.hourly-squad__reason { color: var(--warning); }
.hourly-squad__reason .iconify { color: inherit; }
.hourly-squad__toggle { flex: 0 0 auto; margin-inline-start: auto; width: 2.25rem; min-width: 2.25rem; height: 2.25rem; min-height: 2.25rem; padding: 0; }
.hourly-squad__toggle--blocked { color: var(--text-faint); background: color-mix(in srgb, var(--text-faint) 12%, transparent); }
.hourly-squad__toggle--blocked:disabled { opacity: 0.85; }
</style>
