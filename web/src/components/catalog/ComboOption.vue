<script setup lang="ts">
import { motion } from 'motion-v'

import SquadFeatures from './SquadFeatures.vue'
import type { Combo } from '@/api/types'
import { motionSpring } from '@/composables/motionPresets'
import { useI18n } from '@/i18n'
import { formatBytes } from '@/utils/format'
import { selectionHaptic } from '@/utils/telegram'
import { useMotionPreferences } from '@/composables/useMotionPreferences'

const props = defineProps<{
  combo: Combo
  selected: boolean
}>()

const emit = defineEmits<{ select: [id: string] }>()
const { t } = useI18n()
const { reducedMotion } = useMotionPreferences()

function trafficTerm(): string {
  const key = props.combo.resetStrategy === 'DAY' ? 'day' : props.combo.resetStrategy === 'WEEK' ? 'week' : 'month'
  return `${formatBytes(props.combo.trafficLimitBytes)}/${t(`catalog.resetUnit.${key}`)}`
}

function selectCombo(): void {
  if (props.selected) return
  selectionHaptic()
  emit('select', props.combo.id)
}
</script>

<template>
  <motion.article class="combo-option" :class="{ 'combo-option--selected': selected }" layout :transition="reducedMotion ? { duration: 0.08 } : motionSpring" @click="selectCombo">
    <UButton class="combo-option__select" color="neutral" variant="ghost" type="button" :aria-label="combo.name" :aria-pressed="selected" />
    <motion.span v-if="selected" layout-id="catalog-combo-selection-indicator" class="combo-option__selection" :transition="reducedMotion ? { duration: 0.08 } : motionSpring" aria-hidden="true" />
    <span class="combo-option__top">
      <span>
        <strong>{{ combo.name }}</strong>
      </span>
    </span>
    <span v-if="combo.description" class="combo-option__description">{{ combo.description }}</span>
    <span class="combo-option__metrics">
      <span><UIcon name="i-ph-gauge" />{{ trafficTerm() }}</span>
      <span><UIcon name="i-ph-calendar" />{{ $t('catalog.perDays', { count: combo.validityDays }) }}</span>
      <span><UIcon name="i-ph-stack" />{{ $t('catalog.includedSquadCount', { count: combo.includedSquads.length }) }}</span>
    </span>
    <SquadFeatures :features="combo.features ?? []" />
  </motion.article>
</template>

<style scoped>
.combo-option { position: relative; align-self: start; }
.combo-option__select { position: absolute; inset: 0; z-index: 1; width: 100%; padding: 0; border-radius: inherit; }
.combo-option__selection { position: absolute; inset: 0; z-index: 3; border: 2px solid var(--accent); border-radius: inherit; background: transparent; pointer-events: none; }
.combo-option > :not(.combo-option__selection):not(.combo-option__select) { position: relative; z-index: 2; }
.combo-option__select:focus-visible { outline: 2px solid var(--accent); outline-offset: 3px; }
.combo-option:active { transform: scale(.995); }
@media (prefers-reduced-motion: reduce) { .combo-option { transition: none; } .combo-option:hover, .combo-option:active { transform: none; } }
.combo-option__description { white-space: pre-wrap; overflow-wrap: anywhere; }
@media (min-width: 900px) {
  .combo-option { gap: 0.6rem; padding: 0.85rem; }
}
</style>
