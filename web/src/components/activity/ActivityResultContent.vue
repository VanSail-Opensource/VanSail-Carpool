<script setup lang="ts">
import { computed } from 'vue'
import { AnimatePresence, motion } from 'motion-v'

import type { ActivityResult } from '@/api/features'
import { useMotionPreferences } from '@/composables/useMotionPreferences'
import { useI18n } from '@/i18n'
import { formatMemberMoney } from '@/utils/displayCurrency'
import BetSuccessFireworks from './BetSuccessFireworks.vue'
import { isSuccessfulBet } from './feedback'

const props = defineProps<{ result: ActivityResult }>()
const { t } = useI18n()
const { reducedMotion } = useMotionPreferences()
const showFireworks = computed(() => isSuccessfulBet(props.result))

const rewardLabel = computed(() => {
  const reward = props.result.reward
  if (reward.kind === 'none') return t('activity.noReward')
  if (reward.kind === 'txb_delta') {
    const amount = formatMemberMoney({ currency: 'TXB', minor: reward.txbDeltaMinor ?? String(reward.resolvedValue ?? 0), display: '' })
    return props.result.kind === 'check_in' ? t('activity.checkInReward', { amount }) : amount
  }
  if (reward.kind === 'coupon_grant') return t('activity.rewardCoupon')
  if (reward.kind === 'subscription_extension') return t('activityDrawReward.hours', { value: reward.resolvedValue ?? (reward.extensionDays ?? 0) * 24 })
  if (reward.kind === 'traffic_grant') return t('activityDrawReward.traffic', { value: reward.resolvedValue ?? 0 })
  if (reward.kind === 'balance_multiplier') return t('activityDrawReward.multiplier', { value: ((reward.resolvedValue ?? 10000) / 10000).toFixed(4) })
  if (reward.kind === 'coupon_recurring' || reward.kind === 'coupon_once') {
    return reward.discountMode === 'percent'
      ? t('activityDrawReward.couponPercent', { value: ((reward.resolvedValue ?? 0) / 100).toFixed(2) })
      : t('activityDrawReward.couponFixed', { value: ((reward.resolvedValue ?? 0) / 100).toFixed(2) })
  }
  if (reward.kind === 'entitlement_grant') return t('activityDrawReward.customCombo')
  if (reward.kind === 'squad_access') return t('activityDrawReward.squads')
  if (reward.kind === 'core_combo_switch') return t('activityDrawReward.comboSwitch')
  return t('activityDrawReward.reset')
})
</script>

<template>
  <div class="result-body" :class="{ 'result-body--draw': result.kind === 'draw', 'result-body--check-in': result.kind === 'check_in' }">
    <BetSuccessFireworks v-if="showFireworks" :key="result.id" />
    <AnimatePresence mode="wait" :initial="false">
      <motion.div
        :key="result.id"
        :initial="reducedMotion ? { opacity: 0 } : { opacity: 0, y: 5 }"
        :animate="{ opacity: 1, y: 0 }"
        :exit="{ opacity: 0 }"
        :transition="{ duration: reducedMotion ? 0.08 : 0.22, delay: reducedMotion ? 0 : 0.05, ease: 'easeOut' }"
      >
        <motion.div
          v-if="result.kind === 'bet'"
          :initial="reducedMotion ? { opacity: 0 } : { opacity: 0, scale: 0.86 }"
          :animate="{ opacity: 1, scale: 1 }"
          :transition="{ duration: reducedMotion ? 0.08 : 0.2, ease: 'easeOut' }"
        >
          <UIcon
            :name="result.outcome === 'loss' ? 'i-ph-warning-circle-fill' : 'i-ph-check-circle-fill'"
            class="feature-icon"
            :class="{ 'feature-icon--warning': result.outcome === 'loss' }"
            aria-hidden="true"
          />
        </motion.div>
        <div v-if="result.kind === 'draw' && result.prizeName" class="result-reward result-prize">
          <span>{{ $t('activity.drawPrize') }}</span>
          <strong>{{ result.prizeName }}</strong>
        </div>
        <div class="result-balance">
          <span>{{ $t('activity.balanceAfter') }}</span>
          <strong>{{ formatMemberMoney(result.balanceAfter) }}</strong>
        </div>
        <div v-if="(result.kind === 'draw' || result.kind === 'check_in') && result.reward.kind !== 'none'" class="result-reward">
          <span>{{ $t('activity.reward') }}</span>
          <strong>{{ rewardLabel }}</strong>
        </div>
      </motion.div>
    </AnimatePresence>
  </div>
</template>

<style scoped>
.result-body { position: relative; overflow: hidden; }
.result-body > :not(.bet-fireworks) { position: relative; z-index: 1; }
.result-balance, .result-reward { display: flex; align-items: baseline; justify-content: space-between; gap: 0.8rem; padding: 0.8rem; border: 1px solid var(--line); border-radius: var(--radius-control); background: var(--surface-raised); }
.result-balance { margin-top: 0.55rem; }
.result-balance span, .result-reward span { color: var(--text-muted); font-size: 0.74rem; }
.result-balance strong, .result-reward strong { font-size: 1rem; text-align: right; }
.result-reward { margin-top: 0.55rem; border-color: #304138; color: var(--accent); background: var(--accent-soft); }
.result-prize { margin-top: 0.2rem; }
.result-body--draw .result-reward { border-color: var(--line); background: var(--surface-raised); color: var(--text); }
.result-body--check-in .result-balance { margin-top: 0; }
.result-body--check-in .result-balance, .result-body--check-in .result-reward { padding: .55rem .65rem; }
</style>
