<script setup lang="ts">
import { useI18n } from '@/i18n'

defineProps<{ chapters: readonly { icon: string; label: string }[]; step: number; progress: number; elapsed: number; duration: number; playing: boolean; reducedMotion: boolean }>()
const emit = defineEmits<{ select: [step: number]; seek: [progress: number]; toggle: []; replay: [] }>()
const { t } = useI18n()
function clock(milliseconds: number): string {
  const seconds = Math.floor(milliseconds / 1000)
  return `${Math.floor(seconds / 60)}:${String(seconds % 60).padStart(2, '0')}`
}
function seek(value: number | undefined) { if (value !== undefined) emit('seek', value / 1000) }
</script>

<template>
  <div class="demo-transport">
    <div class="demo-transport__timeline">
      <UButton v-if="!reducedMotion" square color="neutral" variant="soft" :icon="playing ? 'i-ph-pause-fill' : 'i-ph-play-fill'" :aria-label="t(playing ? 'payg.demo.pause' : 'payg.demo.play')" :title="t(playing ? 'payg.demo.pause' : 'payg.demo.play')" @click="emit('toggle')" />
      <USlider class="demo-transport__seek" :min="0" :max="1000" :step="1" :model-value="Math.round(progress * 1000)" :aria-label="t('payg.demo.seek')" :aria-valuetext="`${clock(elapsed)} / ${clock(duration)}`" :ui="{ root: 'min-h-11', track: 'h-1', thumb: 'size-3.5' }" @update:model-value="seek" />
      <span class="demo-transport__clock" aria-hidden="true">{{ clock(elapsed) }}<span> / {{ clock(duration) }}</span></span>
      <UButton square color="neutral" variant="ghost" icon="i-ph-arrow-counter-clockwise-bold" :aria-label="t('payg.demo.replay')" :title="t('payg.demo.replay')" @click="emit('replay')" />
    </div>
    <nav class="demo-transport__chapters" :aria-label="t('payg.demo.steps')">
      <UButton v-for="(chapter, index) in chapters" :key="chapter.label" square color="neutral" variant="ghost" :icon="chapter.icon" :aria-label="chapter.label" :title="chapter.label" :aria-current="index === step ? 'step' : undefined" :class="{ 'is-current': index === step, 'is-complete': index < step }" :data-demo-step="index" @click="emit('select', index)" />
    </nav>
  </div>
</template>

<style scoped>
.demo-transport { display: grid; gap: .35rem; min-width: 0; border-top: 1px solid var(--line); padding-top: .55rem; }
.demo-transport__timeline { display: flex; align-items: center; gap: .6rem; min-width: 0; }
.demo-transport__timeline :deep(button), .demo-transport__chapters :deep(button) { flex: none; min-width: 44px; min-height: 44px; }
.demo-transport__seek { min-width: 0; flex: 1; width: 100%; height: 44px; cursor: pointer; }
.demo-transport__clock { flex: none; color: var(--text); font: .65rem var(--font-mono); font-variant-numeric: tabular-nums; }
.demo-transport__clock > span { color: var(--text-faint); }
.demo-transport__chapters { display: flex; justify-content: safe center; gap: .15rem; min-width: 0; overflow-x: auto; scrollbar-width: thin; }
.demo-transport__chapters .is-current { color: var(--accent); background: var(--accent-soft); }
.demo-transport__chapters .is-complete { color: var(--accent); }
@media (max-width: 380px) { .demo-transport__timeline { gap: .25rem; } .demo-transport__clock > span { display: none; } }
</style>
