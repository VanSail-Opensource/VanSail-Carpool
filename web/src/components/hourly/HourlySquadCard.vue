<script setup lang="ts">
import { computed } from 'vue'
import type { HourlySquad } from '@/api/hourly'
import StatusBadge from '@/components/common/StatusBadge.vue'
import SquadProfileSummary from '@/components/squad-profile/SquadProfileSummary.vue'
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
  <article class="hourly-squad">
    <header><h3>{{ item.squad.name }}</h3><StatusBadge v-if="item.enrollment" :tone="item.enrollment.status === 'active' ? 'success' : 'neutral'" :label="t(`hourly.status.${item.enrollment.status}`)" /></header>
    <SquadProfileSummary :profile="item.squad.profile" compact />
    <p class="hourly-squad__price">{{ t('hourly.hourPrice', { price: formatMoney(item.offer.hourlyPrice) }) }}</p>
    <p v-if="item.enrollment?.paidThrough" class="hourly-squad__detail">{{ t('hourly.paidThrough', { at: formatDateTime(item.enrollment.paidThrough) }) }}</p>
    <p v-if="!enrolled && item.requiredBalance" class="hourly-squad__detail">{{ t('hourly.requiredBalance', { price: formatMoney(item.requiredBalance), hours }) }}</p>
    <p v-if="!enrolled && !canActivate" class="hourly-squad__detail">{{ reason }}</p>
    <UButton v-if="enrolled" color="neutral" variant="outline" :disabled="busy" :label="t('hourly.disable')" data-haptic="open" @click="$emit('disable')" />
    <UButton v-else :disabled="busy || !canActivate" :label="t('hourly.activate')" data-haptic="confirm" @click="$emit('activate')" />
  </article>
</template>

<style scoped>
.hourly-squad { min-width: 0; display: flex; flex-direction: column; align-items: flex-start; gap: .75rem; padding: 1rem 0; border-top: 1px solid var(--border); }
header { width: 100%; display: flex; flex-wrap: wrap; align-items: center; justify-content: space-between; gap: .5rem; }
h3 { margin: 0; font-size: 1rem; font-weight: 600; overflow-wrap: anywhere; }
.hourly-squad__price { margin: 0; font-weight: 600; font-variant-numeric: tabular-nums; }
.hourly-squad__detail { margin: 0; color: var(--text-muted); font-size: .85rem; line-height: 1.5; }
.hourly-squad > :last-child { margin-top: auto; }
</style>
