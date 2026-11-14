<script setup lang="ts">
import { computed } from 'vue'
import { useIllustrationPlayback } from '@/composables/useIllustrationPlayback'
import { useTelegramBackButton } from '@/composables/useTelegramBackButton'
import { useI18n } from '@/i18n'
import DemoTransport from './DemoTransport.vue'

const open = defineModel<boolean>('open', { required: true })
const props = defineProps<{
  title: string
  description: string
  chapters: readonly { icon: string; label: string; caption?: string; detail?: string; points?: readonly { icon: string; label: string }[] }[]
  interval?: number
  resetKey?: string
  returnFocus?: globalThis.HTMLElement | null
  contentClass?: string
  billingMode?: string
}>()
const emit = defineEmits<{ replay: [] }>()
const { t } = useI18n()
const playback = useIllustrationPlayback({ active: () => open.value, stepCount: () => props.chapters.length, resetKey: () => props.resetKey, interval: () => props.interval ?? 3200 })
const { step, phase, progress, elapsed, duration, playing, reducedMotion } = playback
const current = computed(() => props.chapters[step.value] ?? props.chapters[0])
useTelegramBackButton(computed(() => open.value), () => { open.value = false })
function restoreFocus(event: globalThis.Event): void {
  if (!props.returnFocus?.isConnected) return
  event.preventDefault()
  props.returnFocus.focus({ preventScroll: true })
}
function replay(): void { emit('replay'); playback.replay() }
</script>

<template>
  <UModal v-model:open="open" :title="title" :description="description" :content="{ onCloseAutoFocus: restoreFocus }" :transition="!reducedMotion" :close="{ 'aria-label': t('common.close') }" :ui="{ content: 'sm:max-w-2xl', body: 'p-4 sm:p-6', description: 'sr-only' }">
    <template #body>
      <div class="demo-player" :class="contentClass" :data-demo-playing="playing" :data-demo-duration="duration" :data-billing-mode="billingMode">
        <div class="demo-player__film">
          <slot :step="step" :phase="phase" :reduced-motion="reducedMotion" :playing="playing" :select-step="playback.selectStep" />
          <div class="demo-player__story" role="status" aria-live="polite" aria-atomic="true">
            <p v-if="current" class="demo-player__caption"><UIcon :name="current.icon" aria-hidden="true" /><span>{{ current.caption ?? current.label }}</span></p>
            <ul v-if="current?.points?.length" class="demo-player__points"><li v-for="point in current.points" :key="point.label"><UIcon :name="point.icon" aria-hidden="true" /><span>{{ point.label }}</span></li></ul>
            <p v-if="current?.detail" class="demo-player__detail">{{ current.detail }}</p>
          </div>
        </div>
        <slot name="controls" :step="step" :phase="phase" :reduced-motion="reducedMotion" :select-step="playback.selectStep" />
        <DemoTransport :chapters="chapters" :step="step" :progress="progress" :elapsed="elapsed" :duration="duration" :playing="playing" :reduced-motion="reducedMotion" @select="playback.selectStep" @seek="playback.seek" @toggle="playback.toggle" @replay="replay" />
        <slot name="details" :step="step" />
      </div>
    </template>
  </UModal>
</template>

<style scoped>
.demo-player { display: grid; gap: .75rem; min-width: 0; }
.demo-player__film { min-width: 0; overflow: hidden; border: 1px solid var(--line); border-radius: .9rem; background: var(--surface-soft, var(--surface)); }
.demo-player__story { display: grid; gap: .65rem; padding: .9rem 1rem 1rem; border-top: 1px solid var(--line); }
.demo-player__caption { display: flex; align-items: center; gap: .5rem; margin: 0; min-height: 2rem; color: var(--text); font-size: .95rem; font-weight: 650; line-height: 1.4; letter-spacing: -.02em; }
.demo-player__caption .iconify { flex: none; color: var(--accent); font-size: 1.2rem; }
.demo-player__points { display: grid; grid-template-columns: repeat(3, minmax(0, 1fr)); gap: .55rem; padding: 0; margin: 0; list-style: none; }
.demo-player__points li { display: flex; align-items: start; gap: .35rem; min-width: 0; color: var(--text-muted); font-size: .72rem; line-height: 1.45; }
.demo-player__points .iconify { flex: none; margin-top: .1rem; color: var(--accent); font-size: 1rem; }
.demo-player__detail { min-height: 3em; margin: 0; color: var(--text-muted); font-size: .76rem; line-height: 1.6; overflow-wrap: anywhere; }
@media (max-width: 420px) { .demo-player__story { padding: .75rem; gap: .6rem; } .demo-player__points { grid-template-columns: 1fr; gap: .35rem; } .demo-player__caption { font-size: .88rem; } }
</style>
