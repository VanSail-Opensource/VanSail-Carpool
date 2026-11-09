<script setup lang="ts">
import { computed } from 'vue'
import type { HourlySquad } from '@/api/hourly'
import SquadPricingCard from '@/components/catalog/SquadPricingCard.vue'
import StatusBadge from '@/components/common/StatusBadge.vue'
import { useI18n } from '@/i18n'
import { formatDateTime, formatMoney } from '@/utils/format'
const props = defineProps<{ item: HourlySquad; busy: boolean; salesEnabled: boolean; legacyBlocked: boolean; hours: number }>()
defineEmits<{ activate: []; disable: [] }>()
const { t } = useI18n()
const enrolled = computed(() => props.item.enrollment?.status === 'active' || props.item.enrollment?.status === 'preparing')
const canActivate = computed(() => !enrolled.value && props.salesEnabled && !props.legacyBlocked && props.item.ready && props.item.offer.enabled)
const reason = computed(() => {
  if (props.item.cooldownUntil && Date.parse(props.item.cooldownUntil) > Date.now()) return t('hourly.cooldownUntil', { at: formatDateTime(props.item.cooldownUntil) })
  if (props.legacyBlocked) return t('hourly.legacyShort')
  const key = `hourly.reasons.${props.item.reasonCode}`
  return props.item.reasonCode && t(key) !== key ? t(key) : t('hourly.notReady')
})
</script>

<template>
  <SquadPricingCard :squad="item.squad" :selected="false" :included="false" :featured="false" informational class="hourly-squad">
    <template #status><StatusBadge v-if="item.enrollment" :tone="item.enrollment.status === 'active' ? 'success' : 'neutral'" :label="t(`hourly.status.${item.enrollment.status}`)" /></template>
    <template #price>
      <span class="hourly-squad__price"><strong>{{ formatMoney(item.offer.hourlyPrice) }}</strong><small>{{ t('hourly.perHour') }}</small></span>
    </template>
    <div class="hourly-squad__facts">
      <span v-if="item.enrollment?.paidThrough"><UIcon name="i-ph-calendar-check" aria-hidden="true" />{{ t('hourly.paidThrough', { at: formatDateTime(item.enrollment.paidThrough) }) }}</span>
      <span v-if="!enrolled && item.requiredBalance"><UIcon name="i-ph-wallet" aria-hidden="true" />{{ t('hourly.requiredBalance', { price: formatMoney(item.requiredBalance), hours }) }}</span>
      <span v-if="!enrolled && !canActivate" class="hourly-squad__reason"><UIcon name="i-ph-info" aria-hidden="true" />{{ reason }}</span>
    </div>
    <footer class="hourly-squad__action">
      <UButton v-if="enrolled" color="neutral" variant="outline" :disabled="busy" :label="t('hourly.disable')" data-haptic="open" @click="$emit('disable')" />
      <UButton v-else :disabled="busy || !canActivate" :label="t('hourly.activate')" data-haptic="confirm" @click="$emit('activate')" />
    </footer>
  </SquadPricingCard>
</template>

<style scoped>
.hourly-squad { align-self: start; }
.hourly-squad__price { display: flex; flex-wrap: nowrap; align-items: baseline; justify-content: flex-end; gap: .2rem; }
.hourly-squad__price strong { font: 600 .95rem var(--font-mono); white-space: nowrap; }
.hourly-squad__price small { color: var(--text-muted); font-size: .65rem; white-space: nowrap; }
.hourly-squad__facts { display: flex; flex-wrap: wrap; gap: .2rem .8rem; color: var(--text-muted); font-size: .74rem; line-height: 1.45; }
.hourly-squad__facts > span { display: inline-flex; align-items: center; gap: .28rem; min-width: 0; }
.hourly-squad__facts .iconify { flex: 0 0 auto; color: var(--text-faint); font-size: .85rem; }
.hourly-squad__reason { color: var(--warning); }
.hourly-squad__reason .iconify { color: inherit; }
.hourly-squad__action { display: flex; justify-content: flex-end; padding-top: .45rem; border-top: 1px solid var(--line); }
</style>
