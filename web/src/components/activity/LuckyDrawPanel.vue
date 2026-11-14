<script setup lang="ts">
import { computed, reactive, watch } from 'vue'
import { AnimatePresence, motion } from 'motion-v'

import type { LuckyDraw } from '@/api/features'
import { useMotionPreferences } from '@/composables/useMotionPreferences'
import { useI18n } from '@/i18n'
import { formatMemberMoney } from '@/utils/displayCurrency'
import { drawStyles, type DrawStyleChoice } from './draw/selection'

const props = defineProps<{ draws: readonly LuckyDraw[]; busy: boolean; disabled?: boolean; resetToken: number; drawingId?: string | null }>()
const emit = defineEmits<{ draw: [draw: LuckyDraw, style: DrawStyleChoice] }>()
const { reducedMotion, offset } = useMotionPreferences()
const { t } = useI18n()
const selectedStyles = reactive<Record<string, DrawStyleChoice>>({})
const styleOptions = computed(() => [
  { value: 'random', label: t('activity.drawStyle.random') },
  ...drawStyles.map((style) => ({ value: style, label: t('activity.drawStyle.' + style) })),
])

watch(() => props.resetToken, () => {
  for (const id of Object.keys(selectedStyles)) delete selectedStyles[id]
})

function choose(drawId: string, value: string): void {
  if (value === 'random' || drawStyles.some((style) => style === value)) selectedStyles[drawId] = value as DrawStyleChoice
}

function start(draw: LuckyDraw): void {
  if (props.busy || props.disabled) return
  emit('draw', draw, selectedStyles[draw.id] ?? 'random')
}

function displayMinor(minor: string): string {
  return formatMemberMoney({ currency: 'TXB', minor, display: '' })
}
</script>

<template>
  <motion.section layout class="section-block draw-list">
    <div class="section-heading section-heading--stacked"><h2>{{ $t('activity.luckyDraws') }}</h2><p>{{ $t('activity.drawCopy') }}</p></div>
    <AnimatePresence :initial="false" mode="popLayout">
      <motion.article v-for="draw in draws" :key="draw.id" layout class="draw-panel" :initial="reducedMotion ? false : { opacity: 0, y: offset(8) }" :animate="{ opacity: 1, y: 0 }" :exit="{ opacity: 0, y: reducedMotion ? 0 : -6 }" :transition="{ duration: reducedMotion ? 0.08 : 0.18, ease: 'easeOut' }">
        <span class="feature-icon"><UIcon name="i-ph-gift" /></span>
        <div class="draw-panel__copy">
          <h3>{{ draw.name }}</h3>
          <p>{{ draw.description || $t('activity.weightedPrize') }}</p>
          <span class="draw-panel__safety"><UIcon name="i-ph-shield-check" /> {{ $t('activity.drawSafety') }}</span>
        </div>
        <div class="draw-panel__actions">
          <UFormField :label="$t('activity.presentation')">
            <USelect
              :items="styleOptions"
              :model-value="selectedStyles[draw.id] ?? 'random'"
              :disabled="busy || disabled"
              @update:model-value="choose(draw.id, String($event))"
            />
          </UFormField>
          <UButton
            block
            :disabled="!draw.enabled || !draw.prizes?.length || busy || disabled"
            :loading="busy && props.drawingId === draw.id"
            :label="!draw.prizes?.length ? $t('activity.drawUnavailable') : busy && props.drawingId === draw.id ? $t('activity.drawing') : $t('activity.drawFor', { amount: displayMinor(draw.feeTxbMinor) })"
            data-haptic="confirm"
            @click="start(draw)"
          />
        </div>
      </motion.article>
      <motion.div v-if="!draws.length" key="empty" class="empty-inline" :initial="{ opacity: 0 }" :animate="{ opacity: 1 }" :exit="{ opacity: 0 }"><div><h3>{{ $t('activity.noDraws') }}</h3><p>{{ $t('activity.publishDraw') }}</p></div></motion.div>
    </AnimatePresence>
  </motion.section>
</template>

<style scoped>
.draw-panel { display: grid; grid-template-columns: auto minmax(0, 1fr); align-items: start; gap: 0.8rem; }
.draw-list { display: grid; gap: 0.7rem; }
.draw-panel { padding: 0.7rem; border: 1px solid var(--line); border-radius: var(--radius-control); background: var(--surface-raised); }
.draw-panel__copy h3, .draw-panel__copy p { margin: 0; }
.draw-panel__copy h3 { font-size: 1.05rem; }
.draw-panel__copy p { margin-top: 0.35rem; color: var(--text-muted); font-size: 0.82rem; line-height: 1.5; }
.draw-panel__safety { display: flex; gap: 0.4rem; margin-top: 0.65rem; color: var(--text-faint); font-size: 0.66rem; line-height: 1.4; }
.draw-panel__actions { display: grid; grid-column: 1 / -1; gap: 0.55rem; min-width: 10rem; }
@media (min-width: 640px) { .draw-panel { grid-template-columns: auto minmax(0, 1fr) minmax(10rem, 12rem); align-items: center; } .draw-panel__actions { grid-column: auto; } }
</style>
