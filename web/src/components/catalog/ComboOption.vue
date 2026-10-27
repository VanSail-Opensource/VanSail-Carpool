<script setup lang="ts">
import { AnimatePresence, motion } from 'motion-v'

import type { Combo } from '@/api/types'
import MarkdownContent from '@/components/common/MarkdownContent.vue'
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
  <UButton
    class="combo-option"
    :class="{ 'combo-option--selected': selected }"
    color="neutral"
    variant="ghost"
    type="button"
    :aria-pressed="selected"
    @click="selectCombo"
  >
    <span class="combo-option__top">
      <span>
        <strong>{{ combo.name }}</strong>
      </span>
      <AnimatePresence :initial="false">
        <motion.span v-if="selected" key="selected" class="selection-mark" :initial="reducedMotion ? false : { opacity: 0, scale: 0.84 }" :animate="{ opacity: 1, scale: 1 }" :exit="{ opacity: 0 }" :transition="{ duration: reducedMotion ? 0.08 : 0.14, ease: 'easeOut' }"><UIcon name="i-ph-check-bold" /></motion.span>
      </AnimatePresence>
    </span>
    <MarkdownContent class="combo-option__description" :source="combo.description" compact />
    <span class="combo-option__metrics">
      <span><UIcon name="i-ph-gauge" />{{ trafficTerm() }}</span>
      <span><UIcon name="i-ph-stack" />{{ $t('catalog.includedSquadCount', { count: combo.includedSquads.length }) }}</span>
    </span>
    <span class="combo-option__price"><small>{{ $t('catalog.perDays', { count: combo.validityDays }) }}</small></span>
  </UButton>
</template>

<style scoped>
@media (min-width: 900px) {
  .combo-option { gap: 0.6rem; padding: 0.85rem; }
}
</style>
