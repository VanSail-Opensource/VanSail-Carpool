<script setup lang="ts">
import { motion } from 'motion-v'

import { useMotionPreferences } from '@/composables/useMotionPreferences'
import { formatMemberMoney } from '@/utils/displayCurrency'

defineProps<{
  checkedIn: boolean
  rewardMinTxbMinor: string
  rewardMaxTxbMinor: string
  timeZone: string
  busy: boolean
}>()

defineEmits<{ checkIn: [] }>()

const { reducedMotion } = useMotionPreferences()

function displayMinor(minor: string): string {
  return formatMemberMoney({ currency: 'TXB', minor, display: '' })
}
</script>

<template>
  <section class="section-block activity-card">
    <motion.span class="feature-icon" :animate="checkedIn && !reducedMotion ? { scale: [0.9, 1.12, 1] } : { scale: 1 }" :transition="{ duration: reducedMotion ? 0.08 : 0.2, ease: 'easeOut' }"><UIcon name="i-ph-calendar-check" /></motion.span>
    <div class="activity-card__copy">
      <h2>{{ $t('activity.dailyCheckIn') }}</h2>
      <p>
        {{ checkedIn ? $t('activity.todayComplete') : rewardMinTxbMinor === rewardMaxTxbMinor
          ? $t('activity.claimToday', { amount: displayMinor(rewardMinTxbMinor) })
          : $t('activity.claimRangeToday', { minimum: displayMinor(rewardMinTxbMinor), maximum: displayMinor(rewardMaxTxbMinor) }) }}
      </p>
      <small>{{ $t('activity.resetsAt', { timezone: timeZone }) }}</small>
    </div>
    <UButton
      color="neutral"
      icon="i-ph-check-circle-fill"
      :disabled="checkedIn || busy"
      :loading="busy"
      :label="checkedIn ? $t('activity.checkedIn') : busy ? $t('activity.checkingIn') : $t('activity.checkIn')"
      data-haptic="confirm"
      @click="$emit('checkIn')"
    />
  </section>
</template>

<style scoped>
.activity-card { display: grid; grid-template-columns: auto minmax(0, 1fr); align-items: start; gap: .6rem; }
.activity-card__copy h2, .activity-card__copy p { margin: 0; }
.activity-card__copy h2 { font-size: 1.05rem; }
.activity-card__copy p { margin-top: 0.35rem; color: var(--text-muted); font-size: 0.82rem; }
.activity-card__copy small { display: block; margin-top: .25rem; color: var(--text-faint); font-size: 0.68rem; }
.activity-card :deep(button) { grid-column: 1 / -1; width: 100%; justify-content: center; text-align: center; }
@media (min-width: 640px) { .activity-card { grid-template-columns: auto minmax(0, 1fr) auto; align-items: center; } .activity-card :deep(button) { grid-column: auto; width: auto; } }
</style>
