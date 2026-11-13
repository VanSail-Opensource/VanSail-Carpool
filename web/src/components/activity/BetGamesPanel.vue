<script setup lang="ts">
import { computed, shallowRef, watch } from 'vue'
import { AnimatePresence, motion } from 'motion-v'

import type { BetGame } from '@/api/features'
import TxbAmountField from '@/components/common/TxbAmountField.vue'
import { useMotionPreferences } from '@/composables/useMotionPreferences'
import { displayInputCurrency, displayInputFromTXBMinor, displayInputMinimumFromTXBMinor, displayInputToTXBMinor, formatMemberMoney } from '@/utils/displayCurrency'
import { selectionHaptic } from '@/utils/telegram'
import { gameIcon } from './gameIcons'

const props = defineProps<{
  games: readonly BetGame[]
  busy: boolean
}>()
const emit = defineEmits<{ bet: [payload: { gameId: string; stakeTxbMinor: string }] }>()
const { reducedMotion, offset } = useMotionPreferences()
const inputCurrency = computed(displayInputCurrency)

const selectedId = shallowRef<string | null>(null)
const stake = shallowRef('')
const selected = computed(() => props.games.find((game) => game.id === selectedId.value))
const stakeMinor = computed(() => displayInputToTXBMinor(stake.value))
const canBet = computed(() => {
  if (props.busy || !selected.value?.enabled || !stakeMinor.value) return false
  try {
    const amount = BigInt(stakeMinor.value)
    return amount >= BigInt(selected.value.minimumStakeMinor)
      && amount <= BigInt(selected.value.maximumStakeMinor)
  } catch { return false }
})

watch(() => props.games, (games) => {
  if (!selectedId.value || !games.some((game) => game.id === selectedId.value && game.enabled)) {
    selectedId.value = games.find((game) => game.enabled)?.id ?? null
  }
}, { immediate: true })

watch(() => selected.value?.id, (gameId) => {
	const game = props.games.find((candidate) => candidate.id === gameId)
	stake.value = game ? displayInputMinimumFromTXBMinor(game.minimumStakeMinor) : ''
}, { immediate: true })

watch(inputCurrency, () => {
  stake.value = selected.value ? displayInputMinimumFromTXBMinor(selected.value.minimumStakeMinor) : ''
})

function displayMinor(minor: string): string {
  return formatMemberMoney({ currency: 'TXB', minor, display: '' })
}

function displayMinimum(minor: string): string {
  return `${displayInputMinimumFromTXBMinor(minor)} ${inputCurrency.value}`
}

function submit(): void {
  if (!selected.value || !canBet.value) return
  emit('bet', { gameId: selected.value.id, stakeTxbMinor: stakeMinor.value })
}

function selectGame(id: string): void {
  if (props.busy || selectedId.value === id || !props.games.some(game => game.id === id && game.enabled)) return
  selectedId.value = id
  selectionHaptic()
}
</script>

<template>
  <section class="section-block bet-panel">
    <div class="section-heading section-heading--stacked">
      <h2>{{ $t('activity.betGames') }}</h2>
      <p>{{ $t('activity.betCopy') }}</p>
    </div>
    <motion.div v-if="games.length" layout class="bet-grid">
      <AnimatePresence :initial="false" mode="popLayout">
        <motion.div v-for="game in games" :key="game.id" layout :initial="reducedMotion ? false : { opacity: 0, y: offset(8) }" :animate="{ opacity: 1, y: 0 }" :exit="{ opacity: 0 }" :transition="{ duration: reducedMotion ? 0.08 : 0.2, ease: 'easeOut' }">
          <UButton
            class="bet-option"
            :class="{ 'bet-option--selected': game.id === selectedId }"
            color="neutral"
            variant="ghost"
            :disabled="!game.enabled || busy"
            :aria-pressed="game.id === selectedId"
            @click="selectGame(game.id)"
          >
            <span class="bet-option__icon"><UIcon :name="gameIcon(game.icon)" /></span>
            <span>
              <strong>{{ game.name }}</strong>
              <small>{{ game.description || $t('activity.stakeRange', { minimum: displayMinimum(game.minimumStakeMinor), maximum: displayMinor(game.maximumStakeMinor) }) }}</small>
            </span>
            <span class="bet-option__odds">{{ (game.winChanceBps / 100).toFixed(2) }}%</span>
          </UButton>
        </motion.div>
      </AnimatePresence>
    </motion.div>
    <div v-if="selected" class="bet-form">
      <TxbAmountField
        id="bet-stake"
        v-model="stake"
        :label="$t('activity.stake')"
        :min-minor="selected.minimumStakeMinor"
        :max-minor="selected.maximumStakeMinor"
        :hint="$t('activity.stakeRange', { minimum: displayMinimum(selected.minimumStakeMinor), maximum: displayMinor(selected.maximumStakeMinor) })"
        :suffix="inputCurrency"
        :display-value="displayInputFromTXBMinor"
        :display-minimum="displayInputMinimumFromTXBMinor"
        :parse-display-value="displayInputToTXBMinor"
        required
      />
      <div class="bet-disclosure">
        <UIcon name="i-ph-trend-up" />
        <span><strong>{{ $t('activity.totalReturn', { multiplier: (selected.returnMultiplierBps / 10000).toFixed(2) }) }}</strong><small>{{ $t('activity.lossReturn', { amount: displayMinor('0') }) }}</small></span>
      </div>
      <UButton :disabled="!canBet || busy" :loading="busy" :label="busy ? $t('activity.resolving') : $t('activity.confirmBet')" data-haptic="confirm" @click="submit" />
    </div>
    <div v-else class="empty-inline"><div><h3>{{ $t('activity.noBetGames') }}</h3><p>{{ $t('activity.publishGames') }}</p></div></div>
  </section>
</template>

<style scoped>
.bet-grid { display: grid; gap: 0.55rem; }
.bet-option { min-height: 68px; display: grid; grid-template-columns: auto minmax(0, 1fr) auto; align-items: center; gap: 0.7rem; padding: 0.7rem; border: 1px solid var(--line); border-radius: var(--radius-control); color: var(--text); background: var(--surface-raised); text-align: left; cursor: pointer; }
.bet-option--selected { border-color: #557763; background: var(--accent-soft); }
.bet-option__icon { width: 40px; height: 40px; display: grid; place-items: center; border-radius: 12px; color: var(--accent); background: #15241b; }
.bet-option strong, .bet-option small { display: block; }
.bet-option strong { font-size: 0.82rem; }
.bet-option small { margin-top: 0.2rem; color: var(--text-faint); font-size: 0.68rem; }
.bet-option__odds { color: var(--accent); font-family: var(--font-mono); font-size: 0.72rem; }
.bet-form { display: grid; gap: 0.8rem; margin-top: 1rem; padding-top: 1rem; border-top: 1px solid var(--line); }
.bet-disclosure { min-height: 52px; display: flex; align-items: center; gap: 0.65rem; padding: 0.65rem; border-radius: var(--radius-control); color: var(--accent); background: var(--accent-soft); }
.bet-disclosure strong, .bet-disclosure small { display: block; }
.bet-disclosure strong { font-size: 0.76rem; }
.bet-disclosure small { margin-top: 0.2rem; color: var(--text-muted); font-size: 0.65rem; }
@media (min-width: 720px) { .bet-form { grid-template-columns: minmax(220px, 1fr) minmax(180px, 0.8fr) auto; align-items: end; } }
</style>
